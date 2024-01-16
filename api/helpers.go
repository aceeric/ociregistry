package api

import (
	"crypto/md5"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// where image tarballs are unarchived to
var image_path string

func SetImagePath(_image_path string) {
	image_path = _image_path
}

// computes an MD5 sum on a file
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

// Looks for a file on the file system. Two use cases:
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
		arr := strings.Split(shapat, ":")
		srch = arr[len(arr)-1]
		srch = strings.Replace(srch, ".tar.gz", "", 1)
	}
	filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if srch != "" && strings.Contains(path, srch) {
			found = path
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

// the GET /v2/{org}/{image}/manifests/{reference} /v2/{image}/manifests/{reference} handlers
// return a manifest and set a header Docker-Content-Digest with the SHA of that manifest. A client
// (e.g. when you docker run that image) may ask for the manifest with that SHA. So this function
// is called to write the calculated sha to an empty file as a child of the tag that holds the
// image artifacts. So for example if a SHA is calculated for the image component in the directory
// images/library/hello-world/latest, then the caller passes that path in 'manifest_path' and the
// manifest digest in 'sha' and this function creates a file
// 'images/library/hello-world/latest/sha256:<sha>.manifest.digest'. The presence of the file
// is later used in a call to xlatManifestDigest with the same sha to determine that the SHA was
// calculated from the image artifacts in 'images/library/hello-world/latest'.
func saveManifestDigest(manifest_path string, sha hash.Hash) {
	shafile := fmt.Sprintf("sha256:%x.manifest.digest", sha.Sum(nil))
	filename := filepath.Join(filepath.Dir(manifest_path), shafile)
	if _, err := os.Stat(filename); err != nil {
		os.Create(filename)
	}
}

// Example: if exists 'images/library/hello-world/latest/sha256:<somesha>.manifest.digest'
// then return 'latest'. (See 'saveManifestDigest' above.)
func xlatManifestDigest(image_path string, org string, image, manifest_sha string) string {
	pat := filepath.Join(image_path, org, image, "*", manifest_sha+".manifest.digest")
	if files, err := filepath.Glob(pat); err == nil && len(files) == 1 {
		return filepath.Base(filepath.Dir(files[0]))
	}
	return ""
}
