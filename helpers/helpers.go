package helpers

import (
	"ociregistry/globals"
	"os"
	"path/filepath"
	"regexp"
)

var srch = `.*([a-f0-9]{64}).*`
var re = regexp.MustCompile(srch)

func GetSHAfrom(str string) string {
	tmpdgst := re.FindStringSubmatch(str)
	if len(tmpdgst) == 2 {
		return tmpdgst[1]
	}
	return ""
}

func GetBlobPath(base string, shapat string) string {
	shapat = GetSHAfrom(shapat)
	blobFile := filepath.Join(base, globals.BlobsDir, shapat)
	_, err := os.Stat(blobFile)
	if err != nil {
		return ""
	}
	return blobFile
}
