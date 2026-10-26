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
// to parse the REST API components into URL components. The result is canonical: see canonicalize.
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
		Reference: reference,
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
	if frst >= len(segments) {
		return pr, fmt.Errorf("no image name after namespace: %s", pr.Remote)
	}
	if strings.Contains(segments[frst], ".") {
		return pr, fmt.Errorf("two namespaces: %s and %s", pr.Remote, segments[frst])

	}
	pr.Repository = strings.Join((segments[frst:]), "/")
	pr.canonicalize()
	return pr, nil
}

// NewPullRequestFromUrl parses the passed image url (e.g. docker.io/hello-world:latest,
// or docker.io/library/hello-world@sha256:...) into a 'PullRequest' struct. The url
// MUST begin with a registry ref (e.g. quay.io). The right-most component MUST be a tag
// or digest. Everything in between is the repository. The result is canonical: see
// canonicalize.
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
	pr := PullRequest{
		PullType:   typeFromRef(ref),
		Repository: repository,
		Reference:  ref,
		Remote:     remote,
	}
	pr.canonicalize()
	return pr, nil
}

// canonicalize puts the receiver in the one form used to identify an image everywhere in the
// server (cache keys, file names, concurrent-pull tracking, pattern matching):
//
//   - The registry and repository are lowercased. Both are case-insensitive by spec.
//   - A digest is lowercased. A tag keeps its case, because tags are case-sensitive: RC1 and rc1
//     are different tags.
//   - A single-component docker.io repository gets the implicit "library/" prefix, so
//     docker.io/hello-world:latest and docker.io/library/hello-world:latest are the same image.
func (pr *PullRequest) canonicalize() {
	pr.Remote = strings.ToLower(pr.Remote)
	pr.Repository = strings.ToLower(pr.Repository)
	if pr.PullType == ByDigest {
		pr.Reference = strings.ToLower(pr.Reference)
	}
	if pr.Remote == "docker.io" && !strings.Contains(pr.Repository, "/") {
		pr.Repository = "library/" + pr.Repository
	}
}

// Url formats the instance as an image reference like 'quay.io/appzygy/ociregistry:n.n.n'
func (pr *PullRequest) Url() string {
	separator := ":"
	if strings.HasPrefix(pr.Reference, "sha256:") {
		separator = "@"
	}
	return fmt.Sprintf("%s/%s%s%s", pr.Remote, pr.Repository, separator, pr.Reference)
}

// UrlWithDigest is like Url except it overrides the ref in the receiver with the passed digest
func (pr *PullRequest) UrlWithDigest(digest string) string {
	separator := "@"
	return fmt.Sprintf("%s/%s%s%s", pr.Remote, pr.Repository, separator, digest)
}

// IsLatest returns true if the ref in the receiver has tag "latest". Tags are case-sensitive,
// so "Latest" is a different tag.
func (pr *PullRequest) IsLatest() bool {
	return pr.Reference == "latest"
}

// typeFromRef looks at the passed 'ref' and if it's a digest ref then returns
// 'ByDigest' else returns 'ByTag'.
func typeFromRef(ref string) PullType {
	if strings.HasPrefix(strings.ToLower(ref), "sha256:") {
		return ByDigest
	}
	return ByTag
}
