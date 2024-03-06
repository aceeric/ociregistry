package extractor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUnarchive(t *testing.T) {
	d, _ := os.MkdirTemp("", "")
	defer os.RemoveAll(d)
	Extract("test.tgz", d, false)
	_, err := os.Stat(filepath.Join(d, "blobs", "e91e582c25553bf9cfbf3cfb997e70a810ccd28f79af7a1d0d774de62b7d8bde"))
	if err != nil {
		t.Fail()
	}
}
