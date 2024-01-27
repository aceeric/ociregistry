package helpers

import "regexp"

var srch = `.*([a-f0-9]{64}).*`
var re = regexp.MustCompile(srch)

func GetSHAfromPath(shaExpr string) string {
	tmpdgst := re.FindStringSubmatch(shaExpr)
	if len(tmpdgst) == 2 {
		return tmpdgst[1]
	}
	return ""
}
