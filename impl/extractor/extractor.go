package extractor

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"errors"
	"io"
	"ociregistry/globals"
	"ociregistry/helpers"
	"os"
	"path/filepath"
	"strings"
)

func Extract(tarfile string, tarfilePath string, deleteAfter bool) error {
	defer deleteFile(tarfile, deleteAfter)
	blobPath := filepath.Join(tarfilePath, globals.BlobsDir)
	if err := os.MkdirAll(blobPath, 0755); err != nil {
		return err
	}
	f, err := os.Open(tarfile)
	if err != nil {
		return err
	}
	defer f.Close()
	r := bufio.NewReader(f)

	var tarReader *tar.Reader
	if strings.HasSuffix(tarfile, ".tgz") || strings.HasSuffix(tarfile, ".tar.gz") {
		gzr, err := gzip.NewReader(r)
		if err != nil {
			return err
		}
		defer gzr.Close()
		tarReader = tar.NewReader(gzr)
	} else if strings.HasSuffix(tarfile, ".tar") {
		tarReader = tar.NewReader(r)
	} else {
		return errors.New("archive format not supported for: " + filepath.Base(tarfile))
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
			if sha == "" {
				continue
			}
			filePath := filepath.Join(blobPath, sha)
			if _, err := os.Stat(filePath); err != nil {
				f, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0766)
				if err != nil {
					return err
				}
				defer f.Close()
				if _, err := io.Copy(f, tarReader); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func deleteFile(f string, shouldDelete bool) {
	if shouldDelete {
		os.Remove(f)
	}
}
