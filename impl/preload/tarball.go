package preload

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/aceeric/imgpull/pkg/imgpull"
	"github.com/aceeric/ociregistry/impl/config"
	"github.com/aceeric/ociregistry/impl/globals"
	"github.com/aceeric/ociregistry/impl/pullrequest"
	"github.com/aceeric/ociregistry/impl/serialize"

	log "github.com/sirupsen/logrus"
)

// LoadTarball loads every image found in the OCI-layout or docker-save
// tarball at tarPath into the file system cache, the same way Load does for
// a text file of image URLs. resolveRef allows to replace the image url in the
// tarball with an override from the command line. Since the server stores manifest
// by digest, if you override the url for a manifest that is already cached by
// digest, the image is skipped. This feature is only supported for single entry
// tarballs and will error otherwise.
func LoadTarball(tarPath string, resolveRef string) error {
	imagePath := config.GetImagePath()
	blobDir := filepath.Join(imagePath, globals.BlobPath)
	platformOs := config.GetOs()
	platformArch := config.GetArch()

	itemcnt := 0
	start := time.Now()
	log.Infof("loading images from tarball: %s", tarPath)

	itb, err := imgpull.OpenImageTarBall(tarPath, platformOs, platformArch)
	if err != nil {
		return err
	}
	defer itb.Close()

	if err := chkResolveRef(itb, resolveRef, tarPath); err != nil {
		return err
	}

	for mh, err := range itb.TarManifestReader() {
		if err != nil {
			return err
		}
		if resolveRef != "" {
			mh.ImageUrl = resolveRef
		}
		pr, err := pullrequest.NewPullRequestFromUrl(mh.ImageUrl)
		if err != nil {
			return fmt.Errorf("unable to parse image ref %q", mh.ImageUrl)
		}
		if _, found := serialize.MhFromFilesystem(mh.Digest, pr.IsLatest(), imagePath); found {
			log.Infof("already cached: %s (%s)", mh.ImageUrl, mh.Digest)
			continue
		}
		log.Infof("saving image: %s", mh.ImageUrl)
		mh.Created = globals.CurTime()
		itemcnt++
		if err := serialize.MhToFilesystem(mh, imagePath, false); err != nil {
			return err
		}
		if mh.IsImageManifest() {
			if err := itb.SaveBlobs(mh, blobDir); err != nil {
				return err
			}
		}
	}
	log.Infof("loaded %d images from tarball %q to the file system cache in %s", itemcnt, tarPath, time.Since(start))
	return nil
}

// chkResolveRef guards against applying a single fixed --resolve-ref
// override across more than one manifest in a single tarball
func chkResolveRef(itb *imgpull.ImageTarBall, resolveRef string, tarPath string) error {
	if resolveRef == "" {
		return nil
	}
	count := 0
	for _, err := range itb.TarManifestReader() {
		if err != nil {
			return err
		}
		count++
	}
	if count != 1 {
		return fmt.Errorf("--resolve-ref was supplied but %q contains %d manifests; only supported for single-image tarballs", tarPath, count)
	}
	return nil
}
