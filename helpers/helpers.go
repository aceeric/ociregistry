package helpers

import "regexp"

var srch = `.*([a-f0-9]{64}).*`
var re = regexp.MustCompile(srch)

// GetSHAfromPath extracts a SHA from the patch, or, if the path
// does not contain a SHA, it returns the empty string.
func GetSHAfromPath(shaExpr string) string {
	tmpdgst := re.FindStringSubmatch(shaExpr)
	if len(tmpdgst) == 2 {
		return tmpdgst[1]
	}
	return ""
}
