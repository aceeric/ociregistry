package mock

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"strings"
)

// TarImage is one image for DockerSaveTarball: its ref (e.g. quay.io/org/app:v1), platform,
// and layer contents.
type TarImage struct {
	Ref    string
	Os     string
	Arch   string
	Layers []string
}

// DockerSaveTarball writes a docker-save format tarball (as from 'docker save') with the passed
// images to path, gzip-compressed if path ends in .tgz or .tar.gz. It's for tests: the images'
// config blobs only have the platform fields, and the layers are the passed strings. A layer
// shared by two images is stored once, as in a real docker save.
func DockerSaveTarball(path string, images ...TarImage) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	var w io.Writer = f
	if strings.HasSuffix(path, ".tgz") || strings.HasSuffix(path, ".tar.gz") {
		gz := gzip.NewWriter(f)
		defer gz.Close()
		w = gz
	}
	tw := tar.NewWriter(w)
	defer tw.Close()
	type dockerManifest struct {
		Config   string
		RepoTags []string
		Layers   []string
	}
	manifests := []dockerManifest{}
	written := map[string]bool{}
	add := func(content []byte) (string, error) {
		name := "blobs/sha256/" + sha256Hex(content)
		if written[name] {
			return name, nil
		}
		written[name] = true
		return name, writeTarFile(tw, name, content)
	}
	for _, img := range images {
		cfg, _ := json.Marshal(map[string]string{"os": img.Os, "architecture": img.Arch})
		cfgName, err := add(cfg)
		if err != nil {
			return err
		}
		dm := dockerManifest{Config: cfgName, RepoTags: []string{img.Ref}}
		for _, layer := range img.Layers {
			name, err := add([]byte(layer))
			if err != nil {
				return err
			}
			dm.Layers = append(dm.Layers, name)
		}
		manifests = append(manifests, dm)
	}
	b, _ := json.Marshal(manifests)
	return writeTarFile(tw, "manifest.json", b)
}

// sha256Hex returns the hex sha256 digest of b.
func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// writeTarFile writes one regular file to the tar writer.
func writeTarFile(tw *tar.Writer, name string, content []byte) error {
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
		return err
	}
	_, err := tw.Write(content)
	return err
}
