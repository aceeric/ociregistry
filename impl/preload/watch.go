package preload

import (
	"bufio"
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

// fileVersion is a file's size and modification time, which identify a version
// of the file.
type fileVersion struct {
	size  int64
	mtime time.Time
}

// watcher looks for new or changed preload files and loads them into the
// running server's cache. It's an extension of Preload: it resolves the same
// path, and records each file in the same ledger, so neither loads what the
// other already has. The preloadImages field is populated from the
// `--preload-images` arg of serve (or `preloadImages` in `--config-file`)
type watcher struct {
	preloadImages string
	ledger        *Ledger
	// pending has the files seen on the previous pass that aren't in the
	// ledger, with their version info (size and timestamp). A file is only
	// processed once it's unchanged between two passes, so a file still being
	// written isn't loaded partway.
	pending map[string]fileVersion
}

// w is the watch struct. It is initialized by the production server in the
// 'RunWatcher' function but also set by unit tests.
var w watcher

// WatcherEnabled returns true if the preload watcher is configured with a
// non-zero watch frequency, and a preload path specifier.
func WatcherEnabled() bool {
	return config.GetWatchFrequency() != 0 && config.GetPreloadImages() != ""
}

// RunWatcher runs the preload watcher goroutine if enabled (see
// WatcherEnabled). Every watch frequency, it looks for files resolved from the
// preload images (as Preload does) that aren't in the ledger, and loads them
// into the running server's cache. It's meant to be called after the cache is
// loaded, and so after Preload. Stop it by sending on stopWatchCh; it answers
// on watchStoppedCh once it has stopped, after finishing the file it's loading
// if any. The resolve ref is never applied: it's for a single, named tarball.
// It's an error if the preload images can't be resolved, such as a malformed
// glob.
func RunWatcher(stopWatchCh, watchStoppedCh chan bool) error {
	if !WatcherEnabled() {
		if config.GetWatchFrequency() != 0 {
			log.Info("a watch frequency is configured but no preload images, so there's nothing to watch")
		} else {
			log.Info("preload watcher not enabled")
		}
		return nil
	}
	preloadImages, interval := config.GetPreloadImages(), config.GetWatchFrequency()
	if _, err := ResolveFiles(preloadImages); err != nil {
		return err
	}
	if config.GetResolveRef() != "" {
		log.Infof("the preload watcher doesn't apply the resolve ref %q to the files it loads", config.GetResolveRef())
	}
	w = watcher{
		preloadImages: preloadImages,
		ledger:        OpenLedger(config.GetImagePath()),
		pending:       map[string]fileVersion{}}

	log.Infof("starting the preload watcher: %q every %s", preloadImages, interval)

	go func() {
		for {
			select {
			case <-stopWatchCh:
				watchStoppedCh <- true
				return
			case <-time.After(interval):
				if oneWatchPass(stopWatchCh) {
					watchStoppedCh <- true
					return
				}
			}
		}
	}()
	return nil
}

// oneWatchPass makes one pass over the files resolved from the watched path in
// var 'w', loading each one that is not in the ledger and unchanged since the
// previous pass, and recording each result in the ledger. It checks for a stop
// request on the passed channel between files, and returns true if stopped.
func oneWatchPass(stopWatchCh chan bool) bool {
	files, err := ResolveFiles(w.preloadImages)
	if err != nil {
		log.Errorf("preload watcher: error resolving %q: %s", w.preloadImages, err)
		return false
	}
	if err := w.ledger.RemoveMissing(); err != nil {
		log.Errorf("preload watcher: error updating the preload ledger: %s", err)
	}
	log.Debugf("beginning watch pass with %d files on %q", len(files), w.preloadImages)
	seen := map[string]bool{}
	for _, f := range files {
		select {
		case <-stopWatchCh:
			return true
		default:
		}
		fi, err := os.Stat(f.Path)
		if err != nil || w.ledger.HasEntry(f.Path, fi) {
			continue
		}
		seen[f.Path] = true
		v := fileVersion{
			size:  fi.Size(),
			mtime: fi.ModTime(),
		}
		if prev, ok := w.pending[f.Path]; !ok || prev.size != v.size || !prev.mtime.Equal(v.mtime) {
			// new or still changing: wait for the next pass
			w.pending[f.Path] = v
			continue
		}
		if f.Err != nil {
			// not ledgered, so it's tried on every pass: fixing it (e.g. its permissions)
			// doesn't change its size or modification time
			log.Errorf("preload watcher: %s", f.Err)
			continue
		}
		var lr LedgerResult
		record := true
		pullTimeout := time.Duration(config.GetPullTimeout()) * time.Millisecond
		if f.Tarball {
			lr = loadTarballW(f.Path, pullTimeout)
		} else {
			lr, record = loadListFileW(f.Path, pullTimeout)
		}
		if !record {
			// left in pending, so it's tried again on the next pass
			continue
		}
		delete(w.pending, f.Path)

		if err := w.ledger.Record(f.Path, fi, lr); err != nil {
			log.Errorf("preload watcher: error updating the preload ledger: %s", err)
		}
	}
	for p := range w.pending {
		if !seen[p] {
			// file was removed from the watched path
			delete(w.pending, p)
		}
	}
	log.Debugf("finished watch pass - processed %d new files", len(seen))
	return false
}

// loadTarballW delegates to `cache.LoadTarball` to load the tarball in `f` into
// the running server's cache, then interprets results, logs, and returns the
// result to the caller.
func loadTarballW(path string, pullTimeout time.Duration) LedgerResult {
	results, err := cache.LoadTarball(path, "", config.GetImagePath(), config.GetOs(), config.GetArch(), pullTimeout)
	if err != nil {
		log.Errorf("preload watcher: error loading %q (it won't be loaded again until it changes): %s", path, err)
		return ResultFailed
	}
	counts := map[cache.TarballOutcome]int{}
	for _, r := range results {
		counts[r.Outcome]++
	}
	log.Infof("preload watcher: %q: %d image(s) loaded, %d already cached, %d error(s)", path,
		counts[cache.TarballLoaded], counts[cache.TarballAlreadyCached], counts[cache.TarballError])
	switch {
	case counts[cache.TarballError] > 0:
		log.Errorf("preload watcher: %q had errors (it won't be loaded again until it changes)", path)
		return ResultFailed
	case counts[cache.TarballLoaded] > 0:
		return ResultLoaded
	}
	return ResultSkipped
}

// loadListFileW delegates to `loadImage` to load each image listed in the passed
// image list file into the running server's cache, then logs, and returns the
// result for the ledger and whether to record it. Every image is tried, as for
// the images in an archive: one that fails is logged, and the file's result is
// ResultFailed. A file that can't be opened isn't recorded, so it's tried again
// on the next pass, as for a file that can't be read when it's resolved. When
// air-gapped, a list file fails: its images can only come from the upstream.
func loadListFileW(path string, pullTimeout time.Duration) (LedgerResult, bool) {
	if config.GetAirGapped() {
		log.Errorf("preload watcher: %q: image list files can't be processed when the server is air-gapped (it won't be loaded again until it changes)", path)
		return ResultFailed, true
	}
	f, err := os.Open(path)
	if err != nil {
		log.Errorf("preload watcher: error opening %q (it will be tried again): %s", path, err)
		return ResultFailed, false
	}
	defer f.Close()
	pulled, errs := 0, 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		url := strings.TrimSpace(scanner.Text())
		if url == "" || strings.HasPrefix(url, "#") {
			continue
		}
		n, err := loadImage(url, pullTimeout)
		pulled += n
		if err != nil {
			log.Errorf("preload watcher: %q: error loading %q: %s", path, url, err)
			errs++
		}
	}
	if err := scanner.Err(); err != nil {
		// e.g. a line too long to be a url: the rest of the file isn't read
		log.Errorf("preload watcher: error reading %q: %s", path, err)
		errs++
	}
	log.Infof("preload watcher: %q: %d manifest(s) pulled, %d error(s)", path, pulled, errs)
	switch {
	case errs > 0:
		log.Errorf("preload watcher: %q had errors (it won't be loaded again until it changes)", path)
		return ResultFailed, true
	case pulled > 0:
		return ResultLoaded, true
	}
	return ResultSkipped, true
}

// loadImage loads the image identified by the passed url into the running
// server's cache, as preload does for the file system cache: the upstream is
// asked for the url's current digest (a HEAD request), and if the cache has the
// url with that digest nothing is done. Otherwise the url is pulled into the
// cache (cache.LoadFromUpstream), replacing a moved tag. For an image list, the
// same is done for the image for the server's platform, by its digest url. The
// function returns how many manifests were pulled as described next:
//
//	0 - Nothing loaded
//	1 - One manifest loaded (an image manifest)
//	2 - Two manifests loaded (image list, then image)
func loadImage(url string, pullTimeout time.Duration) (int, error) {
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
