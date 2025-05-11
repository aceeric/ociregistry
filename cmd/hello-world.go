package main

import (
	_ "embed"
	"os"
	"path/filepath"

	"github.com/aceeric/ociregistry/impl/config"
	"github.com/aceeric/ociregistry/impl/globals"
	"github.com/aceeric/ociregistry/impl/serialize"
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
	if err := serialize.CreateDirs(config.GetImagePath()); err != nil {
		return "", err
	}
	filelist := []struct {
		Name  string
		Dir   string
		Bytes *[]byte
	}{
		{Name: "424f1f86cdf501deb591ace8d14d2f40272617b51b374915a87a2886b2025ece", Dir: globals.LtsPath, Bytes: &manifestList},
		{Name: "03b62250a3cb1abd125271d393fc08bf0cc713391eda6b57c02d1ef85efcc25c", Dir: globals.LtsPath, Bytes: &imageManifest},
		{Name: "74cc54e27dc41bb10dc4b2226072d469509f2f22f1a3ce74f4a59661a1d44602", Dir: globals.BlobPath, Bytes: &configJson},
		{Name: "e6590344b1a5dc518829d6ea1524fc12f8bcd14ee9a02aa6ad8360cce3a9a9e9", Dir: globals.BlobPath, Bytes: &blobTarGz},
	}
	for _, file := range filelist {
		if err := os.WriteFile(filepath.Join(tmpdir, file.Dir, file.Name), *file.Bytes, 0600); err != nil {
			return "", err
		}
	}
	return tmpdir, nil
}
