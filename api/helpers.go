package api

import (
	"crypto/md5"
	"errors"
	"fmt"
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

// wip
// func manifewstShaToOrg(base string, sha string) string {
// 	pat := filepath.Join(base, "*", "manifest.json")
// 	files, err := filepath.Glob(pat)
// 	if err != nil {
// 		for _, file := range files {
// 			SHA, err := computeMd5Sum(file)
// 			if err == nil && SHA == sha {
// 				return filepath.Base(filepath.Dir(file))
// 			}
// 		}
// 	}
// 	return ""
// }
