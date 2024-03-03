package helpers

import (
	"ociregistry/impl/globals"
	"os"
	"path/filepath"
	"testing"
)

func TestGetBlobPath(t *testing.T) {
	d, _ := os.MkdirTemp("", "")
	defer os.RemoveAll(d)
	os.Mkdir(filepath.Join(d, globals.BlobsDir), 0777)
	digest := "aef95111cc41a3028623128d631ef867ab83911b6eaf1a03d97dea5fa3578893"
	tf := filepath.Join(d, globals.BlobsDir, digest)
	os.WriteFile(tf, []byte("bar"), 0777)
	b := GetBlobPath(d, "sha256:"+digest)
	if b != tf {
		t.Fail()
	}
}
