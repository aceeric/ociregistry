package v2docker

type ManifestList struct {
	SchemaVersion int64                `json:"schemaVersion"`
	MediaType     string               `json:"mediaType"`
	Manifests     []ManifestDescriptor `json:"manifests"`
}

type ManifestDescriptor struct {
	MediaType   string            `json:"mediaType,omitempty"`
	Digest      string            `json:"digest,omitempty"`
	Size        int64             `json:"size,omitempty"`
	URLs        []string          `json:"urls,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Platform    PlatformSpec      `json:"platform"`
}

type PlatformSpec struct {
	Architecture string   `json:"architecture"`
	OS           string   `json:"os"`
	OSVersion    string   `json:"os.version,omitempty"`
	OSFeatures   []string `json:"os.features,omitempty"`
	Variant      string   `json:"variant,omitempty"`
	Features     []string `json:"features,omitempty"`
}

type Manifest struct {
	SchemaVersion int64             `json:"schemaVersion"`
	MediaType     string            `json:"mediaType,omitempty"`
	Config        Descriptor        `json:"config"`
	Layers        []Descriptor      `json:"layers"`
	Annotations   map[string]string `json:"annotations,omitempty"`
}

type Descriptor struct {
	MediaType   string            `json:"mediaType,omitempty"`
	Digest      string            `json:"digest,omitempty"`
	Size        int64             `json:"size,omitempty"`
	URLs        []string          `json:"urls,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Platform    *Platform         `json:"platform,omitempty"`
}

type Platform struct {
	Architecture string   `json:"architecture"`
	OS           string   `json:"os"`
	OSVersion    string   `json:"os.version,omitempty"`
	OSFeatures   []string `json:"os.features,omitempty"`
	Variant      string   `json:"variant,omitempty"`
}
