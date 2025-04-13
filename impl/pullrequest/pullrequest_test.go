package pullrequest

import (
	"reflect"
	"testing"
)

type parsetest struct {
	url         string
	shouldParse bool
	expectPr    PullRequest
}

// this test shows a current weakness which is that it's not possible
// to validate these URLs in all cases. For example what does 'bar/baz:tag' mean?
// The URL parser can't assume that the left-most segment has a dot because you could
// pull thru to somecorporatehost/hello-world:latest. Not sure what do do about this.
func TestPRs(t *testing.T) {
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
		} else if !pullRequest.shouldParse && err == nil {
			t.Fail()
		} else if !reflect.DeepEqual(pr, pullRequest.expectPr) {
			t.Fail()
		}
	}
}
