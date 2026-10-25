package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aceeric/ociregistry/impl/config"
	"github.com/aceeric/ociregistry/impl/globals"
	"github.com/aceeric/ociregistry/impl/pullrequest"
	"github.com/aceeric/ociregistry/impl/serialize"

	"github.com/aceeric/imgpull/pkg/imgpull"
	"github.com/aceeric/imgpull/pkg/imgpull/v1oci"
	log "github.com/sirupsen/logrus"
)

func init() {
	log.SetOutput(io.Discard)
}

// Create three manifests and configure the background pruner to prune two. Let
// the pruner run as a goroutine. Then terminate the pruner and verify that in fact
// if pruned two manifests.
func TestRunPruner(t *testing.T) {
	td, err := setupPrune()
	if err != nil {
		t.FailNow()
	}
	defer os.RemoveAll(td)
	stopPruneCh := make(chan bool)
	pruneStoppedCh := make(chan bool)
	if err := RunPruner(stopPruneCh, pruneStoppedCh); err != nil {
		t.FailNow()
	}
	time.Sleep(time.Second)
	stopPruneCh <- true
	<-pruneStoppedCh
	// each manifest stored twice - once by tag and once by digest
	if mc.len() != 2 {
		t.Fail()
	}
}

// Create three manifests and call the pruner used by the REST API. Same
// logic and expected outcome as TestRunPruner.
func TestPrunerFromApi(t *testing.T) {
	td, err := setupPrune()
	if err != nil {
		t.FailNow()
	}
	defer os.RemoveAll(td)

	duration := "1d"
	count := -1
	expr := ""
	dryRun := "false"

	_, err = Prune("created", &duration, &expr, &dryRun, &count)
	if err != nil {
		t.FailNow()
	}
	if mc.len() != 2 {
		t.Fail()
	}
}

// Test the mechanics of pruning
func TestPrune(t *testing.T) {
	ResetCache()
	td, _ := os.MkdirTemp("", "")
	defer os.RemoveAll(td)
	serialize.CreateDirs(td, true)
	mh := imgpull.ManifestHolder{
		Type:     imgpull.V1ociManifest,
		Digest:   strconv.Itoa(int(imgpull.V1ociManifest)),
		ImageUrl: "docker.io/test/manifest:v1.2.3",
	}
	digests := []string{
		"1111111111111111111111111111111111111111111111111111111111111111",
		"1111111111111111111111111111111111111111111111111111111111111112",
		"1111111111111111111111111111111111111111111111111111111111111113",
	}
	for _, digest := range digests {
		if err := os.WriteFile(filepath.Join(td, globals.BlobPath, digest), []byte(digest), 0755); err != nil {
			t.Fail()
		}
	}
	if err := json.Unmarshal(fmt.Appendf(nil, v1ociManifest, digests[0], digests[1], digests[2]), &mh.V1ociManifest); err != nil {
		t.Fail()
	}
	pr, err := pullrequest.NewPullRequestFromUrl(mh.ImageUrl)
	if err != nil {
		t.Fail()
	}

	err = addToCache(pr, mh, td)
	if err != nil {
		t.Fail()
	}
	// manifests that are pulled by tag are added twice - one by tag and a second by digest
	if mc.len() != 2 || len(bc.blobs) != 3 {
		t.Fail()
	}
	prune(mh, pruneAll, td)
	if mc.len() != 0 {
		t.Fail()
	}
	for _, digest := range digests {
		if cnt, exists := bc.blobs[digest]; cnt != 0 || exists {
			t.Fail()
		}
		// prune only drops the refs - the files stay until the sweep
		if !blobFileExists(td, digest) {
			t.Fatalf("blob %s removed before sweep", digest)
		}
	}
	pullGate.Lock()
	sweepBlobs(td)
	pullGate.Unlock()
	for _, digest := range digests {
		if blobFileExists(td, digest) {
			t.Fatalf("blob %s not removed by sweep", digest)
		}
	}
}

