package serialize

import (
	"bytes"
	"encoding/json"
	"io"
	"ociregistry/impl/pullrequest"
	"ociregistry/impl/upstream"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/aceeric/imgpull/pkg/imgpull"
	log "github.com/sirupsen/logrus"
)

var v2dockerManifest = `{
   "schemaVersion": 2,
   "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
   "config": {
      "mediaType": "application/vnd.docker.container.image.v1+json",
      "digest": "sha256:4873874c08efc72e9729683a83ffbb7502ee729e9a5ac097723806ea7fa13517",
      "size": 973
   },
   "layers": [
      {
         "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
         "digest": "sha256:9457426d68990df190301d2e20b8450c4f67d7559bdb7ded6c40d41ced6731f7",
         "size": 307026
      }
   ]
}`

var v2dockerManifestList = `{
   "schemaVersion": 2,
   "mediaType": "application/vnd.docker.distribution.manifest.list.v2+json",
   "manifests": [
      {
         "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
         "digest": "sha256:f5944f2d1daf66463768a1503d0c8c5e8dde7c1674d3f85abc70cef9c7e32e95",
         "size": 526,
         "platform": {
            "architecture": "amd64",
            "os": "linux"
         }
      },
      {
         "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
         "digest": "sha256:27295ffe5a75328e8230ff9bcabe2b54ebb9079ff70344d73a7b7c7e163ee1a6",
         "size": 526,
         "platform": {
            "architecture": "arm",
            "os": "linux",
            "variant": "v7"
         }
      },
      {
         "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
         "digest": "sha256:566af08540f378a70a03588f3963b035f33c49ebab3e4e13a4f5edbcd78c6689",
         "size": 526,
         "platform": {
            "architecture": "arm64",
            "os": "linux"
         }
      },
      {
         "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
         "digest": "sha256:2f205253a51c641263b155d48460ee2056c5b5013f8239ae3811792ec63b3546",
         "size": 526,
         "platform": {
            "architecture": "ppc64le",
            "os": "linux"
         }
      },
      {
         "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
         "digest": "sha256:7eaeb31509d7f370599ef78d55956e170eafb7f4a75b8dc14b5c06071d13aae0",
         "size": 526,
         "platform": {
            "architecture": "s390x",
            "os": "linux"
         }
      },
      {
         "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
         "digest": "sha256:78bfb9d8999c190fca79871c4b2f8d69d94a0605266f0bbb2dbaa1b6dfd03720",
         "size": 1969,
         "platform": {
            "architecture": "amd64",
            "os": "windows",
            "os.version": "10.0.17763.2928"
         }
      },
      {
         "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
         "digest": "sha256:9d05676469a08d6dba9889297333b7d1768e44e38075ab5350a4f8edd97f5be1",
         "size": 1969,
         "platform": {
            "architecture": "amd64",
            "os": "windows",
            "os.version": "10.0.19042.1706"
         }
      },
      {
         "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
         "digest": "sha256:e8fb66bcfe1a85ec1299652d28e6f7f9cfbb01d33c6260582a42971d30dcb77d",
         "size": 1969,
         "platform": {
            "architecture": "amd64",
            "os": "windows",
            "os.version": "10.0.20348.707"
         }
      }
   ]
}`

var v1ociIndex = `{
   "schemaVersion": 2,
   "mediaType": "application/vnd.oci.image.index.v1+json",
   "manifests": [
      {
         "mediaType": "application/vnd.oci.image.manifest.v1+json",
         "digest": "sha256:a1fbaea309fa27bad418200539a69cffb4c9336fe1a6b0af23874cd15293c8f8",
         "size": 2698,
         "platform": {
            "architecture": "amd64",
            "os": "linux"
         }
      },
      {
         "mediaType": "application/vnd.oci.image.manifest.v1+json",
         "digest": "sha256:e3abb4dd6a65d41ab07ab7bc55f9d37f55ec938a65a9459fa14b68118c3adc4a",
         "size": 2698,
         "platform": {
            "architecture": "arm64",
            "os": "linux"
         }
      },
      {
         "mediaType": "application/vnd.oci.image.manifest.v1+json",
         "digest": "sha256:f7fe7319870e8d3665db3df375cdec996b1e1428b62ac4bd5e4373b16692925b",
         "size": 2698,
         "platform": {
            "architecture": "ppc64le",
            "os": "linux"
         }
      },
      {
         "mediaType": "application/vnd.oci.image.manifest.v1+json",
         "digest": "sha256:de11765000a4c3504b08489dc64b8758c68c43425d4f9093485f6dd18156fa64",
         "size": 2698,
         "platform": {
            "architecture": "s390x",
            "os": "linux"
         }
      }
   ]
}`

