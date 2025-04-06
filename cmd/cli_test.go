package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"ociregistry/impl/upstream"
	"ociregistry/impl/upstream/v1oci"
	"ociregistry/impl/upstream/v2docker"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/aceeric/imgpull/pkg/imgpull"
	"github.com/opencontainers/go-digest"
)

// Test prune manifests and blobs by url pattern
func TestPrunebyPattern(t *testing.T) {
	manifestCnt := 10
	expectPrune := 3
	d, sharedBlobDigest, err := makeTestFiles(manifestCnt)
	if d != "" {
		defer os.RemoveAll(d)
	}
	if err != nil {
		t.Fail()
	}
	c := cmdLine{
		imagePath: d,
		prune:     "2,4,6",
	}
	if _, err = prunePattern(c); err != nil {
		t.Fail()
	} else if entries, err := os.ReadDir(filepath.Join(d, "img")); err != nil {
		t.Fail()
	} else if len(entries) != manifestCnt-expectPrune {
		t.Fail()
	} else if verifyBlobPrune(d, len(entries), sharedBlobDigest) != nil {
		t.Fail()
	}
}

// Test prune manifests and blobs by date/time.
func TestPrunebyDate(t *testing.T) {
	manifestCnt := 10
	expectPrune := 5
	d, sharedBlobDigest, err := makeTestFiles(10)
	if d != "" {
		defer os.RemoveAll(d)
	}
	if err != nil {
		t.Fail()
	}
	var cutoff time.Time
	if entries, err := os.ReadDir(filepath.Join(d, "img")); err != nil {
		t.Fail()
	} else {
		for i := 0; i < manifestCnt; i++ {
			// change create date - one per month
			datestr := fmt.Sprintf("2025-%02d-01T23:24:25", i+1)
			tstamp, err := time.Parse(dateFormat, datestr)
			if err != nil {
				t.Fail()
			}
			if i == expectPrune-1 {
				cutoff = tstamp.Add(time.Second)
			}
			fname := filepath.Join(d, "img", entries[i].Name())
			if err := os.Chtimes(fname, tstamp, tstamp); err != nil {
				fmt.Println(err)
			}
		}
	}
	c := cmdLine{
		imagePath:   d,
		pruneBefore: cutoff.Format(dateFormat),
	}
	if _, err = pruneBefore(c); err != nil {
		t.Fail()
	} else if entries, err := os.ReadDir(filepath.Join(d, "img")); err != nil {
		t.Fail()
	} else if len(entries) != manifestCnt-expectPrune {
		t.Fail()
	} else if verifyBlobPrune(d, len(entries), sharedBlobDigest) != nil {
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
// z1z, z2z, z3z, ...
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
		mh := imgpull.ManifestHolder{
			ImageUrl: "z" + strconv.Itoa(i) + "z",
			Digest:   manifestDigests[i],
		}
		if i%2 == 0 {
			mh.Type = upstream.V1ociDescriptor
			mh.V1ociManifest = v1oci.Manifest{
				Config: v1oci.Descriptor{
					Digest: blobDigests[0],
				},
				Layers: []v1oci.Descriptor{
					{Digest: blobDigests[1]},
					{Digest: sharedBlobDigest},
				},
			}
		} else {
			mh.Type = upstream.V2dockerManifest
			mh.V2dockerManifest = v2docker.Manifest{
				Config: v2docker.Descriptor{
					Digest: blobDigests[0],
				},
				Layers: []v2docker.Descriptor{
					{Digest: blobDigests[1]},
					{Digest: sharedBlobDigest},
				},
			}
		}
		mb, _ := json.Marshal(mh)
		err := os.WriteFile(filepath.Join(dir, "img", manifestDigests[i]), mb, 0777)
		if err != nil {
			return "", "", err
		}
	}
	return dir, sharedBlobDigest, nil
}
