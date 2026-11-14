package helpers

import (
	"os"
	"path/filepath"

	"github.com/aceeric/ociregistry/impl/globals"
)

// digestLen is the length of a sha256 digest in hex characters.
const digestLen = 64

// GetDigestFrom looks in the passed arg for a 64-character digest and, if found, returns the
// digest without a sha256: prefix. If there's more than one run of 64 or more lowercase hex
// characters, the last 64 characters of the last such run are returned, as the regex
// `.*([a-f0-9]{64}).*` did. It's called at least once for every blob request, so it scans the
// string once from the end instead of using that regex, whose backtracking was the server's
// largest CPU cost under load.
func GetDigestFrom(str string) string {
	run := 0
	for i := len(str) - 1; i >= 0; i-- {
		if c := str[i]; ('0' <= c && c <= '9') || ('a' <= c && c <= 'f') {
			if run++; run == digestLen {
				return str[i : i+digestLen]
			}
		} else {
			run = 0
		}
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
