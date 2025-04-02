package main

import (
	"fmt"
	"math/rand/v2"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"
)

type concurrentPulls struct {
	sync.Mutex
	pulls map[string][]chan bool
}

type manifestCache struct {
	sync.Mutex
	manifests map[string][]byte
}

type blobCache struct {
	sync.Mutex
	blobs map[string]int
}

var (
	cp concurrentPulls = concurrentPulls{
		pulls: make(map[string][]chan bool),
	}
	mc manifestCache = manifestCache{
		manifests: map[string][]byte{},
	}
	bc blobCache = blobCache{
		blobs: map[string]int{},
	}
	waitMillis = 10000
)

// <----------------- NEXT HANDLE 3-4 MANIFESTS AND IMPLEMENT PRUNE

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		segments := strings.Split(r.URL.Path, "/")
		if segments[1] == "manifest" {
			m := getManifest(segments[2])
			fmt.Printf("retrieved manifest %q in %s\n", segments[1], time.Since(start))
			m = append(m, []byte("\n")[0])
			w.Write(m)
		} else if segments[1] == "delete" {
			m := getManifest(segments[2])
			prune(string(m))
		} else if segments[1] == "blob" {
			cnt := getBlob(segments[2])
			msg := fmt.Sprintf("blob: %s; count: %d\n", segments[2], cnt)
			w.Write([]byte(msg))
		} else if segments[1] == "blobcache" {
			for key, val := range bc.blobs {
				fmt.Printf("key: %q, count :%d\n", key, val)
			}
		}
	})
	http.ListenAndServe(":3333", nil)
}

func getManifest(key string) []byte {
	val, ch := getManifestOrEnqueue(key)
	if val != nil {
		return val
	} else if ch == nil {
		defer signalWaiters(key)
		b := simulatePull(key)
		digests := simulateDigestList(string(b))
		addBlobsToCache(digests)
		addManifestToCache(key, b)
		return b
	}
	select {
	case <-ch:
		return getManifestFromCache(key)
	case <-time.After(time.Duration(waitMillis) * time.Millisecond):
		return nil
	}
}

// maybe just return count - zero means not cached
func getBlob(digest string) int {
	bc.Lock()
	defer bc.Unlock()
	if count, exists := bc.blobs[digest]; exists {
		return count
	}
	return -1
}

func simulateDigestList(key string) []string {
	return strings.Split(key, "/")[1:]
}

func simulatePull(key string) []byte {
	t := rand.IntN(4)
	time.Sleep(time.Second * time.Duration(t))
	// make a manifest consisting of the passed key and a random number of digests
	// separated by /.
	digests := ""
	cnt := rand.IntN(10)
	for i := 0; i < cnt; i++ {
		idx := rand.IntN(26)
		str := string("ABCDEFGHIJKLMNOPQRSTUVWXYZ"[idx])
		digests = digests + "/" + str
	}
	return []byte(key + digests)
}

func addBlobsToCache(digests []string) {
	bc.Lock()
	defer bc.Unlock()
	for _, digest := range slices.Compact(digests) {
		bc.blobs[digest] = bc.blobs[digest] + 1
	}
}

// todo lazy delete from file system??
func delBlobsFromCache(digests []string) {
	bc.Lock()
	defer bc.Unlock()
	for _, digest := range digests {
		if cnt := bc.blobs[digest]; cnt > 0 {
			bc.blobs[digest] = cnt - 1
		}
	}
}

func addManifestToCache(key string, val []byte) {
	mc.Lock()
	defer mc.Unlock()
	mc.manifests[key] = val // manifestHolder
}

// todo from file system
// this could delete a manifest right after it was pulled and waiters
// are waiting which could cause the waiters to return
func delManifestFromCache(key string) {
	mc.Lock()
	defer mc.Unlock()
	delete(mc.manifests, key)
}

// if nil does not exist
func getManifestFromCache(key string) []byte {
	mc.Lock()
	defer mc.Unlock()
	return mc.manifests[key]
}

// think about doing this in on transaction?
func prune(key string) {
	// get blobs associated with manifest
	digests := simulateDigestList(key)
	delManifestFromCache(key)
	delBlobsFromCache(digests)
}

// return values:
// - if in cache, []byte will be non-nil. If []byte is nil then the manifest is not
// in cache. In that case:
//   - if chan non-nil caller must wait on channel because another go routine is pulling
//   - else chan is nil: so caller must get and then signal waiters
func getManifestOrEnqueue(key string) ([]byte, chan bool) {
	mc.Lock()
	defer mc.Unlock()
	if val, exists := mc.manifests[key]; exists {
		return val, nil
	}
	return nil, enqueuePull(key)
}

// return nil means not enqueued, non-nil means enqueued and caller
// must wait on the channel
func enqueuePull(key string) chan bool {
	cp.Lock()
	defer cp.Unlock()
	if chans, exists := cp.pulls[key]; exists {
		ch := make(chan bool)
		cp.pulls[key] = append(chans, ch)
		return ch
	} else {
		cp.pulls[key] = []chan bool{}
	}
	return nil
}

func signalWaiters(key string) {
	cp.Lock()
	defer cp.Unlock()
	if chans, exists := cp.pulls[key]; exists {
		for _, ch := range chans {
			defer func() {
				if err := recover(); err != nil {
					fmt.Printf("attempt to write to closed channel for key %q", key)
				}
			}()
			ch <- true
		}
		delete(cp.pulls, key)
	}
}
