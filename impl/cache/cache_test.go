package cache

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/aceeric/ociregistry/impl/config"
	"github.com/aceeric/ociregistry/impl/pullrequest"
	"github.com/aceeric/ociregistry/impl/serialize"
	"github.com/aceeric/ociregistry/mock"

	"github.com/aceeric/imgpull/pkg/imgpull"
	"github.com/aceeric/imgpull/pkg/imgpull/v1oci"
	"github.com/opencontainers/go-digest"
	log "github.com/sirupsen/logrus"
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

func init() {
	log.SetOutput(io.Discard)
}

// Setup: creates one each of the four supported manifest types, writes them to the file system,
// along with blobs. Tests that the cache.Load function loads them correctly.
func TestLoad(t *testing.T) {
	mts := []imgpull.ManifestType{imgpull.V2dockerManifestList, imgpull.V2dockerManifest, imgpull.V1ociIndex, imgpull.V1ociManifest}
	ResetCache()
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
		pr, err := pullrequest.NewPullRequestFromUrl(urlTag)
		if err != nil {
			t.Fail()
		}
		mh, _ := getManifestFromCache(pr, td)
		if reflect.DeepEqual(mh, emptyMH) {
			t.Fail()
		}
		urlSha := fmt.Sprintf("foo.io/foo/%d@sha256:%d", mt, mt)
		pr, err = pullrequest.NewPullRequestFromUrl(urlSha)
		if err != nil {
			t.Fail()
		}
		mh, _ = getManifestFromCache(pr, td)
		if reflect.DeepEqual(mh, emptyMH) {
			t.Fail()
		}
	}
}

// setupTestLoad creates manifests on the file system for each of the manifest types
// in the passed ManifestType array. Each image manifest gets three layers.
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
		if err := serialize.MhToFilesystem(mh, td, false); err != nil {
			return "", err
		}
	}
	return td, nil
}

var regConfig = `
---
registries:
  - name: %s
    scheme: http
`

// Tests concurrent pulls of the same manifest URL. Set a delay on the mock OCI distr.
// server to allow enough time to enqueue two "concurrent" pull to exercise the code path
// where the first pull actually goes to the upstream and the second pulls waits to be
// signalled by the first pull.
func TestConcurrentGet(t *testing.T) {
	ResetCache()
	params := mock.NewMockParams(mock.NONE, mock.HTTP)
	params.DelayMs = 500
	var upstreamPulls atomic.Int32
	callback := func(url string) {
		// if pulls are concurrent, only one should be going to the upstream
		if url == "/v2/hello-world/manifests/latest" {
			upstreamPulls.Add(1)
		}
	}
	server, url := mock.ServerWithCallback(params, &callback)
	cfg := fmt.Sprintf(regConfig, url)
	if err := config.SetConfigFromStr([]byte(cfg)); err != nil {
		t.Fail()
	}
	defer server.Close()
	td, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fail()
	}
	defer os.RemoveAll(td)
	pr, err := pullrequest.NewPullRequestFromUrl(fmt.Sprintf("%s/hello-world:latest", url))
	if err != nil {
		t.Fail()
	}
	var wg sync.WaitGroup
	var errs atomic.Int32
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			const twoSeconds = 2000
			if _, err := GetManifest(pr, td, twoSeconds, false); err != nil {
				errs.Add(1)
			}
			//fmt.Print("DONE")
		}()
	}
	wg.Wait()
	if upstreamPulls.Load() != 1 || errs.Load() != 0 {
		t.Fail()
	}
}

// Test that the left-minus-right digest function works as described.
func TestLMR(t *testing.T) {
	mhl := imgpull.ManifestHolder{
		Type: imgpull.V1ociManifest,
		V1ociManifest: v1oci.Manifest{
			Config: v1oci.Descriptor{Digest: "A"},
			Layers: []v1oci.Descriptor{
				{Digest: "B"},
				{Digest: "C"},
				{Digest: "D"},
			},
		},
	}
	mhr := imgpull.ManifestHolder{
		Type: imgpull.V1ociManifest,
		V1ociManifest: v1oci.Manifest{
			Config: v1oci.Descriptor{Digest: "C"},
			Layers: []v1oci.Descriptor{
				{Digest: "D"},
				{Digest: "E"},
				{Digest: "F"},
			},
		},
	}
	v1 := LmR(mhl, mhr)
	v2 := LmR(mhr, mhl)
	slices.Sort(v1)
	slices.Sort(v2)
	if slices.Compare(v1, []string{"A", "B"}) != 0 || slices.Compare(v2, []string{"E", "F"}) != 0 {
		t.Fail()
	}
}

