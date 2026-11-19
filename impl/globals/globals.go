package globals

import (
	"fmt"
	"time"
)

const (
	// ManifestPath is the subdirectory under the image cache root where manifests are stored,
	// one file per canonical image url, named by the SHA-256 hex of the url.
	ManifestPath = "manifests"
	// LtsPath is the legacy subdirectory where "latest" manifests were stored, named by
	// manifest digest. It is only read by the cache migration and never written.
	LtsPath = "lts"
	// ImgPath is the legacy subdirectory where non-"latest" manifests were stored, named by
	// manifest digest. It is only read by the cache migration and never written.
	ImgPath = "img"
	// BlobPath is the subdirectory under the image cache root where blobs are stored
	BlobPath = "blobs"
	// MetadataPath is the subdirectory under the image cache root for files about the cache
	// that aren't manifests or blobs, such as the preload ledger
	MetadataPath = "metadata"
	// DateFormat is the format of the ManifestHolder dates (Created and Pulled): UTC, with a Z
	// suffix. It has magic numbers from 'format.go' in package 'time'.
	DateFormat = "2006-01-02T15:04:05Z"
	// LegacyDateFormat is the format of the ManifestHolder dates before they were UTC: the local
	// time of the server that wrote them, with no zone. Only LegacyToUtc reads it.
	LegacyDateFormat = "2006-01-02T15:04:05"
)

// CurTime gets the current time in UTC as YYYY-MM-DDTHH:MM:SSZ
func CurTime() string {
	return time.Now().UTC().Format(DateFormat)
}

// ParseTime parses a date/time created by CurTime. Any other format is an error, including a
// legacy date (see LegacyToUtc).
func ParseTime(dt string) (time.Time, error) {
	return time.Parse(DateFormat, dt)
}

// LegacyToUtc returns the passed ManifestHolder date in DateFormat, and whether it was changed.
// A date that is empty or already in DateFormat is returned unchanged. A legacy date
// (LegacyDateFormat) is the local time of the server that wrote it, so it's read in the local
// time zone of this process and converted to UTC. Any other value is an error.
func LegacyToUtc(dt string) (string, bool, error) {
	if dt == "" {
		return dt, false, nil
	} else if _, err := ParseTime(dt); err == nil {
		return dt, false, nil
	}
	t, err := time.ParseInLocation(LegacyDateFormat, dt, time.Local)
	if err != nil {
		return "", false, fmt.Errorf("date %q is neither UTC (YYYY-MM-DDTHH:MM:SSZ) nor legacy local time (YYYY-MM-DDTHH:MM:SS)", dt)
	}
	return t.UTC().Format(DateFormat), true, nil
}

// IsUtc returns true if the passed ManifestHolder date is empty or in DateFormat.
func IsUtc(dt string) bool {
	if dt == "" {
		return true
	}
	_, err := ParseTime(dt)
	return err == nil
}
