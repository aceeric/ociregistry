package pullsync

import (
	"fmt"
	"ociregistry/globals"
	"ociregistry/importer"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

func PullImage(image string, image_path string, waitMillis int) {
	// if isPulled(image) {
	// 	return
	// }
	ch := make(chan bool)
	var result bool = false
	go func(image string, ch chan bool) {
		if enqueue(image, ch) {
			globals.Logger().Debug(fmt.Sprintf("already enqueued: %s, added chan %v", image, ch))
			return
		}
		globals.Logger().Debug(fmt.Sprintf("newly enqueued - calling crane pull: %s", image))
		callCranePull(image, image_path)
		globals.Logger().Debug(fmt.Sprintf("back from crane pull: %s", image))
		pullComplete(image)

	}(image, ch)
	select {
	case result = <-ch:
		globals.Logger().Debug(fmt.Sprintf("successful pull: %s", image))
	case <-time.After(time.Duration(waitMillis) * time.Millisecond):
		globals.Logger().Debug(fmt.Sprintf("error: time out waiting for pull: %s", image))
		result = false
	}
	close(ch)
	globals.Logger().Debug(fmt.Sprintf("image: %s, result: %t", image, result))
}

// callCranePull pulls the image specified by the 'image' arg to the file system
// path specified by the `image_path` arg. The function creates a subdirectory under
// that path, then generates a UUID-based name for the downwload file. After downloading
// the images is extracted into the images directory to be subsequently served, and then
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
	if err == nil {
		return err
	}
	return os.Remove(imageTar)
}
