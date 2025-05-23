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

// PullRequest has the individual components of an image pull. If initialized with
// 'index.docker.io/hello-world:latest' then the struct members are like so:
//
//	PullType  = ByTag
//	Org       = ""
//	Image     = hello-world
//	Reference = latest
//	Remote    = docker.io
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
		Org:       strings.ToLower(org),
		Image:     strings.ToLower(image),
		Reference: strings.ToLower(reference),
		Remote:    strings.ToLower(remote),
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
		Org:       strings.ToLower(org),
		Image:     strings.ToLower(img),
		Reference: strings.ToLower(ref),
		Remote:    strings.ToLower(remote),
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

// If the receiver has "docker.io/library/..." then returns "docker.io/...". If receiver has
// "docker.io/..." then returns "docker.io/library/...". If neither of those cases are true,
// returns the empty string.
func (pr *PullRequest) AltDockerUrl() string {
	if pr.Remote != "docker.io" {
		return ""
	}
	separator := ":"
	if strings.HasPrefix(pr.Reference, "sha256:") {
		separator = "@"
	}
	if pr.Org == "library" {
		return fmt.Sprintf("%s/%s%s%s", pr.Remote, pr.Image, separator, pr.Reference)
	}
	return fmt.Sprintf("%s/%s/%s%s%s", pr.Remote, "library", pr.Image, separator, pr.Reference)
}

// UrlWithDigest is like Url except it overrides the ref in the receiver with the passed digest
func (pr *PullRequest) UrlWithDigest(digest string) string {
	separator := "@"
	if pr.Org == "" {
		return fmt.Sprintf("%s/%s%s%s", pr.Remote, pr.Image, separator, digest)
	}
	return fmt.Sprintf("%s/%s/%s%s%s", pr.Remote, pr.Org, pr.Image, separator, digest)
}

// IsLatest returns true if the ref in the receiver has tag "latest"
func (pr *PullRequest) IsLatest() bool {
	return strings.ToLower(pr.Reference) == "latest"
}

// typeFromRef looks at the passed 'ref' and if it's a digest ref then returns
// 'ByDigest' else returns 'ByTag'.
func typeFromRef(ref string) PullType {
	if strings.HasPrefix(ref, "sha256:") {
		return ByDigest
	}
	return ByTag
}
