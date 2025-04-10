package cache

import (
	"ociregistry/impl/helpers"
	"ociregistry/impl/pullrequest"
	"ociregistry/impl/serialize"

	"github.com/aceeric/imgpull/pkg/imgpull"
	log "github.com/sirupsen/logrus"
)

type PruneComparer func(imgpull.ManifestHolder) bool

func getManifestsToPrune(comparer PruneComparer) []imgpull.ManifestHolder {
	mc.Lock()
	defer mc.Unlock()
	mhs := make([]imgpull.ManifestHolder, 0, len(mc.manifests))
	for _, mh := range mc.manifests {
		if comparer(mh) {
			mhs = append(mhs, mh)
		}
	}
	return mhs
}

// prune removes the passed manifest and decrements blob ref counts from the in-mem
// cache (if the manifest is an image manifest.)
func prune(pr pullrequest.PullRequest, mh imgpull.ManifestHolder, imagePath string) {
	mc.Lock()
	defer mc.Unlock()
	delManifestFromCache(pr, mh, imagePath)
	if mh.IsImageManifest() {
		bc.Lock()
		defer bc.Unlock()
		decBlobRefs(mh)
	}
}

// delManifestFromCache removes the passed manifest from the manifest cache and the
// file system. If the manifest is by tag, then the by-digest manifest is also removed from in-mem
// cache. (It only exists once on the file system.)
func delManifestFromCache(pr pullrequest.PullRequest, mh imgpull.ManifestHolder, imagePath string) {
	delete(mc.manifests, pr.Url())
	if pr.PullType == pullrequest.ByTag {
		delete(mc.manifests, pr.UrlWithDigest("sha256:"+mh.Digest))
		if err := serialize.RmManifest(imagePath, mh); err != nil {
			log.Errorf("error removing manifest %q from the file system. the error was: %s", pr.Url(), err)
		}
	}
}

// TODO if refcnt == 0 then remove and remove from filesystem
// decBlobRefs decrements the ref count for all blobs in the passed image manifest.
func decBlobRefs(mh imgpull.ManifestHolder) {
	for _, layer := range mh.Layers() {
		digest := helpers.GetDigestFrom(layer.Digest)
		bc.blobs[digest]--
	}
}
