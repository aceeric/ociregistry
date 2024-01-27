package apiimpl

type OciRegistry struct{}

// a pretend auth token
type Token struct {
	Token string `json:"token"`
}

// corresponds to the 'config' part of the manifest returned by this call
// curl http://localhost:8080/v2/appzygy/smallmain/manifests/v1.0.0
// {
//   "schemaVersion": 2,
//   "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
//   "config": {
//     "mediaType": "application/vnd.docker.container.image.v1+json",
//     "size": 1018,
//     "digest": "sha256:368bf668cf7b6b5e6d9e63798b84244da900a647a3d9da2a083e3e7a203e14e4"
//   },
// "layers": [
// ...

type ManifestConfig struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int    `json:"size"`
}

// one element of the 'layers' list above
type ManifestLayer struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int    `json:"size"`
}

// The enture manifest returned by
// curl http://localhost:8080/v2/appzygy/smallmain/manifests/v1.0.0
type ImageManifest struct {
	SchemaVersion int             `json:"schemaVersion"`
	MediaType     string          `json:"mediaType"`
	Config        ManifestConfig  `json:"config"`
	Layers        []ManifestLayer `json:"layers"`
}

// the manifest.json file in a tarball created by 'crane pull' or 'docker save'
type ManifestJson struct {
	Config   string   `json:"Config"`
	RepoTags []string `json:"RepoTags"`
	Layers   []string `json:"Layers"`
}