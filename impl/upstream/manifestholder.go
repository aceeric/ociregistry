package upstream

func (mh *ManifestHolder) IsImageManifest() bool {
	return mh.Type == V2dockerManifest || mh.Type == V1ociDescriptor
}
