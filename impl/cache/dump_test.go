package cache

import (
	"strings"
	"testing"

	"github.com/aceeric/imgpull/pkg/imgpull"
	"github.com/aceeric/ociregistry/impl/pullrequest"
	"github.com/aceeric/ociregistry/impl/serialize"
)

// Tests the cache dump: a manifest list by tag, an image by digest, and two tags sharing a
// digest, so one alias has two tags. Digests are shortened to digestLen, and shown in full by
// default.
func TestDumpCache(t *testing.T) {
	_, build := aliasFixture(t)
	add := func(pr pullrequest.PullRequest, mh imgpull.ManifestHolder) {
		mc.Lock()
		defer mc.Unlock()
		mc.set(pr, mh)
	}
	prList, mhList := build("docker.io/hello-world:latest", aliasDigestE, aliasBlob1)
	mhList.Type = imgpull.V1ociIndex
	add(prList, mhList)
	add(build("docker.io/hello-world@sha256:"+aliasDigestD, aliasDigestD, aliasBlob1))
	add(build("quay.io/org/app:v1.0.0", aliasDigestD, aliasBlob1))
	add(build("quay.io/org/app:nightly-pr", aliasDigestD, aliasBlob1))

	want := `ENTRIES:

ENTRY URL                                           URL DIGEST     MANIFEST DIGEST  MANIFEST TYPE
docker.io/library/hello-world:latest                05c465a76e...  eeeeeeeeee...    image list
docker.io/library/hello-world@sha256:dddddddddd...  6952a3f1a2...  dddddddddd...    image
quay.io/org/app:nightly-pr                          e9391462b7...  dddddddddd...    image
quay.io/org/app:v1.0.0                              286d9fc2e1...  dddddddddd...    image

ALIASES:

ALIAS URL / TAGS
docker.io/library/hello-world@sha256:eeeeeeeeee...
    docker.io/library/hello-world:latest
quay.io/org/app@sha256:dddddddddd...
    quay.io/org/app:nightly-pr
    quay.io/org/app:v1.0.0
`
	if got := DumpCache(10); got != want {
		t.Fatalf("expected:\n%s\ngot:\n%s", want, got)
	}
	full := DumpCache(64)
	for _, s := range []string{"docker.io/library/hello-world@sha256:" + aliasDigestD + "  ", serialize.UrlDigest("quay.io/org/app:v1.0.0"),
		"quay.io/org/app@sha256:" + aliasDigestD + "\n"} {
		if !strings.Contains(full, s) {
			t.Errorf("expected the full form %q, got:\n%s", s, full)
		}
	}
	if strings.Contains(full, "...") {
		t.Errorf("nothing should be shortened at 64, got:\n%s", full)
	}
}

// Tests the dump of an empty cache: the headers only.
func TestDumpEmptyCache(t *testing.T) {
	ResetCache()
	want := "ENTRIES:\n\nENTRY URL  URL DIGEST  MANIFEST DIGEST  MANIFEST TYPE\n\nALIASES:\n\nALIAS URL / TAGS\n"
	if got := DumpCache(64); got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