var v1ociManifest = `{
   "schemaVersion": 2,
   "mediaType": "application/vnd.oci.image.manifest.v1+json",
   "config": {
      "mediaType": "application/vnd.oci.image.config.v1+json",
      "digest": "sha256:b9e6889272c9e672fa749795344385882b2696b0f302c6430a427a4377044a7a",
      "size": 2963
   },
   "layers": [
      {
         "mediaType": "application/vnd.oci.image.layer.v1.tar+gzip",
         "digest": "sha256:804c8aba2cc61168600515a6831474978d0ea8faddd8a66f99cc9f2bbd576105",
         "size": 84007
      },
      {
         "mediaType": "application/vnd.oci.image.layer.v1.tar+gzip",
         "digest": "sha256:2ae710cd8bfef4545fa3a6dc274d6b7a991ca379cdaa3cdf460d5cb5840a3c88",
         "size": 20316
      },
      {
         "mediaType": "application/vnd.oci.image.layer.v1.tar+gzip",
         "digest": "sha256:d462aa3453675bb1f9a271a72cc72a53e628521a7d0e94b720bd07f9ca4962dc",
         "size": 634160
      },
      {
         "mediaType": "application/vnd.oci.image.layer.v1.tar+gzip",
         "digest": "sha256:0f8b424aa0b96c1c388a5fd4d90735604459256336853082afb61733438872b5",
         "size": 75
      },
      {
         "mediaType": "application/vnd.oci.image.layer.v1.tar+gzip",
         "digest": "sha256:d557676654e572af3e3173c90e7874644207fda32cd87e9d3d66b5d7b98a7b21",
         "size": 193
      },
      {
         "mediaType": "application/vnd.oci.image.layer.v1.tar+gzip",
         "digest": "sha256:c8022d07192eddbb2a548ba83be5e412f7ba863bbba158d133c9653bb8a47768",
         "size": 130
      },
      {
         "mediaType": "application/vnd.oci.image.layer.v1.tar+gzip",
         "digest": "sha256:d858cbc252ade14879807ff8dbc3043a26bbdb92087da98cda831ee040b172b3",
         "size": 173
      },
      {
         "mediaType": "application/vnd.oci.image.layer.v1.tar+gzip",
         "digest": "sha256:1069fc2daed1aceff7232f4b8ab21200dd3d8b04f61be9da86977a34a105dfdc",
         "size": 97
      },
      {
         "mediaType": "application/vnd.oci.image.layer.v1.tar+gzip",
         "digest": "sha256:b40161cd83fc5d470d6abe50e87aa288481b6b89137012881d74187cfbf9f502",
         "size": 382
      },
      {
         "mediaType": "application/vnd.oci.image.layer.v1.tar+gzip",
         "digest": "sha256:5318d93a3a6582d0351c833fa3cf04ab41352b2e6c77c9ec3d330581eb267683",
         "size": 327
      },
      {
         "mediaType": "application/vnd.oci.image.layer.v1.tar+gzip",
         "digest": "sha256:307c1adadb60e6e9b8aca553ec620d77fedc112737cc54e9ee73ac165e7f3cbc",
         "size": 122110
      },
      {
         "mediaType": "application/vnd.oci.image.layer.v1.tar+gzip",
         "digest": "sha256:de7e62b1dbc9b34ca90c74b5d488902526b0d0c9831b50b17d7b1177bc26ad59",
         "size": 9001101
      },
      {
         "mediaType": "application/vnd.oci.image.layer.v1.tar+gzip",
         "digest": "sha256:28be117d08e0c80f3951e6c2d7368a4e256f9dcffcc705afe59ca22b7d887d17",
         "size": 6884032
      },
      {
         "mediaType": "application/vnd.oci.image.layer.v1.tar+gzip",
         "digest": "sha256:01b51d4646ff74c5148e9912c89ee06dbff48bfddfb34270b4f58cac0bbbd698",
         "size": 5689289
      }
   ],
   "annotations": {
      "org.opencontainers.image.base.digest": "sha256:e6d589f36c6c7d9a14df69da026b446ac03c0d2027bfca82981b6a1256c2019c",
      "org.opencontainers.image.base.name": "gcr.io/distroless/static-debian11@sha256:1dbe426d60caed5d19597532a2d74c8056cd7b1674042b88f7328690b5ead8ed"
   }
}`

