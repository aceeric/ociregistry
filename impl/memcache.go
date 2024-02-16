package impl

import (
	"ociregistry/impl/pullrequest"
	"ociregistry/impl/upstream"
	"sync"
)

var (
	prCache = struct {
		sync.Mutex
		pullRequestCache map[string]upstream.ManifestHolder
	}{
		pullRequestCache: make(map[string]upstream.ManifestHolder),
	}
)

func isCached(pr pullrequest.PullRequest) (upstream.ManifestHolder, bool) {
	prCache.Lock()
	fb, exists := prCache.pullRequestCache[pr.Id()]
	prCache.Unlock()
	return fb, exists
}

func addToCache(pr pullrequest.PullRequest, fb upstream.ManifestHolder) {
	prCache.Lock()
	prCache.pullRequestCache[pr.Id()] = fb
	prCache.Unlock()
}
