package helpers

import (
	"ociregistry/impl/globals"
	"os"
	"path/filepath"
	"regexp"
)

var srch = `.*([a-f0-9]{64}).*`
var re = regexp.MustCompile(srch)

// GetDigestFrom looks in the passed arg for a 64-character digest and, if
// found, returns the digest *without* a "sha256:" prefix.
func GetDigestFrom(str string) string {
	tmpdgst := re.FindStringSubmatch(str)
	if len(tmpdgst) == 2 {
		return tmpdgst[1]
	}
	return ""
}

// GetBlobPath is looking for a blob. It makes a path specifier from the two args,
// and if a file exists at that path, returns the path, otherwise returns the empty
// string
func GetBlobPath(base string, shapat string) string {
	shapat = GetDigestFrom(shapat)
	blobFile := filepath.Join(base, globals.BlobsDir, shapat)
	_, err := os.Stat(blobFile)
	if err != nil {
		return ""
	}
	return blobFile
}
