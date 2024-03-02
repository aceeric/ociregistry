package upstream

import (
	"encoding/json"
	"fmt"
	"ociregistry/impl/extractor"
	"ociregistry/impl/globals"
	"ociregistry/impl/pullrequest"
	"os"
	"path/filepath"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/uuid"
)

var (
	// ps is a synchronized map of pulls in progress so that if multiple
	// handlers concurrently try to pull the same manifest or image, only
	// the first will pull, but all (including the first) will return when
	// the pull is complete.
	ps = struct {
		mu      sync.Mutex
		pullMap map[string][]chan bool
	}{
		pullMap: make(map[string][]chan bool),
	}
	alreadyEnqueued bool = true
)

// Get gets fat manifests, image manifests, and image blobs
func Get(pr pullrequest.PullRequest, imagePath string, waitMillis int) (ManifestHolder, error) {
	imageUrl := pr.Url()
	ch := make(chan bool)
	var mh = ManifestHolder{}
	var err error = nil
	// a goroutine because it signals the outer function so must run independently
	go func(imageUrl string, ch chan bool) {
		if enqueueGet(imageUrl, ch) == alreadyEnqueued {
			return
		}
		defer doneGet(imageUrl)
		descriptor, ierr := cranePull(imageUrl)
		if ierr != nil {
			err = ierr
			return
		}
		mh, ierr = manifestHolderFromDescriptor(descriptor)
		if ierr != nil {
			err = ierr
			return
		}
		if isImageDescriptor(descriptor) {
			log.Debugf("downloading image %s", imageUrl)
			tarfile, ierr := craneDownloadImg(imageUrl, descriptor, imagePath)
			if ierr != nil {
				err = ierr
				return
			}
			mh.Tarfile = tarfile
			err = extractor.Extract(tarfile, imagePath, true)
		}
	}(imageUrl, ch)
	select {
	case <-ch:
	case <-time.After(time.Duration(waitMillis) * time.Millisecond):
	}
	mh.Pr = pr
	mh.ImageUrl = imageUrl
	return mh, err
}

func manifestHolderFromDescriptor(d *remote.Descriptor) (ManifestHolder, error) {
	mh := ManifestHolder{
		MediaType: string(d.MediaType),
		Digest:    d.Digest.Hex,
		Size:      int(d.Size),
		Bytes:     d.Manifest,
	}
	var err error
	switch d.Descriptor.MediaType {
	case V2dockerManifestListMt:
		mh.Type = V2dockerManifestList
		err = json.Unmarshal(d.Manifest, &mh.V2dockerManifestList)
	case V2dockerManifestMt:
		mh.Type = V2dockerManifest
		err = json.Unmarshal(d.Manifest, &mh.V2dockerManifest)
	case V1ociIndexMt:
		mh.Type = V1ociIndex
		err = json.Unmarshal(d.Manifest, &mh.V1ociIndex)
	case V1ociManifestMt:
		mh.Type = V1ociDescriptor
		err = json.Unmarshal(d.Manifest, &mh.V1ociDescriptor)
	default:
		return ManifestHolder{}, fmt.Errorf("unsupported media type: %s", d.Descriptor.MediaType)
	}
	return mh, err

}

func isImageDescriptor(d *remote.Descriptor) bool {
	return d.Descriptor.MediaType == V2dockerManifestMt || d.Descriptor.MediaType == V1ociManifestMt
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
	ps.mu.Lock()
	chans, exists := ps.pullMap[imageUrl]
	if exists {
		for _, ch := range chans {
			defer func() {
				if err := recover(); err != nil {
					log.Errorf("attempt to write to closed channel pulling %s", imageUrl)
				}
			}()
			ch <- true
		}
		delete(ps.pullMap, imageUrl)
	}
	ps.mu.Unlock()
}

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

func CraneHead(imageUrl string) (*v1.Descriptor, error) {
	ref, err := name.ParseReference(imageUrl, make([]name.Option, 0)...)
	if err != nil {
		return nil, err
	}
	opts, err := configFor(ref.Context().Registry.Name())
	if err != nil {
		log.Warn(err.Error())
	}
	return remote.Head(ref, opts...)
}

func craneDownloadImg(imageUrl string, d *remote.Descriptor, imagePath string) (string, error) {
	var imageTar = filepath.Join(imagePath, globals.PullsDir)
	if _, err := os.Stat(imageTar); err != nil {
		if err := os.MkdirAll(imageTar, 0755); err != nil {
			return "", err
		}
	}
	imageTar = filepath.Join(imageTar, uuid.New().String()+".tar")
	img, err := d.Image()
	if err != nil {
		return "", err
	}
	log.Debugf("save image to %s", imageTar)
	return imageTar, crane.Save(img, imageUrl, imageTar)
}
