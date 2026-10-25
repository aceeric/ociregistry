package cache

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/aceeric/ociregistry/impl/config"
	"github.com/aceeric/ociregistry/impl/globals"
	"github.com/aceeric/ociregistry/impl/helpers"
	"github.com/aceeric/ociregistry/impl/metrics"
	"github.com/aceeric/ociregistry/impl/pullrequest"
	"github.com/aceeric/ociregistry/impl/serialize"

	"github.com/aceeric/imgpull/pkg/imgpull"
	log "github.com/sirupsen/logrus"
)

// Type concurrentPulls handles the case where multiple goroutines request a manifest from
// an upstream concurrently. When that happens, the first-in goroutine will actually do
// the pull and all other goroutines will wait on the first puller. The thing to know is
// that pulling an image manifest *also* pulls the image blobs. This can be relatively
// time-consuming may use a lot of network bandwidth. This type avoids multiple goroutines
// pulling the same manifests (and hence blobs) from the upstream at the same time, thus
// more efficiently utilizing system resources.
type concurrentPulls struct {
	sync.Mutex
	pulls map[string]chan struct{}
}

// Type manifestCache is the in-mem representation of the manifest cache. The key of each
// map is a manifest URL like docker.io/calico/cni:v3.27.0 or docker.io/calico/cni@sha256:3163eabb....
// The content is a 'ManifestHolder'. Every manifest GET is also an UPDATE (of the last accessed
// timestamp) so a mutex is used. "Latest" manifests are stored separately from non-latest.
type manifestCache struct {
	sync.Mutex
	manifests map[string]imgpull.ManifestHolder
	latest    map[string]imgpull.ManifestHolder
}

// Type enqueueResult disambiguates the three-valued return from getting a manifest from cache:
//
//	cacheHit - found in cache
//	mustPull - not found - caller must pull
//	mustWait - not found - caller must wait for puller
type enqueueResult int

const (
	cacheHit enqueueResult = iota
	mustPull
	mustWait
)

// Type blobCache is the in-mem representation of the blob cache. The key is a digest, and the value
// is a ref count. The ref count is the number of image manifests that reference that particular
// blob. When a manifest is added the ref count is inc'd and when a manifest is removed the ref
// count is dec'd. This ref count is used to prune the blobs. A blob with no refs can be safely
// removed. Blobs are downloaded from upstreams infrequently, but pulled frequently, so reads
// vastly outnumber updates or deletes. Hence a RWMutex for a little better concurrency.
type blobCache struct {
	sync.RWMutex
	blobs map[string]int
}

var (
	// cp has pulls in progress. Will only have entries when a pull from an upstream is actively
	// ocurring otherwise empty. The key is a manifest url, and the value is a channel for parked
	// goroutines to be signaled on by a pulling goroutine when the pull is complete. The instant
	// any other goroutine concurrently requests the image, the channel is provided to that
	// goroutine to wait on.
	cp concurrentPulls = concurrentPulls{
		pulls: make(map[string]chan struct{}),
	}
	// mc is the manifest in-mem cache, keyed by url. When a manifest is pulled by tag
	// it is placed in the map twice for efficient retrieval - once by tag and a second
	// time by digest.
	mc manifestCache = manifestCache{
		manifests: map[string]imgpull.ManifestHolder{},
		latest:    map[string]imgpull.ManifestHolder{},
	}
	// bc is the blob cache, keyed by digest with a ref count of cached manifests that
	// reference each blob
	bc blobCache = blobCache{
		blobs: map[string]int{},
	}
	emptyManifestHolder = imgpull.ManifestHolder{}
	// pullGate keeps prune from deleting a blob that an upstream pull is relying on. A pull
	// holds it shared from before it checks the blob directory until its manifest is in the
	// cache. Prune holds it exclusively. Only sweepBlobs deletes blob files, and only with
	// pullGate held exclusively. Lock order: pullGate, then mc, then bc.
	pullGate sync.RWMutex
)

