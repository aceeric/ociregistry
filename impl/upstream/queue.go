package upstream

import (
	"encoding/json"
	"fmt"
	"ociregistry/globals"
	"os"
	"path/filepath"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/uuid"
)

var (
	// ps is a synchronizable map of pulls in progress so that if multiple
	// handlers concurrently try to pull the same manifest or image, only
	// the first will pull and all will return when the pull is complete.
	ps = struct {
		mu      sync.Mutex
		pullMap map[string][]chan bool
	}{
		pullMap: make(map[string][]chan bool),
	}
	alreadyEnqueued bool = true
)

func Get(imageUrl string, imagePath string, waitMillis int) (ManifestHolder, error) {
	ch := make(chan bool)
	var mh = ManifestHolder{}
	var err error = nil
	// a goroutine because it signals the outer function
	go func(imageUrl string, ch chan bool) {
		if enqueueGet(imageUrl, ch) == alreadyEnqueued {
			return
		}
		descriptor, ierr := cranePull(imageUrl)
		if ierr == nil {
			mh, ierr = manifestFromDescriptor(descriptor)
			if ierr == nil && isImageDescriptor(descriptor) {
				ierr = craneDownloadImg(imageUrl, descriptor, imagePath)
			}
		}
		err = ierr
		doneGet(imageUrl)
	}(imageUrl, ch)
	select {
	case <-ch:
	case <-time.After(time.Duration(waitMillis) * time.Millisecond):
	}
	return mh, err
}

func isImageDescriptor(d *remote.Descriptor) bool {
	return d.Descriptor.MediaType == "application/vnd.docker.distribution.manifest.v2+json"
}

func manifestFromDescriptor(d *remote.Descriptor) (ManifestHolder, error) {
	mh := ManifestHolder{}
	var err error
	if isImageDescriptor(d) {
		var m = ImageManifest{}
		err = json.Unmarshal(d.Manifest, &m)
		if err == nil {
			mh.im = m
		}
	} else {
		var m = ManifestList{}
		fmt.Println(string(d.Manifest))
		err = json.Unmarshal(d.Manifest, &m)
		if err == nil {
			mh.ml = m
		}
	}
	return mh, err
}

func enqueueGet(imageUrl string, ch chan bool) bool {
	ps.mu.Lock()
	chans, exists := ps.pullMap[imageUrl]
	if exists {
		ps.pullMap[imageUrl] = append(chans, ch)
	} else {
		ps.pullMap[imageUrl] = []chan bool{ch}
	}
	ps.mu.Unlock()
	return exists
}

func doneGet(imageUrl string) {
	log.Debugf("pull image: %s", imageUrl)
	ps.mu.Lock()
	chans, exists := ps.pullMap[imageUrl]
	if exists {
		log.Debugf("signaling waiters for image: %s", imageUrl)
		for _, ch := range chans {
			defer func() {
				if err := recover(); err != nil {
					log.Debugf("write to closed channel for image: %s ignore", imageUrl)
				}
			}()
			log.Debugf("signal done for chan: %v", ch)
			ch <- true
		}
		log.Debugf("remove image %s from map", imageUrl)
		delete(ps.pullMap, imageUrl)
	} else {
		log.Debugf("not found image: %s", imageUrl)
	}
	ps.mu.Unlock()
}

// TODO cachepath
func cranePull(imageUrl string) (*remote.Descriptor, error) {
	ref, err := name.ParseReference(imageUrl, make([]name.Option, 0)...)
	if err != nil {
		return nil, err
	}
	opts, err := configFor(ref.Context().Registry.Name())
	if err != nil {
		log.Warn(err.Error())
	}
	return remote.Get(ref, opts...)
}

func craneDownloadImg(imageUrl string, d *remote.Descriptor, imagePath string) error {
	var imageTar = filepath.Join(imagePath, globals.PullsDir)
	if _, err := os.Stat(imageTar); err != nil {
		if err := os.MkdirAll(imageTar, 0755); err != nil {
			return err
		}
	}
	imageTar = filepath.Join(imageTar, uuid.New().String()+".tar")
	img, err := d.Image()
	if err != nil {
		return err
	}
	return crane.Save(img, imageUrl, imageTar)
}
