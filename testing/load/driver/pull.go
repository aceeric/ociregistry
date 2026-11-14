// This file simulates pulling an image tarball except it does not actually write
// any blob files or package them into an image tarball. It does this by omitting
// tarball creation and by passing `emptyPath` to puller.PullBlobs which causes that
// function to write to `io.Discard`.
//
// The goal of the load tester is to test the server, and the overhead of the driver
// having to write blobs to files, create tarballs, and then clean up the filesystem
// prevents the driver from fully exercising the server. The approach taken accesses
// all the ociregistry server endpoints for serving images, and streams all the blob
// data down to the client but dispenses with file IO on the client side.

package main

import (
	"fmt"

	"github.com/aceeric/imgpull/pkg/imgpull"
)

var emptyPath = ""

func pull(puller imgpull.Puller) error {
	md, err := puller.HeadManifest()
	if err != nil {
		return err
	}
	mh, err := getFromRemote(puller, md.Digest, md.IsImageManifest())
	if err != nil {
		return err
	}
	if mh.IsManifestList() {
		digest, err := mh.GetImageDigestFor(puller.GetOpts().OStype, puller.GetOpts().ArchType)
		if err != nil {
			return err
		}
		remote, registry := puller.GetRepoAndRemote()
		puller.SetUrl(urlWithDigest(remote, registry, digest))
		if _, err := getFromRemote(puller, digest, true); err != nil {
			return err
		}
	}
	return nil
}

func getFromRemote(puller imgpull.Puller, digest string, isImageManifest bool) (imgpull.ManifestHolder, error) {
	var mh imgpull.ManifestHolder
	var err error
	if isImageManifest {
		mh, err = puller.GetManifestByDigest(digest)
	} else {
		mh, err = puller.GetManifest()
	}
	if err != nil {
		return imgpull.ManifestHolder{}, err
	}
	if mh.IsImageManifest() {
		// emptyPath causes function to write blob output to io.Discard
		if err = puller.PullBlobs(mh, emptyPath); err != nil {
			return imgpull.ManifestHolder{}, err
		}
	}
	return mh, nil
}

func urlWithDigest(remote, repository, digest string) string {
	separator := "@"
	return fmt.Sprintf("%s/%s%s%s", remote, repository, separator, digest)
}
