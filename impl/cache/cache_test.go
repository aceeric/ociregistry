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
	"github.com/aceeric/ociregistry/impl/globals"
	"github.com/aceeric/ociregistry/impl/pullrequest"
	"github.com/aceeric/ociregistry/impl/serialize"
	"github.com/aceeric/ociregistry/mock"

	"github.com/aceeric/imgpull/pkg/imgpull"
	"github.com/aceeric/imgpull/pkg/imgpull/v1oci"
	"github.com/opencontainers/go-digest"
	log "github.com/sirupsen/logrus"
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

// Creates one each of the four supported manifest types, writes them to the file system,
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
	if mc.len() != len(mts)*2 {
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
	serialize.CreateDirs(td, true)
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
				if err := os.WriteFile(filepath.Join(td, globals.BlobPath, digest), []byte(digest), 0755); err != nil {
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
			if err := json.Unmarshal(fmt.Appendf(nil, v2dockerManifest, digests[0], digests[1], digests[2]), &mh.V2dockerManifest); err != nil {
				return "", err
			}
		case imgpull.V1ociIndex:
			if err := json.Unmarshal([]byte(v1ociIndex), &mh.V1ociIndex); err != nil {
				return "", err
			}
		case imgpull.V1ociManifest:
			if err := json.Unmarshal(fmt.Appendf(nil, v1ociManifest, digests[0], digests[1], digests[2]), &mh.V1ociManifest); err != nil {
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
		// if pulls are concurrent, only one should be going to the upstream which means
		// this gets called twice for the pulling goroutine - once to check auth and second
		// to actually pull
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
	serialize.CreateDirs(td, true)
	pr, err := pullrequest.NewPullRequestFromUrl(fmt.Sprintf("%s/hello-world:latest", url))
	if err != nil {
		t.Fail()
	}
	var wg sync.WaitGroup
	var errs atomic.Int32
	expectCnt := int32(2)
	for range 3 {
		wg.Go(func() {
			const twoSeconds = 2000
			if _, err := GetManifest(pr, td, twoSeconds, false); err != nil {
				errs.Add(1)
			}
		})
	}
	wg.Wait()
	if upstreamPulls.Load() != expectCnt || errs.Load() != 0 {
		t.Fail()
	}
}

// Tests replacing a "latest" image in the cache with another "latest" image with a different
// digest. This is the case where the server is configured to always pull latest, and a latest
// manifest is pulled with one digest, and a subsequent pull of latest gets a different digest,
// such as when a newer latest is pushed.
func TestReplace(t *testing.T) {
	ResetCache()
	imageUrl := "foo.io/my-image:latest"
	td, _ := os.MkdirTemp("", "")
	defer os.RemoveAll(td)
	serialize.CreateDirs(td, true)

	uniq := []string{"a", "b", "c", "d", "e", "f"}
	digests := make([]string, len(uniq))
	for idx, letter := range uniq {
		digests[idx] = fmt.Sprintf("000000000000000000000000000000000000000000000000000000000000000%s", letter)
	}

	// first test - should create a manifest since the cache is empty
	for i := range 4 {
		os.WriteFile(filepath.Join(td, globals.BlobPath, digests[i]), []byte(digests[i]), 0777)
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
	if err := replaceInCache(pr, mhFirst, td); err != nil {
		t.FailNow()
	}
	if !validateReplace(digests[0:4], td, pr, mhFirst) {
		t.FailNow()
	}

	// second test - second manifest has the same digest so is interpreted as
	// the same manifest so: NOP. Here we change the manifest content which
	// wouldn't happen in real life because if the digest is the same the content
	// must be the same but this will ensure that this different manifest is not
	// replacing the other
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
	// don't write this to the file system - only check to make sure the in-mem cache
	// is not altered
	if err := replaceInCache(pr, mhNewSameDigest, td); err != nil {
		t.FailNow()
	}
	if !validateReplace(digests[0:4], td, pr, mhFirst) {
		t.FailNow()
	}

	// third test - different digest (same url) triggers the replace
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
	// write the new blobs
	for i := 4; i < 6; i++ {
		os.WriteFile(filepath.Join(td, globals.BlobPath, digests[i]), []byte(digests[i]), 0777)
	}
	if err := replaceInCache(pr, mhNewDiffDigest, td); err != nil {
		t.FailNow()
	}
	if !validateReplace(digests[2:6], td, pr, mhNewDiffDigest) {
		t.FailNow()
	}
}

func validateReplace(expCachedBlobs []string, testDir string, pr pullrequest.PullRequest, mhExp imgpull.ManifestHolder) bool {
	if len(bc.blobs) != len(expCachedBlobs) {
		return false
	}
	for digest := range bc.blobs {
		if !slices.Contains(expCachedBlobs, digest) {
			return false
		}
		if _, err := os.Stat(filepath.Join(testDir, globals.BlobPath, digest)); err != nil {
			return false
		}
	}
	if mhFound, exists := fromCache(pr.Url()); !exists {
		return false
	} else if mhFound.Digest != mhExp.Digest {
		return false
	} else {
		isLatest, err := mhExp.IsLatest()
		if err != nil {
			return false
		}
		mhFromFs, found := serialize.MhFromFilesystem(mhExp.Digest, isLatest, testDir)
		if !found {
			return false
		}
		if !reflect.DeepEqual(mhFromFs, mhExp) {
			return false
		}
	}
	return true
}

// Tests what happens when a "v1" and "v2" manifest are cached, then "latest" comes
// in with the same digest as "v1", then latest comes in again with the same digest as "v2",
// and then latest comes in AGAIN with the same digest as "v1". The cache has to handle
// the latest flip-flopping correctly and manage the blob ref counts correctly.
func TestLatestChangesTagAlignment(t *testing.T) {
	manifestDigests := []string{
		"0000000000000000000000000000000000000000000000000000000000000000",
		"1111111111111111111111111111111111111111111111111111111111111111",
	}
	blobDigests := []string{
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
	}
	const (
		v1 = 0
		v2 = 1
	)
	mhs := []imgpull.ManifestHolder{
		{
			Type:     imgpull.V1ociManifest,
			ImageUrl: "foo.io/bar/baz:v1",
			Digest:   manifestDigests[v1],
			V1ociManifest: v1oci.Manifest{
				Config: v1oci.Descriptor{Digest: blobDigests[0]},
				Layers: []v1oci.Descriptor{
					{Digest: blobDigests[1]},
					{Digest: blobDigests[2]},
				},
			},
		},
		{
			Type:     imgpull.V1ociManifest,
			ImageUrl: "foo.io/bar/baz:v2",
			Digest:   manifestDigests[v2],
			V1ociManifest: v1oci.Manifest{
				Config: v1oci.Descriptor{Digest: blobDigests[1]},
				Layers: []v1oci.Descriptor{
					{Digest: blobDigests[2]},
					{Digest: blobDigests[3]},
				},
			},
		},
	}
	ResetCache()
	td, _ := os.MkdirTemp("", "")
	defer os.RemoveAll(td)
	serialize.CreateDirs(td, true)
	for _, digest := range blobDigests {
		os.WriteFile(filepath.Join(td, globals.BlobPath, digest), []byte("foo"), 0777)
	}

	pr, _ := pullrequest.NewPullRequestFromUrl(mhs[v1].ImageUrl)
	if err := serialize.MhToFilesystem(mhs[v1], td, true); err != nil {
		t.FailNow()
	}
	if err := addToCache(pr, mhs[v1], td); err != nil {
		t.FailNow()
	}

	pr, _ = pullrequest.NewPullRequestFromUrl(mhs[v2].ImageUrl)
	if err := serialize.MhToFilesystem(mhs[v2], td, true); err != nil {
		t.FailNow()
	}
	if err := addToCache(pr, mhs[v2], td); err != nil {
		t.FailNow()
	}

	// change V1 to latest and "pull"
	mhs[v1].ImageUrl = "foo.io/bar/baz:latest"
	if err := serialize.MhToFilesystem(mhs[v1], td, true); err != nil {
		t.FailNow()
	}
	pr, _ = pullrequest.NewPullRequestFromUrl(mhs[v1].ImageUrl)
	if err := replaceInCache(pr, mhs[v1], td); err != nil {
		t.FailNow()
	}

	// change V2 to latest and "pull"
	mhs[v2].ImageUrl = "foo.io/bar/baz:latest"
	if err := serialize.MhToFilesystem(mhs[v2], td, true); err != nil {
		t.FailNow()
	}
	pr, _ = pullrequest.NewPullRequestFromUrl(mhs[v2].ImageUrl)
	if err := replaceInCache(pr, mhs[v2], td); err != nil {
		t.FailNow()
	}

	// "pull" V1 (still latest) again to overwrite V2 "latest"
	if err := serialize.MhToFilesystem(mhs[v1], td, true); err != nil {
		t.FailNow()
	}
	pr, _ = pullrequest.NewPullRequestFromUrl(mhs[v1].ImageUrl)
	if err := replaceInCache(pr, mhs[v1], td); err != nil {
		t.FailNow()
	}
	// no blobs should have been removed
	if len(bc.blobs) != 4 {
		t.FailNow()
	}
	// should be two V1 (one by tag, one by digest), two V2 (same reason), and
	// two latest (same reason)
	if mc.len() != 6 {
		t.FailNow()
	}
}
