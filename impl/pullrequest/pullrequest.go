package pullrequest

import (
	"fmt"
	"strings"
)

const (
	byTag int = iota
	byDigest
)

type PullRequest struct {
	pullType  int
	org       string
	image     string
	reference string
	remote    string
}

func NewPullRequest(org, image, reference, remote string) PullRequest {
	return PullRequest{
		pullType:  typeFromRef(reference),
		org:       org,
		image:     image,
		reference: reference,
		remote:    remote,
	}
}

func (pr *PullRequest) Url() string {
	separator := ":"
	if strings.HasPrefix(pr.reference, "sha256:") {
		separator = "@"
	}
	if pr.org == "" {
		return fmt.Sprintf("%s/%s%s%s", pr.remote, pr.image, separator, pr.reference)
	}
	return fmt.Sprintf("%s/%s/%s%s%s", pr.remote, pr.org, pr.image, separator, pr.reference)
}

func (pr *PullRequest) isByTag() bool {
	return pr.pullType == byTag
}

func (pr *PullRequest) isByDigest() bool {
	return pr.pullType == byDigest
}

// calico/node:v1.23.0 becomes "calico/node/v1.23.0 and"
// hello-world:v1.0.0 becomes "/hello-world/v1.0.0"
// foo/bar@sha256:a15f3c... becomes "foo/bar/sha256:a15f3c..."
func (pr *PullRequest) Id() string {
	return fmt.Sprintf("%s/%s/%s/", pr.org, pr.image, pr.reference)
}

func ByTag() int {
	return byTag
}

func ByDigest() int {
	return byDigest
}

func typeFromRef(ref string) int {
	if strings.HasPrefix(ref, "sha256:") {
		return ByDigest()
	}
	return ByTag()

}

func isValidPullType(pullType int) bool {
	return 0 <= pullType && pullType <= byDigest
}
