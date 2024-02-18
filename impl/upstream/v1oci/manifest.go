package v1oci

type Index struct {
	SchemaVersion int64             `json:"schemaVersion"`
	MediaType     string            `json:"mediaType,omitempty"`
	ArtifactType  string            `json:"artifactType,omitempty"`
	Manifests     []Descriptor      `json:"manifests"`
	Subject       *Descriptor       `json:"subject,omitempty"`
	Annotations   map[string]string `json:"annotations,omitempty"`
}

// Descriptor holds a reference from the manifest to one of its constituent elements.
type Descriptor struct {
	MediaType    string            `json:"mediaType"`
	Digest       string            `json:"digest"`
	Size         int64             `json:"size"`
	URLs         []string          `json:"urls,omitempty"`
	Annotations  map[string]string `json:"annotations,omitempty"`
	Data         []byte            `json:"data,omitempty"`
	Platform     *Platform         `json:"platform,omitempty"`
	ArtifactType string            `json:"artifactType,omitempty"`
}

type Platform struct {
	Architecture string   `json:"architecture"`
	Os           string   `json:"os"`
	OsVersion    string   `json:"os.version,omitempty"`
	OsFeatures   []string `json:"os.features,omitempty"`
	Variant      string   `json:"variant,omitempty"`
}

type Manifest struct {
	SchemaVersion int64             `json:"schemaVersion"`
	MediaType     string            `json:"mediaType,omitempty"`
	Config        Descriptor        `json:"config"`
	Layers        []Descriptor      `json:"layers"`
	Annotations   map[string]string `json:"annotations,omitempty"`
	Subject       *Descriptor       `json:"subject,omitempty"`
}
