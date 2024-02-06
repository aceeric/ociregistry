package pullsync

import (
	"fmt"
	"ociregistry/globals"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/cache"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// cranePull uses the 'crane pull' functionality from
// github.com/google/go-containerregistry to pull an image from an
// upstream registry. The image is in the 'image' arg, which must contain
// a registry server ref. For example: docker.io/hello-world:latest. The
// image is pulled saved to a tarball at the fqpn specified in the 'path' arg,
// e.g. /var/ociregistry/images/<uuid>.tar
func cranePull(image string, path string) error {
	// TODO make configurable
	var cachePath string = "/tmp"
	ref, err := name.ParseReference(image, make([]name.Option, 0)...)
	if err != nil {
		return err
	}
	opts, err := configFor(ref.Context().Registry.Name())
	if err != nil {
		globals.Logger().Warn(err.Error())
	}
	o := crane.GetOptions(opts...)
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
	if err := crane.MultiSave(map[string]v1.Image{image: img}, path); err != nil {
		return fmt.Errorf("error saving tarball %s: %w", path, err)
	}
	return nil
}
