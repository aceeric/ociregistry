package upstream

import "ociregistry/impl/pullrequest"

// ManifestConfig corresponds to the 'config' part of an 'ImageManifest'
type ManifestConfig struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int    `json:"size"`
}

// ManifestLayer is one element of the 'layers' list in an 'ImageManifest'
type ManifestLayer struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int    `json:"size"`
}

// ImageManifest is an image manifest provided when querying a manifest by digest
type ImageManifest struct {
	SchemaVersion int             `json:"schemaVersion"`
	MediaType     string          `json:"mediaType"`
	Config        ManifestConfig  `json:"config"`
	Layers        []ManifestLayer `json:"layers"`
}

// ManifestJson is the 'manifest.json' file in a saved image tarball
type ManifestJson struct {
	Config   string   `json:"config"`
	RepoTags []string `json:"repoTags"`
	Layers   []string `json:"layers"`
}

// ManifestPlatform is the 'platform' entry of a 'ManifestItem'
type ManifestPlatform struct {
	Architecture string `json:"architecture"`
	Os           string `json:"os"`
	Variant      string `json:"variant"`
}

// ManifestItem is one manifest in the 'manifests' list of 'ManifestList'
type ManifestItem struct {
	MediaType string           `json:"mediaType"`
	Size      int              `json:"size"`
	Digest    string           `json:"digest"`
	Platform  ManifestPlatform `json:"platform"`
}

// ManifestList is an image manifest provided when querying a manifest by tag
type ManifestList struct {
	SchemaVersion int            `json:"schemaVersion"`
	MediaType     string         `json:"mediaType"`
	Manifests     []ManifestItem `json:"manifests"`
}

type ManifestType int

const (
	ManifestListType ManifestType = iota
	ImageManifestType
)

type ManifestHolder struct {
	Pr        pullrequest.PullRequest `json:"pullRequest"`
	ImageUrl  string                  `json:"imageUrl"`
	MediaType string                  `json:"mediaType"`
	Digest    string                  `json:"digest"`
	Size      int                     `json:"size"`
	Bytes     []byte                  `json:"bytes"`
	Ml        ManifestList            `json:"ml"`
	Im        ImageManifest           `json:"im"`
	Tarfile   string                  `json:"tarfile"`
	Type      ManifestType            `json:"type"`
}
