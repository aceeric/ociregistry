package memcache

import (
	"ociregistry/impl/pullrequest"
	"ociregistry/impl/upstream"
	"strconv"
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
