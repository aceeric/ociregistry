// implements the "pull-only" registry server. Provides implementations for methods
// required to pull an image. This file is lean to simplify handling any changes to
// the API - each function simply calls a handler in 'handlers.go' if one is defined
// otherwise returns 405 not allowed.
package impl

import (
	"net/http"
	"regexp"

	"github.com/aceeric/ociregistry/api/models"
	"github.com/aceeric/ociregistry/impl/config"

	_ "crypto/sha256"
	_ "crypto/sha512"
)

// OciRegistry implements the OCI Distribution REST API.
type OciRegistry struct {
	// base location of the image and metadata cache
	imagePath string
	// timeout in milliseconds for pulling from upstreams
	pullTimeout int
	// images whose canonical url matches any of these are always pulled from the upstream
	// (unless air-gapped), with the cached copy served if the pull fails
	noCache []*regexp.Regexp
	// if air-gapped, we can't pull so don't try just return 404
	airGapped bool
	// supports pull thru like 'docker pull ociregistry:8080/hello-word' (i.e. no namespace so assume docker.io)
	defaultNs string
	// allows to shut down the HTTP server
	shutdownCh chan bool
}

// NewOciRegistry creates and returns an OciRegistry struct from global configuration. The
// passed channel allows the /cmd/stop endpoint to signal the REST server to shut down.
// The OciRegistry struct returned by the function implements the api.ServerInterface interface,
// which is generated from the api/ociregistry.yaml openapi spec for the distribution server.
func NewOciRegistry(ch chan bool) *OciRegistry {
	return &OciRegistry{
		imagePath:   config.GetImagePath(),
		pullTimeout: int(config.GetPullTimeout()),
		noCache:     config.NoCachePatterns(),
		airGapped:   config.GetAirGapped(),
		defaultNs:   config.GetDefaultNs(),
		shutdownCh:  ch,
	}
}

// GET /
func (r *OciRegistry) Root(w http.ResponseWriter, req *http.Request) {
	writeJSON(w, http.StatusOK, nil)
}

// GET /v2/auth
func (r *OciRegistry) V2Auth(w http.ResponseWriter, req *http.Request, params models.V2AuthParams) {
	r.handleV2Auth(w, req, params)
}

// GET /v2/
func (r *OciRegistry) V2Default(w http.ResponseWriter, req *http.Request) {
	r.handleV2Default(w, req)
}

// HEAD /v2/
func (r *OciRegistry) V2HeadDefault(w http.ResponseWriter, req *http.Request) {
	r.handleV2HeadDefault(w, req)
}

// GET /v2/{s1}/blobs/{digest}
func (r *OciRegistry) V2GetS1BlobsDigest(w http.ResponseWriter, req *http.Request, s1 string, digest string) {
	r.handleV2BlobsDigest(w, req, digest, s1)
}

// GET /v2/{s1}/{s2}/blobs/{digest}
func (r *OciRegistry) V2GetS1S2BlobsDigest(w http.ResponseWriter, req *http.Request, s1 string, s2 string, digest string) {
	r.handleV2BlobsDigest(w, req, digest, s1, s2)
}

// GET /v2/{s1}/{s2}/{s3}/blobs/{digest}
func (r *OciRegistry) V2GetS1S2S3BlobsDigest(w http.ResponseWriter, req *http.Request, s1 string, s2 string, s3 string, digest string) {
	r.handleV2BlobsDigest(w, req, digest, s1, s2, s3)
}

// GET /v2/{s1}/{s2}/{s3}/{s4}/blobs/{digest}
func (r *OciRegistry) V2GetS1S2S3S4BlobsDigest(w http.ResponseWriter, req *http.Request, s1 string, s2 string, s3 string, s4 string, digest string) {
	r.handleV2BlobsDigest(w, req, digest, s1, s2, s3, s4)
}

// HEAD /v2/{s1}/manifests/{reference}
func (r *OciRegistry) V2HeadS1ManifestsReference(w http.ResponseWriter, req *http.Request, s1 string, reference string, params models.V2HeadS1ManifestsReferenceParams) {
	r.handleV2ManifestsReference(w, req, reference, params.Ns, http.MethodHead, s1)
}

// HEAD /v2/{s1}/{s2}/manifests/{reference}
func (r *OciRegistry) V2HeadS1S2ManifestsReference(w http.ResponseWriter, req *http.Request, s1 string, s2 string, reference string, params models.V2HeadS1S2ManifestsReferenceParams) {
	r.handleV2ManifestsReference(w, req, reference, params.Ns, http.MethodHead, s1, s2)
}

// HEAD /v2/{s1}/{s2}/{s3}/manifests/{reference}
func (r *OciRegistry) V2HeadS1S2S3ManifestsReference(w http.ResponseWriter, req *http.Request, s1 string, s2 string, s3 string, reference string, params models.V2HeadS1S2S3ManifestsReferenceParams) {
	r.handleV2ManifestsReference(w, req, reference, params.Ns, http.MethodHead, s1, s2, s3)
}

