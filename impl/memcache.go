package impl

import "sync"

var (
	mu               sync.Mutex
	pullRequestCache = make(map[string]bool)
)

func isCached(pr pullRequest) bool {
	mu.Lock()
	_, exists := pullRequestCache[pr.id()]
	mu.Unlock()
	return exists
}

func addToCache(pr pullRequest) {
	mu.Lock()
	pullRequestCache[pr.id()] = true
	mu.Unlock()
}
