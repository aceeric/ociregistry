package cache

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aceeric/imgpull/pkg/imgpull"
	"github.com/aceeric/ociregistry/impl/globals"
	"github.com/aceeric/ociregistry/impl/pullrequest"
	"github.com/aceeric/ociregistry/impl/serialize"
	log "github.com/sirupsen/logrus"
)

const (
	older = "2026-11-01T10:00:00"
	newer = "2026-11-09T17:09:25"
)

// loadFixture writes two image manifests (app:v1 and app:v2) with the passed pull dates to a
// new cache and returns its path, without loading it.
func loadFixture(t *testing.T, v1Pulled, v2Pulled string) string {
	t.Helper()
	td, build := aliasFixture(t, aliasBlob1, aliasBlob2)
	for url, pulled := range map[string]string{"quay.io/org/app:v1": v1Pulled, "quay.io/org/app:v2": v2Pulled} {
		_, mh := build(url, aliasDigestD, aliasBlob1, aliasBlob2)
		mh.Created, mh.Pulled = older, pulled
		if err := serialize.MhToFilesystem(mh, td, true); err != nil {
			t.Fatal(err)
		}
	}
	return td
}

// startWriter starts the pull date writer for the loaded cache in td with an interval long
// enough that only the test's own flushes write, and stops it when the test ends.
func startWriter(t *testing.T, td string) {
	t.Helper()
	StartPullDateWriter(td, time.Hour)
	t.Cleanup(StopPullDateWriter)
}

// hit serves the url from the cache, like a client pull.
func hit(t *testing.T, url string) {
	t.Helper()
	pr, _ := pullrequest.NewPullRequestFromUrl(url)
	if _, found := getManifestFromCache(pr); !found {
		t.Fatalf("expected a cache hit for %s", url)
	}
}

// pulledOnDisk returns the pull date in the url's manifest file.
func pulledOnDisk(td, url string) string {
	mh, _ := serialize.MhFromFilesystem(url, td)
	return mh.Pulled
}

// journalExists reports whether the pull dates file exists.
func journalExists(td string) bool {
	_, err := os.Stat(filepath.Join(td, globals.ManifestPath, serialize.PullDatesFile))
	return err == nil
}

// Tests that repeated pulls of a url become one journal entry with the last date, and that a
// removal deletes the url's entry straight away.
func TestPullDatesCoalesce(t *testing.T) {
	td := loadFixture(t, older, older)
	if err := Load(td); err != nil {
		t.Fatal(err)
	}
	startWriter(t, td)
	for i := range 100 {
		recordPull("quay.io/org/app:v1", fmt.Sprintf("2026-11-10T08:00:%02d", i%60))
	}
	recordPull("quay.io/org/app:v1", newer)
	recordPull("quay.io/org/app:v2", newer)
	recordRemove("quay.io/org/app:v2")
	if len(pd.urls) != 1 {
		t.Fatalf("expected the updates to coalesce to 1 url, got %v", pd.urls)
	}
	flushPullDates()
	meta, err := serialize.ReadPullDates(td)
	if err != nil || len(meta) != 1 || meta["quay.io/org/app:v1"].Pulled != newer {
		t.Fatalf("expected only app:v1 with the last date, got %v %v", meta, err)
	}
}

// Tests a crash: the journal written while the server ran is applied to the manifests and
// deleted when the cache is next opened (realMain), and Load then loads the dates from the
// manifests.
func TestCrashRecovery(t *testing.T) {
	td := loadFixture(t, older, older)
	if err := Load(td); err != nil {
		t.Fatal(err)
	}
	startWriter(t, td)
	hit(t, "quay.io/org/app:v1")
	pulled := mc.entries["quay.io/org/app:v1"].Pulled
	flushPullDates() // the writer's interval
	// the crash: no UpdateManifestPullDates. The manifest still has the old date.
	if pulledOnDisk(td, "quay.io/org/app:v1") != older || !journalExists(td) {
		t.Fatal("before the restart the date should only be in the journal")
	}
	ResetCache()
	if err := serialize.ReconcilePullDates(td, serialize.MhToFilesystem); err != nil {
		t.Fatal(err)
	}
	if err := Load(td); err != nil {
		t.Fatal(err)
	}
	if mc.entries["quay.io/org/app:v1"].Pulled != pulled || pulledOnDisk(td, "quay.io/org/app:v1") != pulled {
		t.Fatalf("expected the journaled date %s in memory and on disk", pulled)
	}
	if mc.entries["quay.io/org/app:v2"].Pulled != older || journalExists(td) {
		t.Fatal("expected app:v2 unchanged and the journal deleted")
	}
}

