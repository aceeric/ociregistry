package pullsync

import (
	"ociregistry/importer"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo"
)

func PullImage(image string, image_path string, waitMillis int, logger echo.Logger) {
	// if isPulled(image) {
	// 	return
	// }
	ch := make(chan bool)
	var result bool = false
	go func(image string, ch chan bool) {
		if enqueue(image, ch, logger) {
			logger.Info("doPull - already enqueued: %s, added chan %v", image, ch)
			return
		}
		logger.Info("doPull - newly enqueued - calling crane pull: %s", image)
		callCranePull(image, image_path)
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

func callCranePull(image string, image_path string) error {
	var imageTar = filepath.Join(image_path, "pulls")
	if _, err := os.Stat(imageTar); err != nil {
		if err := os.MkdirAll(imageTar, 0755); err != nil {
			return err
		}
	}
	imageTar = filepath.Join(imageTar, uuid.New().String()+".tar")
	err := cranePull(image, imageTar)
	if err != nil {
		return err
	}
	return importer.Extract(imageTar, image_path)
}
