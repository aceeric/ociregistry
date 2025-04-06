package upstream

import (
	"ociregistry/impl/pullrequest"
	"ociregistry/impl/upstream/v1oci"
	"ociregistry/impl/upstream/v2docker"
)

const (
	V2dockerManifestListMt = "application/vnd.docker.distribution.manifest.list.v2+json"
	V2dockerManifestMt     = "application/vnd.docker.distribution.manifest.v2+json"
	V1ociIndexMt           = "application/vnd.oci.image.index.v1+json"
	V1ociManifestMt        = "application/vnd.oci.image.manifest.v1+json"
)

type ManifestType int

const (
	V2dockerManifestList ManifestType = iota
	V2dockerManifest
	V1ociIndex
	V1ociDescriptor
	Unknown
)

type ManifestHolder struct {
	Pr                   pullrequest.PullRequest `json:"pullRequest"`
	ImageUrl             string                  `json:"imageUrl"`
	MediaType            string                  `json:"mediaType"`
	Digest               string                  `json:"digest"`
	Size                 int                     `json:"size"`
	Bytes                []byte                  `json:"bytes"`
	Type                 ManifestType            `json:"type"`
	V1ociIndex           v1oci.Index             `json:"v1.oci.index"`
	V1ociManifest        v1oci.Manifest          `json:"v1.oci.manifest"`
	V2dockerManifestList v2docker.ManifestList   `json:"v2.docker.manifestList"`
	V2dockerManifest     v2docker.Manifest       `json:"v2.docker.Manifest"`
}
