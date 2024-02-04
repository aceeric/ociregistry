package pullsync

import (
	"sync"

	"github.com/labstack/echo"
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
func enqueue(image string, ch chan bool, logger echo.Logger) bool {
	logger.Debug("enqueue image: %s, chan: %v\n", image, ch)
	ps.mu.Lock()
	chans, exists := ps.pullMap[image]
	if exists {
		logger.Debug("image already enqueued: %s - append chan %v\n", image, ch)
		ps.pullMap[image] = append(chans, ch)
	} else {
		logger.Debug("image not enqueued: %s - enqueing with chan: %v\n", image, ch)
		ps.pullMap[image] = []chan bool{ch}
	}
	ps.mu.Unlock()
	return exists
}

// signal all waiters for image and move image from eneuqued to pulled
func pullComplete(image string, logger echo.Logger) {
	logger.Debug("pull image: %s\n", image)
	ps.mu.Lock()
	chans, exists := ps.pullMap[image]
	if exists {
		logger.Debug("signaling waiters for image: %s\n", image)
		for _, ch := range chans {
			defer func() {
				if err := recover(); err != nil {
					logger.Debug("write to closed channel for image: %s - ignore\n", image)
				}
			}()
			logger.Debug("signal done for chan: %v\n", ch)
			ch <- true
		}
		logger.Debug("remove image %s from map\n", image)
		delete(ps.pullMap, image)
	} else {
		logger.Debug("not found image: %s\n", image)
	}
	ps.imageCache[image] = true
	ps.mu.Unlock()
}
