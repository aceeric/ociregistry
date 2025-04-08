package cache

import (
	"ociregistry/impl/helpers"
	"ociregistry/impl/pullrequest"

	"github.com/aceeric/imgpull/pkg/imgpull"
)

// TODO CONSIDER ONE LOCK COVERING BOTH MANIFEST AND BLOBS
// THEN BLOBS COULD BE REMOVED IF DEC TO ZERO...

// prune removes the manifest and blobs
func prune(pr pullrequest.PullRequest, mh imgpull.ManifestHolder) {
	delManifestFromCache(pr, mh.Digest)
	decBlobRef(mh)
}

// TODO delete from file system (or optionally delete since don't need to
// delete on a force pull because prior will be overwritten)
// this could delete a manifest right after it was pulled and waiters
// are waiting which could cause the waiters to return
func delManifestFromCache(pr pullrequest.PullRequest, digest string) {
	mc.Lock()
	defer mc.Unlock()
	delete(mc.manifests, pr.Url())
	if pr.PullType == pullrequest.ByTag {
		delete(mc.manifests, pr.UrlWithDigest("sha256:"+digest))
	}
}

// blobs can only be decremented here for now
func decBlobRef(mh imgpull.ManifestHolder) {
	if mh.IsManifestList() {
		return
	}
	bc.Lock()
	defer bc.Unlock()
	for _, layer := range mh.Layers() {
		digest := helpers.GetDigestFrom(layer.Digest)
		bc.blobs[digest] = bc.blobs[digest] - 1
	}
}
