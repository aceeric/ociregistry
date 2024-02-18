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

func AddToCache(pr pullrequest.PullRequest, mh upstream.ManifestHolder) {
	prCache.Lock()
	prCache.pullRequestCache[pr.Id()] = mh
	prCache.Unlock()
}

func GetCache() *PRCache {
	return &prCache
}

func AddToCacheWithoutLock(pr pullrequest.PullRequest, mh upstream.ManifestHolder) {
	prCache.pullRequestCache[pr.Id()] = mh
}
