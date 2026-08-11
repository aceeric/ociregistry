package globals

import "time"

const (
	// LtsPath is the subdirectory under the image cache root where "latest" manifests
	// are stored. Latest manifests exist side-by-side with non-latest.
	LtsPath = "lts"
	// ImgPath is the subdirectory under the image cache root where non-"latest" manifests
	// are stored
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