func init() {
	log.SetOutput(io.Discard)
}

type manifestTest struct {
	imgurl    string
	mediatype string
	digest    string
	bytes     []byte
	mtype     upstream.ManifestType
}

var compatTests = []manifestTest{
	{
		imgurl:    "registry.k8s.io/pause:3.8",
		mediatype: "application/vnd.docker.distribution.manifest.list.v2+json",
		digest:    "9001185023633d17a2f98ff69b6ff2615b8ea02a825adffa40422f51dfdcde9d",
		bytes:     []byte(v2dockerManifestList),
		mtype:     upstream.V2dockerManifestList,
	},
	{
		imgurl:    "registry.k8s.io/pause@sha256:f5944f2d1daf66463768a1503d0c8c5e8dde7c1674d3f85abc70cef9c7e32e95",
		mediatype: "application/vnd.docker.distribution.manifest.v2+json",
		digest:    "f5944f2d1daf66463768a1503d0c8c5e8dde7c1674d3f85abc70cef9c7e32e95",
		bytes:     []byte(v2dockerManifest),
		mtype:     upstream.V2dockerManifest,
	},
	{
		imgurl:    "quay.io/coreos/etcd:v3.5.18",
		mediatype: "application/vnd.oci.image.index.v1+json",
		digest:    "d0a641d5fbcc89678c931a61b7de7b8a1cf097149f135c9c73bc81d076a1494b",
		bytes:     []byte(v1ociIndex),
		mtype:     upstream.V1ociIndex,
	},
	{
		imgurl:    "quay.io/coreos/etcd@sha256:a1fbaea309fa27bad418200539a69cffb4c9336fe1a6b0af23874cd15293c8f8",
		mediatype: "application/vnd.oci.image.manifest.v1+json",
		digest:    "a1fbaea309fa27bad418200539a69cffb4c9336fe1a6b0af23874cd15293c8f8",
		bytes:     []byte(v1ociManifest),
		mtype:     upstream.V1ociDescriptor,
	},
}

