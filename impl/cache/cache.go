package cache

import (
	"errors"
	"fmt"
	"ociregistry/impl/pullrequest"
	"ociregistry/impl/upstream"
	"sync"
	"time"

	"github.com/aceeric/imgpull/pkg/imgpull"
	log "github.com/sirupsen/logrus"
)

type concurrentPulls struct {
	sync.Mutex
	pulls map[string][]chan bool
}

type manifestCache struct {
	sync.Mutex
	manifests map[string]imgpull.ManifestHolder
}

//type blobCache struct {
//	sync.Mutex
//	blobs map[string]int
//}

var (
	cp concurrentPulls = concurrentPulls{
		pulls: make(map[string][]chan bool),
	}
	mc manifestCache = manifestCache{
		manifests: map[string]imgpull.ManifestHolder{},
	}
	//bc blobCache = blobCache{
	//	blobs: map[string]int{},
	//}
	// TODO CONFIGURABLE
	waitMillis          = 10000
	emptyManifestHolder = imgpull.ManifestHolder{}
)

// build strategy:
// 1. DONE get manifest with no options
// 2. DONE get with options
// 3. handle adding blobs
func GetManifest(pr pullrequest.PullRequest) (imgpull.ManifestHolder, error) {
	url := pr.Url()
	mh, ch, exists := getManifestOrEnqueue(url)
	if exists {
		return mh, nil
	} else if ch == nil {
		defer signalWaiters(url)
		mh, err := doPull(pr)
		if err != nil {
			return emptyManifestHolder, err
		}
		//digests := simulateDigestList(string(b))
		//addBlobsToCache(digests)
		addManifestToCache(url, mh)
		return mh, nil
	}
	select {
	case <-ch:
		// TODO CONSIDER RETURNING AN ERROR IF NOT FOUND?
		return getManifestFromCache(url), nil
	case <-time.After(time.Duration(waitMillis) * time.Millisecond):
		return emptyManifestHolder, errors.New("TODO")
	}
}

// TODO move NewConfigFor
func doPull(pr pullrequest.PullRequest) (imgpull.ManifestHolder, error) {
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
	return mh, nil
}

//// maybe just return count - zero means not cached
//func getBlob(digest string) int {
//	bc.Lock()
//	defer bc.Unlock()
//	if count, exists := bc.blobs[digest]; exists {
//		return count
//	}
//	return -1
//}

//func simulateDigestList(key string) []string {
//	return strings.Split(key, "/")[1:]
//}

//func simulatePull(key string) []byte {
//	t := rand.IntN(4)
//	time.Sleep(time.Second * time.Duration(t))
//	// make a manifest consisting of the passed key and a random number of digests
//	// separated by /.
//	digests := ""
//	cnt := rand.IntN(10)
//	for i := 0; i < cnt; i++ {
//		idx := rand.IntN(26)
//		str := string("ABCDEFGHIJKLMNOPQRSTUVWXYZ"[idx])
//		digests = digests + "/" + str
//	}
//	return []byte(key + digests)
//}

//func addBlobsToCache(digests []string) {
//	bc.Lock()
//	defer bc.Unlock()
//	for _, digest := range slices.Compact(digests) {
//		bc.blobs[digest] = bc.blobs[digest] + 1
//	}
//}
//
//// todo lazy delete from file system??
//func delBlobsFromCache(digests []string) {
//	bc.Lock()
//	defer bc.Unlock()
//	for _, digest := range digests {
//		if cnt := bc.blobs[digest]; cnt > 0 {
//			bc.blobs[digest] = cnt - 1
//		}
//	}
//}

func addManifestToCache(url string, mh imgpull.ManifestHolder) {
	mc.Lock()
	defer mc.Unlock()
	mc.manifests[url] = mh
}

//// todo from file system
//// this could delete a manifest right after it was pulled and waiters
//// are waiting which could cause the waiters to return
//func delManifestFromCache(key string) {
//	mc.Lock()
//	defer mc.Unlock()
//	delete(mc.manifests, key)
//}

// if nil does not exist
func getManifestFromCache(url string) imgpull.ManifestHolder {
	mc.Lock()
	defer mc.Unlock()
	return mc.manifests[url]
}

//// think about doing this in on transaction?
//func prune(key string) {
//	// get blobs associated with manifest
//	digests := simulateDigestList(key)
//	delManifestFromCache(key)
//	delBlobsFromCache(digests)
//}

// getManifestOrEnqueue returns a manifest from the in-memory cache if one exists matching the passed
// url. If a manifest is not cached then there are two possible outcomes. If no other goroutine has attempted
// to concurrently pull the manifest, then an entry is created in the concurrent pulls map but a nil channel
// is returned. This means the caller must pull the manifest, and then signal any other goroutines waiting
// for the image to be pulled. If there is already a concurrent pull in progress then a channel is created
// and added to the concurrent pulls map and returned. In this case, the caller must wait to be signaled on
// this channel at which point the manifest should be cached (unless the puller failed, in which case the
// manifest will not be in cache, which is likely an error condition for the caller.“)
//
//	imgpull.ManifestHolder - if manifest is in cache, then it will be returned in this value
//	chan bool              - nil if manifest is in cache, else: if caller should pull, nil, else non-nil,
//	                         meaning caller must wait on channel because another go routine is pulling
//	bool                   - true if manifest is in cache, else false
func getManifestOrEnqueue(url string) (imgpull.ManifestHolder, chan bool, bool) {
	mc.Lock()
	defer mc.Unlock()
	if val, exists := mc.manifests[url]; exists {
		return val, nil, true
	}
	return emptyManifestHolder, enqueuePull(url), false
}

// return nil means not enqueued, non-nil means enqueued and caller
// must wait on the channel
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
