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
	"github.com/aceeric/ociregistry/impl/pullrequest"
	"github.com/aceeric/ociregistry/impl/serialize"

	"github.com/aceeric/imgpull/pkg/imgpull"
	log "github.com/sirupsen/logrus"
)

// dateFormat has magic numbers from 'format.go' in package 'time' that
// support date parsing
const dateFormat = "2006-01-02T15:04:05"

// Type concurrentPulls handles the case where multiple goroutines request a manifest from
// an upstream concurrently. When that happens, the first-in goroutine will actually do
// the pull and all other goroutines will wait on the first puller. The thing to know is
// that pulling an image manifest *also* pulls the image blobs. This can be relatively
// time-consuming may use a lot of network bandwidth. This type avoids multiple goroutines
// pulling the same manifests (and hence blobs) from the upstream at the same time, thus
// more efficiently utilizing system resources.
type concurrentPulls struct {
	sync.Mutex
	pulls map[string][]chan bool
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
	// ocurring otherwise empty. The key is a manifest url, and the value is a map of channels for
	// parked goroutines waiting for the image to be pulled by a pulling goroutine. If only the
	// pulling goroutine is running then the map of channels is empty. The instant any other goroutine
	// concurrently requests the image, a channel is created for that goroutine and added to the map
	// and each parked goroutine will wait to be signaled on its channel.
	cp concurrentPulls = concurrentPulls{
		pulls: make(map[string][]chan bool),
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
)

// GetManifest returns a manifest from the in-mem cache matching the URL of the passed PullRequest.
// If no manifest is cached then a pull is performed from an upstream OCI distribution server. The
// function blocks until the pull is complete and then the manifest is added to the in-mem cache and
// returned. An error is returned if the manifest can't be pulled or times out. If an *image* manifest
// is requested and it is not already cached, then all the blobs for the image will also be pulled from
// the upstream and added to the blob cache.
//
// If multiple goroutines request to pull the same image at the same time, then only the first goroutine
// will actually perform the pull, and all other goroutines will wait for the first goroutine to complete
// the pull and add the image to the cache. Then, the waiting goroutine(s) will simply get the manifest
// from the cache entry created by the first goroutine.
func GetManifest(pr pullrequest.PullRequest, imagePath string, pullTimeout int, forcePull bool) (imgpull.ManifestHolder, error) {
	url := pr.Url()
	if mh, ch, exists := getManifestOrEnqueue(pr, imagePath, forcePull); exists {
		log.Infof("serving manifest from cache: %q", url)
		return mh, nil
	} else if ch == nil {
		log.Infof("pulling manifest from upstream: %q", url)
		defer signalWaiters(url)
		mh, err := DoPull(pr, imagePath)
		if err != nil {
			log.Errorf("doPull failed for %q: %s", url, err)
			return emptyManifestHolder, err
		}
		if forcePull {
			if err := replaceInCache(pr, mh, imagePath); err != nil {
				return emptyManifestHolder, err
			}
		} else {
			if err := addToCache(pr, mh, imagePath); err != nil {
				return emptyManifestHolder, err
			}
		}
		return mh, nil
	} else {
		select {
		case <-ch:
			log.Infof("serving manifest from cache (after wait): %q", url)
			mh, exists := getManifestFromCache(pr, imagePath)
			if !exists {
				return emptyManifestHolder, fmt.Errorf("manifest not found (after wait) %q", url)
			}
			return mh, nil
		case <-time.After(time.Duration(pullTimeout) * time.Millisecond):
			return emptyManifestHolder, fmt.Errorf("timeout exceeded (%d millis) waiting for signal on %q", pullTimeout, url)
		}
	}
}

// GetBlob returns the ref count for the passed blob digest. If zero then the blob is not referenced
// by any cached manifests. This should never happen because when a manifest is pruned, any blobs
// ref'd by the deleted manifest that decrement to zero should be removed while the blob cache is
// locked. In other words if this ever returns zero, that indicates a defect in the code.
func GetBlob(digest string) int {
	bc.RLock()
	defer bc.RUnlock()
	return bc.blobs[digest]
}

// DoPull gets an image list manifest - or image manifest - from an upstream OCI distribution
// server. If an image manifest is pulled, then all the blobs for the image manifest are
// also pulled. On return, the pulled manifest will have been serialized to the file system by
// the function (along with blobs, if an image manifest.)
func DoPull(pr pullrequest.PullRequest, imagePath string) (imgpull.ManifestHolder, error) {
	opts, err := config.ConfigFor(pr.Remote)
	if err != nil {
		return emptyManifestHolder, err
	}
	opts.Url = pr.Url()
	puller, err := imgpull.NewPullerWith(opts)
	if err != nil {
		return emptyManifestHolder, err
	}
	mh, err := puller.GetManifest()
	if err != nil {
		return emptyManifestHolder, err
	}
	mh.Created = curTime()
	mh.Pulled = curTime()
	if err := serialize.MhToFilesystem(mh, imagePath, true); err != nil {
		return emptyManifestHolder, err
	}
	if mh.IsImageManifest() {
		blobDir := filepath.Join(imagePath, globals.BlobPath)
		err = puller.PullBlobs(mh, blobDir)
		if err != nil {
			log.Error(err)
			return emptyManifestHolder, err
		}
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
	itemcnt := 0
	var outerErr error
	serialize.WalkTheCache(imagePath, func(mh imgpull.ManifestHolder, _ os.FileInfo) error {
		pr, err := pullrequest.NewPullRequestFromUrl(mh.ImageUrl)
		if err != nil {
			outerErr = err
			return err
		}
		log.Debugf("loading manifest for %s", mh.ImageUrl)
		if canAdd(mh, imagePath) {
			addToCache(pr, mh, imagePath)
			itemcnt++
		} else {
			log.Errorf("load: manifest %q missing blobs - will not be cached", mh.ImageUrl)
		}
		return nil
	})
	log.Infof("loaded %d manifest(s) from the file system in %s", itemcnt, time.Since(start))
	return outerErr
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
		pulls: make(map[string][]chan bool),
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

// canAdd checks to see if all the blobs referenced by the passed manifest exist on the file
// system (and can therefore be served.) If all blobs exist then true is returned, else
// false.
func canAdd(mh imgpull.ManifestHolder, imagePath string) bool {
	canAdd := true
	for _, layer := range mh.Layers() {
		digest := helpers.GetDigestFrom(layer.Digest)
		if !serialize.BlobExists(imagePath, digest) {
			canAdd = false
			// don't break - display all the errors
			log.Debugf("load: blob %q referenced by manifest %q not found on the filesystem", digest, mh.ImageUrl)
		}
	}
	return canAdd
}

// pullsInProgress returns the count of in-progress pulls from any upstream
// OCI distribution server.
func pullsInProgress() int {
	cp.Lock()
	defer cp.Unlock()
	return len(cp.pulls)
}

// addToCache adds the passed ManifestHolder to the in-mem manifest cache. If the manifest is
// an image manifest then the blobs are added to the in-mem blob cache. The manifest is expected
// to already exist on the file system before this function is called.
func addToCache(pr pullrequest.PullRequest, mh imgpull.ManifestHolder, imagePath string) error {
	mc.Lock()
	defer mc.Unlock()
	addManifestToCache(pr, mh)
	if mh.IsImageManifest() {
		bc.Lock()
		defer bc.Unlock()
		if err := addBlobsToCache(mh, imagePath); err != nil {
			return err
		}
	}
	return nil
}

// replaceInCache handles the case where the system is configured to "always pull latest", meaning
// that when a ":latest" tag is pulled, the server always goes to the upstream to (re)pull the image.
// By the time this function is called, the passed 'mhNew' manifest is already written to the file system.
//
// If it is an image manifest, then the new image blobs will already have been downloaded and placed
// on the file system as well. The old manifest and the new manifest may have overlapping blobs. The
// function handles that case by only removing blobs that were in the old manifest and not in the new
// manifest, and only adding blobs that are in the new manifest that were not in the old manifest.
//
// The blob adds only happen in the in-mem blob cache because the actual blobs have already been downloaded
// But the deletes happen in both the in-mem blob cache _and_ on the file system.
func replaceInCache(pr pullrequest.PullRequest, mhNew imgpull.ManifestHolder, imagePath string) error {
	mc.Lock()
	defer mc.Unlock()
	// get the existing manifest from cache matching the new manifest url
	mhExisting, exists := fromCache(pr.Url())
	if !exists {
		// same logic as addToCache above
		addManifestToCache(pr, mhNew)
		if mhNew.IsImageManifest() {
			bc.Lock()
			defer bc.Unlock()
			if err := addBlobsToCache(mhNew, imagePath); err != nil {
				return err
			}
		}
		return nil
	}
	if mhExisting.Digest == mhNew.Digest {
		// same digest means same manifest: nothing to do
		log.Debugf("replace manifest %s has same digest - nop", pr.Url())
		return nil
	}
	log.Debugf("replace manifest %s, old digest %s, new digest %s", pr.Url(), mhExisting.Digest, mhNew.Digest)
	rmManifest(mhExisting, imagePath)
	addManifestToCache(pr, mhNew)
	if mhNew.IsImageManifest() || mhExisting.IsImageManifest() {
		bc.Lock()
		defer bc.Unlock()
		if mhNew.IsImageManifest() {
			addBlobsToCache(mhNew, imagePath)
		}
		if mhExisting.IsImageManifest() {
			rmBlobs(mhExisting, imagePath)
		}
	}
	return nil
}

// addManifestToCache adds the passed manifest to the in-mem manifest map, keyed by
// the passed URL. If the passed manifest was pulled by tag, then a second entry
// is added to cache keyed by digest. This enables the cache to serve manifest requests
// by tag and by digest for the same manifest.
func addManifestToCache(pr pullrequest.PullRequest, mh imgpull.ManifestHolder) {
	if pr.IsLatest() {
		mc.latest[pr.Url()] = mh
		// IsLatest means it has tag "latest"
		mc.latest[pr.UrlWithDigest("sha256:"+mh.Digest)] = mh
	} else {
		mc.manifests[pr.Url()] = mh
		if pr.PullType == pullrequest.ByTag {
			mc.manifests[pr.UrlWithDigest("sha256:"+mh.Digest)] = mh
		}
	}
}

// addBlobsToCache adds entries to the in-mem blob map and/or increments the ref count
// for existing blobs in the blob map based on the layers (and the config blob) in the
// passed manifest. The blobs are expected to already exist on the file system before
// this function is called.
func addBlobsToCache(mh imgpull.ManifestHolder, imagePath string) error {
	for _, layer := range mh.Layers() {
		digest := helpers.GetDigestFrom(layer.Digest)
		// if not in the map, is added
		bc.blobs[digest]++
		if !serialize.BlobExists(imagePath, digest) {
			return fmt.Errorf("blob %q referenced by manifest %q not found on the filesystem", digest, mh.ImageUrl)
		}
	}
	return nil
}

// getManifestOrEnqueue looks in the in-mem manifest cache for the passed manifest URL. If found,
// then the manifest holder is returned. If not in cache, then the function enqueues a pull for
// the manifest from the upstream. In that case, then the return values are to be handled by the
// caller in specific ways as follows:
//
// 1) If the current goroutine is the first to enqueue a pull, then a nil channel is returned. This
// means the caller must pull the image and signal any other goroutines waiting for the pull
// to complete.
//
// 2) If a non-nil channel is returned, it means another goroutine is already doing the pull for the
// requested url so the caller in *this* goroutine should wait to be signaled on the non-nil
// channel. (The pulling go routine will signal all waiters when the image has been added to cache.)
//
// In summary, the return values from the function are interpreted as follows:
//
// Manifest in cache:
//
//	ManifestHolder: The manifest
//	channel: nil
//	bool: true
//
// Manifest NOT in cache, and NO other goroutine is already pulling from the upstream
//
//	ManifestHolder: Empty
//	channel: nil - caller must pull and then signal waiting goroutines after pull
//	bool: false
//
// Manifest NOT in cache, and another goroutine IS already pulling from the upstream
//
//	ManifestHolder: Empty
//	channel: non-nil - this channel will be signaled when the pulling goroutine finishes
//	bool: false
//
// As a final bit of complexity, if forcePull is true then the manifest is always enqueued.
// In this case the server acts like a simple proxy meaning it will always pull from the
// upstream.
func getManifestOrEnqueue(pr pullrequest.PullRequest, imagePath string, forcePull bool) (imgpull.ManifestHolder, chan bool, bool) {
	if !forcePull {
		if mh, exists := getManifestFromCache(pr, imagePath); exists {
			return mh, nil, true
		}
	}
	return emptyManifestHolder, enqueuePull(pr), false
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
		mh.Pulled = curTime()
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

// enqueuePull enqueues a pull request from the upstream. A return value of nil means
// the pull request has not already been enqueued by another goroutine. Non-nil means another
// goroutine HAS already enqueued the pull and the caller must wait on the returned
// channel to be signalled when the pull completes by the pulling goroutine.
func enqueuePull(pr pullrequest.PullRequest) chan bool {
	cp.Lock()
	defer cp.Unlock()
	url := pr.Url()
	if chans, exists := cp.pulls[url]; exists {
		ch := make(chan bool)
		cp.pulls[url] = append(chans, ch)
		return ch
	}
	cp.pulls[url] = []chan bool{}
	return nil
}

// signalWaiters signals any goroutines waiting on the passed url to be pulled. (Which could
// be none.)
func signalWaiters(url string) {
	cp.Lock()
	defer cp.Unlock()
	if chans, exists := cp.pulls[url]; exists {
		for _, ch := range chans {
			defer func() {
				if err := recover(); err != nil {
					fmt.Printf("attempt to write to closed channel for url %q", url)
				}
			}()
			ch <- true
		}
		delete(cp.pulls, url)
	}
}

// curTime gets the current time as YYYY-MM-DDTHH:MM:SS
func curTime() string {
	return time.Now().Format(dateFormat)
}