// Test that old 'upstream.ManifestHolder' can be read as new 'imgpull.ManifestHolder'. Some fields in the
// old struct were not carried forward but that's ok they are not needed in the new implementation.
func TestReadOldManifestHolders(t *testing.T) {
	td, _ := os.MkdirTemp("", "")
	defer os.RemoveAll(td)

	var mhOut upstream.ManifestHolder
	fname := "testfile"
	pr := pullrequest.NewPullRequest("foo", "bar", "baz", "frobozz")
	for _, tst := range compatTests {
		mhOut = upstream.ManifestHolder{
			Pr:        pr,
			ImageUrl:  tst.imgurl,
			MediaType: tst.mediatype,
			Digest:    tst.digest,
			Bytes:     tst.bytes,
			Type:      tst.mtype,
		}
		switch tst.mtype {
		case upstream.V2dockerManifestList:
			if err := json.Unmarshal(mhOut.Bytes, &mhOut.V2dockerManifestList); err != nil {
				t.Fail()
			}
		case upstream.V2dockerManifest:
			if err := json.Unmarshal(mhOut.Bytes, &mhOut.V2dockerManifest); err != nil {
				t.Fail()
			}

		case upstream.V1ociIndex:
			if err := json.Unmarshal(mhOut.Bytes, &mhOut.V1ociIndex); err != nil {
				t.Fail()
			}

		case upstream.V1ociDescriptor:
			if err := json.Unmarshal(mhOut.Bytes, &mhOut.V1ociManifest); err != nil {
				t.Fail()
			}
		}
		outBytes, _ := json.Marshal(mhOut)
		if err := os.WriteFile(fname, outBytes, 0755); err != nil {
			t.Fail()
		}
		inBytes, err := os.ReadFile(fname)
		if err != nil {
			t.Fail()
		}
		mhIn := imgpull.ManifestHolder{}
		if err = json.Unmarshal(inBytes, &mhIn); err != nil {
			t.Fail()
		}
		if int(mhOut.Type) != int(mhIn.Type) {
			t.Fail()
		}
		if mhOut.Digest != mhIn.Digest {
			t.Fail()
		}
		if !bytes.Equal(mhOut.Bytes, mhIn.Bytes) {
			t.Fail()
		}
		if mhOut.ImageUrl != mhIn.ImageUrl {
			t.Fail()
		}
		if mhOut.MediaType != mhIn.MediaType() {
			t.Fail()
		}
		mhOutStr, err := mhOut.ToString()
		if err != nil {
			t.Fail()
		}
		mhInStr, err := mhIn.ToString()
		if err != nil {
			t.Fail()
		}
		if mhOutStr != mhInStr {
			t.Fail()
		}
	}
}
func TestSaveLoadCompare(t *testing.T) {
	td, _ := os.MkdirTemp("", "")
	defer os.RemoveAll(td)

	imageURL := "registry.k8s.io/pause:3.8"
	digest := "9001185023633d17a2f98ff69b6ff2615b8ea02a825adffa40422f51dfdcde9d"

	mhOut, err := imgpull.NewManifestHolder(imgpull.MediaTypeFrom[imgpull.V2dockerManifestList], []byte(v2dockerManifestList), digest, imageURL)
	if err != nil {
		t.Fail()
	}
	MhToFilesystem(mhOut, td)
	fname := filepath.Join(td, "fat", mhOut.Digest)
	_, err = os.Stat(fname)
	if err != nil {
		t.Fail()
	}
	b, err := os.ReadFile(fname)
	if err != nil {
		t.Fail()
	}
	mhIn := imgpull.ManifestHolder{}
	err = json.Unmarshal(b, &mhIn)
	if err != nil {
		t.Fail()
	}
	same := reflect.DeepEqual(mhOut, mhIn)
	if !same {
		t.Fail()
	}
}

// // TODO TEST THIS
// func TestSaveLoadAddToCache(t *testing.T) {
// 	td, _ := os.MkdirTemp("", "")
// 	defer os.RemoveAll(td)
// 	pr := pullrequest.NewPullRequest("foo", "bar", "baz", "frobozz")
// 	mhOut := upstream.ManifestHolder{
// 		Pr:        pr,
// 		ImageUrl:  "registry.k8s.io/pause:3.8",
// 		MediaType: "application/vnd.docker.distribution.manifest.list.v2+json",
// 		Digest:    "9001185023633d17a2f98ff69b6ff2615b8ea02a825adffa40422f51dfdcde9d",
// 		Size:      2761,
// 		Bytes:     []byte{},
// 		Type:      upstream.V2dockerManifestList,
// 	}
// 	MhToFilesystem(mhOut, td)
// 	err := FromFilesystem(td)
// 	if err != nil {
// 		t.Fail()
// 	}
// 	mh, exists := memcache.IsCached(pr)
// 	if !exists {
// 		t.Fail()
// 	}
// 	if mh.Pr != pr {
// 		t.Fail()
// 	}
// }
