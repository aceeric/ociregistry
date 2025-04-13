package cache

import (
	"fmt"
	"ociregistry/impl/config"
	"ociregistry/impl/globals"
	"ociregistry/impl/helpers"
	"ociregistry/impl/pullrequest"
	"ociregistry/impl/serialize"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/aceeric/imgpull/pkg/imgpull"
	log "github.com/sirupsen/logrus"
)

// magic numbers from 'format.go' in package 'time'
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

// Type manifestCache is the in-mem representation of the manifest cache. The key is a manifest
// URL like docker.io/calico/cni:v3.27.0 or docker.io/calico/cni@sha256:3163eabb.... The content
// is a 'ManifestHolder'. Every manifest GET is also an UPDATE (of the last accessed timestamp)
// so a mutex is used.
type manifestCache struct {
	sync.Mutex
	manifests map[string]imgpull.ManifestHolder
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
	// cp has concurrent pulls in progress. 99.999% of the time this will be empty.
	cp concurrentPulls = concurrentPulls{
		pulls: make(map[string][]chan bool),
	}
	// mc is the manifest in-mem cache, keyed by url. When a manifest is pulled by tag
	// it is placed in the map twice for efficient retrieval - once by tag and a second
	// time by digest.
	mc manifestCache = manifestCache{
		manifests: map[string]imgpull.ManifestHolder{},
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
	if mh, ch, exists := getManifestOrEnqueue(url, imagePath, forcePull); exists {
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
			// remove the old because it will be replaced by the new
			prune(mh, imagePath)
		}
		addToCache(pr, mh, imagePath, true)
		return mh, nil
	} else {
		select {
		case <-ch:
			log.Infof("serving manifest from cache (after wait): %q", url)
			mh, exists := getManifestFromCache(url, imagePath)
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
// by any cached manifests and therefore technically does not exist and and will eventually be pruned.
// If no map entry then zero is returned.
func GetBlob(digest string) int {
	bc.RLock()
	defer bc.RUnlock()
	return bc.blobs[digest]
}

// DoPull gets an image list manifest - or image manifest - from an upstream OCI distribution
// server. If an image manifest is pulled, then all the blobs for the image manifest are
// also pulled.
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
	serialize.MhToFilesystem(mh, imagePath, true)
	if mh.IsImageManifest() {
		blobDir := filepath.Join(imagePath, globals.BlobsDir)
		err = puller.PullBlobs(mh, blobDir)
		if err != nil {
			log.Error(err)
			return emptyManifestHolder, err
		}
	}
	return mh, nil
}

// Load copies all the manifests and blobs into the two in-memory caches - mc.manifests,
// and bc.blobs. The manifests are loaded in their entirety. For the blobs, only the digests
// are loaded with a ref count indicating the number of manifests that ref each blob.
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
		addToCache(pr, mh, imagePath, false)
		itemcnt++
		return nil
	})
	log.Infof("loaded %d manifest(s) from the file system in %s", itemcnt, time.Since(start))
	return outerErr
}

// addToCache adds the passed ManifestHolder to the manifest cache. If the manifest is an image
// manifest then the blobs are added to the blob cache.
func addToCache(pr pullrequest.PullRequest, mh imgpull.ManifestHolder, imagePath string, readOnly bool) {
	mc.Lock()
	defer mc.Unlock()
	if !readOnly {
		mh.Created = curTime()
	}
	addManifestToCache(pr, mh)
	if mh.IsImageManifest() {
		bc.Lock()
		defer bc.Unlock()
		addBlobsToCache(mh, imagePath)
	}
}

// addManifestToCache adds the passed manifest to the in-mem manifest map, keyed by
// the passed URL. If the passed manifest was pulled by tag, then a second entry
// is added to cache keyed by digest. This enables the cache to serve manifest requests
// by tag and by digest for the same manifest.
func addManifestToCache(pr pullrequest.PullRequest, mh imgpull.ManifestHolder) {
	mc.manifests[pr.Url()] = mh
	if pr.PullType == pullrequest.ByTag {
		mc.manifests[pr.UrlWithDigest("sha256:"+mh.Digest)] = mh
	}
}

// addBlobsToCache adds entries to the in-mem blob map and/or increments the ref count
// for existing blobs in the blob map based on the layers (and the config blob) in the
// passed manifest.
func addBlobsToCache(mh imgpull.ManifestHolder, imagePath string) {
	for _, layer := range mh.Layers() {
		digest := helpers.GetDigestFrom(layer.Digest)
		// if not in the map, is added
		bc.blobs[digest]++
		if !serialize.BlobExists(imagePath, digest) {
			log.Errorf("blob %q referenced by manifest %q not found on the filesystem", digest, mh.ImageUrl)
		}
	}
}

// getManifestOrEnqueue looks in the in-mem manifest cache for the passed manifest URL. If found,
// then the manifest holder is returned. If not in cache, then the function enqueues a pull for
// the manifest from the upstream. In that case, then the return values are to be handled in
// specific ways as follows:
//
// 1) If the current goroutine is the first to enqueue a pull, then a nil channel is returned. This
// means the current goroutine must pull the image and signal any other goroutines waiting for the
// pull to complete.
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
func getManifestOrEnqueue(url string, imagePath string, forcePull bool) (imgpull.ManifestHolder, chan bool, bool) {
	if !forcePull {
		if mh, exists := getManifestFromCache(url, imagePath); exists {
			return mh, nil, true
		}
	}
	return emptyManifestHolder, enqueuePull(url), false
}

// IsCached checks if the manifest is cached to support efficiently handle air-gapped
// sites. If the manifest is cached, true is returned, else false.
func IsCached(pr pullrequest.PullRequest) bool {
	mc.Lock()
	defer mc.Unlock()
	_, exists := mc.manifests[pr.Url()]
	return exists
}

// getManifestFromCache gets a manifest from the in-mem manifest cache, or returns
// an empty manifest holder if the manifest for the passed URL is not cached. If the
// manifest exists, the 'Pulled' field is update to reflect the current time and the
// manifest is written back to the file system.
func getManifestFromCache(url string, imagePath string) (imgpull.ManifestHolder, bool) {
	mc.Lock()
	defer mc.Unlock()
	if mh, exists := mc.manifests[url]; exists {
		mh.Pulled = curTime()
		serialize.MhToFilesystem(mh, imagePath, true)
		return mh, exists
	}
	return emptyManifestHolder, false
}

// enqueuePull enqueues a pull request from the upstream. A return value of nil means
// the pull request has not already been enqueued by another goroutine. Non-nil means another
// goroutine HAS already enqueued the pull and the caller must wait on the returned
// channel to be signalled when the pull completes by the pulling goroutine.
func enqueuePull(url string) chan bool {
	cp.Lock()
	defer cp.Unlock()
	if chans, exists := cp.pulls[url]; exists {
		ch := make(chan bool)
		cp.pulls[url] = append(chans, ch)
		return ch
	} else {
		cp.pulls[url] = []chan bool{}
	}
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