// Test a simple comparer to test the functionality that traverses the manifest cache
// and calls the comparer.
func TestGetManifestsToPrune(t *testing.T) {
	ResetCache()
	for i := range 100 {
		pr, err := pullrequest.NewPullRequestFromUrl(fmt.Sprintf("foo.io/my-image:%d", i))
		if err != nil {
			t.Fail()
		}
		mh := imgpull.ManifestHolder{
			Type:     imgpull.V1ociIndex,
			Digest:   strconv.Itoa(i),
			ImageUrl: pr.Url(),
		}
		addToCache(pr, mh, "")
	}
	comparer := func(mh imgpull.ManifestHolder) bool {
		return strings.Contains(mh.ImageUrl, "2")
	}
	toPrune := GetManifestsCompare(comparer, -1)
	for _, mh := range toPrune {
		if !strings.Contains(mh.ImageUrl, "2") {
			t.Fail()
		}
	}
}

// Test creating comparers from various prune configurations
func TestParseCriteria(t *testing.T) {
	type parseTest struct {
		cfg        config.PruneConfig
		shouldPass bool
	}
	parseTests := []parseTest{
		{config.PruneConfig{Duration: "invalid", Type: "created"}, false},
		{config.PruneConfig{Duration: "1d", Type: "created"}, true},
		{config.PruneConfig{Duration: "1d", Type: "accessed"}, true},
		{config.PruneConfig{Duration: "12h", Type: "accessed"}, true},
		{config.PruneConfig{Duration: "", Type: "accessed"}, false},
		{config.PruneConfig{Duration: "1d", Type: ""}, false},
		{config.PruneConfig{Duration: "1d", Type: "invalid"}, false},
	}

	for _, parseTest := range parseTests {
		_, err := ParseCriteria(parseTest.cfg)
		if (err != nil && parseTest.shouldPass) || (err == nil && !parseTest.shouldPass) {
			t.FailNow()
		}
	}
}

// Create comparers from various prune configuration and test that they find images
// correctly.
func TestComparer(t *testing.T) {
	timeStr := func(dur string) string {
		t := time.Now()
		d, _ := time.ParseDuration(dur)
		t = t.Add(d)
		return t.Format(globals.DateFormat)
	}
	type parseTest struct {
		id            int
		cfg           config.PruneConfig
		mh            imgpull.ManifestHolder
		shouldPrune   bool
		failureReason string
	}
	threeDaysAgo := timeStr("-72h")
	oneDayAgo := timeStr("-24h")
	present := timeStr("0h")

	parseTests := []parseTest{
		{1, config.PruneConfig{Duration: "2d", Type: "created"}, imgpull.ManifestHolder{Created: present}, false, "not earlier"},
		{2, config.PruneConfig{Duration: "2d", Type: "created"}, imgpull.ManifestHolder{Created: threeDaysAgo}, true, ""},
		{3, config.PruneConfig{Duration: "2d", Type: "created"}, imgpull.ManifestHolder{Created: oneDayAgo}, false, "not earlier"},
		{4, config.PruneConfig{Duration: "2d", Type: "accessed"}, imgpull.ManifestHolder{Pulled: present}, false, "not earlier"},
		{5, config.PruneConfig{Duration: "2d", Type: "accessed"}, imgpull.ManifestHolder{Pulled: threeDaysAgo}, true, ""},
		{6, config.PruneConfig{Duration: "2d", Type: "accessed"}, imgpull.ManifestHolder{Pulled: oneDayAgo}, false, "not earlier"},
		{7, config.PruneConfig{Duration: "2d", Type: "created"}, imgpull.ManifestHolder{}, false, "no date to compare"},
		{8, config.PruneConfig{Duration: "2d", Type: "accessed"}, imgpull.ManifestHolder{}, false, "no date to compare"},
		{9, config.PruneConfig{Duration: "2d", Type: "created"}, imgpull.ManifestHolder{Created: "foobar"}, false, "un-parseable date"},
		{10, config.PruneConfig{Duration: "2d", Type: "accessed"}, imgpull.ManifestHolder{Pulled: "foobar"}, false, "un-parseable date"},
	}
	for _, parseTest := range parseTests {
		comparer, err := ParseCriteria(parseTest.cfg)
		if err != nil {
			t.FailNow()
		}
		willPrune := comparer(parseTest.mh)
		if willPrune != parseTest.shouldPrune {
			t.FailNow()
		}
	}
}

