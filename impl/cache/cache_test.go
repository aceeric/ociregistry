package cache

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"ociregistry/impl/serialize"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"

	"github.com/aceeric/imgpull/pkg/imgpull"
	"github.com/opencontainers/go-digest"
)

// copy impull test?? ALL PULLS

// test concurrent pulls

// test enqueueing

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

var v2dockerManifestList = `{
	"schemaVersion": 2,
	"mediaType": "application/vnd.docker.distribution.manifest.list.v2+json",
	"manifests": [
	]
}`

var v1ociIndex = `{
	"schemaVersion": 2,
	"mediaType": "application/vnd.oci.image.index.v1+json",
	"manifests": [
	]
}`

func resetCache() {
	cp = concurrentPulls{
		pulls: make(map[string][]chan bool),
	}
	mc = manifestCache{
		manifests: map[string]imgpull.ManifestHolder{},
	}
	bc = blobCache{
		blobs: map[string]int{},
	}
}

// Setup: creates one each of the four supported manifest types, writes them to the file system,
// along with blobs. Tests that the cache.Load function loads them correctly.
func TestLoad(t *testing.T) {
	mts := []imgpull.ManifestType{imgpull.V2dockerManifestList, imgpull.V2dockerManifest, imgpull.V1ociIndex, imgpull.V1ociManifest}
	resetCache()
	td, err := setupTestLoad(mts)
	if td != "" {
		defer os.RemoveAll(td)
	}
	if err != nil {
		t.Fail()
	}
	if Load(td) != nil {
		t.Fail()
	}
	// each manifest is cached twice: one with the tag (foo.io/foo/0:v1.2.3) and one with the
	// digest (foo.io/foo/0@sha256:0)
	if len(mc.manifests) != len(mts)*2 {
		t.Fail()
	}
	// two image manifests each with a config and two layers
	if len(bc.blobs) != 6 {
		t.Fail()
	}
	for _, mt := range mts {
		emptyMH := imgpull.ManifestHolder{}
		urlTag := fmt.Sprintf("foo.io/foo/%d:v1.2.3", mt)
		mh := getManifestFromCache(urlTag)
		if reflect.DeepEqual(mh, emptyMH) {
			t.Fail()
		}
		urlSha := fmt.Sprintf("foo.io/foo/%d@sha256:%d", mt, mt)
		mh = getManifestFromCache(urlSha)
		if reflect.DeepEqual(mh, emptyMH) {
			t.Fail()
		}
	}
}

func setupTestLoad(mts []imgpull.ManifestType) (string, error) {
	td, _ := os.MkdirTemp("", "")
	for _, dir := range []string{"fat", "img", "blobs"} {
		os.Mkdir(filepath.Join(td, dir), 0777)
	}
	randomDigest := func() string {
		s := fmt.Sprintf("%d%d%d%d", rand.Uint64(), rand.Uint64(), rand.Uint64(), rand.Uint64())
		return digest.FromBytes([]byte(s)).Hex()
	}
	for _, mt := range mts {
		mh := imgpull.ManifestHolder{
			Type:     mt,
			Digest:   strconv.Itoa(int(mt)),
			ImageUrl: fmt.Sprintf("foo.io/foo/%d:v1.2.3", mt),
		}
		var digests []string
		if mh.IsImageManifest() {
			digests = []string{randomDigest(), randomDigest(), randomDigest()}
			for _, digest := range digests {
				if err := os.WriteFile(filepath.Join(td, "blobs", digest), []byte(digest), 0755); err != nil {
					return "", err
				}
			}
		}
		switch mt {
		case imgpull.V2dockerManifestList:
			if err := json.Unmarshal([]byte(v2dockerManifestList), &mh.V2dockerManifestList); err != nil {
				return "", err
			}
		case imgpull.V2dockerManifest:
			if err := json.Unmarshal([]byte(fmt.Sprintf(v2dockerManifest, digests[0], digests[1], digests[2])), &mh.V2dockerManifest); err != nil {
				return "", err
			}
		case imgpull.V1ociIndex:
			if err := json.Unmarshal([]byte(v1ociIndex), &mh.V1ociIndex); err != nil {
				return "", err
			}
		case imgpull.V1ociManifest:
			if err := json.Unmarshal([]byte(fmt.Sprintf(v1ociManifest, digests[0], digests[1], digests[2])), &mh.V1ociManifest); err != nil {
				return "", err
			}
		}
		if err := serialize.MhToFilesystem(mh, td); err != nil {
			return "", err
		}
	}
	return td, nil
}
