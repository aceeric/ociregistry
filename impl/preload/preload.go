package preload

import (
	"bufio"
	"fmt"
	"ociregistry/impl/pullrequest"
	"ociregistry/impl/serialize"
	"ociregistry/impl/upstream"
	"os"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// Preload loads the manifest and blob cache at the passed 'imagePath' location from
// the list of images enumerated in the file identified by the passed 'imageListFile'
// arg. If an image is already present in cache, it is skipped. Otherwise the image is
// pulled from the upstream using the upstream registry encoded into the file entry.
// Here is a example of what one entry in the file identified by the 'imageListFile'
// arg should look like. It's a standard repository URL. If you can 'docker pull' it,
// then it should be valid in the file.
//
//	'registry.k8s.io/metrics-server/metrics-server:v0.6.2'
//
// The platform architecture and OS args are used to select an image from a "fat" manifest
// that contains a list of images. IMPORTANT: each item in the list MUST begin with
// a remote registry ref - i.e. to the left of the first forward slash
func Preload(imageListFile string, imagePath string, platformArch string, platformOs string, pullTimeout int) error {
	start := time.Now()
	log.Infof("loading images from file: %s", imageListFile)
	itemcnt := 0
	f, err := os.Open(imageListFile)
	if err != nil {
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(string(scanner.Bytes()))
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}
		log.Debugf("pulling image: %s", line)
		cnt, err := preloadOneImage(line, imagePath, platformArch, platformOs, pullTimeout)
		if err != nil {
			return err
		}
		itemcnt += cnt
	}
	log.Infof("loaded %d images to the file system cache in %s", itemcnt, time.Since(start))
	return nil
}

// preloadOneImage loads the image specified by the 'imageUrl' arg (e.g.: docker.io/foo:latest)
// into the cache, if not already cached. If already cached, nothing happens. In the case where the
// image url specifies a manifest list, the function retrieves from the manifest list the image
// manifest that matches the passed platform architecture and OS and also downloads that image
// manifest and the blobs for that image. So this function can perform zero, one, or two
// pulls from the upstream registry. The number of pulls is returned to the caller.
//
// If the image can't be pulled then a log entry is emanated but the function does not return
// an error.
func preloadOneImage(imageUrl string, imagePath string, platformArch string, platformOs string, pullTimeout int) (int, error) {
	itemcnt := 0
	pr, err := pullrequest.NewPullRequestFromUrl(imageUrl)
	if err != nil {
		return 0, fmt.Errorf("unable to parse image ref: %s", imageUrl)
	}
	// first HEAD could be a manifest list or an image manifest
	log.Infof("head remote: %s", pr.Url())
	d, err := upstream.CraneHead(pr.Url())
	if err != nil {
		log.Errorf("Error: %s", err)
		return itemcnt, nil
	}
	isImageManifest := upstream.IsImageManifest(string(d.MediaType))
	mh, found := serialize.MhFromFileSystem(d.Digest.Hex, isImageManifest, imagePath)
	if found {
		t := "image list"
		if isImageManifest {
			t = "image"
		}
		log.Infof("%s manifest already cached for: %s", t, pr.Url())
	} else {
		mh, err = upstream.Get(pr, imagePath, pullTimeout)
		if err != nil {
			log.Errorf("Error: %s", err)
			return itemcnt, nil
		}
		err = serialize.ToFilesystem(mh, imagePath)
		if err != nil {
			log.Errorf("Error: %s", err)
			// if we can't write the the file system we're probably not in good shape to keep running
			return itemcnt, err
		}
		itemcnt++
	}
	if mh.IsImageManifest() {
		// it's possible that the server will not return a manifest list
		return itemcnt, nil
	}
	// get the digest from the manifest list for the platform & os of interest
	// and see if an *image* manifest is cached for that digest
	digest, err := getImageManifestDigest(mh, platformArch, platformOs)
	if err != nil {
		log.Errorf("Error: %s", err)
		return itemcnt, nil
	}
	mh, found = serialize.MhFromFileSystem(digest, true, imagePath)
	if found {
		log.Infof("image manifest already cached for: %s", pr.Url())
		return itemcnt, nil
	}
	pr = pullrequest.NewPullRequest(pr.Org, pr.Image, digest, pr.Remote)
	mh, found = serialize.MhFromFileSystem(digest, true, imagePath)
	if found {
		log.Infof("image manifest already cached for: %s", pr.Url())
		return itemcnt, nil
	}
	log.Infof("get remote: %s", pr.Url())
	mh, err = upstream.Get(pr, imagePath, pullTimeout)
	if err != nil {
		log.Errorf("Error: %s", err)
		return itemcnt, nil
	}
	err = serialize.ToFilesystem(mh, imagePath)
	if err != nil {
		log.Errorf("Error: %s", err)
		return itemcnt, err
	}
	itemcnt++
	return itemcnt, nil
}

// getImageManifestDigest uses the passed platform architecture and os to select a
// manifest from the manifest list wrapped in the passed 'ManifestHolder'. If found,
// then  the digest is returned. If not found then the empty string and an error are
// returned.
func getImageManifestDigest(mh upstream.ManifestHolder, platformArch, platformOs string) (string, error) {
	if mh.Type == upstream.V2dockerManifestList {
		for _, m := range mh.V2dockerManifestList.Manifests {
			if m.Platform.Architecture == platformArch && m.Platform.OS == platformOs {
				return m.Digest, nil
			}
		}
	} else if mh.Type == upstream.V1ociIndex {
		for _, m := range mh.V1ociIndex.Manifests {
			if m.Platform.Architecture == platformArch && m.Platform.Os == platformOs {
				return m.Digest, nil
			}
		}
	} else {
		return "", fmt.Errorf("unexpected manifest type for url %s", mh.ImageUrl)
	}
	return "", fmt.Errorf("no manifest found for url %s matching arch=%s and os=%s", mh.ImageUrl, platformArch, platformOs)
}