// Creates three manifests in cache with create dates: today, 2 days ago,
// and 6 days ago. Returns the name of the test directory and an error.
func setupPrune() (string, error) {
	ResetCache()
	td, _ := os.MkdirTemp("", "")
	serialize.CreateDirs(td, true)
	curDt := time.Now()
	for i := range 3 {
		digest := fmt.Sprintf("000000000000000000000000000000000000000000000000000000000000000%d", i)
		if err := os.WriteFile(filepath.Join(td, globals.BlobPath, digest), []byte(digest), 0755); err != nil {
			return "", err
		}
		mhDate := curDt.AddDate(0, 0, -(i * 2))
		mh := imgpull.ManifestHolder{
			Type: imgpull.V1ociManifest,
			//Digest:   strconv.Itoa(int(imgpull.V1ociManifest)),
			Digest:   digest,
			ImageUrl: fmt.Sprintf("docker.io/test/manifest:v1.2.%d", i),
			Created:  mhDate.Format(globals.DateFormat),
			V1ociManifest: v1oci.Manifest{
				Config: v1oci.Descriptor{
					Digest: digest,
					Size:   64,
				},
			},
		}
		pr, err := pullrequest.NewPullRequestFromUrl(mh.ImageUrl)
		if err != nil {
			return "", err
		}
		err = addToCache(pr, mh, td)
		if err != nil {
			return "", err
		}
	}
	cfg := config.Configuration{
		// prune everything created older than a day ago, run every 100 milliseconds
		PruneConfig: config.PruneConfig{
			Enabled:  true,
			Duration: "1d",
			Type:     "created",
			Freq:     "100ms",
			Count:    -1,
			Expr:     "",
			DryRun:   false,
		},
	}
	config.Set(cfg)
	return td, nil
}

// pruneAll is a comparer that selects every manifest.
func pruneAll(imgpull.ManifestHolder) bool { return true }

func blobFileExists(imagePath, digest string) bool {
	_, err := os.Stat(filepath.Join(imagePath, globals.BlobPath, digest))
	return err == nil
}

// pruneFixture caches one image manifest, "docker.io/test/manifest:v1", whose config and
// layer blobs are the passed digests (64 bytes each), and returns the cache directory and
// the cached manifest.
func pruneFixture(t *testing.T, manifestDigest string, blobs ...string) (string, imgpull.ManifestHolder) {
	t.Helper()
	ResetCache()
	td := t.TempDir()
	serialize.CreateDirs(td, true)
	for _, d := range blobs {
		if err := os.WriteFile(filepath.Join(td, globals.BlobPath, d), []byte(d), 0644); err != nil {
			t.Fatal(err)
		}
	}
	layers := []v1oci.Descriptor{}
	for _, d := range blobs[1:] {
		layers = append(layers, v1oci.Descriptor{Digest: d, Size: 64})
	}
	mh := imgpull.ManifestHolder{
		Type:     imgpull.V1ociManifest,
		Digest:   manifestDigest,
		ImageUrl: "docker.io/test/manifest:v1",
		V1ociManifest: v1oci.Manifest{
			Config: v1oci.Descriptor{Digest: blobs[0], Size: 64},
			Layers: layers,
		},
	}
	pr, err := pullrequest.NewPullRequestFromUrl(mh.ImageUrl)
	if err != nil {
		t.Fatal(err)
	}
	if err := addToCache(pr, mh, td); err != nil {
		t.Fatal(err)
	}
	return td, mh
}

var (
	pruneDigestA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	pruneDigestB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	pruneBlob1   = "1111111111111111111111111111111111111111111111111111111111111111"
	pruneBlob2   = "2222222222222222222222222222222222222222222222222222222222222222"
)

