package cache

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/aceeric/imgpull/pkg/imgpull"
	"github.com/aceeric/ociregistry/impl/globals"
	"github.com/aceeric/ociregistry/impl/pullrequest"
	"github.com/aceeric/ociregistry/impl/serialize"

	log "github.com/sirupsen/logrus"
)

// TarballOutcome is what happened to one manifest in a tarball loaded by LoadTarball.
type TarballOutcome string

const (
	// TarballLoaded means the manifest was added to the cache, or replaced the cached manifest
	// for its url.
	TarballLoaded TarballOutcome = "loaded"
	// TarballAlreadyCached means the manifest was already cached for its url, so nothing changed.
	TarballAlreadyCached TarballOutcome = "cached"
	// TarballError means the manifest couldn't be loaded. Error has the reason.
	TarballError TarballOutcome = "error"
)

// TarballResult is the result of loading one manifest from a tarball.
type TarballResult struct {
	Url     string
	Digest  string
	Outcome TarballOutcome
	Error   string
}

// ErrResolveRef is returned by LoadTarball if a resolve ref is passed for a tarball that
// doesn't have exactly one manifest.
var ErrResolveRef = errors.New("a resolve ref is only supported for a tarball with one manifest")

// LoadTarball loads the images in the OCI-layout or docker-save tarball at tarPath, optionally
// gzip-compressed, into the cache of the running server: blobs and manifests on the file
// system, and the in-mem cache, as an upstream pull would. Only manifests for the passed
// platform are loaded (see imgpull.OpenImageTarBall). If resolveRef is not empty, the one
// manifest in the tarball is cached under that url instead of its own.
//
// Each manifest is loaded on its own, in the order the tarball reader yields them (an image
// list before its images), and has a result: loaded, already cached, or an error, which
// doesn't stop the others loading. A manifest that replaces a cached one with a different
// digest (a moved tag) leaves the old manifest's blobs without refs, so once the tarball is
// loaded they're removed, if no pull or prune is in progress (see trySweepBlobs).
//
// An error is returned, and nothing is loaded, if the tarball can't be opened or a resolve ref
// is passed for a tarball without exactly one manifest (ErrResolveRef).
func LoadTarball(tarPath, resolveRef, imagePath, platformOs, platformArch string, pullTimeout time.Duration) ([]TarballResult, error) {
	itb, err := imgpull.OpenImageTarBall(tarPath, platformOs, platformArch)
	if err != nil {
		return nil, err
	}
	defer itb.Close()
	if resolveRef != "" {
		cnt := 0
		for range itb.TarManifestReader() {
			cnt++
		}
		if cnt != 1 {
			return nil, fmt.Errorf("%w: the tarball has %d", ErrResolveRef, cnt)
		}
	}
	results := []TarballResult{}
	replaced := false
	for mh, err := range itb.TarManifestReader() {
		if err != nil {
			results = append(results, TarballResult{Outcome: TarballError, Error: err.Error()})
			continue
		}
		if resolveRef != "" {
			mh.ImageUrl = resolveRef
		}
		result := TarballResult{Url: mh.ImageUrl, Digest: mh.Digest}
		pr, err := pullrequest.NewPullRequestFromUrl(mh.ImageUrl)
		if err != nil {
			result.Outcome, result.Error = TarballError, fmt.Sprintf("unable to parse image ref %q", mh.ImageUrl)
			results = append(results, result)
			continue
		}
		mh.ImageUrl, result.Url = pr.Url(), pr.Url()
		outcome, rep, err := loadFromTarball(itb, pr, mh, imagePath, pullTimeout)
		if err != nil {
			log.Errorf("error loading %q from tarball %q: %s", pr.Url(), tarPath, err)
			result.Error = err.Error()
		} else {
			log.Infof("tarball manifest %s: %q (%s)", outcome, pr.Url(), mh.Digest)
		}
		result.Outcome = outcome
		replaced = replaced || rep
		results = append(results, result)
	}
	if replaced {
		trySweepBlobs(imagePath)
	}
	return results, nil
}

// loadFromTarball loads one image from the tarball. It takes part in the
// concurrent pulls protocol like an upstream pull of the same url (see
// becomePuller), so the tarball's manifest is the one left in the cache. Once
// it's the puller, a client request for the url waits for the manifest from the
// tarball rather than pulling it from the upstream, and is signaled when it's
// loaded (or failed to load). It returns the outcome, and whether a cached
// manifest with a different digest was replaced.
func loadFromTarball(itb *imgpull.ImageTarBall, pr pullrequest.PullRequest, mh imgpull.ManifestHolder, imagePath string, pullTimeout time.Duration) (TarballOutcome, bool, error) {
	if err := becomePuller(pr, pullTimeout); err != nil {
		return TarballError, false, err
	}
	defer signalWaiters(pr.Url())
	pullGate.RLock()
	defer pullGate.RUnlock()
	mc.Lock()
	cached, _, exists := lookup(imageUrl(pr.Url()))
	mc.Unlock()
	if exists && cached.Digest == mh.Digest {
		return TarballAlreadyCached, false, nil
	}
	mh.Created = globals.CurTime()
	// blobs first, so a manifest on the file system always has its blobs
	if mh.IsImageManifest() {
		if err := itb.SaveBlobs(mh, filepath.Join(imagePath, globals.BlobPath)); err != nil {
			return TarballError, false, err
		}
	}
	if err := serialize.MhToFilesystem(mh, imagePath, true); err != nil {
		return TarballError, false, err
	}
	if err := replaceInCache(pr, mh, imagePath); err != nil {
		return TarballError, false, err
	}
	return TarballLoaded, exists, nil
}
