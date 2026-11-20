package preload

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aceeric/ociregistry/impl/cache"
	"github.com/aceeric/ociregistry/impl/config"
	"github.com/aceeric/ociregistry/impl/pullrequest"
	"github.com/aceeric/ociregistry/impl/serialize"
	"github.com/aceeric/ociregistry/mock"
)

// watchFixture loads an empty cache into the running server's in-mem cache, configured for
// linux/amd64 with the passed upstream (none if empty), air-gapped setting and pull timeout in
// milliseconds, and returns a watcher for a new, empty preload directory.
func watchFixture(t *testing.T, upstream string, airGapped bool, pullTimeout int) (*watcher, string) {
	t.Helper()
	cache.ResetCache()
	td := t.TempDir()
	cfg := fmt.Sprintf("imagePath: %s\nos: linux\narch: amd64\nairGapped: %t\npullTimeout: %d\n", td, airGapped, pullTimeout)
	if upstream != "" {
		cfg += fmt.Sprintf("registries:\n  - name: %s\n    scheme: http\n", upstream)
	}
	if err := config.SetConfigFromStr([]byte(cfg)); err != nil {
		t.Fatal(err)
	}
	if err := serialize.CreateDirs(td, true); err != nil {
		t.Fatal(err)
	}
	if err := cache.Load(td); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	return &watcher{path: dir, ledger: OpenLedger(td), pending: map[string]fileVersion{}}, dir
}

// cached returns true if the url is in the running server's cache.
func cached(t *testing.T, url string) bool {
	t.Helper()
	pr, err := pullrequest.NewPullRequestFromUrl(url)
	if err != nil {
		t.Fatal(err)
	}
	return cache.IsCached(pr)
}

// result returns the ledger result for the file, or "" if it's not in the ledger.
func result(w *watcher, path string) LedgerResult {
	for _, e := range w.ledger.Entries() {
		if e.Name == path {
			return e.Result
		}
	}
	return ""
}

var watchImage = mock.TarImage{Ref: "quay.io/org/app:v1", Os: "linux", Arch: "amd64", Layers: []string{"a layer"}}

// Tests that a new archive is loaded into the running cache on the pass after it's first seen,
// recorded in the ledger, and not processed again; and that a file still changing between
// passes waits until it's unchanged.
func TestWatcherTarball(t *testing.T) {
	w, dir := watchFixture(t, "", false, 5000)
	tarball := filepath.Join(dir, "app.tar")
	if err := mock.DockerSaveTarball(tarball, watchImage); err != nil {
		t.Fatal(err)
	}
	w.pass(nil)
	if cached(t, watchImage.Ref) || result(w, tarball) != "" {
		t.Fatal("a file seen for the first time should wait for the next pass")
	}
	w.pass(nil)
	if !cached(t, watchImage.Ref) || result(w, tarball) != ResultLoaded {
		t.Fatalf("expected the archive loaded and recorded, got cached=%t result=%q", cached(t, watchImage.Ref), result(w, tarball))
	}
	processed := w.ledger.Entries()[0].Processed
	time.Sleep(1100 * time.Millisecond)
	w.pass(nil)
	if w.ledger.Entries()[0].Processed != processed {
		t.Fatal("a ledgered file must not be processed again")
	}

	// a file being written: partial on one pass, complete on the next, then loaded
	growing := filepath.Join(dir, "other.tar")
	writePlainFile(t, growing, []byte("partial"))
	w.pass(nil)
	other := mock.TarImage{Ref: "quay.io/org/other:v1", Os: "linux", Arch: "amd64", Layers: []string{"other layer"}}
	if err := mock.DockerSaveTarball(growing, other); err != nil {
		t.Fatal(err)
	}
	w.pass(nil)
	if cached(t, other.Ref) || result(w, growing) != "" {
		t.Fatal("a file that changed since the last pass should wait")
	}
	w.pass(nil)
	if !cached(t, other.Ref) || result(w, growing) != ResultLoaded {
		t.Fatalf("expected the completed archive loaded, got %q", result(w, growing))
	}
}

