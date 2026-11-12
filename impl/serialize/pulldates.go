package serialize

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/aceeric/imgpull/pkg/imgpull"
	"github.com/aceeric/ociregistry/impl/globals"
	log "github.com/sirupsen/logrus"
)

// CacheMetadata is the content of the pull dates file: per-url metadata keyed by canonical
// image url.
type CacheMetadata struct {
	Version int                `json:"version"`
	Meta    map[string]UrlMeta `json:"meta"`
}

// UrlMeta is the metadata for one canonical image url. Pulled is the last time the url was
// served, in globals.DateFormat. A field added in the future must also increment
// PullDatesVersion, so an older binary refuses the file rather than applying part of it.
type UrlMeta struct {
	Pulled string `json:"pulled"`
}

const (
	// PullDatesFile is the name of the pull dates file in the manifests directory. The
	// manifests hold the pull dates: the server updates them from the in-mem cache when it
	// shuts down. While the server runs, this file is a journal of the urls pulled since it
	// started, so a crash loses no more than one write interval. A clean shutdown deletes it,
	// and any command that opens the cache applies a leftover file to the manifests and deletes
	// it (ReconcilePullDates). It is not a manifest, so WalkTheCache doesn't read it.
	PullDatesFile = "pull-dates.json"
	// PullDatesVersion is the pull dates file version this binary reads and writes.
	PullDatesVersion = 1
)

// ManifestWriter writes a ManifestHolder to the file system. Its signature matches
// MhToFilesystem, which is the writer every caller passes except tests.
type ManifestWriter func(mh imgpull.ManifestHolder, imagePath string, replace bool) error

// ErrPullDatesVersion is returned by ReadPullDates for a pull dates file written by a newer
// version of the server.
var ErrPullDatesVersion = errors.New("unsupported pull dates file version")

// ReadPullDates returns the per-url metadata from the pull dates file. A missing file is an
// empty map, not an error. A file that can't be parsed is an error, and so is a file written by
// a newer version of the server (ErrPullDatesVersion).
func ReadPullDates(imagePath string) (map[string]UrlMeta, error) {
	fname := pullDatesFile(imagePath)
	b, err := os.ReadFile(fname)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]UrlMeta{}, nil
	} else if err != nil {
		return nil, err
	}
	md := CacheMetadata{}
	if err := json.Unmarshal(b, &md); err != nil {
		return nil, fmt.Errorf("can't parse %s: %w", fname, err)
	}
	if md.Version > PullDatesVersion {
		return nil, fmt.Errorf("%w: %s is version %d, this version supports version %d", ErrPullDatesVersion, fname, md.Version, PullDatesVersion)
	} else if md.Version < 1 {
		return nil, fmt.Errorf("can't parse %s: missing or invalid version %d", fname, md.Version)
	}
	if md.Meta == nil {
		md.Meta = map[string]UrlMeta{}
	}
	return md.Meta, nil
}

// WritePullDates atomically replaces the pull dates file with the passed per-url metadata. The
// file is indented, with urls in sorted order, so it can be read and compared by hand.
func WritePullDates(imagePath string, meta map[string]UrlMeta) error {
	b, err := json.MarshalIndent(CacheMetadata{Version: PullDatesVersion, Meta: meta}, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(pullDatesFile(imagePath), append(b, '\n'), 0644)
}

// PullDatesExist reports whether the pull dates file exists.
func PullDatesExist(imagePath string) (bool, error) {
	_, err := os.Stat(pullDatesFile(imagePath))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// RemovePullDates deletes the pull dates file. A missing file is not an error.
func RemovePullDates(imagePath string) error {
	if err := os.Remove(pullDatesFile(imagePath)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// ReconcilePullDates applies a pull dates file left by a server that didn't shut down cleanly
// to the manifests, and deletes it. It is called once, before any command uses the cache, so
// the manifests hold the pull dates. For each url in the file whose manifest exists, a later pull
// date is written to the manifest. A url without a manifest (e.g. pruned after the file was
// last written) is skipped: a manifest is never created. Applying the file again gives the
// same result, so a crash during this function is harmless. Manifests are written with the
// passed writer, normally MhToFilesystem.
//
// Any error is returned and the file is kept: one that can't be read or parsed, one written by
// a newer version, or a manifest that can't be written. The caller must not go on, because a
// file system that can't take a manifest write can't take a cache write either.
func ReconcilePullDates(imagePath string, write ManifestWriter) error {
	meta, err := ReadPullDates(imagePath)
	if err != nil {
		return fmt.Errorf("%w (to start without its pull dates, delete it)", err)
	} else if exists, err := PullDatesExist(imagePath); err != nil || !exists {
		return err
	}
	updated, skipped := 0, 0
	for url, um := range meta {
		mh, found := MhFromFilesystem(url, imagePath)
		if !found {
			skipped++
			continue
		}
		if globals.LaterTime(mh.Pulled, um.Pulled) == mh.Pulled {
			continue
		}
		mh.Pulled = um.Pulled
		if err := write(mh, imagePath, true); err != nil {
			return fmt.Errorf("unable to update the pull date of %s from %s: %w", url, pullDatesFile(imagePath), err)
		}
		updated++
	}
	if err := RemovePullDates(imagePath); err != nil {
		return err
	}
	log.Infof("applied %s left by an unclean shutdown: %d manifest pull date(s) updated, %d url(s) without a manifest skipped", PullDatesFile, updated, skipped)
	return nil
}

// pullDatesFile returns the path of the pull dates file.
func pullDatesFile(imagePath string) string {
	return filepath.Join(imagePath, globals.ManifestPath, PullDatesFile)
}
