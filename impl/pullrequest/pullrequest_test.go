package pullrequest

import (
	"fmt"
	"testing"
)

type parsetest struct {
	url         string
	shouldParse bool
}

// this test shows a current weakness which is that it's not possible
// to validate these URLs in all cases...
func TestPRs(t *testing.T) {
	urls := []parsetest{
		{"foo.io/bar/baz:tag", true},
		{"foo.io/baz:tag", true},
		{"foo.io/bar/baz@sha256:123", true},
		{"foo.io/baz@sha256:123", true},
		{"bar/baz:tag", true},
		{"baz:tag", false},
		{"bar/baz@sha256:123", true},
		{"baz@sha256:123", false},
	}
	for _, url := range urls {
		_, err := NewPullRequestFromUrl(url.url)
		if url.shouldParse && err != nil {
			t.Fail()
		} else if !url.shouldParse && err == nil {
			t.Fail()
		}
	}
}

func TestDigestPr(t *testing.T) {
	digest := "sha256:123"
	pr, err := NewPullRequestFromUrl(fmt.Sprintf("foo.io/frobozz@%s", digest))
	if err != nil {
		t.Fail()
	}
	prId := pr.Id()
	prIdDigest := pr.IdDigest(digest)
	if prId != prIdDigest {
		t.Fail()
	}
}
