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
	PullType  int    `json:"pullType"`
	Org       string `json:"org"`
	Image     string `json:"image"`
	Reference string `json:"reference"`
	Remote    string `json:"remote"`
}

func NewPullRequest(org, image, reference, remote string) PullRequest {
	return PullRequest{
		PullType:  typeFromRef(reference),
		Org:       org,
		Image:     image,
		Reference: reference,
		Remote:    remote,
	}
}

// docker.io/hello-world:latest
// docker.io/library/hello-world@sha256:...
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

func (pr *PullRequest) isByTag() bool {
	return pr.PullType == byTag
}

func (pr *PullRequest) isByDigest() bool {
	return pr.PullType == byDigest
}

// calico/node:v1.23.0 becomes "calico/node/v1.23.0 and"
// hello-world:v1.0.0 becomes "/hello-world/v1.0.0"
// foo/bar@sha256:a15f3c... becomes "foo/bar/sha256:a15f3c..."
func (pr *PullRequest) Id() string {
	return fmt.Sprintf("%s/%s/%s", pr.Org, pr.Image, pr.Reference)
}

func (pr *PullRequest) IdDigest(digest string) string {
	return fmt.Sprintf("%s/%s/%s", pr.Org, pr.Image, digest)
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