func TestReplace(t *testing.T) {
	ResetCache()
	imageUrl := "foo.io/my-image:latest"
	td, _ := os.MkdirTemp("", "")
	defer os.RemoveAll(td)
	for _, dir := range []string{"fat", "img", "blobs"} {
		os.Mkdir(filepath.Join(td, dir), 0777)
	}
	// these are blobs - write them all now for simplicity even though E and F
	// in reality would arrive when pulling the second "latest" manifest
	digests := make([]string, 6)
	for idx, letter := range []string{"a", "b", "c", "d", "e", "f"} {
		digests[idx] = fmt.Sprintf("000000000000000000000000000000000000000000000000000000000000000%s", letter)
		os.WriteFile(filepath.Join(td, "blobs", digests[idx]), []byte(digests[idx]), 0777)
	}
	firstDigest := "1111111111111111111111111111111111111111111111111111111111111123"
	mhFirst := imgpull.ManifestHolder{
		Type:     imgpull.V1ociManifest,
		ImageUrl: imageUrl,
		Digest:   firstDigest,
		V1ociManifest: v1oci.Manifest{
			Config: v1oci.Descriptor{Digest: digests[0]},
			Layers: []v1oci.Descriptor{
				{Digest: digests[1]},
				{Digest: digests[2]},
				{Digest: digests[3]},
			},
		},
	}
	if err := serialize.MhToFilesystem(mhFirst, td, true); err != nil {
		t.FailNow()
	}
	pr, err := pullrequest.NewPullRequestFromUrl(imageUrl)
	if err != nil {
		t.FailNow()
	}
	// add first manifest to in-mem cache
	addToCache(pr, mhFirst, td)

	// second manifest has the same digest so is interpreted as
	// the same manifest so - NOP
	mhNewSameDigest := imgpull.ManifestHolder{
		Type:     imgpull.V1ociManifest,
		ImageUrl: imageUrl,
		Digest:   firstDigest,
		V1ociManifest: v1oci.Manifest{
			Config: v1oci.Descriptor{Digest: digests[2]},
			Layers: []v1oci.Descriptor{
				{Digest: digests[3]},
				{Digest: digests[4]},
				{Digest: digests[5]},
			},
		},
	}
	if err := serialize.MhToFilesystem(mhNewSameDigest, td, true); err != nil {
		t.FailNow()
	}
	// NOP because same digest
	replaceInCache(pr, mhNewSameDigest, td)
	files, err := os.ReadDir(filepath.Join(td, "blobs"))
	if err != nil {
		t.FailNow()
	}
	if len(files) != 6 || len(bc.blobs) != 4 {
		t.FailNow()
	}
	for digest := range bc.blobs {
		if !slices.Contains(digests[0:4], digest) {
			t.FailNow()
		}
	}
	_, exists := serialize.MhFromFilesystem(firstDigest, true, td)
	if !exists {
		t.FailNow()
	}

	// third manifest has the different digest so should trigger the
	// replace path
	secondDigest := "1111111111111111111111111111111111111111111111111111111111111456"
	mhNewDiffDigest := imgpull.ManifestHolder{
		Type:     imgpull.V1ociManifest,
		ImageUrl: imageUrl,
		Digest:   secondDigest,
		V1ociManifest: v1oci.Manifest{
			Config: v1oci.Descriptor{Digest: digests[2]},
			Layers: []v1oci.Descriptor{
				{Digest: digests[3]},
				{Digest: digests[4]},
				{Digest: digests[5]},
			},
		},
	}
	if err := serialize.MhToFilesystem(mhNewDiffDigest, td, true); err != nil {
		t.FailNow()
	}
	replaceInCache(pr, mhNewDiffDigest, td)
	files, err = os.ReadDir(filepath.Join(td, "blobs"))
	if err != nil {
		t.FailNow()
	}
	if len(files) != 4 || len(bc.blobs) != 4 {
		t.FailNow()
	}
	// only the new blobs should remain
	for digest := range bc.blobs {
		if !slices.Contains(digests[2:], digest) {
			t.FailNow()
		}
	}
	_, exists = serialize.MhFromFilesystem(firstDigest, true, td)
	if exists {
		t.FailNow()
	}
	_, exists = serialize.MhFromFilesystem(secondDigest, true, td)
	if !exists {
		t.FailNow()
	}
}
