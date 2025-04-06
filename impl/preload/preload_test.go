package preload

import (
	"fmt"
	"ociregistry/impl/globals"
	"ociregistry/impl/upstream"
	"ociregistry/mock"
	"os"
	"path/filepath"
	"testing"
)

// configures the mock distribution server
var regConfig = `
---
- name: %s
  scheme: http
`

// Tests the preload capability
func TestPreload(t *testing.T) {
	globals.ConfigureLogging("error")
	server, url := mock.Server(mock.NewMockParams(mock.NONE, mock.HTTP))
	cfg := fmt.Sprintf(regConfig, url)
	if err := upstream.AddConfig([]byte(cfg)); err != nil {
		t.Fail()
	}
	defer server.Close()
	d, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fail()
	}
	defer os.RemoveAll(d)
	cnt, err := preloadOneImage(url+"/hello-world:latest", d, "amd64", "linux", 1000)
	// count is 2 because one manifest list and one image manifest
	if err != nil || cnt != 2 {
		t.Fail()
	}
	// the hello-world latest image has two blobs
	blobs, _ := os.ReadDir(filepath.Join(d, "blobs"))
	if len(blobs) != 2 {
		t.Fail()
	}
	cnt, err = preloadOneImage(url+"/hello-world:latest", d, "amd64", "linux", 1000)
	// count should now be zero because the two manifests were already cached
	if err != nil || cnt != 0 {
		t.Fail()
	}
}
