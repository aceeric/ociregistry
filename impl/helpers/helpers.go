package helpers

import (
	"os"
	"path/filepath"
	"regexp"

	"github.com/aceeric/ociregistry/impl/globals"
)

var srch = `.*([a-f0-9]{64}).*`
var re = regexp.MustCompile(srch)

// GetDigestFrom looks in the passed arg for a 64-character digest and, if
// found, returns the digest without a sha256: prefix.
func GetDigestFrom(str string) string {
	tmpdgst := re.FindStringSubmatch(str)
	if len(tmpdgst) == 2 {
		return tmpdgst[1]
	}
	return ""
}

// GetBlob makes a blob path specifier from the two args, and returns the result
// of os.Stat on that path, along with the full path.
func GetBlob(base string, shaPat string) (os.FileInfo, error, string) {
	shaPat = GetDigestFrom(shaPat)
	blobFile := filepath.Join(base, globals.BlobPath, shaPat)
	fi, err := os.Stat(blobFile)
	return fi, err, blobFile
}
