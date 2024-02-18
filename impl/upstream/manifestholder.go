package upstream

func (mh *ManifestHolder) IsImageManifest() bool {
	return mh.Type == ImageManifestType
}
