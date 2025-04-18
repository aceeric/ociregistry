package cache

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/aceeric/imgpull/pkg/imgpull"
)

const srch = `.*sha256:([a-f0-9]{64}).*`

var re = regexp.MustCompile(srch)

type MHReader struct {
	mh  []imgpull.ManifestHolder
	idx int
}

func NewMFReader(mh []imgpull.ManifestHolder) *MHReader {
	sort.Slice(mh, func(i, j int) bool {
		return mh[i].ImageUrl < mh[j].ImageUrl
	})
	return &MHReader{
		mh:  mh,
		idx: 0,
	}
}

func (mhr *MHReader) Read(b []byte) (n int, err error) {
	if mhr.idx >= len(mhr.mh) {
		return 0, io.EOF
	}
	mh := mhr.mh[mhr.idx]

	hdr := ""
	if mhr.idx == 0 {
		hdr = "URL DIGEST\n"
	}
	url := mh.ImageUrl
	dgst := re.FindStringSubmatch(url)
	if len(dgst) == 2 {
		url = strings.Replace(url, dgst[1], dgst[1][:10], 1)
	}
	line := fmt.Sprintf("%s%s %s\n", hdr, url, mh.Digest)
	n = copy(b, line)
	mhr.idx++
	return n, nil
}
