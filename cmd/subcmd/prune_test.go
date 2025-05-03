package subcmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/aceeric/ociregistry/impl/config"
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
  expr: %s
  dryRun: false
`

// Test prune by url pattern
func TestPrunebyPattern(t *testing.T) {
	manifestCnt := 10
	expectPrune := 3
	td, sharedBlobDigest, err := makeTestFiles(manifestCnt)
	if td != "" {
		defer os.RemoveAll(td)
	}
	if err != nil {
		t.Fail()
	}
	cfg := fmt.Sprintf(cfgYaml, td, "pattern", "2,4,6")
	if err := config.SetConfigFromStr([]byte(cfg)); err != nil {
		t.Fail()
	}
	if err := Prune(); err != nil {
		t.Fail()
	} else if entries, err := os.ReadDir(filepath.Join(td, "img")); err != nil {
		t.Fail()
	} else if len(entries) != manifestCnt-expectPrune {
		t.Fail()
	} else if verifyBlobPrune(td, len(entries), sharedBlobDigest) != nil {
		t.Fail()
	}
}

// Test prune by date/time.
func TestPrunebyDate(t *testing.T) {
	manifestCnt := 10
	expectPrune := 5
	td, sharedBlobDigest, err := makeTestFiles(10)
	if td != "" {
		defer os.RemoveAll(td)
	}
	if err != nil {
		t.Fail()
	}
	var cutoff time.Time
	// all files will be named by digest - the digests are irrelevant we just use
	// them to get some files to change the dates on
	if entries, err := os.ReadDir(filepath.Join(td, "img")); err != nil {
		t.Fail()
	} else {
		for i := 0; i < manifestCnt; i++ {
			// change create date - one per month
			year, month, day := time.Now().AddDate(0, -1, 0).Date()
			datestr := fmt.Sprintf("%04d-%02d-%02dT23:24:25", year, month, day+i)
			tstamp, err := time.Parse(dateFormat, datestr)
			if err != nil {
				t.Fail()
			}
			if i == expectPrune-1 {
				// set the cutoff halfway through
				cutoff = tstamp.Add(time.Second)
			}
			mh, exists := serialize.MhFromFilesystem(entries[i].Name(), true, td)
			if !exists {
				t.Fail()
			}
			mh.Created = datestr
			if serialize.MhToFilesystem(mh, td, true) != nil {
				t.Fail()
			}
		}
	}
	cfg := fmt.Sprintf(cfgYaml, td, "date", cutoff.Format(dateFormat))
	if err := config.SetConfigFromStr([]byte(cfg)); err != nil {
		t.Fail()
	}
	if err := Prune(); err != nil {
		t.Fail()
	} else if entries, err := os.ReadDir(filepath.Join(td, "img")); err != nil {
		t.Fail()
	} else if len(entries) != manifestCnt-expectPrune {
		t.Fail()
	} else if verifyBlobPrune(td, len(entries), sharedBlobDigest) != nil {
		t.Fail()
	}
}

// verifyBlobPrune looks for a blob that is shared by all manifests in the test
// file set so it should never be deleted. Also verifies that the expected number
// of blobs were pruned. Each blob has two unique manifests. So the correct blob
// count is the passed count times two + one for the shared.
func verifyBlobPrune(testdir string, cnt int, sharedBlobDigest string) error {
	if entries, err := os.ReadDir(filepath.Join(testdir, "blobs")); err != nil {
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
func makeTestFiles(cnt int) (string, string, error) {
	dir, _ := os.MkdirTemp("", "")
	os.Mkdir(filepath.Join(dir, "fat"), 0777)
	os.Mkdir(filepath.Join(dir, "img"), 0777)
	os.Mkdir(filepath.Join(dir, "blobs"), 0777)
	r := fmt.Sprintf("%d", rand.Uint64())
	sharedBlobDigest := digest.FromBytes([]byte(r)).Hex()
	if err := os.WriteFile(filepath.Join(dir, "blobs", sharedBlobDigest), []byte("foo\n"), 0777); err != nil {
		return "", "", err
	}

	manifestDigests := make([]string, cnt)
	for i := 0; i < cnt; i++ {
		r := fmt.Sprintf("%d", rand.Uint64())
		manifestDigests[i] = digest.FromBytes([]byte(r)).Hex()
		// create 2 unique blob digests for the manifest
		blobDigests := make([]string, 2)
		for bd := 0; bd < 2; bd++ {
			r := fmt.Sprintf("%d", rand.Uint64())
			blobDigests[bd] = digest.FromBytes([]byte(r)).Hex()
			if err := os.WriteFile(filepath.Join(dir, "blobs", blobDigests[bd]), []byte("foo"), 0777); err != nil {
				return "", "", err
			}
		}
		var mh imgpull.ManifestHolder
		var err error
		if i%2 == 0 {
			foo := fmt.Sprintf(v1ociManifest, blobDigests[0], blobDigests[1], sharedBlobDigest)
			mh, err = imgpull.NewManifestHolder("application/vnd.oci.image.manifest.v1+json", []byte(foo), manifestDigests[i], "z"+strconv.Itoa(i)+"z")
			if err != nil {
				return "", "", err
			}
		} else {
			foo := fmt.Sprintf(v2dockerManifest, blobDigests[0], blobDigests[1], sharedBlobDigest)
			mh, err = imgpull.NewManifestHolder("application/vnd.docker.distribution.manifest.v2+json", []byte(foo), manifestDigests[i], "z"+strconv.Itoa(i)+"z")
			if err != nil {
				return "", "", err
			}
		}
		mh.Created = time.Now().Format(dateFormat)
		mb, _ := json.Marshal(mh)
		err = os.WriteFile(filepath.Join(dir, "img", manifestDigests[i]), mb, 0777)
		if err != nil {
			return "", "", err
		}
	}
	return dir, sharedBlobDigest, nil
}