// GetManifest returns a manifest from the in-mem cache matching the URL of the passed PullRequest.
// If no manifest is cached then a pull is performed from an upstream OCI distribution server. The
// function blocks until the pull is complete and then the manifest is added to the in-mem cache and
// returned. An error is returned if the manifest can't be pulled or times out. If an image manifest
// is requested and it is not already cached, then all the blobs for the image will also be pulled
// from the upstream and added to the blob cache.
//
// If multiple goroutines request to pull the same image at the same time, then only the first goroutine
// will actually perform the pull, and all other goroutines will wait for the first goroutine to complete
// the pull and add the image to the cache. Then, the waiting goroutine(s) will simply get the manifest
// from the cache entry created by the first goroutine.
func GetManifest(pr pullrequest.PullRequest, imagePath string, pullTimeout int, forcePull bool) (imgpull.ManifestHolder, error) {
	url := pr.Url()
	mh, ch, er := getManifestOrEnqueue(pr, imagePath, forcePull)
	switch er {
	case cacheHit:
		log.Infof("serving manifest from cache: %q", url)
		metrics.IncCachedPullsByNs(pr.Remote)
		return mh, nil
	case mustPull:
		log.Infof("pulling manifest from upstream: %q", url)
		defer signalWaiters(url)
		mh, err := pullToCache(pr, imagePath, forcePull)
		if err != nil {
			return emptyManifestHolder, err
		}
		if forcePull {
			// a moved tag may have left blobs with no refs - remove them now if no other pull is
			// in progress, rather than waiting for the next prune
			trySweepBlobs(imagePath)
		}
		return mh, nil
	case mustWait:
		select {
		case <-ch:
			log.Infof("serving manifest from cache (after wait): %q", url)
			metrics.IncCachedPullsByNs(pr.Remote)
			mh, exists := getManifestFromCache(pr, imagePath)
			if !exists {
				return emptyManifestHolder, fmt.Errorf("manifest not found (after wait) %q", url)
			}
			return mh, nil
		case <-time.After(time.Duration(pullTimeout) * time.Millisecond):
			return emptyManifestHolder, fmt.Errorf("timeout exceeded (%d millis) waiting for signal on %q", pullTimeout, url)
		}
	default:
		return emptyManifestHolder, fmt.Errorf("unknown enqueueResult %d from getManifestOrEnqueue", er)
	}
}

// pullToCache pulls the manifest (and its blobs) from the upstream and adds it to the cache.
// It holds pullGate shared throughout, so a prune can't remove a blob this pull found on disk
// before the manifest that references it is cached.
func pullToCache(pr pullrequest.PullRequest, imagePath string, forcePull bool) (imgpull.ManifestHolder, error) {
	pullGate.RLock()
	defer pullGate.RUnlock()
	mh, err := doPull(pr, imagePath)
	if err != nil {
		log.Errorf("doPull failed for %q: %s", pr.Url(), err)
		return emptyManifestHolder, err
	}
	if forcePull {
		err = replaceInCache(pr, mh, imagePath)
	} else {
		err = addToCache(pr, mh, imagePath)
	}
	if err != nil {
		return emptyManifestHolder, err
	}
	return mh, nil
}

// GetBlob returns the ref count for the passed blob digest. Zero means no cached manifest
// references the blob, so it should not be served: its file may still be on disk until the
// next sweep removes it.
func GetBlob(digest string) int {
	bc.RLock()
	defer bc.RUnlock()
	return bc.blobs[digest]
}

// doPull gets an image list manifest - or image manifest - from an upstream OCI distribution
// server. If an image manifest is pulled, then all the blobs for the image manifest are
// also pulled. On return, the blobs and the pulled manifest will have been serialized to the
// file system.
func doPull(pr pullrequest.PullRequest, imagePath string) (imgpull.ManifestHolder, error) {
	metrics.IncUpstreamPullsByNs(pr.Remote)
	opts, err := config.ConfigFor(pr.Remote)
	if err != nil {
		return emptyManifestHolder, err
	}
	opts.Url = pr.Url()
	puller, err := imgpull.NewPullerWith(opts)
	if err != nil {
		return emptyManifestHolder, err
	}
	defer puller.Close()
	mh, err := puller.GetManifest()
	if err != nil {
		return emptyManifestHolder, err
	}
	if mh.IsImageManifest() {
		if err := puller.PullBlobs(mh, filepath.Join(imagePath, globals.BlobPath)); err != nil {
			return emptyManifestHolder, err
		}
	}
	mh.Created = globals.CurTime()
	mh.Pulled = mh.Created
	if err := serialize.MhToFilesystem(mh, imagePath, true); err != nil {
		return emptyManifestHolder, err
	}
	return mh, nil
}

