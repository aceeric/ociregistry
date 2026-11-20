package preload

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aceeric/imgpull/pkg/imgpull"
	"github.com/aceeric/ociregistry/impl/cache"
	"github.com/aceeric/ociregistry/impl/config"
	"github.com/aceeric/ociregistry/impl/helpers"
	"github.com/aceeric/ociregistry/impl/pullrequest"
	log "github.com/sirupsen/logrus"
)

// errAirGapped is the error for an image list file found by the watcher when the server is
// air-gapped: its images can only come from the upstream.
var errAirGapped = errors.New("image list files can't be loaded when the server is air-gapped")

// fileVersion is a file's size and modification time, which identify a version of the file.
type fileVersion struct {
	size  int64
	mtime time.Time
}

// watcher looks for new or changed preload files and loads them into the running server's
// cache. It's an extension of Preload: it resolves the same path, and records each file in the
// same ledger, so neither loads what the other already has.
type watcher struct {
	path   string
	ledger *Ledger
	// pending has the files seen on the previous pass that aren't in the ledger, with their
	// version then. A file is only processed once it's unchanged between two passes, so a
	// file still being written isn't loaded partway.
	pending map[string]fileVersion
}

// WatcherEnabled returns true if the preload watcher is configured: a watch frequency, and
// preload images to watch.
func WatcherEnabled() bool {
	return config.GetWatchFrequency() != 0 && config.GetPreloadImages() != ""
}

// RunWatcher runs the preload watcher goroutine if enabled (see WatcherEnabled). Every watch
// frequency, it looks for files resolved from the preload images (as Preload does) that aren't
// in the ledger, and loads them into the running server's cache. It's meant to be called after
// the cache is loaded, and so after Preload. Stop it by sending on stopWatchCh; it answers on
// watchStoppedCh once it has stopped, after finishing the file it's loading if any. The resolve
// ref is never applied: it's for a single, named tarball. It's an error if the preload images
// can't be resolved, such as a malformed glob.
func RunWatcher(stopWatchCh, watchStoppedCh chan bool) error {
	if !WatcherEnabled() {
		if config.GetWatchFrequency() != 0 {
			log.Info("a watch frequency is configured but no preload images, so there's nothing to watch")
		} else {
			log.Info("preload watcher not enabled")
		}
		return nil
	}
	path, interval := config.GetPreloadImages(), config.GetWatchFrequency()
	if _, err := ResolveFiles(path, ResolveOpts{AllowNoMatch: true}); err != nil {
		return err
	}
	if config.GetResolveRef() != "" {
		log.Infof("the preload watcher doesn't apply the resolve ref %q to the files it loads", config.GetResolveRef())
	}
	w := &watcher{path: path, ledger: OpenLedger(config.GetImagePath()), pending: map[string]fileVersion{}}
	log.Infof("starting the preload watcher: %q every %s", path, interval)
	go func() {
		for {
			select {
			case <-stopWatchCh:
				watchStoppedCh <- true
				return
			case <-time.After(interval):
				if w.pass(stopWatchCh) {
					watchStoppedCh <- true
					return
				}
			}
		}
	}()
	return nil
}

// pass makes one pass over the files resolved from the watched path, loading each one that's
// not in the ledger and unchanged since the previous pass, and recording it in the ledger. It
// checks for a stop request between files, and returns true if it got one.
func (w *watcher) pass(stopWatchCh chan bool) bool {
	files, err := ResolveFiles(w.path, ResolveOpts{AllowNoMatch: true})
	if err != nil {
		log.Errorf("preload watcher: error resolving %q: %s", w.path, err)
		return false
	}
	if err := w.ledger.RemoveMissing(); err != nil {
		log.Errorf("preload watcher: error updating the preload ledger: %s", err)
	}
	seen := map[string]bool{}
	for _, f := range files {
		select {
		case <-stopWatchCh:
			return true
		default:
		}
		fi, err := os.Stat(f.Path)
		if err != nil || w.ledger.Done(f.Path, fi) {
			continue
		}
		seen[f.Path] = true
		v := fileVersion{size: fi.Size(), mtime: fi.ModTime()}
		if prev, ok := w.pending[f.Path]; !ok || prev.size != v.size || !prev.mtime.Equal(v.mtime) {
			// new or still changing: wait for the next pass
			w.pending[f.Path] = v
			continue
		}
		delete(w.pending, f.Path)
		if err := w.ledger.Record(f.Path, fi, w.load(f)); err != nil {
			log.Errorf("preload watcher: error updating the preload ledger: %s", err)
		}
	}
	for p := range w.pending {
		if !seen[p] {
			delete(w.pending, p)
		}
	}
	return false
}

