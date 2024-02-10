package pullsync

import (
	"ociregistry/globals"
	"ociregistry/importer"
	"os"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/google/uuid"
)

// PullImage pulls an image from a remote. The image is specified in the
// 'image' arg and the resulting tarball will be placed in the 'image_path' arg.
// The call blocks for the number of milliseconds specified in the 'waitMillis' arg.
// To handle cases where multiple callers concurrently ask for the same image, the
// function uses a queue in which the first caller does the pull and all other
// concurrent callers wait until the first caller finishes. Once the first caller
// finishes (or times out) then all other callers return at the same time.
func PullImage(image string, image_path string, waitMillis int) {
	ch := make(chan bool)
	var result bool = false
	go func(image string, ch chan bool) {
		if enqueue(image, ch) {
			log.Debugf("already enqueued: %s, added waiter on chan %v", image, ch)
			return
		}
		log.Debugf("newly enqueued calling crane pull: %s", image)
		callCranePull(image, image_path)
		log.Debugf("back from crane pull: %s", image)
		pullComplete(image)

	}(image, ch)
	select {
	case result = <-ch:
		log.Debugf("successful pull: %s", image)
	case <-time.After(time.Duration(waitMillis) * time.Millisecond):
		log.Debugf("error: time out waiting for pull: %s", image)
		result = false
	}
	close(ch)
	log.Debugf("image: %s, result: %t", image, result)
}

// callCranePull pulls the image specified by the 'image' arg to the file system
// path specified by the `image_path` arg. The function creates a subdirectory under
// that path, then generates a UUID-based name for the download file. After downloading,
// the image is extracted into the images directory to be subsequently served, and then
// the downloaded archive file is deleted.
func callCranePull(image string, image_path string) error {
	var imageTar = filepath.Join(image_path, globals.PullsDir)
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
	err = importer.Extract(imageTar, image_path)
	if err != nil {
		return err
	}
	return os.Remove(imageTar)
}
