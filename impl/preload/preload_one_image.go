package preload

import (
	"fmt"
	"ociregistry/impl/config"
	"ociregistry/impl/globals"
	"ociregistry/impl/pullrequest"
	"ociregistry/impl/serialize"
	"path/filepath"

	"github.com/aceeric/imgpull/pkg/imgpull"
)

func preloadOneImage(imageUrl string, imagePath string, platformArch string, platformOs string, _ int) (int, error) {
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
	mh, err := getFromCacheOrRemote(puller, md.Digest, md.IsImageManifest(), imagePath, &itemcnt)
	if err != nil {
		return itemcnt, err
	}
	if mh.IsManifestList() {
		digest, err := mh.GetImageDigestFor(platformOs, platformArch)
		if err != nil {
			return itemcnt, err
		}
		if mh, err = getFromCacheOrRemote(puller, digest, true, imagePath, &itemcnt); err != nil {
			return itemcnt, err
		}
	}
	return itemcnt, nil
}

func getFromCacheOrRemote(puller imgpull.Puller, digest string, isImageManifest bool, imagePath string, cnt *int) (imgpull.ManifestHolder, error) {
	if mh, found := serialize.MhFromFilesystem(digest, isImageManifest, imagePath); found {
		return mh, nil
	}
	var mh imgpull.ManifestHolder
	var err error
	if isImageManifest {
		mh, err = puller.GetManifestByDigest(digest)
	} else {
		mh, err = puller.GetManifest()
	}
	if err != nil {
		return imgpull.ManifestHolder{}, err
	}
	serialize.MhToFilesystem(mh, imagePath, false)
	if mh.IsImageManifest() {
		blobDir := filepath.Join(imagePath, globals.BlobsDir)
		if err = puller.PullBlobs(mh, blobDir); err != nil {
			return mh, err
		}
	}
	*cnt++
	return mh, nil
}
