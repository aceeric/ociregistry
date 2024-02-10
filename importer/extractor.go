package importer

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"ociregistry/globals"
	"ociregistry/helpers"
	"ociregistry/types"
	"os"
	"path/filepath"
	"strings"
)

// Extract inflates the archive at the path specified by the 'fileName' arg
// into the directory specified by the 'destPath' arg. The blobs are inflated
// first, and then non-blobs are handled last. The reason for this is that
// other threads of execution in the server use the presence of the 'manifest.json'
// file to determine whether the image is present. So - we want to create the
// manifest last - otherwise other threads might believe the image is cached
// and try to get image blobs which would not be present if the tarball was
// inflated in a random order.
func Extract(fileName string, tarfilePath string) error {
	f, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer f.Close()
	r := bufio.NewReader(f)

	var nonBlobs map[string]*bytes.Buffer = make(map[string]*bytes.Buffer)
	var tarReader *tar.Reader

	if strings.HasSuffix(fileName, ".tgz") || strings.HasSuffix(fileName, ".tar.gz") {
		gzr, err := gzip.NewReader(r)
		if err != nil {
			return err
		}
		defer gzr.Close()
		tarReader = tar.NewReader(gzr)
	} else if strings.HasSuffix(fileName, ".tar") {
		tarReader = tar.NewReader(r)
	} else {
		return errors.New("archive format not supported for: " + filepath.Base(fileName))
	}
	for {
		header, err := tarReader.Next()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return err
		}
		if header == nil {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			// ignore directories
			continue
		case tar.TypeReg:
			sha := helpers.GetSHAfromPath(header.Name)
			if sha != "" {
				filePath := filepath.Join(tarfilePath, globals.BlobsDir, sha)
				if _, err := os.Stat(filePath); err != nil {
					if err := createAllDirs(filePath); err != nil {
						return err
					}
					f, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0766)
					if err != nil {
						return err
					}
					if _, err := io.Copy(f, tarReader); err != nil {
						return err
					}
					f.Close()
				}
			} else {
				// hold non-blobs to handle after all blobs
				tmp := new(bytes.Buffer)
				nonBlobs[header.Name] = tmp
				if _, err := io.Copy(tmp, tarReader); err != nil {
					return err
				}
			}
		}
	}
	var m []types.ManifestJson
	jerr := json.Unmarshal(nonBlobs["manifest.json"].Bytes(), &m)
	if jerr != nil {
		return jerr
	}
	manifestPath, err := createAllDirs2(m[0].RepoTags[0], tarfilePath)
	if err != nil {
		return err
	}

	// defer writing the manifest until the end since the presence of the manifest
	// is used to determine whether the image is cached when a client pulls
	for fName, bytes := range nonBlobs {
		f, err := os.OpenFile(filepath.Join(manifestPath, fName), os.O_CREATE|os.O_RDWR, 0766)
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, bytes); err != nil {
			return err
		}
		f.Close()
	}
	return nil
}

// createAllDirs creates all directories in the passed 'filePath' arg
func createAllDirs(filePath string) error {
	targetPathDir := filepath.Dir(filePath)
	if _, err := os.Stat(targetPathDir); err != nil {
		if err := os.MkdirAll(targetPathDir, 0755); err != nil {
			return err
		}
	}
	return nil
}

// createAllDirs2 parses the repo tag in the 'repoTag' and uses it to create a
// path under 'tarfilePath'. E.g. if 'tarfilePath' is /var/frobozz and 'repoTag'
// is foo/bar:v1.2.3, then a path /var/frobozz/foo/bar/v1.2.3 is created and
// returned to the caller. As a nuance: if no "organization" is present in the
// repotag, then 'library' is assigned by this function as the organization. So
// for example 'repoTag' "bar:v1.2.3" becomes path <tarfilePath>/library/bar/v1.2.3
func createAllDirs2(repoTag string, tarfilePath string) (string, error) {
	var filePath = repoTag
	if strings.Count(repoTag, "/") == 0 {
		filePath = filepath.Join("library", filePath)
	}
	filePath = strings.Replace(filePath, ":", "/", 1)
	filePath = filepath.Join(tarfilePath, filePath)
	if _, err := os.Stat(filePath); err != nil {
		if err := os.MkdirAll(filePath, 0755); err != nil {
			return "", err
		}
	}
	return filePath, nil
}