// Load copies all the manifests and blobs from the file system into the two in-memory
// caches - mc (manifests), and bc (blobs.) The manifests are loaded in their entirety. For
// the blobs, only the digests are loaded with a ref count indicating the number of
// manifests that ref each blob. This function performs a consistency check: if any blobs
// associated with a manifest are not present on the file system then the function will
// not load the manifest into cache. The manifest technically does not exist then from
// the perspective of a client. (A re-pull will heal that.)
func Load(imagePath string) error {
	start := time.Now()
	log.Infof("load in-mem cache from file system")
	if err := serialize.RemoveTempFiles(imagePath); err != nil {
		return err
	}
	itemcnt := 0
	err := serialize.WalkTheCache(imagePath, func(mh imgpull.ManifestHolder, fi os.FileInfo) error {
		pr, err := pullrequest.NewPullRequestFromUrl(mh.ImageUrl)
		if err != nil {
			log.Errorf("load: manifest url %q can't be parsed - will not be cached: %s", mh.ImageUrl, err)
			return nil
		}
		log.Debugf("loading manifest for %s", mh.ImageUrl)
		if err := addToCache(pr, mh, imagePath); err != nil {
			log.Errorf("load: %s - will not be cached", err)
			return nil
		}
		metrics.DeltaManifestBytesOnDisk(float64(fi.Size()))
		itemcnt++
		return nil
	})
	log.Infof("loaded %d manifest(s) from the file system in %s", itemcnt, time.Since(start))
	return err
}

// WaitPulls waits 60 seconds for any in-progress pulls to complete and then
// returns. If a pull in progress is partially complete (some of the blobs are written
// and some aren't) then the cache will be in an inconsistent state. When the server
// is restarted - it will detect this and exclude any such manifests from being
// loaded into the in-mem cache.
func WaitPulls() {
	log.Info("checking for pulls in progress")
	if pullsInProgress() == 0 {
		log.Info("no pulls in progress")
		return
	}
	log.Info("waiting for pulls in progress to complete")
	start := time.Now()
	for {
		time.Sleep(time.Second * 2)
		if pullsInProgress() == 0 {
			break
		}
		if time.Since(start) > time.Minute {
			log.Error("timeout waiting for pulls in progress to complete")
		}
	}
	log.Info("in-progress pulls have completed")
}

// IsCached checks if the manifest is cached to support efficiently handle air-gapped
// sites. If the manifest is cached, true is returned, else false.
func IsCached(pr pullrequest.PullRequest) bool {
	mc.Lock()
	defer mc.Unlock()
	_, exists := fromCache(pr.Url())
	return exists
}

// ResetCache supports unit tests
func ResetCache() {
	cp = concurrentPulls{
		pulls: make(map[string]chan struct{}),
	}
	mc = manifestCache{
		manifests: map[string]imgpull.ManifestHolder{},
		latest:    map[string]imgpull.ManifestHolder{},
	}
	bc = blobCache{
		blobs: map[string]int{},
	}
}

// allManifests is an iterator over the in-mem manifest cache. It first returns non-latest
// manifests, then returns latest manifests.
func (mc *manifestCache) allManifests(yield func(string, imgpull.ManifestHolder) bool) {
	for url, mh := range mc.manifests {
		if !yield(url, mh) {
			return
		}
	}
	for url, mh := range mc.latest {
		if !yield(url, mh) {
			return
		}
	}
}

// len returns the number of cached manifests.
func (mc *manifestCache) len() int {
	return len(mc.manifests) + len(mc.latest)
}

