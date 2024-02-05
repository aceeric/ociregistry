package pullsync

import (
	"fmt"
	"ociregistry/globals"
	"sync"
)

var ps *pullSyncer = newPullSyncer()

type pullSyncer struct {
	mu sync.Mutex
	// in progress pulls
	pullMap map[string][]chan bool
	// already pulled
	imageCache map[string]bool
}

func newPullSyncer() *pullSyncer {
	return &pullSyncer{
		pullMap:    make(map[string][]chan bool),
		imageCache: make(map[string]bool),
	}
}

func isPulled(image string) bool {
	ps.mu.Lock()
	_, exists := ps.imageCache[image]
	ps.mu.Unlock()
	return exists
}

// if already enqueued, add channel and return true, else enqueue
// image and channel and return false
func enqueue(image string, ch chan bool) bool {
	globals.Logger().Debug(fmt.Sprintf("enqueue image: %s, chan: %v", image, ch))
	ps.mu.Lock()
	chans, exists := ps.pullMap[image]
	if exists {
		globals.Logger().Debug(fmt.Sprintf("image already enqueued: %s - append chan %v", image, ch))
		ps.pullMap[image] = append(chans, ch)
	} else {
		globals.Logger().Debug(fmt.Sprintf("image not enqueued: %s - enqueing with chan: %v", image, ch))
		ps.pullMap[image] = []chan bool{ch}
	}
	ps.mu.Unlock()
	return exists
}

// signal all waiters for image and move image from eneuqued to pulled
func pullComplete(image string) {
	globals.Logger().Debug(fmt.Sprintf("pull image: %s", image))
	ps.mu.Lock()
	chans, exists := ps.pullMap[image]
	if exists {
		globals.Logger().Debug(fmt.Sprintf("signaling waiters for image: %s", image))
		for _, ch := range chans {
			defer func() {
				if err := recover(); err != nil {
					globals.Logger().Debug(fmt.Sprintf("write to closed channel for image: %s - ignore", image))
				}
			}()
			globals.Logger().Debug(fmt.Sprintf("signal done for chan: %v", ch))
			ch <- true
		}
		globals.Logger().Debug(fmt.Sprintf("remove image %s from map", image))
		delete(ps.pullMap, image)
	} else {
		globals.Logger().Debug(fmt.Sprintf("not found image: %s", image))
	}
	ps.imageCache[image] = true
	ps.mu.Unlock()
}
