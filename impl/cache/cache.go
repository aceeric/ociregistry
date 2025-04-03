package cache

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/aceeric/imgpull/pkg/imgpull"
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
	waitMillis = 10000
)

// build strategy:
// 1. get manifest with no options
// 2. get with options
// 3. handle adding blobs
func GetManifest(url string) (imgpull.ManifestHolder, error) {
	emptyMh := imgpull.ManifestHolder{}
	mh, ch := getManifestOrEnqueue(url)
	if !reflect.DeepEqual(mh, emptyMh) {
		return mh, nil
	} else if ch == nil {
		defer signalWaiters(url)
		mh, err := doPull(url)
		if err != nil {
			return emptyMh, err
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
		return emptyMh, errors.New("TODO")
	}
}

func doPull(url string) (imgpull.ManifestHolder, error) {
	opts := imgpull.NewPullerOpts(url)
	// TODO OPTS INITALIZATION FROM REG CONFIG
	opts.Scheme = "http"
	puller, err := imgpull.NewPullerWith(opts)
	if err != nil {
		return imgpull.ManifestHolder{}, err
	}
	mh, err := puller.GetManifest()
	if err != nil {
		return imgpull.ManifestHolder{}, err
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
	mc.manifests[url] = mh // manifestHolder
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

// return values:
// - if in cache, []byte will be non-nil. If []byte is nil then the manifest is not
// in cache. In that case:
//   - if chan non-nil caller must wait on channel because another go routine is pulling
//   - else chan is nil: so caller must get and then signal waiters
func getManifestOrEnqueue(url string) (imgpull.ManifestHolder, chan bool) {
	mc.Lock()
	defer mc.Unlock()
	if val, exists := mc.manifests[url]; exists {
		return val, nil
	}
	return imgpull.ManifestHolder{}, enqueuePull(url)
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