// Tests that Load refuses a cache whose journal wasn't applied, whatever the journal holds,
// and leaves it untouched: otherwise the writer would overwrite it and lose its pull dates.
func TestLoadRefusesJournal(t *testing.T) {
	valid := `{"version": 1, "meta": {"quay.io/org/app:v1": {"pulled": "2026-11-09T17:09:25"}}}`
	for _, content := range []string{valid, "not json", `{"version": 2, "meta": {}}`} {
		td := loadFixture(t, older, older)
		fname := filepath.Join(td, globals.ManifestPath, serialize.PullDatesFile)
		os.WriteFile(fname, []byte(content), 0644)
		if err := Load(td); err == nil || !strings.Contains(err.Error(), "was not applied") {
			t.Errorf("%q: expected Load to refuse the cache, got %v", content, err)
		}
		if b, _ := os.ReadFile(fname); string(b) != content {
			t.Errorf("%q: the journal should be untouched", content)
		}
	}
}

// Tests a normal shutdown: the manifests of the urls pulled since the start are updated from
// the in-mem cache, others are left alone, a pruned manifest isn't written back, and the
// journal is deleted.
func TestUpdateManifestPullDates(t *testing.T) {
	td := loadFixture(t, older, older)
	if err := Load(td); err != nil {
		t.Fatal(err)
	}
	StartPullDateWriter(td, time.Hour)
	hit(t, "quay.io/org/app:v1")
	hit(t, "quay.io/org/app:v2")
	pulled := mc.entries["quay.io/org/app:v1"].Pulled
	mc.Lock()
	rmManifest(mc.entries["quay.io/org/app:v2"], td, log.StandardLogger())
	// a journaled url that isn't cached: only ever written from the in-mem cache
	recordPull("quay.io/org/never:v1", newer)
	mc.Unlock()
	if err := UpdateManifestPullDates(serialize.MhToFilesystem); err == nil {
		t.Fatal("expected an error while the writer runs")
	}
	StopPullDateWriter()
	if err := UpdateManifestPullDates(serialize.MhToFilesystem); err != nil {
		t.Fatal(err)
	}
	if pulledOnDisk(td, "quay.io/org/app:v1") != pulled {
		t.Errorf("expected app:v1's manifest to have the pull date %s", pulled)
	}
	for _, url := range []string{"quay.io/org/app:v2", "quay.io/org/never:v1"} {
		if _, found := serialize.MhFromFilesystem(url, td); found {
			t.Errorf("%s: a manifest that isn't cached must not be written", url)
		}
	}
	if journalExists(td) {
		t.Error("a clean shutdown should delete the journal")
	}
}

// Tests that a shutdown that can't write a manifest fails and keeps the journal, which the next
// Load applies.
func TestUpdateManifestPullDatesFails(t *testing.T) {
	td := loadFixture(t, older, older)
	if err := Load(td); err != nil {
		t.Fatal(err)
	}
	startWriter(t, td)
	hit(t, "quay.io/org/app:v1")
	pulled := mc.entries["quay.io/org/app:v1"].Pulled
	StopPullDateWriter() // the final flush, as in a shutdown
	failingWrite := func(imgpull.ManifestHolder, string, bool) error { return errors.New("no space left on device") }
	if err := UpdateManifestPullDates(failingWrite); err == nil || !strings.Contains(err.Error(), "no space left on device") {
		t.Fatalf("expected the write error, got %v", err)
	}
	if !journalExists(td) {
		t.Fatal("the journal must be kept when a manifest can't be written")
	}
	ResetCache()
	if err := serialize.ReconcilePullDates(td, serialize.MhToFilesystem); err != nil || pulledOnDisk(td, "quay.io/org/app:v1") != pulled {
		t.Fatalf("expected the journal to be applied when the cache is next opened: %v", err)
	}
	if err := Load(td); err != nil || mc.entries["quay.io/org/app:v1"].Pulled != pulled {
		t.Fatalf("expected Load to load the applied date: %v", err)
	}
}

// Tests that the writer journals on its interval, and once more when it's stopped.
func TestPullDateWriter(t *testing.T) {
	td := loadFixture(t, older, older)
	if err := Load(td); err != nil {
		t.Fatal(err)
	}
	StartPullDateWriter(td, 20*time.Millisecond)
	recordPull("quay.io/org/app:v1", newer)
	deadline := time.Now().Add(5 * time.Second)
	for {
		if meta, _ := serialize.ReadPullDates(td); meta["quay.io/org/app:v1"].Pulled == newer {
			break
		} else if time.Now().After(deadline) {
			t.Fatal("the writer didn't flush on its interval")
		}
		time.Sleep(10 * time.Millisecond)
	}
	StopPullDateWriter()

	// a long interval: only the final flush on stop can write this
	StartPullDateWriter(td, time.Hour)
	recordPull("quay.io/org/app:v2", newer)
	StopPullDateWriter()
	if meta, _ := serialize.ReadPullDates(td); meta["quay.io/org/app:v2"].Pulled != newer {
		t.Fatalf("expected the final flush on stop, got %v", meta)
	}
	StopPullDateWriter() // stopping a writer that isn't running does nothing
}

