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

// TODO HANDLE ALWAYS PULL LATEST
// if tag is latest and cached
//   head upstream
//   if digest matches
//     return from cache
//   pull
//   delete existing manifest (for blob ref counts)
//   add new manifest

// Type concurrentPulls handles the case where multiple goroutines might request a manifest
// from an upstream concurrently. When that happens, the first goroutine in will actually do
// the pull and all other goroutines will wait on the first puller. Its important to understand
// than pulling an *image* manifest also pulls the image blobs. So this type avoids multiple
// goroutines pulling the same blobs from the upstream at the same time.
type concurrentPulls struct {
	sync.Mutex
	pulls map[string][]chan bool
}

// Type manifestCache is the in-mem representation of the manifest cache. The key is a manifest
// URL like docker.io/calico/cni:v3.27.0 or docker.io/calico/cni@sha256:3163eabb.... The content
// is a 'ManifestHolder'.
type manifestCache struct {
	sync.Mutex
	manifests map[string]imgpull.ManifestHolder
}

// Type blobCache is the in-mem representation of the blob cache. The key is a digest, and the value
// is a ref count. The ref count is the number of image manifests that reference that particular.
// blob. This ref count is used to prune the blobs. A blob with no refs can be safely removed.
type blobCache struct {
	sync.Mutex
	blobs map[string]int
}

var (
	cp concurrentPulls = concurrentPulls{
		pulls: make(map[string][]chan bool),
	}
	mc manifestCache = manifestCache{
		manifests: map[string]imgpull.ManifestHolder{},
	}
	bc blobCache = blobCache{
		blobs: map[string]int{},
	}
	emptyManifestHolder = imgpull.ManifestHolder{}
)

// GetManifest returns a manifest from the in-mem cache matching the URL of the passed 'PullRequest'.
// If no manifest is cached then a pull is performed from an upstream OCI distribution server. The
// function blocks until the pull is complete and then the manifest is added to the in-mem cache and
// returned. Or an error is returned if the manifest can't be pulled or times out. If an *image* manifest
// is requested and it is not already cached, then all the blobs for the image will also be pulled and
// added to the blob cache.
//
// If multiple goroutines pull the same image at the same time, then only the first goroutine will actually
// perform the pull, and all other gouroutines will wait for the first gouroutine to complete the pull and
// add the image to the cache. Then the waiting goroutine(s) will simply pull from the cache entry created
// by the first goroutine.
func GetManifest(pr pullrequest.PullRequest, imagePath string, pullTimeout int, forcePull bool) (imgpull.ManifestHolder, error) {
	url := pr.Url()
	if mh, ch, exists := getManifestOrEnqueue(url, forcePull); exists {
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
			prune(pr, mh)
		}
		addBlobsToCache(mh)
		addManifestToCache(pr, mh)
		return mh, nil
	} else {
		select {
		case <-ch:
			log.Infof("serving manifest from cache (after wait): %q", url)
			return getManifestFromCache(url), nil
		case <-time.After(time.Duration(pullTimeout) * time.Millisecond):
			return emptyManifestHolder, fmt.Errorf("timeout exceeded (%d millis) waiting for signal on %q", pullTimeout, url)
		}
	}
}

// GetBlob returns the ref count for the passed blob digest. If zero then the blob technically
// does not exist and is not referenced by any cached manifests and will eventually be pruned.
func GetBlob(digest string) int {
	bc.Lock()
	defer bc.Unlock()
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
	serialize.MhToFilesystem(mh, imagePath)
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
		addManifestToCache(pr, mh)
		addBlobsToCache(mh)
		itemcnt++
		return nil
	})
	log.Infof("loaded %d manifest(s) from the file system in %s", itemcnt, time.Since(start))
	return outerErr
}

// TODO MAKE SURE THEY EXIST ON THE FILESYSTEM?
// addBlobsToCache adds entries to the in-mem blob map and/or increments the ref count
// for existing blobs in the blob map based on the layers (and the config blob) in the
// passed manifest.
func addBlobsToCache(mh imgpull.ManifestHolder) {
	if mh.IsManifestList() {
		return
	}
	bc.Lock()
	defer bc.Unlock()
	for _, layer := range mh.Layers() {
		digest := helpers.GetDigestFrom(layer.Digest)
		bc.blobs[digest] = bc.blobs[digest] + 1
	}
}

// addManifestToCache adds the passed manifest to the in-mem manifest map, keyed by
// the passed URL. If the passed manifest was pulled by tag, then a second entry
// is added to cache keyed by digest. This enables the cache to serve manifest requests
// by tag and by digest for the same manifest.
func addManifestToCache(pr pullrequest.PullRequest, mh imgpull.ManifestHolder) {
	mc.Lock()
	defer mc.Unlock()
	mc.manifests[pr.Url()] = mh
	if pr.PullType == pullrequest.ByTag {
		mc.manifests[pr.UrlWithDigest("sha256:"+mh.Digest)] = mh
	}
}

// getManifestFromCache gets a manifest from the in-mem manifest cache, or returns
// an empty manifest holder if the manifest for the passed URL is not cached.
func getManifestFromCache(url string) imgpull.ManifestHolder {
	mc.Lock()
	defer mc.Unlock()
	return mc.manifests[url]
}

// getManifestOrEnqueue looks in the in-mem manifest cache for the passed manifest URL. If found,
// then the manifest holder is returned. If not in cache, then the function enqueues a pull for
// the manifest from the upstream.
//
// If the current goroutine is the first to enqueue a pull, then a nil channel is returned. Otherwise
// a nin-nil channel is returned. This means that another goroutine is already doing the pull for the
// requested url so the caller in *this* goroutine should wait to be signled. The pulling go routine
// will signal all waiters when the image has been cached.
//
// In summary, the return values from the function are interpreted as follows:
//
// Manifest in cache:
//
//	ManifestHolder: The manifest
//	channel: nil
//	bool: true
//
// Manifest NOT in cache, and no other goroutine is already pulling from the upstream
//
//	ManifestHolder: Empty
//	channel: nil - caller must pull and then signal waiting goroutines after pull
//	bool: false
//
// Manifest NOT in cache, and another goroutine IS already pulling from the upstream
//
//	ManifestHolder: Empty
//	channel: non-nil - caller must wait to be signaled when the pulling goroutine finishes
//	bool: false
//
// As a final bit of complexity if forcePull is true then the manifest is always pulled.
// In this case the server acts like a simple proxy.
func getManifestOrEnqueue(url string, forcePull bool) (imgpull.ManifestHolder, chan bool, bool) {
	mc.Lock()
	defer mc.Unlock()
	if !forcePull {
		if val, exists := mc.manifests[url]; exists {
			return val, nil, true
		}
	}
	return emptyManifestHolder, enqueuePull(url), false
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
