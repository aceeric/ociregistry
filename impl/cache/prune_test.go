package cache

import (
	"encoding/json"
	"fmt"
	"ociregistry/impl/pullrequest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/aceeric/imgpull/pkg/imgpull"
)

var pruneTest = `{
	"schemaVersion": 2,
	"mediaType": "application/vnd.oci.image.manifest.v1+json",
	"config": {
	   "digest": "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	},
	"layers": [
	   {
		  "digest": "sha256:1111111111111111111111111111111111111111111111111111111111111112"
	   },
	   {
		  "digest": "sha256:1111111111111111111111111111111111111111111111111111111111111113"
	   }
	]
}`

func TestPrune(t *testing.T) {
	td, _ := os.MkdirTemp("", "")
	defer os.RemoveAll(td)
	for _, dir := range []string{"img", "blobs"} {
		os.Mkdir(filepath.Join(td, dir), 0777)
	}
	mh := imgpull.ManifestHolder{
		Type:     imgpull.V1ociManifest,
		Digest:   strconv.Itoa(int(imgpull.V1ociManifest)),
		ImageUrl: "docker.io/test/manifest:v1.2.3",
	}
	digests := []string{
		"1111111111111111111111111111111111111111111111111111111111111111",
		"1111111111111111111111111111111111111111111111111111111111111112",
		"1111111111111111111111111111111111111111111111111111111111111113",
	}
	for _, digest := range digests {
		if err := os.WriteFile(filepath.Join(td, "blobs", digest), []byte(digest), 0755); err != nil {
			t.Fail()
		}
	}
	if err := json.Unmarshal([]byte(fmt.Sprintf(v1ociManifest, digests[0], digests[1], digests[2])), &mh.V1ociManifest); err != nil {
		t.Fail()
	}
	pr, err := pullrequest.NewPullRequestFromUrl(mh.ImageUrl)
	if err != nil {
		t.Fail()
	}
	addManifestToCache(pr, mh)
	addBlobsToCache(mh)
	// manifests are added twice - one by tag and a second by digest
	if len(mc.manifests) != 2 || len(bc.blobs) != 3 {
		t.Fail()
	}
	prune(pr, mh)
	if len(mc.manifests) != 0 {
		t.Fail()
	}
	// blobs are only decremented - actual deletion happens elsewhere (TODO)
	for _, digest := range digests {
		if cnt, exists := bc.blobs[digest]; cnt != 0 || !exists {
			t.Fail()
		}
	}
}
