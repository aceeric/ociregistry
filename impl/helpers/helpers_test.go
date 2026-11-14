package helpers

import (
	"math/rand/v2"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/aceeric/ociregistry/impl/globals"
)

func TestGetBlobPath(t *testing.T) {
	d, _ := os.MkdirTemp("", "")
	defer os.RemoveAll(d)
	os.Mkdir(filepath.Join(d, globals.BlobPath), 0777)
	digest := "aef95111cc41a3028623128d631ef867ab83911b6eaf1a03d97dea5fa3578893"
	tf := filepath.Join(d, globals.BlobPath, digest)
	os.WriteFile(tf, []byte("bar"), 0777)
	if _, err, _ := GetBlob(d, "sha256:"+digest); err != nil {
		t.Fail()
	}
}

// oldDigestRe is the regex GetDigestFrom used to use: the scan must return the same result.
var oldDigestRe = regexp.MustCompile(`.*([a-f0-9]{64}).*`)

// oldGetDigestFrom is GetDigestFrom as it was, for comparison.
func oldGetDigestFrom(str string) string {
	if m := oldDigestRe.FindStringSubmatch(str); len(m) == 2 {
		return m[1]
	}
	return ""
}

// Tests that GetDigestFrom returns what the regex did, for the inputs callers pass, edge cases,
// and random strings of hex and other characters.
func TestGetDigestFrom(t *testing.T) {
	d := strings.Repeat("a1", 32)
	e := strings.Repeat("e9", 32)
	tests := []string{
		"", d, "sha256:" + d, "sha256:" + d[:63], d[:63], "SHA256:" + strings.ToUpper(d),
		d + "f", "f" + d, d + ":" + e, "sha256:" + d + "/sha256:" + e, "x" + d + "y",
		"/v2/org/app/blobs/sha256:" + d, strings.Repeat("0", 200), d[:63] + "-" + d[:63],
	}
	rnd := rand.New(rand.NewPCG(1, 2))
	alphabet := "0123456789abcdefABg:/-"
	for range 5000 {
		b := make([]byte, rnd.IntN(200))
		for i := range b {
			if rnd.IntN(10) == 0 {
				b[i] = alphabet[rnd.IntN(len(alphabet))]
			} else {
				b[i] = alphabet[rnd.IntN(16)]
			}
		}
		tests = append(tests, string(b))
	}
	for _, s := range tests {
		if got, want := GetDigestFrom(s), oldGetDigestFrom(s); got != want {
			t.Fatalf("%q: expected %q, got %q", s, want, got)
		}
	}
	if GetDigestFrom("sha256:"+d) != d {
		t.Fatal("expected the digest without the prefix")
	}
}

// BenchmarkGetDigestFrom compares the scan with the regex it replaced, on a blob request's digest.
func BenchmarkGetDigestFrom(b *testing.B) {
	in := "sha256:" + strings.Repeat("a1", 32)
	b.Run("scan", func(b *testing.B) {
		for b.Loop() {
			GetDigestFrom(in)
		}
	})
	b.Run("regex", func(b *testing.B) {
		for b.Loop() {
			oldGetDigestFrom(in)
		}
	})
}
