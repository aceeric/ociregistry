package pullrequest

import (
	"reflect"
	"testing"
)

// this test shows a current weakness which is that it's not possible
// to validate these URLs in all cases. For example what does 'bar/baz:tag' mean?
// The URL parser can't assume that the left-most segment has a dot because you could
// pull thru to somecorporatehost/hello-world:latest. Not sure what do do about this.
func TestPRs(t *testing.T) {
	type parsetest struct {
		url         string
		shouldParse bool
		expectPr    PullRequest
	}
	pullRequests := []parsetest{
		{"foo.io/bar/baz:tag", true, PullRequest{PullType: ByTag, Org: "bar", Image: "baz", Reference: "tag", Remote: "foo.io"}},
		{"foo.io/baz:tag", true, PullRequest{PullType: ByTag, Org: "", Image: "baz", Reference: "tag", Remote: "foo.io"}},
		{"foo.io/bar/baz@sha256:123", true, PullRequest{PullType: ByDigest, Org: "bar", Image: "baz", Reference: "sha256:123", Remote: "foo.io"}},
		{"foo.io/baz@sha256:123", true, PullRequest{PullType: ByDigest, Org: "", Image: "baz", Reference: "sha256:123", Remote: "foo.io"}},
		{"bar/baz:tag", true, PullRequest{PullType: ByTag, Org: "", Image: "baz", Reference: "tag", Remote: "bar"}},
		{"baz:tag", false, PullRequest{}},
		{"bar/baz@sha256:123", true, PullRequest{PullType: ByDigest, Org: "", Image: "baz", Reference: "sha256:123", Remote: "bar"}},
		{"baz@sha256:123", false, PullRequest{}},
	}
	for _, pullRequest := range pullRequests {
		pr, err := NewPullRequestFromUrl(pullRequest.url)
		if pullRequest.shouldParse && err != nil {
			t.Fail()
		} else if !pullRequest.shouldParse {
			if err == nil {
				t.Fail()
			}
			continue
		} else if !reflect.DeepEqual(pr, pullRequest.expectPr) {
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
		{1, "", nil, "", "v1.2.3", []string{"docker.io", "foo", "bar"}, PullRequest{ByTag, "", "", "foo/bar", "v1.2.3", "docker.io"}, true, "the basic"},
		{1, "", nil, "", "v1.2.3", []string{"docker.io", "foo", "bar", "baz", "ding", "dang", "doo"}, PullRequest{ByTag, "", "", "foo/bar/baz/ding/dang/doo", "v1.2.3", "docker.io"}, true, "many repo segments"},
		{2, "", nil, "", "latest", []string{"docker.io", "foo"}, PullRequest{ByTag, "", "", "foo", "latest", "docker.io"}, true, "in-path ns"},
		{3, "", nil, "", "latest", []string{"foo"}, PullRequest{}, false, "no namespace"},
		{4, "quay.io", nil, "", "sha256:123", []string{"foo"}, PullRequest{ByDigest, "", "", "foo", "sha256:123", "quay.io"}, true, "ns from header"},
		{5, "", &ghcr, "", "sha256:123", []string{"foo"}, PullRequest{ByDigest, "", "", "foo", "sha256:123", "ghcr.io"}, true, "ns from query param"},
		{6, "", nil, "docker.io", "sha256:123", []string{"foo"}, PullRequest{ByDigest, "", "", "foo", "sha256:123", "docker.io"}, true, "ns from default"},
		{7, "quay.io", nil, "", "latest", []string{"docker.io", "foo"}, PullRequest{}, false, "two ns"},
		{8, "", &ghcr, "", "latest", []string{"docker.io", "foo"}, PullRequest{}, false, "two ns"},
		{9, "", nil, "registry.gitlab.com", "latest", []string{"docker.io", "foo"}, PullRequest{ByTag, "", "", "foo", "latest", "docker.io"}, true, "in-path ns ignore default ns"},
	}
	for _, pt := range parseTests {
		pr, err := NewPullRequest2(pt.regHdr, pt.ns, pt.defaultNs, pt.reference, pt.segments...)
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
