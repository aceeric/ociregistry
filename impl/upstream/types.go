package upstream

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
	Config   string   `json:"Config"`
	RepoTags []string `json:"RepoTags"`
	Layers   []string `json:"Layers"`
}

// ManifestPlatform is the 'platform' entry of a 'ManifestItem'
type ManifestPlatform struct {
	architecture string `json:"Architecture"`
	os           string `json:"Os"`
	variant      string `json:"Variant"`
}

// ManifestItem is one manifest in the 'manifests' list of 'ManifestList'
type ManifestItem struct {
	MediaType string           `json:"MediaType"`
	Size      int              `json:"Size"`
	digest    string           `json:"Digest"`
	platform  ManifestPlatform `json:"Platform"`
}

// ManifestList is an image manifest provided when querying a manifest by tag
type ManifestList struct {
	SchemaVersion int            `json:"SchemaVersion"`
	MediaType     string         `json:"MediaType"`
	Manifests     []ManifestItem `json:"Manifests"`
}

type ManifestType int

const (
	ManfestList ManifestType = iota
)

type ManifestHolder struct {
	ImageUrl  string
	MediaType string
	Digest    string
	Size      int
	Bytes     []byte
	Ml        ManifestList
	Im        ImageManifest
	Tarfile   string
}
