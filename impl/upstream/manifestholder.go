package upstream

import "ociregistry/impl/helpers"

// IsImageManifest returns true if the passed 'ManifestHolder' is an image
// manifest, and false if it is a fat manifest (i.e. a manifest containing
// a list of image manifests.)
func (mh *ManifestHolder) IsImageManifest() bool {
	return mh.Type == V2dockerManifest || mh.Type == V1ociDescriptor
}

// ImageManifestDigests returns an array of the image manifest digests from
// the image list manifest wrapped by the the passed manifest holder. If called
// for a manifest holder wrapping an image manifest, then an empty array is
// returned because only image list manifests have lists of image manifests.
func (mh *ManifestHolder) ImageManifestDigests() []string {
	ims := []string{}
	if !mh.IsImageManifest() {
		switch mh.Type {
		case V2dockerManifestList:
			for _, m := range mh.V2dockerManifestList.Manifests {
				ims = append(ims, m.Digest)
			}
		case V1ociIndex:
			for _, m := range mh.V1ociIndex.Manifests {
				ims = append(ims, m.Digest)
			}
		}
	}
	return ims
}

// ManifestBlobs returns an array of all blobs from the image manifest
// wrapped in the passed holder. The Config blob is also returned.
func (mh *ManifestHolder) ManifestBlobs() []string {
	blobs := []string{}
	if mh.IsImageManifest() {
		switch mh.Type {
		case V2dockerManifest:
			blobs = append(blobs, mh.V2dockerManifest.Config.Digest)
			for _, l := range mh.V2dockerManifest.Layers {
				blobs = append(blobs, helpers.GetDigestFrom(l.Digest))
			}
		case V1ociDescriptor:
			blobs = append(blobs, mh.V1ociManifest.Config.Digest)
			for _, l := range mh.V1ociManifest.Layers {
				blobs = append(blobs, helpers.GetDigestFrom(l.Digest))
			}
		}
	}
	for idx, blob := range blobs {
		blobs[idx] = helpers.GetDigestFrom(blob)
	}
	return blobs
}
