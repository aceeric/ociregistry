package impl

import (
	"fmt"
	"strings"
)

const (
	byTag int = iota
	byDigest
)

type pullRequest struct {
	pullType  int
	org       string
	image     string
	reference string
	remote    string
}

func NewPullRequest(org, image, reference, remote string) pullRequest {
	return pullRequest{
		pullType:  typeFromRef(reference),
		org:       org,
		image:     image,
		reference: reference,
		remote:    remote,
	}
}

func (pr *pullRequest) isByTag() bool {
	return pr.pullType == byTag
}

func (pr *pullRequest) isByDigest() bool {
	return pr.pullType == byDigest
}

func (pr *pullRequest) id() string {
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
