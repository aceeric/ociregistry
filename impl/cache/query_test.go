package cache

import (
	"fmt"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/aceeric/imgpull/pkg/imgpull"
)

// Tests that the manifest and blob readers return complete entries however small the
// caller's buffer is. net/http reads a response body in pieces as small as a few bytes,
// so a reader that drops the rest of an entry that doesn't fit corrupts the output.
func TestReadersSmallBuffer(t *testing.T) {
	digest := strings.Repeat("a", 64)
	mhs := []imgpull.ManifestHolder{
		{ImageUrl: "docker.io/calico/apiserver:v3.27.0", Digest: digest},
		{ImageUrl: "docker.io/calico/cni:v3.27.0", Digest: strings.Repeat("b", 64)},
	}
	expMH := "URL DIGEST CREATED PULLED\n" +
		fmt.Sprintf("docker.io/calico/apiserver:v3.27.0 %s  %s %s\n", digest, mhs[0].Created, mhs[0].Pulled) +
		fmt.Sprintf("docker.io/calico/cni:v3.27.0 %s  %s %s\n", strings.Repeat("b", 64), mhs[1].Created, mhs[1].Pulled)

	blobs := []blobResult{{digest: digest, refCnt: 2}, {digest: strings.Repeat("c", 64), refCnt: 1}}
	expBlobs := "URL REFCNT\n" + digest + " 2\n" + strings.Repeat("c", 64) + " 1\n"

	tests := []struct {
		name string
		rdr  io.Reader
		exp  string
	}{
		{"manifests", NewMFReader(mhs), expMH},
		{"blobs", NewBlobReader(blobs), expBlobs},
	}
	for _, tst := range tests {
		got, err := io.ReadAll(iotest.OneByteReader(tst.rdr))
		if err != nil {
			t.Fatalf("%s: %s", tst.name, err)
		}
		if string(got) != tst.exp {
			t.Errorf("%s: expected:\n%s\ngot:\n%s", tst.name, tst.exp, got)
		}
	}
}