// Tests that a failed journal write is retried by the next flush with nothing new changed, and
// that unchanged pull dates aren't written again.
func TestFlushRetry(t *testing.T) {
	td := loadFixture(t, older, older)
	if err := Load(td); err != nil {
		t.Fatal(err)
	}
	startWriter(t, td)
	flushPullDates()
	if journalExists(td) {
		t.Fatal("nothing has changed, so there should be no write")
	}
	mdir := filepath.Join(td, globals.ManifestPath)
	os.Rename(mdir, mdir+".moved")
	recordPull("quay.io/org/app:v1", newer)
	flushPullDates()
	flushPullDates()
	os.Rename(mdir+".moved", mdir)
	if journalExists(td) {
		t.Fatal("the writes should have failed")
	}
	flushPullDates()
	if meta, err := serialize.ReadPullDates(td); err != nil || meta["quay.io/org/app:v1"].Pulled != newer {
		t.Fatalf("expected the retry to write the date, got %v %v", meta, err)
	}
	os.Remove(filepath.Join(mdir, serialize.PullDatesFile))
	flushPullDates()
	if journalExists(td) {
		t.Fatal("unchanged pull dates should not be written again")
	}
}

// Tests that a cache hit doesn't write anything: the pull date is only journaled by a flush.
func TestCacheHitNoWrite(t *testing.T) {
	td := loadFixture(t, older, older)
	if err := Load(td); err != nil {
		t.Fatal(err)
	}
	mdir := filepath.Join(td, globals.ManifestPath)
	before, _ := os.ReadDir(mdir)
	hit(t, "quay.io/org/app:v1")
	after, _ := os.ReadDir(mdir)
	if len(after) != len(before) || pulledOnDisk(td, "quay.io/org/app:v1") != older {
		t.Fatal("a cache hit should not write files")
	}
}

// Tests cache hits and prunes running while the writer flushes every millisecond (run with
// -race): afterwards the journal has exactly the urls still cached, and the shutdown writes back
// only those.
func TestPullDatesConcurrent(t *testing.T) {
	td, build := aliasFixture(t, aliasBlob1, aliasBlob2)
	urls := []string{}
	for i := range 20 {
		url := fmt.Sprintf("quay.io/org/app:v%d", i)
		_, mh := build(url, aliasDigestD, aliasBlob1, aliasBlob2)
		mh.Created, mh.Pulled = older, older
		serialize.MhToFilesystem(mh, td, true)
		urls = append(urls, url)
	}
	if err := Load(td); err != nil {
		t.Fatal(err)
	}
	StartPullDateWriter(td, time.Millisecond)
	t.Cleanup(StopPullDateWriter)
	// every url is pulled once before the concurrent part, so each is journaled
	for _, url := range urls {
		hit(t, url)
	}
	var wg sync.WaitGroup
	for _, url := range urls {
		wg.Go(func() {
			pr, _ := pullrequest.NewPullRequestFromUrl(url)
			for range 50 {
				getManifestFromCache(pr)
			}
		})
	}
	// prune the even-numbered urls while the hits and flushes run
	wg.Go(func() {
		mc.Lock()
		defer mc.Unlock()
		for i, url := range urls {
			if i%2 == 0 {
				rmManifest(mc.entries[imageUrl(url)], td, log.StandardLogger())
			}
		}
	})
	wg.Wait()
	StopPullDateWriter() // the final flush
	meta, err := serialize.ReadPullDates(td)
	if err != nil {
		t.Fatal(err)
	}
	for i, url := range urls {
		if _, inJournal := meta[url]; inJournal != (i%2 == 1) {
			t.Errorf("%s: in the journal %t", url, inJournal)
		}
	}
	if err := UpdateManifestPullDates(serialize.MhToFilesystem); err != nil {
		t.Fatal(err)
	}
	for i, url := range urls {
		_, found := serialize.MhFromFilesystem(url, td)
		if found != (i%2 == 1) || (found && pulledOnDisk(td, url) == older) {
			t.Errorf("%s: on disk %t, pulled %s", url, found, pulledOnDisk(td, url))
		}
	}
}

// Tests the pull date listing: two columns sorted by url, at most count rows, a removal and a
// repeated pull reflected, and nothing when no pull dates were recorded.
func TestGetPullDates(t *testing.T) {
	td := loadFixture(t, older, older)
	if err := Load(td); err != nil {
		t.Fatal(err)
	}
	startWriter(t, td)
	if got := GetPullDates(50); got != "" {
		t.Fatalf("expected nothing before any pull, got %q", got)
	}
	recordPull("quay.io/org/app:v2", older)
	recordPull("docker.io/library/hello-world:latest", older)
	recordPull("quay.io/org/app:v1", older)
	recordPull("quay.io/org/app:v2", newer)
	recordPull("quay.io/org/gone:v1", newer)
	recordRemove("quay.io/org/gone:v1")
	want := `URL                                   PULLED
docker.io/library/hello-world:latest  2026-11-01T10:00:00
quay.io/org/app:v1                    2026-11-01T10:00:00
quay.io/org/app:v2                    2026-11-09T17:09:25
`
	if got := GetPullDates(50); got != want {
		t.Fatalf("expected:\n%s\ngot:\n%s", want, got)
	}
	if got := GetPullDates(2); strings.Count(got, "\n") != 3 || strings.Contains(got, "app:v2") {
		t.Fatalf("expected the header and the first 2 urls, got:\n%s", got)
	}
}
