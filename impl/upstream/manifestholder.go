package upstream

// IsImageManifest returns true if the passed 'ManifestHolder' is an image
// manifest, and false if it is a fat manifest (i.e. a manifest containing
// a list of image manifests.)
func (mh *ManifestHolder) IsImageManifest() bool {
	return mh.Type == V2dockerManifest || mh.Type == V1ociDescriptor
}
