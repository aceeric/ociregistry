package subcmd

import (
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/aceeric/ociregistry/impl/config"
	"github.com/aceeric/ociregistry/impl/globals"
	"github.com/aceeric/ociregistry/impl/serialize"

	"github.com/aceeric/imgpull/pkg/imgpull"
	"github.com/opencontainers/go-digest"
)

var v2dockerManifest = `{
	"schemaVersion": 2,
	"mediaType": "application/vnd.docker.distribution.manifest.v2+json",
	"config": {
	   "digest": "sha256:%s"
	},
	"layers": [
	   {
		  "digest": "sha256:%s"
	   },
	   {
		  "digest": "sha256:%s"
	   }
	]
 }`

var v1ociManifest = `{
	"schemaVersion": 2,
	"mediaType": "application/vnd.oci.image.manifest.v1+json",
	"config": {
	   "digest": "sha256:%s"
	},
	"layers": [
	   {
		  "digest": "sha256:%s"
	   },
	   {
		  "digest": "sha256:%s"
	   }
	]
 }`

var cfgYaml = `
---
imagePath: %s
pruneConfig:
  enabled: false
  duration: 30d
  type: %s
  expr: '%s'
  dryRun: false
`

// Test prune by url pattern
func TestPrunebyPattern(t *testing.T) {
	manifestCnt := 10
	expectPrune := 3
	td, sharedBlobDigest, _, err := makeTestFiles(manifestCnt)
	if td != "" {
		defer os.RemoveAll(td)
	}
	if err != nil {
		t.FailNow()
	}
	cfg := fmt.Sprintf(cfgYaml, td, "pattern", "2,4,6")
	if err := config.SetConfigFromStr([]byte(cfg)); err != nil {
		t.FailNow()
	}
	if err := Prune(); err != nil {
		t.FailNow()
	} else if entries, err := os.ReadDir(filepath.Join(td, globals.ManifestPath)); err != nil {
		t.FailNow()
	} else if len(entries) != manifestCnt-expectPrune {
		t.FailNow()
	} else if verifyBlobPrune(td, len(entries), sharedBlobDigest) != nil {
		t.FailNow()
	}
}

// Tests that prune by pattern and by date refuses a cache with a date that isn't UTC, and
// prunes nothing.
func TestPruneRequiresUtc(t *testing.T) {
	manifestCnt := 4
	td, _, mhs, err := makeTestFiles(manifestCnt)
	if td != "" {
		defer os.RemoveAll(td)
	}
	if err != nil {
		t.Fatal(err)
	}
	mh, _ := serialize.MhFromFilesystem(mhs[manifestCnt-1].ImageUrl, td)
	mh.Pulled = "2026-11-14T12:00:00"
	if err := serialize.MhToFilesystem(mh, td, true); err != nil {
		t.Fatal(err)
	}
	for _, crit := range [][2]string{{"pattern", "."}, {"date", "2099-01-01T00:00:00Z"}} {
		cfg := fmt.Sprintf(cfgYaml, td, crit[0], crit[1])
		if err := config.SetConfigFromStr([]byte(cfg)); err != nil {
			t.Fatal(err)
		}
		if err := Prune(); !errors.Is(err, ErrNotUtc) {
			t.Errorf("%s: expected ErrNotUtc, got %v", crit[0], err)
		}
		if entries, _ := os.ReadDir(filepath.Join(td, globals.ManifestPath)); len(entries) != manifestCnt {
			t.Fatalf("%s: nothing should be pruned, %d of %d manifests left", crit[0], len(entries), manifestCnt)
		}
	}
}

// Tests that prune removes at most --count matches, the first ones in url order, all of them
// with -1, and the default count when the config has none, and that the dry run output shows
// the count.
func TestPruneCount(t *testing.T) {
	const cfgCount = `
imagePath: %s
pruneConfig:
  type: pattern
  expr: 'frobozz'
  count: %d
  dryRun: %t
`
	tests := []struct {
		count    int
		dryRun   bool
		wantGone []int
	}{
		{3, true, nil},
		{3, false, []int{0, 1, 2}},
		{0, false, []int{0, 1, 2, 3, 4}}, // config.DefaultPruneCount
		{-1, false, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}},
		{20, false, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}},
	}
	for _, tst := range tests {
		td, _, _, err := makeTestFiles(10)
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(td)
		if err := config.SetConfigFromStr([]byte(fmt.Sprintf(cfgCount, td, tst.count, tst.dryRun))); err != nil {
			t.Fatal(err)
		}
		out, err := captureStdout(Prune)
		if err != nil {
			t.Fatal(err)
		}
		for i := range 10 {
			_, found := serialize.MhFromFilesystem(fmt.Sprintf("foo.io/frobozz:x%dz", i), td)
			if found == slices.Contains(tst.wantGone, i) {
				t.Errorf("count %d dry run %t: x%dz expected pruned=%t", tst.count, tst.dryRun, i, slices.Contains(tst.wantGone, i))
			}
		}
		if wantMsg := fmt.Sprintf("Prune blobs (dry run with count %d):", tst.count); tst.dryRun && !strings.Contains(out, wantMsg) {
			t.Errorf("expected %q in the output, got:\n%s", wantMsg, out)
		}
	}
}

