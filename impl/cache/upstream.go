package cache

import (
	"fmt"
	"time"

	"github.com/aceeric/imgpull/pkg/imgpull"
	"github.com/aceeric/ociregistry/impl/pullrequest"
)

// LoadFromUpstream pulls the manifest for the url (and its blobs, for an image manifest) from
// the upstream into the running server's cache, replacing the cached manifest if there is one,
// e.g. a tag that moved. It's for a load that isn't a client's, such as the preload watcher's,
// so unlike a client pull (GetManifest) it doesn't set the manifest's pull date, it never
// serves a cached copy if the pull fails, and it only reads the cache without touching pull
// dates. Like a client pull, it takes part in the concurrent pulls protocol (see
// becomePuller), and holds pullGate shared while it saves the blobs and the manifest. A moved
// tag leaves the old manifest's blobs without refs, so they're removed afterwards if no pull or
// prune is in progress (see trySweepBlobs).
func LoadFromUpstream(pr pullrequest.PullRequest, imagePath string, pullTimeout time.Duration) (imgpull.ManifestHolder, error) {
	if err := becomePuller(pr, pullTimeout); err != nil {
		return emptyManifestHolder, err
	}
	defer signalWaiters(pr.Url())
	mh, replaced, err := pullFromUpstream(pr, imagePath)
	if err != nil {
		return emptyManifestHolder, err
	}
	if replaced {
		trySweepBlobs(imagePath)
	}
	return mh, nil
}

// pullFromUpstream does LoadFromUpstream's pull under pullGate, and returns the manifest and
// whether it replaced a cached manifest with a different digest. The caller must be the puller
// for the url.
func pullFromUpstream(pr pullrequest.PullRequest, imagePath string) (imgpull.ManifestHolder, bool, error) {
	pullGate.RLock()
	defer pullGate.RUnlock()
	mh, err := doPull(pr, imagePath, false)
	if err != nil {
		return emptyManifestHolder, false, err
	}
	mc.Lock()
	cached, _, exists := mc.lookup(imageUrl(pr.Url()))
	mc.Unlock()
	if err := replaceInCache(pr, mh, imagePath); err != nil {
		return emptyManifestHolder, false, err
	}
	return mh, exists && cached.Digest != mh.Digest, nil
}

// becomePuller makes the caller the puller for the url in the concurrent pulls protocol
// (enqueuePull), for a load that must leave its own manifest in the cache, such as from a
// tarball or a forced pull from the upstream. If another goroutine is pulling the url, it waits
// for that pull to finish, up to pullTimeout, and tries again: so the caller's manifest is the
// one left in the cache, and it doesn't replace it outside the protocol, where a third request
// could get in between. Once it returns nil, the caller must call signalWaiters for the url
// when done, which wakes the client requests that waited on it.
func becomePuller(pr pullrequest.PullRequest, pullTimeout time.Duration) error {
	for {
		ch, er := enqueuePull(pr)
		if er == mustPull {
			return nil
		}
		select {
		case <-ch:
		case <-time.After(pullTimeout):
			return fmt.Errorf("timed out waiting for an in-progress pull of %q", pr.Url())
		}
	}
}
