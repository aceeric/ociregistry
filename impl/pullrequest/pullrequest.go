package pullrequest

import (
	"fmt"
	"strings"
)

// PullType allows to differentiate a pull by tag vs. digest.
type PullType int

const (
	ByTag PullType = iota
	ByDigest
)

// PullRequest has the individual components of an image pull
type PullRequest struct {
	PullType  PullType
	Org       string
	Image     string
	Reference string
	Remote    string
}

// NewPullRequest returns a 'PullRequest' struct from the passed args
func NewPullRequest(org, image, reference, remote string) PullRequest {
	return PullRequest{
		PullType:  typeFromRef(reference),
		Org:       org,
		Image:     image,
		Reference: reference,
		Remote:    remote,
	}
}

// NewPullRequestFromUrl parses the passed image url (e.g. docker.io/hello-world:latest,
// or docker.io/library/hello-world@sha256:...) into a 'PullRequest' struct. The url
// MUST begin with a registry ref (e.g. quay.io) - it is not inferred.
func NewPullRequestFromUrl(url string) (PullRequest, error) {
	parts := strings.Split(url, "/")
	remote := parts[0]
	org := ""
	img := ""
	ref := ""
	if len(parts) == 2 {
		org = ""
		img = parts[1]
	} else if len(parts) == 3 {
		org = parts[1]
		img = parts[2]
	} else {
		return PullRequest{}, fmt.Errorf("unable to parse image url: %s", url)
	}
	for idx, sep := range []string{"@", ":", ""} {
		if idx == 2 {
			return PullRequest{}, fmt.Errorf("unable to parse image url: %s", url)
		}
		if strings.Contains(img, sep) {
			tmp := strings.Split(img, sep)
			img = tmp[0]
			ref = tmp[1]
			break
		}
	}
	return PullRequest{
		PullType:  typeFromRef(ref),
		Org:       org,
		Image:     img,
		Reference: ref,
		Remote:    remote,
	}, nil
}

// Url formats the instance as an image reference like 'quay.io/appzygy/ociregistry:n.n.n'
func (pr *PullRequest) Url() string {
	separator := ":"
	if strings.HasPrefix(pr.Reference, "sha256:") {
		separator = "@"
	}
	if pr.Org == "" {
		return fmt.Sprintf("%s/%s%s%s", pr.Remote, pr.Image, separator, pr.Reference)
	}
	return fmt.Sprintf("%s/%s/%s%s%s", pr.Remote, pr.Org, pr.Image, separator, pr.Reference)
}

// UrlWithDigest is like Url except it overrides the ref with the passed digest
func (pr *PullRequest) UrlWithDigest(digest string) string {
	separator := "@"
	if pr.Org == "" {
		return fmt.Sprintf("%s/%s%s%s", pr.Remote, pr.Image, separator, digest)
	}
	return fmt.Sprintf("%s/%s/%s%s%s", pr.Remote, pr.Org, pr.Image, separator, digest)
}

// Id formats the instance as a slash-separated compound key. E.g. url 'calico/node:v1.23.0'
// becomes key '/calico/node/v1.23.0' and url 'hello-world:v1.0.0' becomes key '/hello-world/v1.0.0'.
// For SHA-based pulls, 'foo/bar@sha256:a15f3c...' becomes key 'foo/bar/sha256:a15f3c...'. Note
// that if there is no org, the Id begins with a forward slash character.
func (pr *PullRequest) Id() string {
	return fmt.Sprintf("%s/%s/%s", pr.Org, pr.Image, pr.Reference)
}

// IdDigest is like 'Id' except it only operates on digest pulls. E.g. 'foo/bar@sha256:a15f3c...'
// is returned as key 'foo/bar/sha256:a15f3c...'.
func (pr *PullRequest) IdDigest(digest string) string {
	return fmt.Sprintf("%s/%s/%s", pr.Org, pr.Image, digest)
}

// typeFromRef looks at the passed 'ref' and if it's a digest ref then returns
// 'byDigest' else returns 'byTag'.
func typeFromRef(ref string) PullType {
	if strings.HasPrefix(ref, "sha256:") {
		return ByDigest
	}
	return ByTag
}
