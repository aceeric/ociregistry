package preload

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/aceeric/ociregistry/impl/config"
	"github.com/aceeric/ociregistry/impl/globals"
	"github.com/aceeric/ociregistry/impl/serialize"
	"github.com/aceeric/ociregistry/mock"

	log "github.com/sirupsen/logrus"
)

var regConfig = `
---
registries:
  - name: %s
    scheme: http
`

func init() {
	log.SetOutput(io.Discard)
}

// Tests the cache capability. Ensures that an image is not re-downloaded if
// already cached.
func TestPreload(t *testing.T) {
	server, url := mock.Server(mock.NewMockParams(mock.NONE, mock.HTTP))
	cfg := fmt.Sprintf(regConfig, url)
	if err := config.SetConfigFromStr([]byte(cfg)); err != nil {
		t.Fail()
	}
	defer server.Close()
	d, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fail()
	}
	defer os.RemoveAll(d)
	serialize.CreateDirs(d, true)
	cnt, err := doPull(url+"/hello-world:latest", d, "amd64", "linux")
	// count is 2 because one manifest list and one image manifest
	if err != nil || cnt != 2 {
		t.Fail()
	}
	// the hello-world latest image has two blobs
	blobs, _ := os.ReadDir(filepath.Join(d, globals.BlobPath))
	if len(blobs) != 2 {
		t.Fail()
	}
	cnt, err = doPull(url+"/hello-world:latest", d, "amd64", "linux")
	// count should now be zero because the two manifests were already cached
	if err != nil || cnt != 0 {
		t.Fail()
	}
}

var loadConfig = `
---
registries:
  - name: %s
    scheme: http
imagePath: %s
os: %s
arch: %s
`

func TestLoad(t *testing.T) {
	d, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fail()
	}
	defer os.RemoveAll(d)
	serialize.CreateDirs(d, true)

	server, url := mock.Server(mock.NewMockParams(mock.NONE, mock.HTTP))
	defer server.Close()

	cfg := fmt.Sprintf(loadConfig, url, d, "linux", "amd64")
	if err := config.SetConfigFromStr([]byte(cfg)); err != nil {
		t.Fail()
	}

	err = os.WriteFile(d+"/image-list", []byte(url+"/hello-world:latest"), 0644)
	if err != nil {
		log.Fatal(err)
	}

	err = Load(d + "/image-list")
	if err != nil {
		t.Fail()
	}
}
