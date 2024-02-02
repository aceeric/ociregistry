package apiimpl

import (
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"ociregistry/helpers"
	"os"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v4"
)

// logRequestHeaders logs the request headers at the DEBUG level
func logRequestHeaders(ctx echo.Context) {
	hdrs := ctx.Request().Header
	for h := range hdrs {
		v := strings.Join(hdrs[h], ",")
		ctx.Logger().Debugf("HDR: %s, VALUES: %s", h, v)
	}
}

// computeMd5Sum computes an MD5 sum of the passed 'file'.
func computeMd5Sum(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", errors.New("not found")
	}
	defer f.Close()
	hash := md5.New()
	_, err = io.Copy(hash, f)
	if err != nil {
		return "", errors.New("md5sum error")
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// getArtifactPath looks for a file on the file system. Two use cases:
//
//  1. Exact match: looking for 'images/appzygy/smallgo/v1.0.0/manifest.json'
//     In this case, pass that exact value in 'base', and leave 'shapat' empty
//
//  2. Pattern match: looking for images/appzygy/smallgo/v1.0.0/<sha>.tar.gz or
//     images/library/hello-world-save/<sha>/layer.tar. In this case pass any
//     of 'sha256:<sha>' or '<sha>' or '<sha>.tar.gz' or '<sha>/layer.tar'
func getArtifactPath(base string, shapat string) string {
	var found string
	var srch = ""
	if shapat != "" {
		srch = helpers.GetSHAfromPath(shapat)
	}
	filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		//if srch != "" && strings.Contains(path, srch) {
		if srch != "" && filepath.Base(path) == srch {
			found = path
			// TODO DOCKER THE IMPORTER SHOULD ALREADY HAVE HANDLED THIS
			// handles the case of a tarball produced by 'docker save'
			if _, err := os.Stat(filepath.Join(path, "layer.tar")); err == nil {
				found = filepath.Join(path, "layer.tar")
			}
			return io.EOF
		} else if srch == "" && path == base {
			found = path
			return io.EOF
		}
		return nil
	})
	return found
}

// saveManifestDigest creates a file in the manifest_map path whose name is a sha
// and whose content is a tag. Enables retrieval of a manifest for a ref like
// "latest" or "v1.2.3" using "sha256:zzz". This is the companion function to
// xlatManifestDigest
func saveManifestDigest(image_path string, reference string, manifest_sha string) {
	map_path := filepath.Join(image_path, "manifest_map")
	if _, err := os.Stat(map_path); os.IsNotExist(err) {
		os.Mkdir(map_path, 0775)
	}
	map_file := filepath.Join(map_path, "sha256:"+manifest_sha)
	if _, err := os.Stat(map_file); err != nil {
		f, _ := os.Create(map_file)
		defer f.Close()
		f.Write([]byte(reference))
	}
}

// xlatManifestDigest reads <image_path>/manifest_map/<manifest_sha> if it exists
// and returns the contents. Basically it maps a SHA to a ref (like "latest" or
// "v1.0.0"). This is the companion function to saveManifestDigest
func xlatManifestDigest(image_path string, manifest_sha string) string {
	map_path := filepath.Join(image_path, "manifest_map")
	if _, err := os.Stat(map_path); os.IsNotExist(err) {
		return ""
	}
	map_file := filepath.Join(map_path, manifest_sha)
	if _, err := os.Stat(map_file); err == nil {
		b, _ := os.ReadFile(map_file)
		return string(b)
	}
	return ""
}
