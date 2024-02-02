package main

import (
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/cache"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func main() {
	//do("docker.io/calico/pod2daemon-flexvol:v3.27.0", "/tmp/cranetest.tar")
	do("localhost:5000/registry:2.8.3", "/tmp/dockertest.tar")
}

func do(src string, path string) error {
	var (
		cachePath string              = "/tmp"
		imageMap  map[string]v1.Image = map[string]v1.Image{}
	)
	ref, err := name.ParseReference(src, make([]name.Option, 0)...)
	if err != nil {
		return err
	}
	// BASIC AUTH WORKS!!
	basic := &authn.Basic{Username: "ericace", Password: "ericace"}
	ba := func(o *crane.Options) {
		// only one is allowed
		o.Remote[0] = remote.WithAuth(basic)
	}
	// TODO TEST TLS
	// TODO https://gist.github.com/ncw/9253562
	tls := func(o *crane.Options) {
		transport := remote.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
		o.Transport = transport
	}
	o := crane.GetOptions(ba, tls)
	rmt, err := remote.Get(ref, o.Remote...)
	if err != nil {
		return err
	}
	img, err := rmt.Image()
	if err != nil {
		return err
	}
	if cachePath != "" {
		img = cache.Image(img, cache.NewFilesystemCache(cachePath))
	}
	imageMap[src] = img
	if err := crane.MultiSave(imageMap, path); err != nil {
		return fmt.Errorf("saving tarball %s: %w", path, err)
	}
	return nil
}
