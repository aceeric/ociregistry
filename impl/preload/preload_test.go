package preload

import (
	"encoding/json"
	"ociregistry/impl/globals"
	"ociregistry/impl/pullrequest"
	"ociregistry/impl/upstream"
	"ociregistry/mock"
	"os"
	"path/filepath"
	"testing"
)

var mfst = `{
	"schemaVersion": 2,
	"mediaType": "application/vnd.docker.distribution.manifest.list.v2+json",
	"manifests": [
	   {
		  "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
		  "size": 526,
		  "digest": "sha256:f5944f2d1daf66463768a1503d0c8c5e8dde7c1674d3f85abc70cef9c7e32e95",
		  "platform": {
			 "architecture": "amd64",
			 "os": "linux"
		  }
	   },
	   {
		  "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
		  "size": 526,
		  "digest": "sha256:27295ffe5a75328e8230ff9bcabe2b54ebb9079ff70344d73a7b7c7e163ee1a6",
		  "platform": {
			 "architecture": "arm",
			 "os": "linux",
			 "variant": "v7"
		  }
	   },
	   {
		  "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
		  "size": 526,
		  "digest": "sha256:566af08540f378a70a03588f3963b035f33c49ebab3e4e13a4f5edbcd78c6689",
		  "platform": {
			 "architecture": "arm64",
			 "os": "linux"
		  }
	   },
	   {
		  "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
		  "size": 526,
		  "digest": "sha256:2f205253a51c641263b155d48460ee2056c5b5013f8239ae3811792ec63b3546",
		  "platform": {
			 "architecture": "ppc64le",
			 "os": "linux"
		  }
	   },
	   {
		  "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
		  "size": 526,
		  "digest": "sha256:7eaeb31509d7f370599ef78d55956e170eafb7f4a75b8dc14b5c06071d13aae0",
		  "platform": {
			 "architecture": "s390x",
			 "os": "linux"
		  }
	   },
	   {
		  "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
		  "size": 1969,
		  "digest": "sha256:78bfb9d8999c190fca79871c4b2f8d69d94a0605266f0bbb2dbaa1b6dfd03720",
		  "platform": {
			 "architecture": "amd64",
			 "os": "windows",
			 "os.version": "10.0.17763.2928"
		  }
	   },
	   {
		  "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
		  "size": 1969,
		  "digest": "sha256:9d05676469a08d6dba9889297333b7d1768e44e38075ab5350a4f8edd97f5be1",
		  "platform": {
			 "architecture": "amd64",
			 "os": "windows",
			 "os.version": "10.0.19042.1706"
		  }
	   },
	   {
		  "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
		  "size": 1969,
		  "digest": "sha256:e8fb66bcfe1a85ec1299652d28e6f7f9cfbb01d33c6260582a42971d30dcb77d",
		  "platform": {
			 "architecture": "amd64",
			 "os": "windows",
			 "os.version": "10.0.20348.707"
		  }
	   }
	]
 }`

func TestManifestHolder(t *testing.T) {
	mh := upstream.ManifestHolder{
		Pr:        pullrequest.PullRequest{},
		ImageUrl:  "registry.k8s.io/pause:3.8",
		MediaType: "application/vnd.docker.distribution.manifest.list.v2+json",
		Digest:    "f5944f2d1daf66463768a1503d0c8c5e8dde7c1674d3f85abc70cef9c7e32e95",
		Size:      2761,
		Bytes:     []byte{},
		Type:      upstream.V2dockerManifestList,
	}
	err := json.Unmarshal([]byte(mfst), &mh.V2dockerManifestList)
	if err != nil {
		t.Fail()
	}
	digest, err := getImageManifestDigest(mh, "amd64", "linux")
	if err != nil {
		t.Fail()
	}
	if digest != "sha256:"+mh.Digest {
		t.Fail()
	}
}

func TestPreload(t *testing.T) {
	globals.ConfigureLogging("error")
	server, url := mock.Server(mock.NewMockParams(mock.NONE, mock.HTTP))
	defer server.Close()
	d, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fail()
	}
	defer os.RemoveAll(d)
	cnt, err := preloadOneImage(url+"/hello-world:latest", d, "amd64", "linux", 1000)
	// count is 2 because one manifest list and one image manifest
	if err != nil || cnt != 2 {
		t.Fail()
	}
	// the hello-world latest image has two blobs
	blobs, _ := os.ReadDir(filepath.Join(d, "blobs"))
	if len(blobs) != 2 {
		t.Fail()
	}
}
