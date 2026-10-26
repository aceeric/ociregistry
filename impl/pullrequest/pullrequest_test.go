package pullrequest

import (
	"reflect"
	"strings"
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
		{2, "", nil, "", "latest", []string{"docker.io", "foo"}, PullRequest{ByTag, "library/foo", "latest", "docker.io"}, true, "in-path ns"},
		{3, "", nil, "", "latest", []string{"foo"}, PullRequest{}, false, "no namespace"},
		{4, "quay.io", nil, "", "sha256:123", []string{"foo"}, PullRequest{ByDigest, "foo", "sha256:123", "quay.io"}, true, "ns from header"},
		{5, "", &ghcr, "", "sha256:123", []string{"foo"}, PullRequest{ByDigest, "foo", "sha256:123", "ghcr.io"}, true, "ns from query param"},
		{6, "", nil, "docker.io", "sha256:123", []string{"foo"}, PullRequest{ByDigest, "library/foo", "sha256:123", "docker.io"}, true, "ns from default"},
		{7, "quay.io", nil, "", "latest", []string{"docker.io", "foo"}, PullRequest{}, false, "two ns"},
		{8, "", &ghcr, "", "latest", []string{"docker.io", "foo"}, PullRequest{}, false, "two ns"},
		{9, "", nil, "registry.gitlab.com", "latest", []string{"docker.io", "foo"}, PullRequest{ByTag, "library/foo", "latest", "docker.io"}, true, "in-path ns ignores default ns"},
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

// Simulates RESTler sending a single path segment that contains a dot
// e.g. GET /v2/ubuntu.me/manifests/latest
// segments[0] = "ubuntu.me" matches the "." case, sets frst=1
// then segments[1] panics because slice length is 1
func TestNewPullRequestPanicSingleSegmentWithDot(t *testing.T) {
	_, err := NewPullRequest(
		"",          // regHdr - empty
		nil,         // ns - nil
		"",          // defaultNs - empty
		"latest",    // reference
		"ubuntu.me", // segments - only one element, but contains a dot
	)
	// Should return an error, not panic
	if err == nil {
		t.Error("expected error for single segment with dot, got nil")
	}
}

// Tests canonicalization: registry and repository lowercased, digests lowercased, tag case
// preserved, and single-component docker.io repositories given the "library/" prefix. Both
// constructors must produce the same canonical url.
func TestCanonical(t *testing.T) {
	dgst := "sha256:ABCDEF0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	tests := []struct {
		url      string
		segments []string
		ref      string
		expect   string
	}{
		{"docker.io/hello-world:latest", []string{"docker.io", "hello-world"}, "latest", "docker.io/library/hello-world:latest"},
		{"docker.io/library/hello-world:latest", []string{"docker.io", "library", "hello-world"}, "latest", "docker.io/library/hello-world:latest"},
		{"docker.io/calico/cni:v3.27.0", []string{"docker.io", "calico", "cni"}, "v3.27.0", "docker.io/calico/cni:v3.27.0"},
		{"quay.io/foo:v1", []string{"quay.io", "foo"}, "v1", "quay.io/foo:v1"},
		{"Quay.IO/Org/App:RC1", []string{"Quay.IO", "Org", "App"}, "RC1", "quay.io/org/app:RC1"},
		{"quay.io/org/app:rc1", []string{"quay.io", "org", "app"}, "rc1", "quay.io/org/app:rc1"},
		{"docker.io/busybox@" + dgst, []string{"docker.io", "busybox"}, dgst, "docker.io/library/busybox@" + strings.ToLower(dgst)},
	}
	for _, tst := range tests {
		fromUrl, err := NewPullRequestFromUrl(tst.url)
		if err != nil {
			t.Fatalf("%s: %s", tst.url, err)
		}
		if got := fromUrl.Url(); got != tst.expect {
			t.Errorf("NewPullRequestFromUrl(%q).Url() = %q, want %q", tst.url, got, tst.expect)
		}
		fromApi, err := NewPullRequest("", nil, "", tst.ref, tst.segments...)
		if err != nil {
			t.Fatalf("%v: %s", tst.segments, err)
		}
		if got := fromApi.Url(); got != tst.expect {
			t.Errorf("NewPullRequest(%v, %q).Url() = %q, want %q", tst.segments, tst.ref, got, tst.expect)
		}
	}
}
