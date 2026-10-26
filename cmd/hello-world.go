package main

import (
	_ "embed"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/aceeric/ociregistry/impl/config"
	"github.com/aceeric/ociregistry/impl/globals"
	"github.com/aceeric/ociregistry/impl/serialize"

	"github.com/aceeric/imgpull/pkg/imgpull"
)

//go:embed hello_world/config.json
var configJson []byte

//go:embed hello_world/blob.tar.gz
var blobTarGz []byte

//go:embed hello_world/imageManifest.json
var imageManifest []byte

//go:embed hello_world/manifestList.json
var manifestList []byte

// helloWorldMode builds an image cache in the system temp directory from embedded files that
// serve docker.io/library/hello-world:latest (as of a point in time.) It sets the server into
// air-gapped mode and serves only that one image. For testing.
func helloWorldMode() (string, error) {
	tmpdir, err := os.MkdirTemp("", "")
	if err != nil {
		return "", err
	}

	config.SetAirGapped(true)
	config.SetPreloadImages("")
	config.SetImagePath(tmpdir)
	if err := serialize.CreateDirs(config.GetImagePath(), true); err != nil {
		return "", err
	}
	blobs := []struct {
		Name  string
		Bytes *[]byte
	}{
		{Name: "74cc54e27dc41bb10dc4b2226072d469509f2f22f1a3ce74f4a59661a1d44602", Bytes: &configJson},
		{Name: "e6590344b1a5dc518829d6ea1524fc12f8bcd14ee9a02aa6ad8360cce3a9a9e9", Bytes: &blobTarGz},
	}
	for _, blob := range blobs {
		if err := os.WriteFile(filepath.Join(tmpdir, globals.BlobPath, blob.Name), *blob.Bytes, 0600); err != nil {
			return "", err
		}
	}
	// the embedded manifests are ManifestHolder JSON, each with its image url, so they are
	// written by url like any other cached manifest
	for _, b := range []*[]byte{&manifestList, &imageManifest} {
		mh := imgpull.ManifestHolder{}
		if err := json.Unmarshal(*b, &mh); err != nil {
			return "", err
		}
		if err := serialize.MhToFilesystem(mh, tmpdir, true); err != nil {
			return "", err
		}
	}
	return tmpdir, nil
}
