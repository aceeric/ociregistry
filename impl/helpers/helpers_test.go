package helpers

import (
	"os"
	"path/filepath"
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
