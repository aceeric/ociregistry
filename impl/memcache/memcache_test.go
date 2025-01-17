package memcache

import (
	"ociregistry/impl/helpers"
	"ociregistry/impl/pullrequest"
	"ociregistry/impl/upstream"
	"strconv"
	"sync"
	"testing"
)

func TestCaching(t *testing.T) {
	for i := 0; i < 10000; i++ {
		pr := pullrequest.NewPullRequest("org", "image", strconv.Itoa(i), "")
		mh := upstream.ManifestHolder{
			Size: i,
		}
		AddToCache(pr, mh, true)
	}
	idx := 5432
	mh, exists := IsCached(pullrequest.NewPullRequest("org", "image", strconv.Itoa(idx), ""))
	if !exists {
		t.Fail()
	}
	if mh.Size != idx {
		t.Fail()
	}
}

func TestShaCache(t *testing.T) {
	tests := []struct {
		ref string
		cnt int
	}{
		{ref: "sha256:6e75a10070b0fcb0bead763c5118a369bc7cc30dfc1b0749c491bbb21f15c3c7", cnt: 1},
		{ref: "v1.2.3", cnt: 2},
	}
	for _, tst := range tests {
		prCache.pullRequestCache = make(map[string]upstream.ManifestHolder)
		pr := pullrequest.NewPullRequest("", "ubuntu", tst.ref, "")
		AddToCache(pr, upstream.ManifestHolder{Digest: helpers.GetDigestFrom(tests[0].ref)}, true)
		if len(prCache.pullRequestCache) != tst.cnt {
			t.Fail()
		}
	}
}

// When adding the same number of identical sha refs only one should be
// added and all others should hit up against the first one added.
func TestConcurrentAdd(t *testing.T) {
	prCache.pullRequestCache = make(map[string]upstream.ManifestHolder)
	digest := "123"
	ref := "sha256:" + digest
	var wg sync.WaitGroup
	goroutines := 10
	hits := make([]int, goroutines)
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			pr := pullrequest.NewPullRequest("", "ubuntu", ref, "docker.io")
			hits[i] = AddToCache(pr, upstream.ManifestHolder{Digest: digest}, true)
		}(i)
	}
	wg.Wait()
	totalHits := 0
	for _, hit := range hits {
		totalHits += hit
	}
	if totalHits != goroutines-1 {
		t.Fail()
	}
}
