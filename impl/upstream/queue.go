package upstream

import (
	"encoding/json"
	"ociregistry/globals"
	"ociregistry/impl/extractor"
	"ociregistry/impl/pullrequest"
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
	return mh, err
}

// TODO err if unknown mediatype
func isImageDescriptor(d *remote.Descriptor) bool {
	return d.Descriptor.MediaType == "application/vnd.docker.distribution.manifest.v2+json"
}

func manifestHolderFromDescriptor(d *remote.Descriptor) (ManifestHolder, error) {
	mh := ManifestHolder{
		MediaType: string(d.MediaType),
		Digest:    d.Digest.Hex,
		Size:      int(d.Size),
		Bytes:     d.Manifest,
	}
	var err error
	if isImageDescriptor(d) {
		var m = ImageManifest{}
		err = json.Unmarshal(d.Manifest, &m)
		if err == nil {
			mh.Im = m
			mh.Type = ImageManifestType
		}
	} else {
		var m = ManifestList{}
		err = json.Unmarshal(d.Manifest, &m)
		if err == nil {
			mh.Ml = m
			mh.Type = ManifestListType
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
	ps.mu.Lock()
	chans, exists := ps.pullMap[imageUrl]
	if exists {
		for _, ch := range chans {
			defer func() {
				if err := recover(); err != nil {
					// TODO log
				}
			}()
			ch <- true
		}
		delete(ps.pullMap, imageUrl)
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
	return imageTar, crane.Save(img, imageUrl, imageTar)
}
