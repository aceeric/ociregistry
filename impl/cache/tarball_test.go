package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/aceeric/ociregistry/impl/globals"
	"github.com/aceeric/ociregistry/impl/helpers"
	"github.com/aceeric/ociregistry/impl/pullrequest"
	"github.com/aceeric/ociregistry/impl/serialize"
	"github.com/aceeric/ociregistry/mock"
)

// tarballFixture returns an empty cache directory and a function that writes a docker-save
// tarball with the passed images to a new file and loads it into the cache, as linux/amd64.
func tarballFixture(t *testing.T) (string, func(name, resolveRef string, timeout time.Duration, images ...mock.TarImage) ([]TarballResult, error)) {
	t.Helper()
	ResetCache()
	td := t.TempDir()
	if err := serialize.CreateDirs(td, true); err != nil {
		t.Fatal(err)
	}
	tars := t.TempDir()
	return td, func(name, resolveRef string, timeout time.Duration, images ...mock.TarImage) ([]TarballResult, error) {
		t.Helper()
		path := filepath.Join(tars, name)
		if err := mock.DockerSaveTarball(path, images...); err != nil {
			t.Fatal(err)
		}
		return LoadTarball(path, resolveRef, td, "linux", "amd64", timeout)
	}
}

// sha256Of returns the hex sha256 digest of s: the blob digest of a mock.TarImage layer.
func sha256Of(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// outcomes returns the results as url=outcome strings, for comparing.
func outcomes(results []TarballResult) []string {
	s := []string{}
	for _, r := range results {
		s = append(s, r.Url+"="+string(r.Outcome))
	}
	return s
}

// layerDigests returns the blob digests of the cached manifest for the url.
func layerDigests(t *testing.T, url string) []string {
	t.Helper()
	mh, _, exists := lookup(imageUrl(url))
	if !exists {
		t.Fatalf("%s is not cached", url)
	}
	digests := []string{}
	for _, l := range mh.Layers() {
		digests = append(digests, helpers.GetDigestFrom(l.Digest))
	}
	return digests
}

var (
	appV1   = mock.TarImage{Ref: "quay.io/org/app:v1", Os: "linux", Arch: "amd64", Layers: []string{"base layer", "app layer"}}
	otherV2 = mock.TarImage{Ref: "quay.io/org/other:v2", Os: "linux", Arch: "amd64", Layers: []string{"base layer", "other layer"}}
	armV1   = mock.TarImage{Ref: "quay.io/org/arm:v1", Os: "linux", Arch: "arm64", Layers: []string{"arm layer"}}
)

// Tests loading a tarball, plain and gzip-compressed: images for the platform are loaded into
// the in-mem cache and the file system, a layer they share has two refs, an image for another
// platform is left out, and loading the same tarball again finds everything cached.
func TestLoadTarball(t *testing.T) {
	for _, name := range []string{"images.tar", "images.tgz"} {
		td, load := tarballFixture(t)
		results, err := load(name, "", time.Second, appV1, otherV2, armV1)
		want := "quay.io/org/app:v1=loaded quay.io/org/other:v2=loaded"
		if got := strings.Join(outcomes(results), " "); err != nil || got != want {
			t.Fatalf("%s: expected %q, got %q %v", name, want, got, err)
		}
		for _, url := range []string{"quay.io/org/app:v1", "quay.io/org/other:v2"} {
			mh, found := serialize.MhFromFilesystem(url, td)
			if !found || !globals.IsUtc(mh.Created) || mh.Created == "" {
				t.Errorf("%s: %s should be on the file system with a created date, got %t %q", name, url, found, mh.Created)
			}
			for _, d := range layerDigests(t, url) {
				if exists, _ := serialize.BlobExists(td, d); !exists {
					t.Errorf("%s: blob %s of %s should be on the file system", name, d, url)
				}
			}
		}
		if base := sha256Of("base layer"); bc.blobs[base] != 2 {
			t.Errorf("%s: the shared layer should have 2 refs, has %d", name, bc.blobs[base])
		}
		if len(mc.entries) != 2 {
			t.Errorf("%s: expected 2 cached manifests, got %d", name, len(mc.entries))
		}
		results, err = load("again-"+name, "", time.Second, appV1, otherV2)
		want = "quay.io/org/app:v1=cached quay.io/org/other:v2=cached"
		if got := strings.Join(outcomes(results), " "); err != nil || got != want {
			t.Errorf("%s: expected %q, got %q %v", name, want, got, err)
		}
	}
}

// Tests CachedManifest: the manifest of a cached tag, the same manifest through the tag's digest
// url, nothing for a url that isn't cached, and that the pull date isn't touched.
func TestCachedManifest(t *testing.T) {
	_, load := tarballFixture(t)
	results, err := load("v1.tar", "", time.Second, appV1)
	if err != nil || len(results) != 1 {
		t.Fatal(results, err)
	}
	tag, _ := pullrequest.NewPullRequestFromUrl("quay.io/org/app:v1")
	byDigest, _ := pullrequest.NewPullRequestFromUrl("quay.io/org/app@sha256:" + results[0].Digest)
	missing, _ := pullrequest.NewPullRequestFromUrl("quay.io/org/app:v2")
	for _, pr := range []pullrequest.PullRequest{tag, byDigest} {
		if mh, ok := CachedManifest(pr); !ok || mh.Digest != results[0].Digest || mh.Pulled != "" {
			t.Errorf("%s: expected %s with no pull date, got %q %t %q", pr.Url(), results[0].Digest, mh.Digest, ok, mh.Pulled)
		}
	}
	if mh, ok := CachedManifest(tag); !ok || mh.Pulled != "" {
		t.Errorf("CachedManifest must not set the pull date, got %q", mh.Pulled)
	}
	if _, ok := CachedManifest(missing); ok {
		t.Error("expected nothing for an uncached url")
	}
}

// Tests that a tarball with a new image for a cached tag replaces it, and that the old image's
// blob, with no refs left, is removed.
func TestLoadTarballMovedTag(t *testing.T) {
	td, load := tarballFixture(t)
	if _, err := load("v1.tar", "", time.Second, appV1); err != nil {
		t.Fatal(err)
	}
	oldLayer := sha256Of("app layer")
	moved := mock.TarImage{Ref: appV1.Ref, Os: "linux", Arch: "amd64", Layers: []string{"base layer", "new app layer"}}
	results, err := load("moved.tar", "", time.Second, moved)
	if err != nil || len(results) != 1 || results[0].Outcome != TarballLoaded {
		t.Fatalf("expected the moved tag to be loaded, got %v %v", results, err)
	}
	if !slices.Contains(layerDigests(t, "quay.io/org/app:v1"), sha256Of("new app layer")) {
		t.Fatal("the cached manifest should be the moved one")
	}
	if exists, _ := serialize.BlobExists(td, oldLayer); exists || bc.blobs[oldLayer] != 0 {
		t.Errorf("the old layer should be removed: on disk %t, refs %d", exists, bc.blobs[oldLayer])
	}
}

// Tests that a resolve ref caches the one image in a tarball under that url, and that it's an
// error for a tarball with more than one image, which loads nothing.
func TestLoadTarballResolveRef(t *testing.T) {
	_, load := tarballFixture(t)
	results, err := load("one.tar", "docker.io/me/renamed:1", time.Second, appV1)
	if err != nil || len(results) != 1 || results[0].Url != "docker.io/me/renamed:1" || results[0].Outcome != TarballLoaded {
		t.Fatalf("expected docker.io/me/renamed:1 loaded, got %v %v", results, err)
	}
	if _, _, exists := lookup("quay.io/org/app:v1"); exists {
		t.Error("the image should only be cached under the resolve ref")
	}
	before := len(mc.entries)
	if _, err := load("two.tar", "docker.io/me/renamed:2", time.Second, appV1, otherV2); !errors.Is(err, ErrResolveRef) {
		t.Fatalf("expected ErrResolveRef, got %v", err)
	}
	if len(mc.entries) != before {
		t.Error("nothing should be loaded for a resolve ref error")
	}
}

// Tests that a file that isn't an image tarball is an error that loads nothing.
func TestLoadTarballMalformed(t *testing.T) {
	td, _ := tarballFixture(t)
	path := filepath.Join(t.TempDir(), "junk.tar")
	os.WriteFile(path, []byte("this is not a tarball"), 0644)
	if _, err := LoadTarball(path, "", td, "linux", "amd64", time.Second); err == nil {
		t.Fatal("expected an error")
	}
	if len(mc.entries) != 0 {
		t.Fatal("nothing should be loaded")
	}
}

// Tests that a tarball image whose url is being pulled from the upstream waits for the pull
// to finish and then loads, and that waiting longer than the pull timeout is an error for
// that image.
func TestLoadTarballWaitsForPull(t *testing.T) {
	_, load := tarballFixture(t)
	pr, _ := pullrequest.NewPullRequestFromUrl("quay.io/org/app:v1")
	if _, er := enqueuePull(pr); er != mustPull {
		t.Fatal("expected to be the puller")
	}
	done := make(chan []TarballResult)
	go func() {
		results, _ := load("v1.tar", "", 5*time.Second, appV1)
		done <- results
	}()
	select {
	case <-done:
		t.Fatal("the load should wait for the pull in progress")
	case <-time.After(200 * time.Millisecond):
	}
	signalWaiters(pr.Url())
	if results := <-done; len(results) != 1 || results[0].Outcome != TarballLoaded {
		t.Fatalf("expected loaded after the pull finished, got %v", results)
	}

	_, load = tarballFixture(t)
	enqueuePull(pr)
	defer signalWaiters(pr.Url())
	results, err := load("v1.tar", "", 50*time.Millisecond, appV1)
	if err != nil || len(results) != 1 || results[0].Outcome != TarballError || !strings.Contains(results[0].Error, "timed out") {
		t.Fatalf("expected a timeout error, got %v %v", results, err)
	}
}

// Tests that a client request for an image while it's loading from a tarball waits for the
// tarball's manifest rather than pulling it from the upstream (none is configured here, so a
// pull would fail).
func TestLoadTarballClientWaits(t *testing.T) {
	td, load := tarballFixture(t)
	// as if a prune is running: the load enqueues the url, then waits for pullGate
	pullGate.Lock()
	done := make(chan []TarballResult)
	go func() {
		results, _ := load("v1.tar", "", 5*time.Second, appV1)
		done <- results
	}()
	for start := time.Now(); pullsInProgress() == 0; time.Sleep(10 * time.Millisecond) {
		if time.Since(start) > 5*time.Second {
			pullGate.Unlock()
			t.Fatal("the load didn't enqueue its url")
		}
	}
	pr, _ := pullrequest.NewPullRequestFromUrl("quay.io/org/app:v1")
	type got struct {
		digest string
		err    error
	}
	client := make(chan got)
	go func() {
		mh, err := GetManifest(pr, td, 5000, false)
		client <- got{mh.Digest, err}
	}()
	time.Sleep(100 * time.Millisecond)
	pullGate.Unlock()
	results := <-done
	c := <-client
	if len(results) != 1 || results[0].Outcome != TarballLoaded || c.err != nil || c.digest != results[0].Digest {
		t.Fatalf("expected the client to get the tarball's manifest, got %v and %q %v", results, c.digest, c.err)
	}
}