// HEAD /v2/{s1}/{s2}/{s3}/{s4}/manifests/{reference}
func (r *OciRegistry) V2HeadS1S2S3S4ManifestsReference(w http.ResponseWriter, req *http.Request, s1 string, s2 string, s3 string, s4 string, reference string, params models.V2HeadS1S2S3S4ManifestsReferenceParams) {
	r.handleV2ManifestsReference(w, req, reference, params.Ns, http.MethodHead, s1, s2, s3, s4)
}

// GET /v2/{s1}/manifests/{reference}
func (r *OciRegistry) V2GetS1ManifestsReference(w http.ResponseWriter, req *http.Request, s1 string, reference string, params models.V2GetS1ManifestsReferenceParams) {
	r.handleV2ManifestsReference(w, req, reference, params.Ns, http.MethodGet, s1)
}

// GET /v2/{s1}/{s2}/manifests/{reference}
func (r *OciRegistry) V2GetS1S2ManifestsReference(w http.ResponseWriter, req *http.Request, s1 string, s2 string, reference string, params models.V2GetS1S2ManifestsReferenceParams) {
	r.handleV2ManifestsReference(w, req, reference, params.Ns, http.MethodGet, s1, s2)
}

// GET /v2/{s1}/{s2}/{s3}/manifests/{reference}
func (r *OciRegistry) V2GetS1S2S3ManifestsReference(w http.ResponseWriter, req *http.Request, s1 string, s2 string, s3 string, reference string, params models.V2GetS1S2S3ManifestsReferenceParams) {
	r.handleV2ManifestsReference(w, req, reference, params.Ns, http.MethodGet, s1, s2, s3)
}

// GET /v2/{s1}/{s2}/{s3}/{s4}/manifests/{reference}
func (r *OciRegistry) V2GetS1S2S3S4ManifestsReference(w http.ResponseWriter, req *http.Request, s1 string, s2 string, s3 string, s4 string, reference string, params models.V2GetS1S2S3S4ManifestsReferenceParams) {
	r.handleV2ManifestsReference(w, req, reference, params.Ns, http.MethodGet, s1, s2, s3, s4)
}

// unimplemented methods of the OCI distribution spec

func (r *OciRegistry) V2HeadS1S2S3S4BlobsDigest(w http.ResponseWriter, req *http.Request, s1 string, s2 string, s3 string, s4 string, digest string) {
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2HeadS1S2S3BlobsDigest(w http.ResponseWriter, req *http.Request, s1 string, s2 string, s3 string, digest string) {
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2HeadS1S2BlobsDigest(w http.ResponseWriter, req *http.Request, s1 string, s2 string, digest string) {
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2HeadS1BlobsDigest(w http.ResponseWriter, req *http.Request, s1 string, digest string) {
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2PostNameBlobsUploads(w http.ResponseWriter, req *http.Request, name string, params models.V2PostNameBlobsUploadsParams) {
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2GetNameBlobsUploadsReference(w http.ResponseWriter, req *http.Request, name string, reference string) {
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2PatchNameBlobsUploadsReference(w http.ResponseWriter, req *http.Request, name string, reference string) {
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2PutNameBlobsUploadsReference(w http.ResponseWriter, req *http.Request, name string, reference string, params models.V2PutNameBlobsUploadsReferenceParams) {
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2PutS1S2S3S4ManifestsReference(w http.ResponseWriter, req *http.Request, s1 string, s2 string, s3 string, s4 string, reference string) {
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2PutS1S2S3ManifestsReference(w http.ResponseWriter, req *http.Request, s1 string, s2 string, s3 string, reference string) {
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2PutS1S2ManifestsReference(w http.ResponseWriter, req *http.Request, s1 string, s2 string, reference string) {
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2GetNameReferrersDigest(w http.ResponseWriter, req *http.Request, name string, digest string, params models.V2GetNameReferrersDigestParams) {
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2GetNameTagsList(w http.ResponseWriter, req *http.Request, name string, params models.V2GetNameTagsListParams) {
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2DeleteS1S2S3S4BlobsDigest(w http.ResponseWriter, req *http.Request, s1 string, s2 string, s3 string, s4 string, digest string) {
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2DeleteS1S2S3BlobsDigest(w http.ResponseWriter, req *http.Request, s1 string, s2 string, s3 string, digest string) {
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2DeleteS1S2BlobsDigest(w http.ResponseWriter, req *http.Request, s1 string, s2 string, digest string) {
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2DeleteS1BlobsDigest(w http.ResponseWriter, req *http.Request, s1 string, digest string) {
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2DeleteS1S2S3S4ManifestsReference(w http.ResponseWriter, req *http.Request, s1 string, s2 string, s3 string, s4 string, reference string) {
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2DeleteS1S2S3ManifestsReference(w http.ResponseWriter, req *http.Request, s1 string, s2 string, s3 string, reference string) {
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2DeleteS1S2ManifestsReference(w http.ResponseWriter, req *http.Request, s1 string, s2 string, reference string) {
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2DeleteS1ManifestsReference(w http.ResponseWriter, req *http.Request, s1 string, reference string) {
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2PutS1ManifestsReference(w http.ResponseWriter, req *http.Request, s1 string, reference string) {
	w.WriteHeader(http.StatusMethodNotAllowed)
}
