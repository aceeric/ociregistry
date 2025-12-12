package pullrequest

import (
	"reflect"
	"testing"
)

// Test creating a PullRequest struct from a url. Urls are expected to follow rules:
//  1. Left-most segment is an upstream like quay.io so it has to have a dot
//  2. Right-most segment is a ref by tag or digest
//  3. Remainder in the middle is the repository
func TestPRs(t *testing.T) {
	type parseTest struct {
		testNum     int
		url         string
		shouldParse bool
		expectPr    PullRequest
	}
	parseTests := []parseTest{
		{1, "foo.io/bar/baz:tag", true, PullRequest{PullType: ByTag, Repository: "bar/baz", Reference: "tag", Remote: "foo.io"}},
		{2, "foo.io/baz:tag", true, PullRequest{PullType: ByTag, Repository: "baz", Reference: "tag", Remote: "foo.io"}},
		{3, "foo.io/bar/baz@sha256:123", true, PullRequest{PullType: ByDigest, Repository: "bar/baz", Reference: "sha256:123", Remote: "foo.io"}},
		{4, "foo.io/baz@sha256:123", true, PullRequest{PullType: ByDigest, Repository: "baz", Reference: "sha256:123", Remote: "foo.io"}},
		{5, "bar/baz:tag", false, PullRequest{}},
		{6, "baz:tag", false, PullRequest{}},
		{7, "bar/baz@sha256:123", false, PullRequest{}},
		{8, "baz@sha256:123", false, PullRequest{}},
	}
	for _, pt := range parseTests {
		pr, err := NewPullRequestFromUrl(pt.url)
		if pt.shouldParse && err != nil {
			t.Fail()
		} else if !pt.shouldParse {
			if err == nil {
				t.Fail()
			}
			continue
		} else if !reflect.DeepEqual(pr, pt.expectPr) {
			t.Fail()
		}
	}
}

func TestNewPr(t *testing.T) {
	type parseTest struct {
		testNum     int
		regHdr      string
		ns          *string
		defaultNs   string
		reference   string
		segments    []string
		expectPr    PullRequest
		shouldParse bool
		rule        string
	}
	ghcr := "ghcr.io"
	parseTests := []parseTest{
		{1, "", nil, "", "v1.2.3", []string{"docker.io", "foo", "bar"}, PullRequest{ByTag, "foo/bar", "v1.2.3", "docker.io"}, true, "the basic"},
		{1, "", nil, "", "v1.2.3", []string{"docker.io", "a", "b", "c", "d", "e"}, PullRequest{ByTag, "a/b/c/d/e", "v1.2.3", "docker.io"}, true, "many repo segments"},
		{2, "", nil, "", "latest", []string{"docker.io", "foo"}, PullRequest{ByTag, "foo", "latest", "docker.io"}, true, "in-path ns"},
		{3, "", nil, "", "latest", []string{"foo"}, PullRequest{}, false, "no namespace"},
		{4, "quay.io", nil, "", "sha256:123", []string{"foo"}, PullRequest{ByDigest, "foo", "sha256:123", "quay.io"}, true, "ns from header"},
		{5, "", &ghcr, "", "sha256:123", []string{"foo"}, PullRequest{ByDigest, "foo", "sha256:123", "ghcr.io"}, true, "ns from query param"},
		{6, "", nil, "docker.io", "sha256:123", []string{"foo"}, PullRequest{ByDigest, "foo", "sha256:123", "docker.io"}, true, "ns from default"},
		{7, "quay.io", nil, "", "latest", []string{"docker.io", "foo"}, PullRequest{}, false, "two ns"},
		{8, "", &ghcr, "", "latest", []string{"docker.io", "foo"}, PullRequest{}, false, "two ns"},
		{9, "", nil, "registry.gitlab.com", "latest", []string{"docker.io", "foo"}, PullRequest{ByTag, "foo", "latest", "docker.io"}, true, "in-path ns ignores default ns"},
	}
	for _, pt := range parseTests {
		pr, err := NewPullRequest(pt.regHdr, pt.ns, pt.defaultNs, pt.reference, pt.segments...)
		if pt.shouldParse && err != nil {
			t.Fail()
		} else if !pt.shouldParse {
			if err == nil {
				t.Fail()
			}
			continue
		} else if !reflect.DeepEqual(pr, pt.expectPr) {
			t.Fail()
		}
	}
}

// Test the UrlWithDigest function
func TestUrlWithDigest(t *testing.T) {
	pr := PullRequest{
		PullType:   ByTag,
		Repository: "foo/bar/baz",
		Reference:  "v1.2.3",
		Remote:     "candyland.com",
	}
	wdg := pr.UrlWithDigest("sha256:12345")
	if wdg != "candyland.com/foo/bar/baz@sha256:12345" {
		t.Fail()
	}
}

// Test AltDockerUrl which enables to find manifests in cache that were pulled through as either
// docker.io/library/xyz or docker.io/xyz (without the "library" component)
func TestAltDockerUrl(t *testing.T) {
	type parseTest struct {
		testNum   int
		pr        PullRequest
		expectUrl string
		rule      string
	}
	parseTests := []parseTest{
		{1, PullRequest{ByTag, "foo/bar/baz", "v1.2.3", "candyland.com"}, "", "ignore: not docker.io"},
		{2, PullRequest{ByTag, "foo/bar", "v1.2.3", "docker.io"}, "docker.io/library/foo/bar:v1.2.3", "add library"},
		{3, PullRequest{ByTag, "library/foo/bar", "v1.2.3", "docker.io"}, "docker.io/foo/bar:v1.2.3", "remove library"},
		{2, PullRequest{ByTag, "foo/bar", "sha256:123", "docker.io"}, "docker.io/library/foo/bar@sha256:123", "add library, sha"},
		{3, PullRequest{ByTag, "library/foo/bar", "sha256:123", "docker.io"}, "docker.io/foo/bar@sha256:123", "remove library, sha"},
	}
	for _, pt := range parseTests {
		url := pt.pr.AltDockerUrl()
		if url != pt.expectUrl {
			t.FailNow()
		}
	}
}
