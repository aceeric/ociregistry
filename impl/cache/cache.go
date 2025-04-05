package cache

import (
	"fmt"
	"ociregistry/impl/globals"
	"ociregistry/impl/helpers"
	"ociregistry/impl/pullrequest"
	"ociregistry/impl/serialize"
	"ociregistry/impl/upstream"
	"path/filepath"
	"sync"
	"time"

	"github.com/aceeric/imgpull/pkg/imgpull"
	log "github.com/sirupsen/logrus"
)

// TODO HANDLE ALWAYS PULL LATEST

// Type concurrentPulls handles the case where multiple goroutines might request a manifest
// from an upstream concurrently. When that happens, the first goroutine in will actuall do
// the pull and all other goroutines will wait on the first puller. Its important to understand
// than pulling an *image* manifest also pulls the image blobs. So this type is intended to
// prevent multiple goroutines from pulling the same blobs from the upstream at the same
// time.
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
// blob. This ref count is used to prune the blobs. A blob with no refs can be safely removed from
// the file system.
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
// If no manifest is cached then a pull is performed. The function blocks until the pull is complete and
// then the manifest is added to the in-mem cache and returned. Or an error is returned if the manifest
// can't be pulled or times out. If an *image* manifest is requested and it is not already cached, then all
// the blobs for the image will also be pulled and added to the blob cache.
//
// If multiple goroutines pull the same image at the same time, then only the first goroutine will actually
// perform the pull, and all other gouroutines will wait for the first gouroutine to complete the pull and
// add the image to the cache. Then the waiting goroutines will pull from the cache.
func GetManifest(pr pullrequest.PullRequest, imagePath string, pullTimeout int) (imgpull.ManifestHolder, error) {
	url := pr.Url()
	if mh, ch, exists := getManifestOrEnqueue(url); exists {
		return mh, nil
	} else if ch == nil {
		defer signalWaiters(url)
		mh, err := doPull(pr, imagePath)
		if err != nil {
			log.Errorf("doPull failed for %q: %s", url, err)
			return emptyManifestHolder, err
		}
		addBlobsToCache(mh)
		addManifestToCache(pr, mh)
		return mh, nil
	} else {
		select {
		case <-ch:
			return getManifestFromCache(url), nil
		case <-time.After(time.Duration(pullTimeout) * time.Millisecond):
			return emptyManifestHolder, fmt.Errorf("timeout exceeded (%d millis) waiting for signal on %q", pullTimeout, url)
		}
	}
}

// GetBlob returns the ref count for the passed blob digest. If zero then the blob is not
// referenced by any cached manifests and will eventually be pruned.
func GetBlob(digest string) int {
	bc.Lock()
	defer bc.Unlock()
	return bc.blobs[digest]
}

// doPull gets an image list manifest - or image manifest - from an upstream OCI distribution
// server. If an image manifest is pulled, then all the blobs for the image manifest are
// also pulled.
func doPull(pr pullrequest.PullRequest, imagePath string) (imgpull.ManifestHolder, error) {
	opts, err := upstream.NewConfigFor(pr.Remote)
	if err != nil {
		log.Warn(err.Error())
	}
	puller, err := imgpull.NewPuller(pr.Url(), opts...)
	if err != nil {
		return emptyManifestHolder, err
	}
	mh, err := puller.GetManifest()
	if err != nil {
		return emptyManifestHolder, err
	}
	serialize.ToFilesystemNEW(mh, imagePath)
	// if this is an image manifest, then get the blobs
	// TODO mh.IsImageManifest()
	if !mh.IsManifestList() {
		blobDir := filepath.Join(imagePath, globals.BlobsDir)
		err = puller.PullBlobs(mh, blobDir)
		if err != nil {
			return emptyManifestHolder, err
		}
	}
	return mh, nil
}

// addBlobsToCache adds entries to the blob map and/or increments the ref count for
// existing blobs in the blob map based on the layers (and the config blob) in the
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

// addManifestToCache adds the passed manifest to the in-mem cache, keyed by
// the passed URL. The the passed manifest was pulled by tag, then a second entry
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
// nil if the manifest for the passed URL is not cached.
func getManifestFromCache(url string) imgpull.ManifestHolder {
	mc.Lock()
	defer mc.Unlock()
	return mc.manifests[url]
}

// getManifestOrEnqueue looks at the in-mem manifest cache for the passed URL. If found, then the
// manifest holder is returned. If not in cache, then the function enqueues a pull for the manifest
// from the upstream.
//
// If the current goroutine is the first to enqueue a pull, then a nil channel is returned. Otherwise
// this goroutine has made a concurrent pull request and another goroutine is already doing the pull
// so a channel is returned for the caller to wait on. So the return values are to be interpreted
// as follows:
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
//	channel: nil - caller must pull and then signal any/all gouroutines waiting
//	bool: false
//
// Manifest NOT in cache, and another goroutine IS already pulling from the upstream
//
//	ManifestHolder: Empty
//	channel: non-nil - caller should wait to be signaled when the other goroutine finishes
//	bool: false
func getManifestOrEnqueue(url string) (imgpull.ManifestHolder, chan bool, bool) {
	mc.Lock()
	defer mc.Unlock()
	if val, exists := mc.manifests[url]; exists {
		return val, nil, true
	}
	return emptyManifestHolder, enqueuePull(url), false
}

// enqueuePull enqueues a pull request from the upstream. A return value of  nil
// means the pull request has not already been enqueued by another goroutine. Non-nil means
// another goroutine HAS already enqueued the pull and the caller must wait on the returned
// channel to be signalled when the pull completes.
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
