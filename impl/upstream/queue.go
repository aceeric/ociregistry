package upstream

import (
	"encoding/json"
	"fmt"
	"ociregistry/impl/extractor"
	"ociregistry/impl/globals"
	"ociregistry/impl/pullrequest"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
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

// Get gets fat manifests, image manifests, and image blobs from an
// upstream registry based on the passed 'PullRequest'
func Get(pr pullrequest.PullRequest, imagePath string, waitMillis int) (ManifestHolder, error) {
	imageUrl := pr.Url()
	ch := make(chan bool)
	var mh = ManifestHolder{}
	var err error = nil
	// a goroutine because it signals the outer function so must run independently
	go func(imageUrl string, ch chan bool) {
		defer func() {
			if err := recover(); err != nil {
				log.Errorf("panic in queue.get: %s", err)
			}
		}()
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
				log.Errorf("error downloading tarball %s for image %s: %s", tarfile, imageUrl, err)
				err = ierr
				return
			} else {
				log.Debugf("no error returned from craneDownloadImg for image %s", imageUrl)
			}
			mh.Tarfile = tarfile
			log.Debugf("extracting image tarball %s for image %s", tarfile, imageUrl)
			err = extractor.Extract(tarfile, imagePath, true)
			if err != nil {
				log.Errorf("error extracting image tarball %s for image %s: %s", tarfile, imageUrl, err)
			}
		}
	}(imageUrl, ch)
	select {
	case <-ch:
		log.Debugf("waiter was signaled for image %s", imageUrl)
	case <-time.After(time.Duration(waitMillis) * time.Millisecond):
		err = fmt.Errorf("timeout exceeded pulling image %s", imageUrl)
	}
	mh.Pr = pr
	mh.ImageUrl = imageUrl
	return mh, err
}

// manifestHolderFromDescriptor does some parsing of the passed descriptor
// and basically wraps it in a 'ManifestHolder' which is returned to the caller.
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

// isImageDescriptor returns true if the passed descriptor is an image
// descriptor, otherwise returns false, meaning that it is a manifest list.
func isImageDescriptor(d *remote.Descriptor) bool {
	return d.Descriptor.MediaType == V2dockerManifestMt || d.Descriptor.MediaType == V1ociManifestMt
}

// enqueueGet enqueues an upstream registry get request for the passed
// 'imageUrl'. If there are no other requesters, then the function returns
// false - meaning the caller is the first requester and therefore will have
// to actaully pull the image. If a request was previously enqueued for the
// image url then true is returned meaning the caller should simply wait for
// a signal on the passed channel which signifies that the prior caller has
// pulled the image and added it to the cache and so this caller can access
// the cached image. In all cases, all callers will be signalled on the passed
// channel when the image is pulled and available in cache.
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

// doneGet signals all waiters for the passed image URL using the channels that
// are associated with the passed URL as populated by 'enqueueGet'.
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

// cranePull gets any defined upstream registry configuration associated with the
// registry in the passed URL, and packages it for the Google Crane code embedded in the
// project, then calls Google Crane to get the object behind the URL which will be either
// a manifest list, or an image manifest.
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

// CraneHead is like 'cranePull' except it does a HEAD request from the upstream
// registry.
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

// craneDownloadImg downloads an image tarball to the "pulls" drectory under
// the passed 'imagePath' directory, or returns an error if unable to do so.
func craneDownloadImg(imageUrl string, d *remote.Descriptor, imagePath string) (string, error) {
	var imageTar = filepath.Join(imagePath, globals.PullsDir)
	if _, err := os.Stat(imageTar); err != nil {
		if err := os.MkdirAll(imageTar, 0755); err != nil {
			return "", err
		}
	}
	imageTar = filepath.Join(imageTar, tarName(d.Digest.Hex))
	img, err := d.Image()
	if err != nil {
		return "", err
	}
	log.Debugf("save image to %s", imageTar)
	if _, err := os.Stat(imageTar); err == nil {
		log.Warnf("image tarfile already exists: %s", imageTar)
	}
	return imageTar, crane.Save(img, imageUrl, imageTar)
}

// tarName concats the passed digest with a uuid and the system time and returns it with a
// ".tar" extension.
func tarName(digest string) string {
	currentTimestamp := time.Now().UnixNano()
	return digest + "." + strconv.FormatInt(currentTimestamp, 10) + ".tar"
}
