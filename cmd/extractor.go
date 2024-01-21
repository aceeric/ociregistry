package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// filename is the full path plus filename
// dest is the root of the images dir
func extract(fileName string, destPath string) error {
	destPath, err := parseAndCreateDirs(fileName, destPath)
	if err != nil {
		return err
	}
	f, err := os.Open(fileName)
	if err != nil {
		return err
	}
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
		return errors.New("archive format not presently supported: " + fileName)
	}
	for {
		header, err := tarReader.Next()
		if err != nil {
			break
		}
		if header == nil {
			continue
		}
		// the target location where the dir/file should be created
		target := filepath.Join(destPath, header.Name)

		// check the file type
		switch header.Typeflag {
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0755); err != nil {
					return err
				}
			}
		case tar.TypeReg:
			// hold manifest for last
			if header.Name == "manifest.json" {
				manifestBytes = new(bytes.Buffer)
				if _, err := io.Copy(manifestBytes, tarReader); err != nil {
					return err
				}
				manifestPath = target
			} else {
				f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, 0766)
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
	// write the manifest last so a concurrent pull can't outrun the extraction of all the
	// other files from the archive since the image is determined to exist on the basis of
	// the manifest.json file being present on the filesystem.
	if manifestBytes != nil {
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

var ignore = []string{"docker.io", "quay.io", "ghcr.io"}

// e.g. if archive name is "docker.io+calico+pod2daemon-flexvol+v3.27.0.tar" then
// create <destPath>/calico/pod2daemon-flexvol/v3.27.0
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
