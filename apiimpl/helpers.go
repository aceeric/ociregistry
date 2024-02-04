package apiimpl

import (
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"ociregistry/globals"
	"ociregistry/helpers"
	"os"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v4"
)

// logRequestHeaders logs the request headers at the DEBUG level to
// the echo logger in teh `ctx` arg.
func logRequestHeaders(ctx echo.Context) {
	hdrs := ctx.Request().Header
	for h := range hdrs {
		v := strings.Join(hdrs[h], ",")
		ctx.Logger().Debugf("HDR: %s=%s", h, v)
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

// getBlobPath extacts just the SHA from the passed 'shapat' arg and then
// looks for the file matching <base>/blobs/SHA - if found, returns that path
// else returns the empty string.
func getBlobPath(base string, shapat string) string {
	shapat = helpers.GetSHAfromPath(shapat)
	blobFile := filepath.Join(base, globals.BlobsDir, shapat)
	_, err := os.Stat(blobFile)
	if err != nil {
		return ""
	}
	return blobFile
}

// getManifestPath searches the tree starting at 'imagesBase' for all files
// with name "manifest.json". For each matching file, it the full path ends
// with <manifestPath>/"manifest.json" then the file is returned. The supported
// scenario is - a GET v2/{org}/{image}/manifests/{reference} request comes in like
// GET v2/swaggerapi/swagger-editor/manifests/latest. The caller builds 'manifestPath'
// like 'swaggerapi/swagger-editor/latest'. This function willmatch on the first file
// whose path ends with 'swaggerapi/swagger-editor/latest/manifest.json'.
func getManifestPath(imagesBase string, manifestPath string) string {
	accept := filepath.Join(manifestPath, "manifest.json")
	var found string = ""
	filepath.WalkDir(imagesBase, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(path, filepath.Join(imagesBase, globals.BlobsDir)) && d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, accept) {
			found = path
			return io.EOF
		}
		return nil
	})
	return found
}

// saveManifestDigest creates a file in the 'manifest_map' path whose name is a sha
// and whose content is a tag. Enables retrieval of a manifest for a ref like
// "sha256:zzz" where "sha256:zzz" is the sha of the a manifest with tag "latest".
// This is the companion function to xlatManifestDigest. Some clients (containerd) get
// the image manifest using a SHA rather than a tag. E.g. you'll see traffic like
// GET v2/swaggerapi/swagger-editor/manifests/sha256:3eaf5ca0004...
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
// and returns the contents. Basically it xlats a SHA to a ref like "latest" or
// "v1.0.0". This is the companion function to saveManifestDigest.
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
