package helpers

import (
	"ociregistry/impl/globals"
	"os"
	"path/filepath"
	"testing"
)

func Test1(t *testing.T) {
	td, _ := os.MkdirTemp("", "")
	os.Mkdir(filepath.Join(td, globals.BlobsDir), 0777)
	digest := "aef95111cc41a3028623128d631ef867ab83911b6eaf1a03d97dea5fa3578893"
	tf := filepath.Join(td, globals.BlobsDir, digest)
	os.WriteFile(tf, []byte("bar"), 0777)
	b := GetBlobPath(td, "sha256:"+digest)
	if b != tf {
		t.Fail()
	}
}
