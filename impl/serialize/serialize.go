package serialize

import (
	"ociregistry/impl/upstream"
	"regexp"
)

func ToDisk(mh upstream.ManifestHolder, imagePath string) {
	a := []string{imagePath}
	a = append(a, regexp.MustCompile(`[:/]`).Split(mh.ImageUrl, -1)...)
}

/*
file system

NO!!!! JUST manifest-lists and image-manifests !!!
images
  pulls (temporary)
    uuid.tar
  manifest-lists
    repo
      org
        image
          tag
            manifest-file (compressed json)
      image
        tag
          manifest-file (compressed json)
  image-manifests
    digest
      manifest json (compressed json)
  blobs
    blob1
    blob2
    ...
if manifest list
  convert image url into path
  create all paths
  get manifest (image or list)
  write to disk
  if image manifest
    extract blobs


*/