// Tests that the watcher skips the files Preload already processed.
func TestWatcherSkipsPreloaded(t *testing.T) {
	w, dir := watchFixture(t, "", false, 5000)
	tarball := filepath.Join(dir, "app.tar")
	if err := mock.DockerSaveTarball(tarball, watchImage); err != nil {
		t.Fatal(err)
	}
	if err := Preload(dir, ""); err != nil {
		t.Fatal(err)
	}
	w.ledger = OpenLedger(config.GetImagePath())
	processed := w.ledger.Entries()[0].Processed
	time.Sleep(1100 * time.Millisecond)
	w.pass(nil)
	w.pass(nil)
	if e := w.ledger.Entries(); len(e) != 1 || e[0].Processed != processed {
		t.Fatalf("the watcher must not process a preloaded file again, got %+v", e)
	}
}

// Tests an image list file: a tag cached with a different digest than the upstream's is
// replaced with the upstream's image list, and the image for the platform is pulled; a new
// version of the file is then skipped, without touching the cached pull date.
func TestWatcherListFile(t *testing.T) {
	server, upstream := mock.Server(mock.NewMockParams(mock.NONE, mock.HTTP))
	defer server.Close()
	w, dir := watchFixture(t, upstream, false, 5000)
	url := upstream + "/hello-world:latest"

	// the tag cached with another digest, as if it moved upstream
	stale := filepath.Join(t.TempDir(), "stale.tar")
	mock.DockerSaveTarball(stale, mock.TarImage{Ref: "x/y:z", Os: "linux", Arch: "amd64", Layers: []string{"stale"}})
	if _, err := cache.LoadTarball(stale, url, config.GetImagePath(), "linux", "amd64", time.Second); err != nil {
		t.Fatal(err)
	}

	list := filepath.Join(dir, "list.txt")
	writePlainFile(t, list, []byte("# a comment\n"+url+"\n"))
	w.pass(nil)
	w.pass(nil)
	pr, _ := pullrequest.NewPullRequestFromUrl(url)
	mh, ok := cache.CachedManifest(pr)
	if result(w, list) != ResultLoaded || !ok || !mh.IsManifestList() {
		t.Fatalf("expected the moved tag replaced by the upstream list, got %q %t %t", result(w, list), ok, mh.IsManifestList())
	}
	digest, _ := mh.GetImageDigestFor("linux", "amd64")
	ipr, _ := pullrequest.NewPullRequestFromUrl(pr.UrlWithDigest(digest))
	img, ok := cache.CachedManifest(ipr)
	if !ok || mh.Pulled != "" || img.Pulled != "" {
		t.Fatalf("expected the list and the platform image cached without pull dates, got %t %q %q", ok, mh.Pulled, img.Pulled)
	}

	// an old pull date, which a client's cache hit would refresh: the watcher must not
	const oldPull = "2020-01-01T00:00:00Z"
	mh.Pulled = oldPull
	if err := serialize.MhToFilesystem(mh, config.GetImagePath(), true); err != nil {
		t.Fatal(err)
	}
	cache.ResetCache()
	if err := cache.Load(config.GetImagePath()); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Hour)
	os.Chtimes(list, later, later)
	w.pass(nil)
	w.pass(nil)
	after, _ := cache.CachedManifest(pr)
	if result(w, list) != ResultSkipped || after.Pulled != oldPull {
		t.Fatalf("expected skipped with the pull date untouched, got %q %q", result(w, list), after.Pulled)
	}
}

// Tests that when air-gapped an image list file fails, even with its upstream reachable, while
// an archive in the same pass loads.
func TestWatcherAirGapped(t *testing.T) {
	server, upstream := mock.Server(mock.NewMockParams(mock.NONE, mock.HTTP))
	defer server.Close()
	w, dir := watchFixture(t, upstream, true, 5000)
	list := filepath.Join(dir, "a-list.txt")
	writePlainFile(t, list, []byte(upstream+"/hello-world:latest\n"))
	tarball := filepath.Join(dir, "b.tar")
	if err := mock.DockerSaveTarball(tarball, watchImage); err != nil {
		t.Fatal(err)
	}
	w.pass(nil)
	w.pass(nil)
	if result(w, list) != ResultFailed || result(w, tarball) != ResultLoaded {
		t.Fatalf("expected the list failed and the archive loaded, got %q %q", result(w, list), result(w, tarball))
	}
}

