package memcache

import (
	"ociregistry/impl/pullrequest"
	"ociregistry/impl/upstream"
	"sync"
)

type PRCache struct {
	sync.Mutex
	pullRequestCache map[string]upstream.ManifestHolder
}

var (
	prCache = PRCache{
		pullRequestCache: make(map[string]upstream.ManifestHolder),
	}
)

func IsCached(pr pullrequest.PullRequest) (upstream.ManifestHolder, bool) {
	prCache.Lock()
	mh, exists := prCache.pullRequestCache[pr.Id()]
	prCache.Unlock()
	return mh, exists
}

// AddToCache adds the pull request to the in-mem cache. The PR is added two ways:
// the way it came in (e.g. "coredns/coredns:1.11.1") and by digest from the manifest
// (e.g. coredns/coredns@sha256:nnn) because the client will likely HEAD the manifest
// using the tag then GET by SHA
func AddToCache(pr pullrequest.PullRequest, mh upstream.ManifestHolder, withlock bool) {
	if withlock {
		prCache.Lock()
		defer prCache.Unlock()
	}
	prCache.pullRequestCache[pr.Id()] = mh
	prCache.pullRequestCache[pr.IdDigest("sha256:"+mh.Digest)] = mh
}

func GetCache() *PRCache {
	return &prCache
}