// Item 10: a stale prune candidate whose tag now points at a different digest must not
// remove the current manifest or touch its blob ref counts.
func TestPruneSkipsChangedManifest(t *testing.T) {
	td, mh := pruneFixture(t, pruneDigestA, pruneBlob1, pruneBlob2)
	stale := mh
	stale.Digest = pruneDigestB // what the prune list saw before the tag moved
	prune(stale, pruneAll, td)
	if mc.len() != 2 || bc.blobs[pruneBlob1] != 1 || bc.blobs[pruneBlob2] != 1 {
		t.Fatalf("current manifest was pruned: mc=%d blobs=%v", mc.len(), bc.blobs)
	}
}

// Item 10: a candidate that was pruned already (e.g. by an overlapping prune) is skipped,
// so ref counts are not decremented a second time.
func TestPruneTwiceDecrementsOnce(t *testing.T) {
	td, mh := pruneFixture(t, pruneDigestA, pruneBlob1, pruneBlob2)
	// a second manifest sharing blob1 keeps it referenced
	other := imgpull.ManifestHolder{
		Type:     imgpull.V1ociManifest,
		Digest:   pruneDigestB,
		ImageUrl: "docker.io/test/other:v1",
		V1ociManifest: v1oci.Manifest{
			Config: v1oci.Descriptor{Digest: pruneBlob1, Size: 64},
		},
	}
	pr, _ := pullrequest.NewPullRequestFromUrl(other.ImageUrl)
	if err := addToCache(pr, other, td); err != nil {
		t.Fatal(err)
	}
	prune(mh, pruneAll, td)
	prune(mh, pruneAll, td)
	if bc.blobs[pruneBlob1] != 1 {
		t.Fatalf("shared blob ref count should be 1, got %d", bc.blobs[pruneBlob1])
	}
}

// Item 10: the comparer is re-run on the current cache entry, so a manifest that no longer
// matches (e.g. it was pulled after the list was built) is kept.
func TestPruneRechecksComparer(t *testing.T) {
	td, mh := pruneFixture(t, pruneDigestA, pruneBlob1, pruneBlob2)
	prune(mh, func(imgpull.ManifestHolder) bool { return false }, td)
	if mc.len() != 2 || len(bc.blobs) != 2 {
		t.Fatalf("manifest should have been kept: mc=%d blobs=%v", mc.len(), bc.blobs)
	}
}