// checkBlobs verifies all blobs for a manifest exist and have correct size. Returns true
// if so, else false. Manifest lists have no layers so always pass.
func checkBlobs(mh imgpull.ManifestHolder, imagePath string) bool {
	ok := true
	for _, layer := range mh.Layers() {
		digest := helpers.GetDigestFrom(layer.Digest)
		if exists, size := serialize.BlobExists(imagePath, digest); !exists {
			ok = false
			log.Errorf("manifest %q: blob %q not found", mh.ImageUrl, digest)
		} else if size != layer.Size {
			ok = false
			log.Errorf("manifest %q: blob %q: expected %d bytes, found %d", mh.ImageUrl, digest, layer.Size, size)
		}
	}
	return ok
}

// pullsInProgress returns the count of in-progress pulls from any upstream
// OCI distribution server.
func pullsInProgress() int {
	cp.Lock()
	defer cp.Unlock()
	return len(cp.pulls)
}

// addToCache adds the passed ManifestHolder to the in-mem manifest cache and, for an image
// manifest, increments the ref counts of its blobs. Every blob must already be on the file
// system with the expected size; otherwise an error is returned and the cache is not changed.
func addToCache(pr pullrequest.PullRequest, mh imgpull.ManifestHolder, imagePath string) error {
	mc.Lock()
	defer mc.Unlock()
	bc.Lock()
	defer bc.Unlock()
	if !checkBlobs(mh, imagePath) {
		return fmt.Errorf("unable to add manifest %q to cache: blob check failed (see logs)", mh.ImageUrl)
	}
	addManifestToCache(pr, mh)
	addBlobsToCache(mh)
	return nil
}

// replaceInCache replaces the cached manifest for the passed URL with mhNew, e.g. when a mutable tag moves.
// A same-digest replace is a no-op. The blobs of mhNew must already be on the file system with the expected
// sizes. New blob refs are added before old ones are removed so shared blobs never reach zero. Blobs left
// with no refs keep their files until sweepBlobs runs.
func replaceInCache(pr pullrequest.PullRequest, mhNew imgpull.ManifestHolder, imagePath string) error {
	mc.Lock()
	defer mc.Unlock()
	bc.Lock()
	defer bc.Unlock()
	mhExisting, exists := fromCache(pr.Url())
	if exists && mhExisting.Digest == mhNew.Digest {
		log.Debugf("replace manifest %s has same digest - nop", pr.Url())
		return nil
	}
	if !checkBlobs(mhNew, imagePath) {
		return fmt.Errorf("unable to replace manifest %q in cache: blob check failed (see logs)", mhNew.ImageUrl)
	}
	if exists {
		log.Debugf("replace manifest %s, old digest %s, new digest %s", pr.Url(), mhExisting.Digest, mhNew.Digest)
		rmManifest(mhExisting, imagePath)
	}
	addManifestToCache(pr, mhNew)
	addBlobsToCache(mhNew) // add new refs before removing old, so shared blobs never reach zero
	if exists {
		rmBlobs(mhExisting)
	}
	return nil
}

// addManifestToCache adds the passed manifest to the in-mem manifest map, keyed by
// the passed URL. If the passed manifest was pulled by tag, then a second entry
// is added to cache keyed by digest. This enables the cache to serve manifest requests
// by tag and by digest for the same manifest.
func addManifestToCache(pr pullrequest.PullRequest, mh imgpull.ManifestHolder) {
	if pr.IsLatest() {
		metrics.DeltaCachedManifestCount(2)
		mc.latest[pr.Url()] = mh
		// IsLatest means it has tag "latest"
		mc.latest[pr.UrlWithDigest("sha256:"+mh.Digest)] = mh
	} else {
		metrics.DeltaCachedManifestCount(1)
		mc.manifests[pr.Url()] = mh
		if pr.PullType == pullrequest.ByTag {
			metrics.DeltaCachedManifestCount(1)
			mc.manifests[pr.UrlWithDigest("sha256:"+mh.Digest)] = mh
		}
	}
}

