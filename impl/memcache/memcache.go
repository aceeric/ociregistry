package memcache

import (
	"ociregistry/impl/pullrequest"
	"ociregistry/impl/upstream"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
)

// PRCache has a synchronization object and a map of cached manifests
type PRCache struct {
	sync.Mutex
	pullRequestCache map[string]upstream.ManifestHolder
}

// prCache is the synchronized in-memory representation of all cached manifests.
var prCache = PRCache{
	pullRequestCache: make(map[string]upstream.ManifestHolder),
}

// IsCached checks the cache for the passed PR and if found returns true
// and the associated manifest holder, else returns false and an empty
// manifest holder
func IsCached(pr pullrequest.PullRequest) (upstream.ManifestHolder, bool) {
	prCache.Lock()
	mh, exists := prCache.pullRequestCache[pr.Id()]
	prCache.Unlock()
	return mh, exists
}

// AddToCache adds the pull request to the in-mem cache. The PR is added two ways:
// the way it came in (e.g. "coredns/coredns:1.11.1") and by digest from the manifest
// (e.g. coredns/coredns@sha256:nnn) This supports get by tag and by digest. In the case
// where a pull is by digest then only one entry is placed in the mem cache. The
// 'withlock' arg can be set to false if you know that the code path that invokes this
// function is single-threaded.
func AddToCache(pr pullrequest.PullRequest, mh upstream.ManifestHolder, withlock bool) int {
	hits := 0
	if withlock {
		prCache.Lock()
		defer prCache.Unlock()
	}
	for _, key := range []string{pr.Id(), pr.IdDigest("sha256:" + mh.Digest)} {
		if _, exists := prCache.pullRequestCache[key]; !exists {
			log.Debugf("add entry to mem cache: %s", key)
			prCache.pullRequestCache[key] = mh
		} else {
			hits++
			log.Debugf("already cached: %s", key)
		}
		if strings.Contains(key, "/sha256:") {
			break
		}
	}
	return hits
}