// Item 12: sweepBlobs removes only blob files with no refs and leaves temp files to
// RemoveTempFiles.
func TestSweepRemovesOnlyUnreferenced(t *testing.T) {
	orphan := "3333333333333333333333333333333333333333333333333333333333333333"
	td, _ := pruneFixture(t, pruneDigestA, pruneBlob1, pruneBlob2)
	for _, name := range []string{orphan, orphan + ".123.tmp"} {
		if err := os.WriteFile(filepath.Join(td, globals.BlobPath, name), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	pullGate.Lock()
	sweepBlobs(td)
	pullGate.Unlock()
	if blobFileExists(td, orphan) {
		t.Fatal("unreferenced blob not removed")
	}
	if !blobFileExists(td, pruneBlob1) || !blobFileExists(td, pruneBlob2) {
		t.Fatal("referenced blob removed")
	}
	if !blobFileExists(td, orphan+".123.tmp") {
		t.Fatal("sweep should leave temp files alone")
	}
}

// Item 12: replacing a manifest drops the old manifest's refs but leaves the files; a
// sweep then removes only the blobs the new manifest doesn't share.
func TestReplaceLeavesFilesForSweep(t *testing.T) {
	td, _ := pruneFixture(t, pruneDigestA, pruneBlob1, pruneBlob2)
	blob3 := "3333333333333333333333333333333333333333333333333333333333333333"
	if err := os.WriteFile(filepath.Join(td, globals.BlobPath, blob3), []byte(blob3), 0644); err != nil {
		t.Fatal(err)
	}
	mhNew := imgpull.ManifestHolder{
		Type:     imgpull.V1ociManifest,
		Digest:   pruneDigestB,
		ImageUrl: "docker.io/test/manifest:v1",
		V1ociManifest: v1oci.Manifest{
			Config: v1oci.Descriptor{Digest: pruneBlob1, Size: 64}, // shared with the old manifest
			Layers: []v1oci.Descriptor{{Digest: blob3, Size: 64}},
		},
	}
	pr, _ := pullrequest.NewPullRequestFromUrl(mhNew.ImageUrl)
	if err := replaceInCache(pr, mhNew, td); err != nil {
		t.Fatal(err)
	}
	if _, ok := bc.blobs[pruneBlob2]; ok {
		t.Fatal("old-only blob should have no refs")
	}
	if !blobFileExists(td, pruneBlob2) {
		t.Fatal("replace must not delete blob files")
	}
	trySweepBlobs(td) // no pull in progress, so this sweeps
	if blobFileExists(td, pruneBlob2) {
		t.Fatal("old-only blob not swept")
	}
	if !blobFileExists(td, pruneBlob1) || !blobFileExists(td, blob3) {
		t.Fatal("blob referenced by the new manifest was swept")
	}
}

// Item 11: a prune waits for an in-progress pull (holding pullGate shared) to finish,
// and trySweepBlobs gives up rather than waiting.
func TestPruneWaitsForPull(t *testing.T) {
	td, _ := pruneFixture(t, pruneDigestA, pruneBlob1, pruneBlob2)
	orphan := "3333333333333333333333333333333333333333333333333333333333333333"
	if err := os.WriteFile(filepath.Join(td, globals.BlobPath, orphan), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	pullGate.RLock() // simulate a pull in progress
	trySweepBlobs(td)
	if !blobFileExists(td, orphan) {
		t.Fatal("trySweepBlobs swept while a pull was in progress")
	}
	var wg sync.WaitGroup
	var pruned atomic.Bool
	wg.Go(func() {
		doPrune(td, pruneAll, noLimit, false)
		pruned.Store(true)
	})
	time.Sleep(200 * time.Millisecond)
	if pruned.Load() {
		pullGate.RUnlock()
		t.Fatal("prune ran while a pull held the gate")
	}
	pullGate.RUnlock() // the pull finishes
	wg.Wait()
	if mc.len() != 0 || blobFileExists(td, pruneBlob1) || blobFileExists(td, orphan) {
		t.Fatalf("prune did not complete after the pull: mc=%d", mc.len())
	}
}

// Item 13: if a pull is still in progress after pruneLockWait, the prune gives up with
// ErrPruneBusy and changes nothing. While the prune is waiting, new pulls are not blocked.
func TestPruneBusy(t *testing.T) {
	td, _ := pruneFixture(t, pruneDigestA, pruneBlob1, pruneBlob2)
	saved := pruneLockWait
	pruneLockWait = 500 * time.Millisecond
	defer func() { pruneLockWait = saved }()

	pullGate.RLock() // a long pull in progress
	defer pullGate.RUnlock()
	errCh := make(chan error, 1)
	go func() { errCh <- doPrune(td, pruneAll, noLimit, false) }()

	// while the prune waits, a new pull can still take the gate
	time.Sleep(150 * time.Millisecond)
	newPull := make(chan struct{})
	go func() {
		pullGate.RLock()
		pullGate.RUnlock()
		close(newPull)
	}()
	select {
	case <-newPull:
	case <-time.After(time.Second):
		t.Fatal("a new pull was blocked by a waiting prune")
	}

	if err := <-errCh; !errors.Is(err, ErrPruneBusy) {
		t.Fatalf("expected ErrPruneBusy, got %v", err)
	}
	if mc.len() != 2 || !blobFileExists(td, pruneBlob1) {
		t.Fatal("a busy prune must not change the cache")
	}
}

// Item 13: the REST entry point returns ErrPruneBusy instead of the prune log.
func TestPruneFromApiBusy(t *testing.T) {
	td, _ := pruneFixture(t, pruneDigestA, pruneBlob1, pruneBlob2)
	config.Set(config.Configuration{ImagePath: td})
	saved := pruneLockWait
	pruneLockWait = 200 * time.Millisecond
	defer func() { pruneLockWait = saved }()

	pullGate.RLock()
	defer pullGate.RUnlock()
	expr, dryRun, count := "docker.io", "false", -1
	if _, err := Prune("pattern", nil, &expr, &dryRun, &count); !errors.Is(err, ErrPruneBusy) {
		t.Fatalf("expected ErrPruneBusy, got %v", err)
	}
}
