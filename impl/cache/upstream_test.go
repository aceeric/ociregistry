package cache

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/aceeric/ociregistry/impl/config"
	"github.com/aceeric/ociregistry/impl/pullrequest"
	"github.com/aceeric/ociregistry/impl/serialize"
	"github.com/aceeric/ociregistry/mock"
)

// upstreamFixture returns an empty cache with the passed mock upstream configured, and the
// pull request for its hello-world:latest image list.
func upstreamFixture(t *testing.T, upstream string) (string, pullrequest.PullRequest) {
	t.Helper()
	ResetCache()
	if err := config.SetConfigFromStr(fmt.Appendf(nil, regConfig, upstream)); err != nil {
		t.Fatal(err)
	}
	td := t.TempDir()
	if err := serialize.CreateDirs(td, true); err != nil {
		t.Fatal(err)
	}
	pr, err := pullrequest.NewPullRequestFromUrl(upstream + "/hello-world:latest")
	if err != nil {
		t.Fatal(err)
	}
	return td, pr
}

// Tests that LoadFromUpstream caches the manifest without a pull date, in memory and on the
// file system, while a client's pull of the image still gets one.
func TestLoadFromUpstream(t *testing.T) {
	server, upstream := mock.Server(mock.NewMockParams(mock.NONE, mock.HTTP))
	defer server.Close()
	td, pr := upstreamFixture(t, upstream)
	mh, err := LoadFromUpstream(pr, td, 5*time.Second)
	if err != nil || !mh.IsManifestList() || mh.Pulled != "" || mh.Created == "" {
		t.Fatalf("expected the list with a create date and no pull date, got %t %q %q %v", mh.IsManifestList(), mh.Created, mh.Pulled, err)
	}
	if cached, ok := CachedManifest(pr); !ok || cached.Pulled != "" {
		t.Fatalf("expected the list cached with no pull date, got %t %q", ok, cached.Pulled)
	}
	if fromFs, _ := serialize.MhFromFilesystem(pr.Url(), td); fromFs.Pulled != "" {
		t.Fatalf("expected no pull date on the file system, got %q", fromFs.Pulled)
	}
	// the url is no longer being pulled: another load doesn't wait for one
	if _, err := LoadFromUpstream(pr, td, 500*time.Millisecond); err != nil {
		t.Fatalf("a second load should not wait, got %v", err)
	}

	// a client's pull of the image for the platform: it gets a pull date
	digest, _ := mh.GetImageDigestFor("linux", "amd64")
	ipr, _ := pullrequest.NewPullRequestFromUrl(pr.UrlWithDigest(digest))
	img, err := GetManifest(ipr, td, 5000, false)
	if err != nil || img.Pulled == "" {
		t.Fatalf("expected a client's pull to get a pull date, got %q %v", img.Pulled, err)
	}
}

// cacheStale caches the pull request's url with a manifest from a tarball, so it has another
// digest than the upstream's, as if the tag moved, and returns that digest.
func cacheStale(t *testing.T, td string, pr pullrequest.PullRequest) string {
	t.Helper()
	stale := filepath.Join(t.TempDir(), "stale.tar")
	if err := mock.DockerSaveTarball(stale, mock.TarImage{Ref: "x/y:z", Os: "linux", Arch: "amd64", Layers: []string{"stale layer"}}); err != nil {
		t.Fatal(err)
	}
	results, err := LoadTarball(stale, pr.Url(), td, "linux", "amd64", time.Second)
	if err != nil || len(results) != 1 || results[0].Outcome != TarballLoaded {
		t.Fatal(results, err)
	}
	return results[0].Digest
}

// Tests that LoadFromUpstream replaces a tag cached with another digest, and removes the old
// manifest's blob.
func TestLoadFromUpstreamReplaces(t *testing.T) {
	server, upstream := mock.Server(mock.NewMockParams(mock.NONE, mock.HTTP))
	defer server.Close()
	td, pr := upstreamFixture(t, upstream)
	staleDigest := cacheStale(t, td, pr)
	mh, err := LoadFromUpstream(pr, td, 5*time.Second)
	if cached, _ := CachedManifest(pr); err != nil || mh.Digest == staleDigest || cached.Digest != mh.Digest {
		t.Fatalf("expected the moved tag replaced, got %q %v", mh.Digest, err)
	}
	if exists, _ := serialize.BlobExists(td, sha256Of("stale layer")); exists || bc.blobs[sha256Of("stale layer")] != 0 {
		t.Error("the replaced manifest's blob should be removed")
	}
}

// Tests that when the upstream can't be reached LoadFromUpstream fails, rather than serving
// the cached copy as a client's forced pull would, and leaves the cached manifest as it was.
func TestLoadFromUpstreamFails(t *testing.T) {
	server, upstream := mock.Server(mock.NewMockParams(mock.NONE, mock.HTTP))
	server.Close()
	td, pr := upstreamFixture(t, upstream)
	staleDigest := cacheStale(t, td, pr)
	if _, err := LoadFromUpstream(pr, td, time.Second); err == nil {
		t.Fatal("expected an error when the upstream can't be reached")
	}
	if cached, _ := CachedManifest(pr); cached.Digest != staleDigest || cached.Pulled != "" {
		t.Fatalf("a failed load must leave the cached manifest as it was, got %q %q", cached.Digest, cached.Pulled)
	}
}

// Tests that LoadFromUpstream waits for a pull of the same url in progress, then loads.
func TestLoadFromUpstreamWaits(t *testing.T) {
	server, upstream := mock.Server(mock.NewMockParams(mock.NONE, mock.HTTP))
	defer server.Close()
	td, pr := upstreamFixture(t, upstream)
	if _, er := enqueuePull(pr); er != mustPull {
		t.Fatal("expected to be the puller")
	}
	done := make(chan error)
	go func() {
		_, err := LoadFromUpstream(pr, td, 5*time.Second)
		done <- err
	}()
	select {
	case <-done:
		t.Fatal("the load should wait for the pull in progress")
	case <-time.After(200 * time.Millisecond):
	}
	signalWaiters(pr.Url())
	if err := <-done; err != nil || !IsCached(pr) {
		t.Fatalf("expected the load to finish after the pull, got %v", err)
	}
}
