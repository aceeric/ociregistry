package importer

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"ociregistry/helpers"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

var ignore = []string{"docker.io", "quay.io", "ghcr.io"}

// extract inflates the archive at the path specified by the "fileName" arg
// into the directory specified by the "destPath" arg. The filename is first
// parsed (see func parseAndCreateDirs) and a directory structure is created
// from the filename. Then the archive is uncompressed into that directory.
// The "fileName" arg is the full path plus archive filename, and the
// "destPath is the full path to the root of the "images" directory.

// TODO PRIVATE??
func Extract(fileName string, tarfilePath string) error {
	targetPath, err := parseAndCreateDirs(fileName, tarfilePath)
	if err != nil {
		return err
	}
	f, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer f.Close()
	r := bufio.NewReader(f)
	var tarReader *tar.Reader
	var manifestBytes *bytes.Buffer
	var manifestPath string

	if strings.HasSuffix(fileName, ".tgz") || strings.HasSuffix(fileName, ".tar.gz") {
		gzr, err := gzip.NewReader(r)
		if err != nil {
			return err
		}
		defer gzr.Close()
		tarReader = tar.NewReader(gzr)
	} else if strings.HasSuffix(fileName, ".tar") {
		tarReader = tar.NewReader(r)
		// ? defer r.Close()
	} else {
		return errors.New("archive format not presently supported for: " + filepath.Base(fileName))
	}
	for {
		header, err := tarReader.Next()
		if err != nil {
			break
		}
		if header == nil {
			continue
		}

		// check the file type
		switch header.Typeflag {
		//case tar.TypeDir:
		//	if _, err := os.Stat(target); err != nil {
		//		if err := os.MkdirAll(target, 0755); err != nil {
		//			return err
		//		}
		//	}
		case tar.TypeReg:
			// hold manifest for last (see below)
			if header.Name == "manifest.json" {
				manifestBytes = new(bytes.Buffer)
				if _, err := io.Copy(manifestBytes, tarReader); err != nil {
					return err
				}
				manifestPath = filepath.Join(targetPath, header.Name)
			} else {
				var filePath = filepath.Join(targetPath, header.Name)
				sha := helpers.GetSHAfromPath(header.Name)
				if sha != "" {
					filePath = filepath.Join(tarfilePath, "blobs", sha)
				}
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
			}
		}
	}
	// write the manifest last so a concurrent pull can't outrun the extraction of all the
	// other files from the archive since the image is determined to exist on the basis of
	// the manifest.json file being present on the filesystem.
	if manifestBytes != nil {
		if err := createAllDirs(manifestPath); err != nil {
			return err
		}
		f, err := os.OpenFile(manifestPath, os.O_CREATE|os.O_RDWR, 0766)
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, manifestBytes); err != nil {
			return err
		}
		f.Close()
	}
	return nil
}

// parseAndCreateDirs parses the archive filename in the 'archiveName' arg and
// creates a directory tree under the directory specified in the 'destPath' arg
// into which the caller can inflate the archive. For example, if the archive
// name is "docker.io+calico+pod2daemon-flexvol+v3.27.0.tar" and the dest path
// is "/foo/bar" then this function will create a directory:
// /foo/bar/calico/pod2daemon-flexvol/v3.27.0 and return that path to the
// caller.
func parseAndCreateDirs(archiveName string, destPath string) (string, error) {
	// get the bare archive name
	archiveName = filepath.Base(archiveName)
	archiveName = archiveName[:len(archiveName)-len(filepath.Ext(archiveName))]
	// split into segments on plus sign
	var segments []string = strings.Split(archiveName, "+")
	for _, segment := range segments {
		if slices.Contains(ignore, segment) {
			continue
		}
		destPath = filepath.Join(destPath, segment)
	}
	return destPath, os.MkdirAll(destPath, 0755)
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
