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
	"time"

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
	   "digest": "sha256:%s",
	   "size": 64
	},
	"layers": [
	   {
		  "digest": "sha256:%s",
	      "size": 64
	   },
	   {
		  "digest": "sha256:%s",
	      "size": 64
	   }
	]
}`

var v1ociManifest = `{
	"schemaVersion": 2,
	"mediaType": "application/vnd.oci.image.manifest.v1+json",
	"config": {
	   "digest": "sha256:%s",
	   "size": 64
	},
	"layers": [
	   {
		  "digest": "sha256:%s",
	      "size": 64
	   },
	   {
		  "digest": "sha256:%s",
	      "size": 64
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
	// one entry per manifest, found by tag (foo.io/foo/0:v1.2.3) and, through the digest
	// alias, by digest (foo.io/foo/0@sha256:0)
	if mc.len() != len(mts) {
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

// Tests that a manifest haivng a blob whose size on disk does not equal the metadata
// size is not Loaded into cache from the filesystem. Since the filesystem only contains
// one manifest with one blob, the manifest and blob caches should be
func TestShortBlobFailsLoad(t *testing.T) {
	ResetCache()
	myImageUrl := "foo.io/my-image:latest"
	td := t.TempDir()
	serialize.CreateDirs(td, true)

	digest := "0000000000000000000000000000000000000000000000000000000000000000"
	// only write 5 bytes of the blob to simulate a corrupted blob download
	os.WriteFile(filepath.Join(td, globals.BlobPath, digest), []byte(digest[:5]), 0777)
	mh := imgpull.ManifestHolder{
		Type:     imgpull.V1ociManifest,
		ImageUrl: myImageUrl,
		Digest:   digest,
		V1ociManifest: v1oci.Manifest{
			Config: v1oci.Descriptor{Digest: digest, Size: 64},
			Layers: []v1oci.Descriptor{
				{Digest: digest, Size: 64},
			},
		},
	}
	if err := serialize.MhToFilesystem(mh, td, true); err != nil {
		t.FailNow()
	}
	if err := Load(td); err != nil {
		t.Fail()
	}
	if mc.len() != 00 || len(bc.blobs) != 0 {
		t.Fail()
	}
}

// Tests replacing a "latest" image in the cache with another "latest" image with a different
// digest. This is the case where the server is configured to always pull latest, and a latest
// manifest is pulled with one digest, and a subsequent pull of latest gets a different digest,
// such as when a newer latest is pushed.
func TestReplace(t *testing.T) {
	ResetCache()
	myImageUrl := "foo.io/my-image:latest"
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
		ImageUrl: myImageUrl,
		Digest:   firstDigest,
		V1ociManifest: v1oci.Manifest{
			Config: v1oci.Descriptor{Digest: digests[0], Size: 64},
			Layers: []v1oci.Descriptor{
				{Digest: digests[1], Size: 64},
				{Digest: digests[2], Size: 64},
				{Digest: digests[3], Size: 64},
			},
		},
	}
	if err := serialize.MhToFilesystem(mhFirst, td, true); err != nil {
		t.FailNow()
	}
	pr, err := pullrequest.NewPullRequestFromUrl(myImageUrl)
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
		ImageUrl: myImageUrl,
		Digest:   firstDigest,
		V1ociManifest: v1oci.Manifest{
			Config: v1oci.Descriptor{Digest: digests[2], Size: 64},
			Layers: []v1oci.Descriptor{
				{Digest: digests[3], Size: 64},
				{Digest: digests[4], Size: 64},
				{Digest: digests[5], Size: 64},
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
		ImageUrl: myImageUrl,
		Digest:   secondDigest,
		V1ociManifest: v1oci.Manifest{
			Config: v1oci.Descriptor{Digest: digests[2], Size: 64},
			Layers: []v1oci.Descriptor{
				{Digest: digests[3], Size: 64},
				{Digest: digests[4], Size: 64},
				{Digest: digests[5], Size: 64},
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
	if mhFound, _, exists := mc.lookup(imageUrl(pr.Url())); !exists {
		return false
	} else if mhFound.Digest != mhExp.Digest {
		return false
	} else {
		mhFromFs, found := serialize.MhFromFilesystem(mhExp.ImageUrl, testDir)
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
				Config: v1oci.Descriptor{Digest: blobDigests[0], Size: 64},
				Layers: []v1oci.Descriptor{
					{Digest: blobDigests[1], Size: 64},
					{Digest: blobDigests[2], Size: 64},
				},
			},
		},
		{
			Type:     imgpull.V1ociManifest,
			ImageUrl: "foo.io/bar/baz:v2",
			Digest:   manifestDigests[v2],
			V1ociManifest: v1oci.Manifest{
				Config: v1oci.Descriptor{Digest: blobDigests[1], Size: 64},
				Layers: []v1oci.Descriptor{
					{Digest: blobDigests[2], Size: 64},
					{Digest: blobDigests[3], Size: 64},
				},
			},
		},
	}
	ResetCache()
	td, _ := os.MkdirTemp("", "")
	defer os.RemoveAll(td)
	serialize.CreateDirs(td, true)
	for _, digest := range blobDigests {
		os.WriteFile(filepath.Join(td, globals.BlobPath, digest), []byte("0000000000000000000000000000000000000000000000000000000000000000"), 0777)
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
	// one entry each for v1, v2 and latest
	if mc.len() != 3 {
		t.FailNow()
	}
	// latest and v1 share a digest: the alias resolves to one of them, and both are tracked
	if tags := mc.aliases[digestUrl("foo.io/bar/baz@sha256:"+manifestDigests[v1])]; len(tags) != 2 {
		t.Fatalf("expected v1 and latest behind the v1 digest, got %v", tags)
	}
	if tags := mc.aliases[digestUrl("foo.io/bar/baz@sha256:"+manifestDigests[v2])]; len(tags) != 1 {
		t.Fatalf("expected only v2 behind the v2 digest, got %v", tags)
	}
}

func TestSignalAfterWaiterTimeout(t *testing.T) {
	ResetCache()
	pr, _ := pullrequest.NewPullRequestFromUrl("docker.io/foo/bar:v1")
	if _, er := enqueuePull(pr); er != mustPull {
		t.Fatal("expected puller")
	}
	if _, er := enqueuePull(pr); er != mustWait {
		t.Fatal("expected waiter")
	}
	// the waiter never receives - simulates a timed-out waiter
	done := make(chan struct{})
	go func() { signalWaiters(pr.Url()); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("signalWaiters blocked")
	}
	if pullsInProgress() != 0 {
		t.Fatal("pull not cleared")
	}
	if _, er := enqueuePull(pr); er != mustPull {
		t.Fatal("expected new puller after signal")
	}
}

// Tests that docker.io/<name> and docker.io/library/<name> are the same image to the cache:
// one entry, found by either spelling (including IsCached, used in air-gapped mode), and one
// concurrent-pull entry so two clients using different spellings don't both go upstream.
func TestDockerIoSpellingsShareCache(t *testing.T) {
	ResetCache()
	td := t.TempDir()
	serialize.CreateDirs(td, true)
	short, err := pullrequest.NewPullRequestFromUrl("docker.io/hello-world:latest")
	if err != nil {
		t.Fatal(err)
	}
	long, err := pullrequest.NewPullRequestFromUrl("docker.io/library/hello-world:latest")
	if err != nil {
		t.Fatal(err)
	}
	mh := imgpull.ManifestHolder{
		Type:     imgpull.V1ociIndex,
		Digest:   "424f1f86cdf501deb591ace8d14d2f40272617b51b374915a87a2886b2025ece",
		ImageUrl: short.Url(),
	}
	if err := addToCache(short, mh, td); err != nil {
		t.Fatal(err)
	}
	if mc.len() != 1 {
		t.Fatalf("expected one entry, got %d", mc.len())
	}
	for _, pr := range []pullrequest.PullRequest{short, long} {
		if !IsCached(pr) {
			t.Errorf("IsCached(%q) = false", pr.Url())
		}
		if _, found := getManifestFromCache(pr, td); !found {
			t.Errorf("getManifestFromCache(%q) not found", pr.Url())
		}
	}
	if _, er := enqueuePull(short); er != mustPull {
		t.Fatal("expected puller")
	}
	if _, er := enqueuePull(long); er != mustWait {
		t.Fatal("the other spelling should wait on the same pull")
	}
	signalWaiters(short.Url())
}

// Tests that Load rewrites a non-canonical ImageUrl from an older cache, so the entry's
// ImageUrl matches its key and prune can select it.
func TestLoadCanonicalizesImageUrl(t *testing.T) {
	ResetCache()
	td := t.TempDir()
	serialize.CreateDirs(td, true)
	mh := imgpull.ManifestHolder{
		Type:     imgpull.V1ociIndex,
		Digest:   "424f1f86cdf501deb591ace8d14d2f40272617b51b374915a87a2886b2025ece",
		ImageUrl: "docker.io/hello-world:v1",
	}
	if err := serialize.MhToFilesystem(mh, td, false); err != nil {
		t.Fatal(err)
	}
	if err := Load(td); err != nil {
		t.Fatal(err)
	}
	got, _, exists := mc.lookup("docker.io/library/hello-world:v1")
	if !exists || got.ImageUrl != "docker.io/library/hello-world:v1" {
		t.Fatalf("expected canonical entry, got exists=%t url=%q", exists, got.ImageUrl)
	}
	if found := GetManifestsCompare(func(imgpull.ManifestHolder) bool { return true }, noLimit); len(found) != 1 {
		t.Fatalf("expected the entry to be a prune candidate, got %d", len(found))
	}
}

// aliasFixture writes blob files for the passed digests (64 bytes each) and returns the cache
// directory and a function that builds an image manifest for a url, digest and blobs.
func aliasFixture(t *testing.T, blobs ...string) (string, func(url, digest string, blobs ...string) (pullrequest.PullRequest, imgpull.ManifestHolder)) {
	t.Helper()
	ResetCache()
	td := t.TempDir()
	serialize.CreateDirs(td, true)
	for _, b := range blobs {
		if err := os.WriteFile(filepath.Join(td, globals.BlobPath, b), []byte(b), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return td, func(url, digest string, blobs ...string) (pullrequest.PullRequest, imgpull.ManifestHolder) {
		t.Helper()
		pr, err := pullrequest.NewPullRequestFromUrl(url)
		if err != nil {
			t.Fatal(err)
		}
		layers := []v1oci.Descriptor{}
		for _, b := range blobs[1:] {
			layers = append(layers, v1oci.Descriptor{Digest: "sha256:" + b, Size: 64})
		}
		return pr, imgpull.ManifestHolder{
			Type:     imgpull.V1ociManifest,
			Digest:   digest,
			ImageUrl: pr.Url(),
			V1ociManifest: v1oci.Manifest{
				Config: v1oci.Descriptor{Digest: "sha256:" + blobs[0], Size: 64},
				Layers: layers,
			},
		}
	}
}

var (
	aliasDigestD = "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	aliasDigestE = "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	aliasBlob1   = "1111111111111111111111111111111111111111111111111111111111111111"
	aliasBlob2   = "2222222222222222222222222222222222222222222222222222222222222222"
	aliasBlob3   = "3333333333333333333333333333333333333333333333333333333333333333"
)

// Tests two tags with the same digest (a release tag and a mutable tag): both are cached,
// a request by digest is served while either tag remains, pruning one tag or moving the other
// doesn't hide the manifest from the remaining tag, and blob ref counts follow the entries.
func TestTwoTagsShareDigest(t *testing.T) {
	td, build := aliasFixture(t, aliasBlob1, aliasBlob2, aliasBlob3)
	prRel, mhRel := build("quay.io/org/app:v1.0.0", aliasDigestD, aliasBlob1, aliasBlob2)
	prNightly, mhNightly := build("quay.io/org/app:nightly-pr", aliasDigestD, aliasBlob1, aliasBlob2)
	for _, add := range []struct {
		pr pullrequest.PullRequest
		mh imgpull.ManifestHolder
	}{{prRel, mhRel}, {prNightly, mhNightly}} {
		// as doPull does: the file first, then the cache entry
		if err := serialize.MhToFilesystem(add.mh, td, true); err != nil {
			t.Fatal(err)
		}
		if err := addToCache(add.pr, add.mh, td); err != nil {
			t.Fatal(err)
		}
	}
	digestOfD := imageUrl("quay.io/org/app@sha256:" + aliasDigestD)
	if mc.len() != 2 || bc.blobs[aliasBlob1] != 2 {
		t.Fatalf("expected 2 entries and 2 refs per blob, got %d entries, refs %v", mc.len(), bc.blobs)
	}
	if _, _, found := mc.lookup(digestOfD); !found {
		t.Fatal("digest request should be served from a tag entry")
	}

	// prune the release tag: the mutable tag still serves the digest
	prune(mhRel, pruneAll, td)
	if _, _, found := mc.lookup(digestOfD); !found {
		t.Fatal("pruning one tag hid the digest from the other")
	}
	if _, found := serialize.MhFromFilesystem("quay.io/org/app:nightly-pr", td); !found {
		t.Fatal("pruning one tag removed the other tag's file")
	}
	if _, found := serialize.MhFromFilesystem("quay.io/org/app:v1.0.0", td); found {
		t.Fatal("the pruned tag's file should be removed")
	}
	if bc.blobs[aliasBlob1] != 1 {
		t.Fatalf("expected 1 ref after the prune, got %d", bc.blobs[aliasBlob1])
	}

	// the mutable tag moves to a new digest: the old digest is no longer cached
	prNew, mhNew := build("quay.io/org/app:nightly-pr", aliasDigestE, aliasBlob1, aliasBlob3)
	if err := replaceInCache(prNew, mhNew, td); err != nil {
		t.Fatal(err)
	}
	if _, _, found := mc.lookup(digestOfD); found {
		t.Fatal("no tag has the old digest any more")
	}
	if mh, _, found := mc.lookup(imageUrl("quay.io/org/app@sha256:" + aliasDigestE)); !found || mh.Digest != aliasDigestE {
		t.Fatal("the new digest should be served from the moved tag")
	}
	if bc.blobs[aliasBlob1] != 1 || bc.blobs[aliasBlob3] != 1 {
		t.Fatalf("shared blob should keep 1 ref, new blob gets 1: %v", bc.blobs)
	}
	if _, ok := bc.blobs[aliasBlob2]; ok {
		t.Fatal("blob only the old digest used should have no refs")
	}
	if len(mc.aliases) != 1 {
		t.Fatalf("expected only the new digest alias, got %v", mc.aliases)
	}
}

// Tests that a request by digest served from a tag entry updates the tag entry and its file,
// and doesn't create a second entry or a file for the digest url.
func TestDigestRequestUpdatesTagEntry(t *testing.T) {
	td, build := aliasFixture(t, aliasBlob1, aliasBlob2)
	pr, mh := build("quay.io/org/app:v1", aliasDigestD, aliasBlob1, aliasBlob2)
	if err := addToCache(pr, mh, td); err != nil {
		t.Fatal(err)
	}
	byDigest, err := pullrequest.NewPullRequestFromUrl("quay.io/org/app@sha256:" + aliasDigestD)
	if err != nil {
		t.Fatal(err)
	}
	got, found := getManifestFromCache(byDigest, td)
	if !found || got.Pulled == "" {
		t.Fatal("expected a hit with a pulled date")
	}
	if mc.len() != 1 || mc.entries["quay.io/org/app:v1"].Pulled != got.Pulled {
		t.Fatalf("the tag entry should be updated in place: %d entries", mc.len())
	}
	if fromFs, _ := serialize.MhFromFilesystem("quay.io/org/app:v1", td); fromFs.Pulled != got.Pulled {
		t.Fatal("the tag's file should have the new pulled date")
	}
	if _, found := serialize.MhFromFilesystem(byDigest.Url(), td); found {
		t.Fatal("no file should be written for the digest url")
	}
}

// Tests that a digest entry of its own is preferred over a tag alias, and that the two are
// independent entries with their own blob refs.
func TestDigestEntryAndTagEntry(t *testing.T) {
	td, build := aliasFixture(t, aliasBlob1, aliasBlob2)
	prDigest, mhDigest := build("quay.io/org/app@sha256:"+aliasDigestD, aliasDigestD, aliasBlob1, aliasBlob2)
	prTag, mhTag := build("quay.io/org/app:v1", aliasDigestD, aliasBlob1, aliasBlob2)
	if err := addToCache(prDigest, mhDigest, td); err != nil {
		t.Fatal(err)
	}
	if err := addToCache(prTag, mhTag, td); err != nil {
		t.Fatal(err)
	}
	if _, key, _ := mc.lookup(imageUrl(prDigest.Url())); key != imageUrl(prDigest.Url()) {
		t.Fatalf("expected the digest entry itself, got %q", key)
	}
	if mc.len() != 2 || bc.blobs[aliasBlob1] != 2 {
		t.Fatalf("expected 2 entries and 2 refs, got %d %v", mc.len(), bc.blobs)
	}
	prune(mhDigest, pruneAll, td)
	if _, key, found := mc.lookup(imageUrl(prDigest.Url())); !found || key != imageUrl(prTag.Url()) {
		t.Fatal("after pruning the digest entry, the tag alias should serve the digest")
	}
}

// Tests that adding a manifest for a url that is already cached replaces it without leaking
// blob refs, whether the digest is the same or different.
func TestAddSameUrlKeepsRefCounts(t *testing.T) {
	td, build := aliasFixture(t, aliasBlob1, aliasBlob2, aliasBlob3)
	pr, mh := build("quay.io/org/app:v1", aliasDigestD, aliasBlob1, aliasBlob2)
	for range 2 {
		if err := addToCache(pr, mh, td); err != nil {
			t.Fatal(err)
		}
	}
	if mc.len() != 1 || bc.blobs[aliasBlob1] != 1 || bc.blobs[aliasBlob2] != 1 {
		t.Fatalf("same manifest added twice: %d entries, refs %v", mc.len(), bc.blobs)
	}
	pr, mh = build("quay.io/org/app:v1", aliasDigestE, aliasBlob1, aliasBlob3)
	if err := addToCache(pr, mh, td); err != nil {
		t.Fatal(err)
	}
	if mc.len() != 1 || bc.blobs[aliasBlob1] != 1 || bc.blobs[aliasBlob3] != 1 {
		t.Fatalf("different manifest for the same url: %d entries, refs %v", mc.len(), bc.blobs)
	}
	if _, ok := bc.blobs[aliasBlob2]; ok {
		t.Fatal("the replaced manifest's blob should have no refs")
	}
	if _, _, found := mc.lookup(imageUrl("quay.io/org/app@sha256:" + aliasDigestD)); found {
		t.Fatal("the replaced digest should no longer resolve")
	}
}

// Tests that digest aliases are scoped to a repository: the same digest in two repositories is
// two aliases, and a digest request is only served from a tag in the requested repository.
func TestDigestAliasPerRepository(t *testing.T) {
	td, build := aliasFixture(t, aliasBlob1, aliasBlob2)
	prA, mhA := build("ghcr.io/aaa/aaa:v1.0.0", aliasDigestD, aliasBlob1, aliasBlob2)
	prB, mhB := build("ghcr.io/bbb/bbb:v2.0.0", aliasDigestD, aliasBlob1, aliasBlob2)
	for _, add := range []struct {
		pr pullrequest.PullRequest
		mh imgpull.ManifestHolder
	}{{prA, mhA}, {prB, mhB}} {
		if err := addToCache(add.pr, add.mh, td); err != nil {
			t.Fatal(err)
		}
	}
	for url, want := range map[imageUrl]imageUrl{
		imageUrl("ghcr.io/aaa/aaa@sha256:" + aliasDigestD): "ghcr.io/aaa/aaa:v1.0.0",
		imageUrl("ghcr.io/bbb/bbb@sha256:" + aliasDigestD): "ghcr.io/bbb/bbb:v2.0.0",
	} {
		if mh, key, found := mc.lookup(url); !found || key != want || mh.ImageUrl != string(want) {
			t.Errorf("lookup(%s): expected %s, got %q (found=%t)", url, want, key, found)
		}
	}
	if _, _, found := mc.lookup(imageUrl("ghcr.io/ccc/ccc@sha256:" + aliasDigestD)); found {
		t.Error("a repository with no tag for the digest must miss")
	}
	// pruning bbb's tag doesn't let bbb's digest fall back to aaa
	prune(mhB, pruneAll, td)
	if _, _, found := mc.lookup(imageUrl("ghcr.io/bbb/bbb@sha256:" + aliasDigestD)); found {
		t.Error("bbb's digest must not be served from another repository")
	}
}

// Tests that set keeps an entry's ImageUrl equal to its key, so a write-back or prune of the
// entry can never touch another url's file. A mismatch is a caller bug and is logged.
func TestSetKeepsImageUrlEqualToKey(t *testing.T) {
	ResetCache()
	pr, err := pullrequest.NewPullRequestFromUrl("quay.io/org/app:v1")
	if err != nil {
		t.Fatal(err)
	}
	mh := imgpull.ManifestHolder{Type: imgpull.V1ociIndex, Digest: aliasDigestD, ImageUrl: "quay.io/org/other:v9"}
	mc.set(pr, mh)
	got, key, found := mc.lookup("quay.io/org/app:v1")
	if !found || got.ImageUrl != string(key) {
		t.Fatalf("expected ImageUrl to equal the key, got %q under %q", got.ImageUrl, key)
	}
	if _, _, found := mc.lookup("quay.io/org/other:v9"); found {
		t.Fatal("nothing should be stored under the mismatched url")
	}
}