// load loads one file into the running server's cache, logs the outcome, and returns the
// result to record in the ledger.
func (w *watcher) load(f ResolvedFile) LedgerResult {
	pullTimeout := time.Duration(config.GetPullTimeout()) * time.Millisecond
	if f.Tarball {
		results, err := cache.LoadTarball(f.Path, "", config.GetImagePath(), config.GetOs(), config.GetArch(), pullTimeout)
		if err != nil {
			log.Errorf("preload watcher: error loading %q (it won't be loaded again until it changes): %s", f.Path, err)
			return ResultFailed
		}
		counts := map[cache.TarballOutcome]int{}
		for _, r := range results {
			counts[r.Outcome]++
		}
		log.Infof("preload watcher: %q: %d image(s) loaded, %d already cached, %d error(s)", f.Path,
			counts[cache.TarballLoaded], counts[cache.TarballCached], counts[cache.TarballError])
		switch {
		case counts[cache.TarballError] > 0:
			log.Errorf("preload watcher: %q had errors (it won't be loaded again until it changes)", f.Path)
			return ResultFailed
		case counts[cache.TarballLoaded] > 0:
			return ResultLoaded
		}
		return ResultSkipped
	}
	cnt, err := w.loadListFile(f.Path, pullTimeout)
	if err != nil {
		log.Errorf("preload watcher: error loading %q (it won't be loaded again until it changes): %s", f.Path, err)
		return ResultFailed
	}
	log.Infof("preload watcher: %q: %d image(s) loaded", f.Path, cnt)
	if cnt > 0 {
		return ResultLoaded
	}
	return ResultSkipped
}

// loadListFile loads the images listed in an image list file (see LoadFromListFile for the
// format) into the running server's cache, and returns how many manifests were pulled. It
// stops at the first image that fails.
func (w *watcher) loadListFile(path string, pullTimeout time.Duration) (int, error) {
	if config.GetAirGapped() {
		return 0, errAirGapped
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	cnt := 0
	for scanner := bufio.NewScanner(f); scanner.Scan(); {
		url := strings.TrimSpace(scanner.Text())
		if url == "" || strings.HasPrefix(url, "#") {
			continue
		}
		n, err := loadListedImage(url, pullTimeout)
		cnt += n
		if err != nil {
			return cnt, fmt.Errorf("image %q: %w", url, err)
		}
	}
	return cnt, nil
}

// loadListedImage loads one listed image into the running server's cache, as preload does for
// the file system cache: the upstream is asked for the url's current digest (a HEAD request),
// and if the cache has the url with that digest nothing is done. Otherwise the url is pulled
// into the cache (cache.LoadFromUpstream), replacing a moved tag. For an image list, the same
// is done for the image for the server's platform, by its digest url. Nothing it loads gets a
// pull date, which is only for clients' pulls, and a cached manifest is checked without
// touching its pull date. It returns how many manifests were pulled.
func loadListedImage(url string, pullTimeout time.Duration) (int, error) {
	pr, err := pullrequest.NewPullRequestFromUrl(url)
	if err != nil {
		return 0, fmt.Errorf("unable to parse image ref %q", url)
	}
	opts, err := config.ConfigFor(pr.Remote)
	if err != nil {
		return 0, err
	}
	opts.Url = pr.Url()
	puller, err := imgpull.NewPullerWith(opts)
	if err != nil {
		return 0, err
	}
	md, err := puller.HeadManifest()
	if err != nil {
		return 0, err
	}
	cnt := 0
	mh, cached := cache.CachedManifest(pr)
	if !cached || mh.Digest != helpers.GetDigestFrom(md.Digest) {
		if mh, err = cache.LoadFromUpstream(pr, config.GetImagePath(), pullTimeout); err != nil {
			return cnt, err
		}
		cnt++
	}
	if !mh.IsManifestList() {
		return cnt, nil
	}
	digest, err := mh.GetImageDigestFor(config.GetOs(), config.GetArch())
	if err != nil {
		return cnt, err
	}
	ipr, err := pullrequest.NewPullRequestFromUrl(pr.UrlWithDigest(digest))
	if err != nil {
		return cnt, err
	}
	// a digest url's content never changes, so a cached one is current
	if _, cached := cache.CachedManifest(ipr); !cached {
		if _, err := cache.LoadFromUpstream(ipr, config.GetImagePath(), pullTimeout); err != nil {
			return cnt, err
		}
		cnt++
	}
	return cnt, nil
}
