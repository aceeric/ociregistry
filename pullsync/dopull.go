package pullsync

import (
	"ociregistry/importer"
	"time"

	"github.com/labstack/echo"
)

func PullImage(image string, waitMillis int, logger echo.Logger) {
	if isPulled(image) {
		return
	}
	ch := make(chan bool)
	var result bool = false
	go func(image string, ch chan bool) {
		if enqueue(image, ch, logger) {
			logger.Info("doPull - already enqueued: %s, added chan %v", image, ch)
			return
		}
		logger.Info("doPull - newly enqueued - calling crane pull: %s", image)
		callCranePull()
		logger.Info("doPull - back from crane pull: %s", image)
		pullComplete(image, logger)

	}(image, ch)
	select {
	case result = <-ch:
		logger.Info("pullImage - successful pull: %s", image)
	case <-time.After(time.Duration(waitMillis) * time.Millisecond):
		logger.Info("pullImage - error: time out waiting for pull: %s", image)
		result = false
	}
	close(ch)
	logger.Info("pullImage - return from pullImage. image: %s, result: %t", image, result)
}

// stub for now
func callCranePull() {
	src := "/home/eace/projects/desktop-kubernetes/images/docker.io+infoblox+dnstools+latest.tar"
	dest := "/home/eace/projects/ociregistry/images"
	importer.Extract(src, dest)
}
