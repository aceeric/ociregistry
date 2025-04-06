package cache

import (
	"fmt"
	"ociregistry/impl/serialize"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"

	"github.com/aceeric/imgpull/pkg/imgpull"
)

// copy impull test?? ALL PULLS

// test concurrent pulls

// test enqueueing

func TestLoad(t *testing.T) {
	td, _ := os.MkdirTemp("", "")
	defer os.RemoveAll(td)
	os.Mkdir(filepath.Join(td, "fat"), 0777)
	os.Mkdir(filepath.Join(td, "img"), 0777)
	os.Mkdir(filepath.Join(td, "blobs"), 0777)

	mts := []imgpull.ManifestType{imgpull.V2dockerManifestList, imgpull.V2dockerManifest, imgpull.V1ociIndex, imgpull.V1ociManifest}

	for _, mt := range mts {
		mh := imgpull.ManifestHolder{
			Type:     mt,
			Digest:   strconv.Itoa(int(mt)),
			ImageUrl: fmt.Sprintf("foo.io/foo/%d:v1.2.3", mt),
		}
		if serialize.MhToFilesystem(mh, td) != nil {
			t.Fail()
		}
	}
	if Load(td) != nil {
		t.Fail()
	}
	// each manifest is cached twice: one with the tag (foo.io/foo/0:v1.2.3) and one with the
	// digest (foo.io/foo/0@sha256:0)
	if len(mc.manifests) != len(mts)*2 {
		t.Fail()
	}
	for _, mt := range mts {
		emptyMH := imgpull.ManifestHolder{}
		urlTag := fmt.Sprintf("foo.io/foo/%d:v1.2.3", mt)
		mh := getManifestFromCache(urlTag)
		if reflect.DeepEqual(mh, emptyMH) {
			t.Fail()
		}
		urlSha := fmt.Sprintf("foo.io/foo/%d@sha256:%d", mt, mt)
		mh = getManifestFromCache(urlSha)
		if reflect.DeepEqual(mh, emptyMH) {
			t.Fail()
		}
	}
}
