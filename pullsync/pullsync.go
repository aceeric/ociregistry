package pullsync

import (
	"sync"

	log "github.com/sirupsen/logrus"
)

var ps *pullSyncer = newPullSyncer()

// pullSyncer contains a map of pulls in progress so that if multiple handlers
// concurrently pull the same image, only the first will actually do the
// work.
type pullSyncer struct {
	mu sync.Mutex
	// in progress pulls
	pullMap map[string][]chan bool
	// already pulled
	//imageCache map[string]bool
}

// newPullSyncer allocates a new 'pullSyncer' struct
func newPullSyncer() *pullSyncer {
	return &pullSyncer{
		pullMap: make(map[string][]chan bool),
		//imageCache: make(map[string]bool),
	}
}

// don't do this because if the image is removed from the file system
// the function has no way to no that if it uses this var
// func isPulled(image string) bool {
// ps.mu.Lock()
// _, exists := ps.imageCache[image]
// ps.mu.Unlock()
// return exists
// }

// enqueue manages a pull queue. The first concurrent caller to pull the
// image in the 'image' arg creates a map entry and is returned 'false'
// meaning this caller is first and needs to actually perform the pull.
// All other concurrent callers have their passed channel added to the
// list of channels for the queue entry keyed by 'image'. Below, the
// 'pullComplete' function uses the list of channels to notify all waiting
// pullers.
func enqueue(image string, ch chan bool) bool {
	ps.mu.Lock()
	chans, exists := ps.pullMap[image]
	if exists {
		log.Debugf("image %s already enqueued, append chan %v to waiters list", image, ch)
		ps.pullMap[image] = append(chans, ch)
	} else {
		log.Debugf("enqueueing image %s with chan: %v", image, ch)
		ps.pullMap[image] = []chan bool{ch}
	}
	ps.mu.Unlock()
	return exists
}

// pullComplete signals all waiters that are waiting for the passed image pull to
// complete, and then deletes the image key from the queue co-managed by this function
// and the enqueue function.
func pullComplete(image string) {
	log.Debugf("pull image: %s", image)
	ps.mu.Lock()
	chans, exists := ps.pullMap[image]
	if exists {
		log.Debugf("signaling waiters for image: %s", image)
		for _, ch := range chans {
			defer func() {
				if err := recover(); err != nil {
					log.Debugf("write to closed channel for image: %s ignore", image)
				}
			}()
			log.Debugf("signal done for chan: %v", ch)
			ch <- true
		}
		log.Debugf("remove image %s from map", image)
		delete(ps.pullMap, image)
	} else {
		log.Debugf("not found image: %s", image)
	}
	//ps.imageCache[image] = true
	ps.mu.Unlock()
}
