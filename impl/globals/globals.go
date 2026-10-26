package globals

import "time"

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
	// DateFormat is the datetimestamp format used in ManifestHolder. It has magic numbers
	// from 'format.go' in package 'time' that support date parsing
	DateFormat = "2006-01-02T15:04:05"
)

// CurTime gets the current time as YYYY-MM-DDTHH:MM:SS
func CurTime() string {
	return time.Now().Format(DateFormat)
}

// ParseTime parses a date/time created by CurTime
func ParseTime(dt string) (time.Time, error) {
	return time.Parse(DateFormat, dt)
}
