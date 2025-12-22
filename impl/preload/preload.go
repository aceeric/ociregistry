package preload

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aceeric/ociregistry/impl/config"
	"github.com/aceeric/ociregistry/impl/globals"
	"github.com/aceeric/ociregistry/impl/pullrequest"
	"github.com/aceeric/ociregistry/impl/serialize"

	"github.com/aceeric/imgpull/pkg/imgpull"
	log "github.com/sirupsen/logrus"
)

// Load loads the manifest and blob file system cache for the images listed in the
// passed file. If an image is already present in cache, it is skipped. Otherwise the
// image is pulled from the upstream using the upstream registry encoded into the file
// entry.  Here is a example of what one entry in the image list file should look like.
// It's a standard repository URL. If you can 'docker pull' it, then it should be valid
// in the file:
//
//	registry.k8s.io/metrics-server/metrics-server:v0.6.2
//
// If the manifest for a url in the file is an image list, then the architecture and
// OS from configuration (args or config file) are used to select an image from the image
// list manifest, which is also downloaded. So each url in the file can pull either an image,
// or a manifest list and  an image. IMPORTANT: each item in the list MUST begin with a remote
// registry ref - i.e. to the left of the first forward slash (docker.io is not inferred.)
func Load(imageListFile string) error {
	imagePath := config.GetImagePath()
	platformArch := config.GetArch()
	platformOs := config.GetOs()

	itemcnt := 0
	start := time.Now()

	log.Infof("loading images from file: %s", imageListFile)

	f, err := os.Open(imageListFile)
	if err != nil {
		return err
	}
	defer f.Close()

	for scanner := bufio.NewScanner(f); scanner.Scan(); {
		imageUrl := strings.TrimSpace(string(scanner.Bytes()))
		if len(imageUrl) == 0 || strings.HasPrefix(imageUrl, "#") {
			continue
		}
		if cnt, err := doPull(imageUrl, imagePath, platformArch, platformOs); err != nil {
			log.Errorf("error loading image %q, the error was: %s", imageUrl, err)
			return err

		} else {
			itemcnt += cnt
		}
	}
	log.Infof("loaded %d images to the file system cache in %s", itemcnt, time.Since(start))
	return nil
}

// doPull pulls the passed url from the upstream registry ref'd by the url. If a manifest list
// comes back from the upstream then an image is also pulled. A return value of 1 means the passed
// url got an image. A return value of 2 means the passed url got an image list, and so an image
// for the passed OS and architecture was also pulled.
func doPull(imageUrl string, imagePath string, platformArch string, platformOs string) (int, error) {
	itemcnt := 0
	pr, err := pullrequest.NewPullRequestFromUrl(imageUrl)
	if err != nil {
		return itemcnt, fmt.Errorf("unable to parse image ref %q", imageUrl)
	}
	opts, err := config.ConfigFor(pr.Remote)
	if err != nil {
		return itemcnt, err
	}
	opts.Url = pr.Url()
	opts.OStype = platformOs
	opts.ArchType = platformArch
	puller, err := imgpull.NewPullerWith(opts)
	if err != nil {
		return itemcnt, err
	}
	md, err := puller.HeadManifest()
	if err != nil {
		return itemcnt, err
	}
	mh, cnt, err := getFromCacheOrRemote(puller, md.Digest, pr.IsLatest(), md.IsImageManifest(), imagePath)
	if err != nil {
		return itemcnt, err
	}
	itemcnt += cnt
	if mh.IsManifestList() {
		digest, err := mh.GetImageDigestFor(platformOs, platformArch)
		if err != nil {
			return itemcnt, err
		}
		if pr.PullType == pullrequest.ByDigest {
			// if the manifest list was pulled by digest rather than by tag, then set the ref for the image
			// manifest to be a digest as well
			puller.SetUrl(pr.UrlWithDigest(digest))
		}
		if mh, cnt, err = getFromCacheOrRemote(puller, digest, pr.IsLatest(), true, imagePath); err != nil {
			return itemcnt, err
		}
		itemcnt += cnt
	}
	return itemcnt, nil
}

// getFromCacheOrRemote first checks the file system for a manifest whose digest matches the
// passed 'digest' arg. If already present on the file system, then does nothing. Otherwise pulls
// from the upstream using the url in the passed puller and saves the manifest (and blobs if an
// image url) to the file system.
func getFromCacheOrRemote(puller imgpull.Puller, digest string, isLatest bool, isImageManifest bool, imagePath string) (imgpull.ManifestHolder, int, error) {
	if mh, found := serialize.MhFromFilesystem(digest, isLatest, imagePath); found {
		log.Infof("already cached: %s", puller.GetUrl())
		return mh, 0, nil
	}
	log.Infof("pulling %s", puller.GetUrl())
	var mh imgpull.ManifestHolder
	var err error
	if isImageManifest {
		mh, err = puller.GetManifestByDigest(digest)
	} else {
		mh, err = puller.GetManifest()
	}
	if err != nil {
		return imgpull.ManifestHolder{}, 0, err
	}
	if err := serialize.MhToFilesystem(mh, imagePath, false); err != nil {
		return imgpull.ManifestHolder{}, 0, err
	}
	if mh.IsImageManifest() {
		blobDir := filepath.Join(imagePath, globals.BlobPath)
		if err = puller.PullBlobs(mh, blobDir); err != nil {
			return mh, 0, err
		}
	}
	return mh, 1, nil
}
