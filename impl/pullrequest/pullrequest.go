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
// 'quay.io/argoproj/argocd:v2.11.11' then the struct members are like so:
//
//	PullType   = ByTag
//	Repository = argoproj/argocd
//	Reference  = v2.11.11
//	Remote     = quay.io
type PullRequest struct {
	// PullType indicates pull by tag or digest. E.g. if initialized with quay.io/argoproj/argocd:v2.11.11
	// the this field has value 'ByTag'
	PullType PullType
	// Repository is the repository. E.g. if initialized with quay.io/argoproj/argocd:v2.11.11
	// the this field has value 'argoproj/argocd'
	Repository string
	// Reference is the tag or digest. E.g. if initialized with quay.io/argoproj/argocd:v2.11.11
	// the this field has value 'v2.11.11'
	Reference string
	// Remote is the remote host. E.g. if initialized with quay.io/argoproj/argocd:v2.11.11
	// the this field has value 'quay.io'
	Remote string
}

// NewPullRequest returns a 'PullRequest' struct from the passed args. It is intended to be used
// to parse the REST API components into URL components.
//
//	regHdr    If the X-Registry HTTP header was set, this is the value, else empty string
//	ns        If the ?ns= query param was set, this is a pointer to the value, else nil
//	defaultNs If the server was started with the --default-ns arg, this is the value, else nil
//	reference Tag or digest. Never empty.
//	segments  if docker pull ociregistryhost:8080/docker.io/foo/bar:latest then this
//	          arg has []string{"docker.io","foo","bar"}
func NewPullRequest(regHdr string, ns *string, defaultNs string, reference string, segments ...string) (PullRequest, error) {
	pr := PullRequest{
		PullType:  typeFromRef(reference),
		Reference: strings.ToLower(reference),
	}
	frst := 0
	switch {
	case regHdr != "":
		pr.Remote = regHdr
	case ns != nil:
		pr.Remote = *ns
	case strings.Contains(segments[0], "."):
		// interpret left-most segment as namespace if it has a period (e.g. quay.io)
		pr.Remote = segments[0]
		frst = 1
	case defaultNs != "":
		pr.Remote = defaultNs
	default:
		return pr, fmt.Errorf("unable to extract remote from segments: %v", segments)
	}
	if strings.Contains(segments[frst], ".") {
		return pr, fmt.Errorf("two namespaces: %s and %s", pr.Remote, segments[frst])

	}
	pr.Repository = strings.Join((segments[frst:]), "/")
	return pr, nil
}

// NewPullRequestFromUrl parses the passed image url (e.g. docker.io/hello-world:latest,
// or docker.io/library/hello-world@sha256:...) into a 'PullRequest' struct. The url
// MUST begin with a registry ref (e.g. quay.io). The right-most component MUST be a tag
// or digest. Everything in between is the repository.
func NewPullRequestFromUrl(url string) (PullRequest, error) {
	before, after, found := strings.Cut(url, "/")
	if !found || after == "" || !strings.Contains(before, ".") {
		return PullRequest{}, fmt.Errorf("unable to parse image url: %s", url)
	}
	remote := before
	repository := ""
	ref := ""
	for _, sep := range []string{"@", ":"} {
		if strings.Contains(after, sep) {
			tmp := strings.Split(after, sep)
			repository = tmp[0]
			ref = tmp[1]
			break
		}
	}
	if repository == "" {
		return PullRequest{}, fmt.Errorf("unable to parse image url: %s", url)
	}
	return PullRequest{
		PullType:   typeFromRef(ref),
		Repository: strings.ToLower(repository),
		Reference:  strings.ToLower(ref),
		Remote:     strings.ToLower(remote),
	}, nil
}

// Url formats the instance as an image reference like 'quay.io/appzygy/ociregistry:n.n.n'
func (pr *PullRequest) Url() string {
	separator := ":"
	if strings.HasPrefix(pr.Reference, "sha256:") {
		separator = "@"
	}
	return fmt.Sprintf("%s/%s%s%s", pr.Remote, pr.Repository, separator, pr.Reference)
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
	before, after, found := strings.Cut(pr.Repository, "/")
	if found && before == "library" {
		return fmt.Sprintf("%s/%s%s%s", pr.Remote, after, separator, pr.Reference)
	}
	return fmt.Sprintf("%s/%s/%s%s%s", pr.Remote, "library", pr.Repository, separator, pr.Reference)
}

// UrlWithDigest is like Url except it overrides the ref in the receiver with the passed digest
func (pr *PullRequest) UrlWithDigest(digest string) string {
	separator := "@"
	return fmt.Sprintf("%s/%s%s%s", pr.Remote, pr.Repository, separator, digest)
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