// Tests that an archive with one image that fails is recorded failed, though its other image
// loads: here the failing image's url is being pulled by a client from an upstream that
// doesn't answer, so waiting for that pull times out.
func TestWatcherImageError(t *testing.T) {
	inPull, release := make(chan bool, 1), make(chan bool)
	callback := func(string) {
		select {
		case inPull <- true:
		default:
		}
		<-release
	}
	server, upstream := mock.ServerWithCallback(mock.NewMockParams(mock.NONE, mock.HTTP), &callback)
	defer server.Close()
	w, dir := watchFixture(t, upstream, false, 300)
	url := upstream + "/hello-world:latest"
	pr, _ := pullrequest.NewPullRequestFromUrl(url)
	clientDone := make(chan bool)
	go func() {
		cache.GetManifest(pr, config.GetImagePath(), 10000, false)
		close(clientDone)
	}()
	// before returning, let the client's pull finish, so it can't touch the next test's cache
	defer func() {
		close(release)
		<-clientDone
	}()
	<-inPull

	tarball := filepath.Join(dir, "two.tar")
	if err := mock.DockerSaveTarball(tarball, mock.TarImage{Ref: url, Os: "linux", Arch: "amd64", Layers: []string{"x"}}, watchImage); err != nil {
		t.Fatal(err)
	}
	w.pass(nil)
	w.pass(nil)
	if result(w, tarball) != ResultFailed || !cached(t, watchImage.Ref) {
		t.Fatalf("expected the file failed with its other image loaded, got %q %t", result(w, tarball), cached(t, watchImage.Ref))
	}
}

// Tests that a pass stops between files when asked.
func TestWatcherStop(t *testing.T) {
	w, dir := watchFixture(t, "", false, 5000)
	tarball := filepath.Join(dir, "app.tar")
	if err := mock.DockerSaveTarball(tarball, watchImage); err != nil {
		t.Fatal(err)
	}
	w.pass(nil)
	stop := make(chan bool, 1)
	stop <- true
	if !w.pass(stop) || result(w, tarball) != "" {
		t.Fatal("a pass asked to stop should stop before loading the next file")
	}
}

// Tests that RunWatcher starts the watcher only with both a watch frequency and preload images,
// that it's an error if the preload images can't be resolved, and that a started watcher stops
// promptly when asked.
func TestRunWatcher(t *testing.T) {
	dir := t.TempDir()
	for _, tst := range []struct {
		cfg     string
		enabled bool
		wantErr bool
	}{
		{fmt.Sprintf("preloadImages: %s\n", dir), false, false},
		{"watchFrequency: 30s\n", false, false},
		{"watchFrequency: 30s\npreloadImages: '['\n", true, true},
		{fmt.Sprintf("watchFrequency: 1h\npreloadImages: %s\nimagePath: %s\n", dir, t.TempDir()), true, false},
	} {
		if err := config.SetConfigFromStr([]byte(tst.cfg)); err != nil {
			t.Fatal(err)
		}
		stopWatchCh, watchStoppedCh := make(chan bool), make(chan bool)
		err := RunWatcher(stopWatchCh, watchStoppedCh)
		if WatcherEnabled() != tst.enabled || (err != nil) != tst.wantErr {
			t.Fatalf("%q: expected enabled=%t error=%t, got %t %v", tst.cfg, tst.enabled, tst.wantErr, WatcherEnabled(), err)
		}
		if tst.enabled && !tst.wantErr {
			stopWatchCh <- true
			select {
			case <-watchStoppedCh:
			case <-time.After(2 * time.Second):
				t.Fatal("the watcher didn't stop")
			}
		}
	}
}