// addBlobsToCache increments the ref count of each blob (layers and config) in the passed
// manifest. The caller must have verified the blobs with checkBlobs and must hold bc.
func addBlobsToCache(mh imgpull.ManifestHolder) {
	for _, layer := range mh.Layers() {
		digest := helpers.GetDigestFrom(layer.Digest)
		bc.blobs[digest]++
		if bc.blobs[digest] == 1 {
			metrics.DeltaBlobBytesOnDisk(float64(layer.Size))
			metrics.DeltaCachedBlobCount(1)
		}
	}
}

// getManifestOrEnqueue looks in the in-mem manifest cache for the passed manifest URL. If found,
// then the manifest holder is returned. If not in cache, then the function enqueues a pull for
// the manifest from the upstream. In that case, then the return values are to be handled by the
// caller based on the value of enqueueResult:
//
// Manifest in cache:
//
//	enqueueResult: cacheHit
//	ManifestHolder: The manifest
//	channel: ignored
//
// Manifest NOT in cache, and NO other goroutine is already pulling from the upstream
//
//	enqueueResult: mustPull
//	ManifestHolder: ignored
//	channel: Caller must pull and then signal waiting goroutines on this channel after pull
//
// Manifest NOT in cache, and another goroutine IS already pulling from the upstream
//
//	enqueueResult: mustWait
//	ManifestHolder: ignored
//	channel: Caller will be signaled on this channel when the pulling goroutine finishes
//
// As a final bit of complexity, if forcePull is true then the manifest is always enqueued.
// In this case the server acts like a simple proxy meaning it will always pull a manifest
// from the upstream.
func getManifestOrEnqueue(pr pullrequest.PullRequest, imagePath string, forcePull bool) (imgpull.ManifestHolder, chan struct{}, enqueueResult) {
	if !forcePull {
		if mh, exists := getManifestFromCache(pr, imagePath); exists {
			return mh, nil, cacheHit
		}
	}
	ch, er := enqueuePull(pr)
	return emptyManifestHolder, ch, er
}

// fromCache is a low-level function that checks the non-latest in-mem cache and the
// latest in-mem cache for the passed image.
func fromCache(url string) (imgpull.ManifestHolder, bool) {
	mh, exists := mc.manifests[url]
	if exists {
		return mh, true
	}
	mh, exists = mc.latest[url]
	return mh, exists
}

// getManifestFromCache gets a manifest from the in-mem manifest cache, or returns
// an empty manifest holder if the manifest for the passed URL is not cached. If the
// manifest exists, the 'Pulled' field is update to reflect the current time and the
// manifest is written back to the file system.
func getManifestFromCache(pr pullrequest.PullRequest, imagePath string) (imgpull.ManifestHolder, bool) {
	mc.Lock()
	defer mc.Unlock()
	url := pr.Url()
	var mh imgpull.ManifestHolder
	var exists bool
	if mh, exists = fromCache(pr.Url()); !exists {
		if url = pr.AltDockerUrl(); url != "" {
			mh, exists = fromCache(url)
		}
	}
	if exists {
		mh.Pulled = globals.CurTime()
		if pr.IsLatest() {
			mc.latest[url] = mh
		} else {
			mc.manifests[url] = mh

		}
		if err := serialize.MhToFilesystem(mh, imagePath, true); err != nil {
			log.Errorf("error serializing manifest %q, the error was: %s", url, err)
			return emptyManifestHolder, false
		}
		return mh, true
	}
	return emptyManifestHolder, false
}

// enqueuePull returns the channel for the url and whether the caller is the puller (mustPull) or
// the waiter (mustWait) via enqueueResult.
func enqueuePull(pr pullrequest.PullRequest) (chan struct{}, enqueueResult) {
	cp.Lock()
	defer cp.Unlock()
	url := pr.Url()
	if ch, exists := cp.pulls[url]; exists {
		return ch, mustWait
	}
	ch := make(chan struct{})
	cp.pulls[url] = ch
	return ch, mustPull
}

// signalWaiters signals any goroutines waiting on the passed url to be pulled. (Which could
// be none.) Closing the channel simultaneously unblocks all waiters.
func signalWaiters(url string) {
	cp.Lock()
	defer cp.Unlock()
	if ch, exists := cp.pulls[url]; exists {
		close(ch)
		delete(cp.pulls, url)
	}
}