// captureStdout returns what the passed function writes to stdout, and its error.
func captureStdout(f func() error) (string, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	saved := os.Stdout
	os.Stdout = w
	ferr := f()
	os.Stdout = saved
	w.Close()
	b, _ := io.ReadAll(r)
	return string(b), ferr
}

// Test prune by date/time.
func TestPrunebyDate(t *testing.T) {
	manifestCnt := 10
	expectPrune := 5
	td, sharedBlobDigest, mhs, err := makeTestFiles(10)
	if td != "" {
		defer os.RemoveAll(td)
	}
	if err != nil {
		t.FailNow()
	}
	var cutoff time.Time

	i := 0
	for _, mh := range mhs {
		// change create date - one per month
		year, month, day := time.Now().AddDate(0, 0, -30+i).Date()
		datestr := fmt.Sprintf("%04d-%02d-%02dT23:24:25Z", year, month, day)
		tstamp, err := time.Parse(globals.DateFormat, datestr)
		if err != nil {
			t.FailNow()
		}
		if i == expectPrune-1 {
			// set the cutoff halfway through
			cutoff = tstamp.Add(time.Second)
		}
		mh, exists := serialize.MhFromFilesystem(mh.ImageUrl, td)
		if !exists {
			t.FailNow()
		}
		mh.Created = datestr
		if serialize.MhToFilesystem(mh, td, true) != nil {
			t.FailNow()
		}
		i++
	}
	cfg := fmt.Sprintf(cfgYaml, td, "date", cutoff.Format(globals.DateFormat))
	if err := config.SetConfigFromStr([]byte(cfg)); err != nil {
		t.FailNow()
	}
	if err := Prune(); err != nil {
		t.FailNow()
	} else if entries, err := os.ReadDir(filepath.Join(td, globals.ManifestPath)); err != nil {
		t.FailNow()
	} else if len(entries) != manifestCnt-expectPrune {
		t.FailNow()
	} else if verifyBlobPrune(td, len(entries), sharedBlobDigest) != nil {
		t.FailNow()
	}
}

// verifyBlobPrune looks for a blob that is shared by all manifests in the test
// file set so it should never be deleted. Also verifies that the expected number
// of blobs were pruned. Each blob has two unique manifests. So the correct blob
// count is the passed count times two + one for the shared.
func verifyBlobPrune(testdir string, cnt int, sharedBlobDigest string) error {
	if entries, err := os.ReadDir(filepath.Join(testdir, globals.BlobPath)); err != nil {
		return err
	} else if len(entries) != (cnt*2)+1 {
		return errors.New("incorrect remaining blob count")
	} else {
		foundSharedBlob := false
		for _, entry := range entries {
			if entry.Name() == sharedBlobDigest {
				foundSharedBlob = true
			}
		}
		if !foundSharedBlob {
			return errors.New("shared blob not found")
		}
	}
	return nil
}

// Makes image manifests and blobs. Each manifest contains two unique
// blobs and one blob shared by all manifests. Manifest urls are like
// z1z, z2z, z3z, ... The function returns:
//  1. The test directory name
//  2. The shared blob digest
//  3. An error (or nil)
func makeTestFiles(cnt int) (string, string, []imgpull.ManifestHolder, error) {
	dir, _ := os.MkdirTemp("", "")
	serialize.CreateDirs(dir, true)
	r := fmt.Sprintf("%d", rand.Uint64())
	sharedBlobDigest := digest.FromBytes([]byte(r)).Hex()
	if err := os.WriteFile(filepath.Join(dir, globals.BlobPath, sharedBlobDigest), []byte("foo\n"), 0777); err != nil {
		return "", "", nil, err
	}

	manifestDigests := make([]string, cnt)
	mhs := make([]imgpull.ManifestHolder, cnt)
	for i := range cnt {
		r := fmt.Sprintf("%d", rand.Uint64())
		manifestDigests[i] = digest.FromBytes([]byte(r)).Hex()
		// create 2 unique blob digests for the manifest
		blobDigests := make([]string, 2)
		for bd := range 2 {
			r := fmt.Sprintf("%d", rand.Uint64())
			blobDigests[bd] = digest.FromBytes([]byte(r)).Hex()
			if err := os.WriteFile(filepath.Join(dir, globals.BlobPath, blobDigests[bd]), []byte("foo"), 0777); err != nil {
				return "", "", nil, err
			}
		}
		var mh imgpull.ManifestHolder
		var err error
		imageUrl := fmt.Sprintf("foo.io/frobozz:x%dz", i)
		if i%2 == 0 {
			foo := fmt.Sprintf(v1ociManifest, blobDigests[0], blobDigests[1], sharedBlobDigest)
			mh, err = imgpull.NewManifestHolder("application/vnd.oci.image.manifest.v1+json", []byte(foo), manifestDigests[i], imageUrl)
			if err != nil {
				return "", "", nil, err
			}
		} else {
			foo := fmt.Sprintf(v2dockerManifest, blobDigests[0], blobDigests[1], sharedBlobDigest)
			mh, err = imgpull.NewManifestHolder("application/vnd.docker.distribution.manifest.v2+json", []byte(foo), manifestDigests[i], imageUrl)
			if err != nil {
				return "", "", nil, err
			}
		}
		mh.Created = globals.CurTime()
		mhs[i] = mh
		err = serialize.MhToFilesystem(mh, dir, true)
		if err != nil {
			return "", "", nil, err
		}
	}
	return dir, sharedBlobDigest, mhs, nil
}
