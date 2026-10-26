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

	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
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
	// allows to shut down the echo server
	shutdownCh chan bool
}

// NewOciRegistry creates and returns an OciRegistry struct from global configuration. The
// passed channel allows the /cmd/stop endpoint to signal the REST server to shut down.
// The OciRegistry struct returned by the function implements the api.ServerInterface interface,
// which is generated from the api/ociregistry.yaml openapi spec for the distribution server.
//
// The --no-cache patterns are validated when the server starts (see subcmd.Serve), so any
// that don't compile have already stopped the server; here they are only logged.
func NewOciRegistry(ch chan bool) *OciRegistry {
	noCache, err := config.NoCachePatterns()
	if err != nil {
		log.Error(err)
	}
	return &OciRegistry{
		imagePath:   config.GetImagePath(),
		pullTimeout: int(config.GetPullTimeout()),
		noCache:     noCache,
		airGapped:   config.GetAirGapped(),
		defaultNs:   config.GetDefaultNs(),
		shutdownCh:  ch,
	}
}

// alwaysPull returns true if the passed canonical image url must be pulled from the upstream
// on every request because it matches a --no-cache pattern. Never true when air-gapped.
func (r *OciRegistry) alwaysPull(url string) bool {
	if r.airGapped {
		return false
	}
	for _, re := range r.noCache {
		if re.MatchString(url) {
			return true
		}
	}
	return false
}

// GET /
func (r *OciRegistry) Root(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, nil)
}

// GET /v2/auth
func (r *OciRegistry) V2Auth(ctx echo.Context, params models.V2AuthParams) error {
	return r.handleV2Auth(ctx, params)
}

// GET /v2/
func (r *OciRegistry) V2Default(ctx echo.Context) error {
	return r.handleV2Default(ctx)
}

// HEAD /v2/
func (r *OciRegistry) V2HeadDefault(ctx echo.Context) error {
	return r.handleV2HeadDefault(ctx)
}

// GET /v2/{s1}/blobs/{digest}
func (r *OciRegistry) V2GetS1BlobsDigest(ctx echo.Context, s1 string, digest string) error {
	return r.handleV2BlobsDigest(ctx, digest, s1)
}

// GET /v2/{s1}/{s2}/blobs/{digest}
func (r *OciRegistry) V2GetS1S2BlobsDigest(ctx echo.Context, s1 string, s2 string, digest string) error {
	return r.handleV2BlobsDigest(ctx, digest, s1, s2)
}

// GET /v2/{s1}/{s2}/{s3}/blobs/{digest}
func (r *OciRegistry) V2GetS1S2S3BlobsDigest(ctx echo.Context, s1 string, s2 string, s3 string, digest string) error {
	return r.handleV2BlobsDigest(ctx, digest, s1, s2, s3)
}

// GET /v2/{s1}/{s2}/{s3}/{s4}/blobs/{digest}
func (r *OciRegistry) V2GetS1S2S3S4BlobsDigest(ctx echo.Context, s1 string, s2 string, s3 string, s4 string, digest string) error {
	return r.handleV2BlobsDigest(ctx, digest, s1, s2, s3, s4)
}

// HEAD /v2/{s1}/manifests/{reference}
func (r *OciRegistry) V2HeadS1ManifestsReference(ctx echo.Context, s1 string, reference string, params models.V2HeadS1ManifestsReferenceParams) error {
	return r.handleV2ManifestsReference(ctx, reference, params.Ns, http.MethodHead, s1)
}

// HEAD /v2/{s1}/{s2}/manifests/{reference}
func (r *OciRegistry) V2HeadS1S2ManifestsReference(ctx echo.Context, s1 string, s2 string, reference string, params models.V2HeadS1S2ManifestsReferenceParams) error {
	return r.handleV2ManifestsReference(ctx, reference, params.Ns, http.MethodHead, s1, s2)
}

// HEAD /v2/{s1}/{s2}/{s3}/manifests/{reference}
func (r *OciRegistry) V2HeadS1S2S3ManifestsReference(ctx echo.Context, s1 string, s2 string, s3 string, reference string, params models.V2HeadS1S2S3ManifestsReferenceParams) error {
	return r.handleV2ManifestsReference(ctx, reference, params.Ns, http.MethodHead, s1, s2, s3)
}

// HEAD /v2/{s1}/{s2}/{s3}/{s4}/manifests/{reference}
func (r *OciRegistry) V2HeadS1S2S3S4ManifestsReference(ctx echo.Context, s1 string, s2 string, s3 string, s4 string, reference string, params models.V2HeadS1S2S3S4ManifestsReferenceParams) error {
	return r.handleV2ManifestsReference(ctx, reference, params.Ns, http.MethodHead, s1, s2, s3, s4)
}

// GET /v2/{s1}/manifests/{reference}
func (r *OciRegistry) V2GetS1ManifestsReference(ctx echo.Context, s1 string, reference string, params models.V2GetS1ManifestsReferenceParams) error {
	return r.handleV2ManifestsReference(ctx, reference, params.Ns, http.MethodGet, s1)
}

// GET /v2/{s1}/{s2}/manifests/{reference}
func (r *OciRegistry) V2GetS1S2ManifestsReference(ctx echo.Context, s1 string, s2 string, reference string, params models.V2GetS1S2ManifestsReferenceParams) error {
	return r.handleV2ManifestsReference(ctx, reference, params.Ns, http.MethodGet, s1, s2)
}

// GET /v2/{s1}/{s2}/{s3}/manifests/{reference}
func (r *OciRegistry) V2GetS1S2S3ManifestsReference(ctx echo.Context, s1 string, s2 string, s3 string, reference string, params models.V2GetS1S2S3ManifestsReferenceParams) error {
	return r.handleV2ManifestsReference(ctx, reference, params.Ns, http.MethodGet, s1, s2, s3)
}

// GET /v2/{s1}/{s2}/{s3}/{s4}/manifests/{reference}
func (r *OciRegistry) V2GetS1S2S3S4ManifestsReference(ctx echo.Context, s1 string, s2 string, s3 string, s4 string, reference string, params models.V2GetS1S2S3S4ManifestsReferenceParams) error {
	return r.handleV2ManifestsReference(ctx, reference, params.Ns, http.MethodGet, s1, s2, s3, s4)
}

// unimplemented methods of the OCI distribution spec

func (r *OciRegistry) V2HeadS1S2S3S4BlobsDigest(ctx echo.Context, s1 string, s2 string, s3 string, s4 string, digest string) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2HeadS1S2S3BlobsDigest(ctx echo.Context, s1 string, s2 string, s3 string, digest string) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2HeadS1S2BlobsDigest(ctx echo.Context, s1 string, s2 string, digest string) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2HeadS1BlobsDigest(ctx echo.Context, s1 string, digest string) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2PostNameBlobsUploads(ctx echo.Context, name string, params models.V2PostNameBlobsUploadsParams) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2GetNameBlobsUploadsReference(ctx echo.Context, name string, reference string) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2PatchNameBlobsUploadsReference(ctx echo.Context, name string, reference string) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2PutNameBlobsUploadsReference(ctx echo.Context, name string, reference string, params models.V2PutNameBlobsUploadsReferenceParams) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2PutS1S2S3S4ManifestsReference(ctx echo.Context, s1 string, s2 string, s3 string, s4 string, reference string) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2PutS1S2S3ManifestsReference(ctx echo.Context, s1 string, s2 string, s3 string, reference string) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2PutS1S2ManifestsReference(ctx echo.Context, s1 string, s2 string, reference string) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2GetNameReferrersDigest(ctx echo.Context, name string, digest string, params models.V2GetNameReferrersDigestParams) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2GetNameTagsList(ctx echo.Context, name string, params models.V2GetNameTagsListParams) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2DeleteS1S2S3S4BlobsDigest(ctx echo.Context, s1 string, s2 string, s3 string, s4 string, digest string) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2DeleteS1S2S3BlobsDigest(ctx echo.Context, s1 string, s2 string, s3 string, digest string) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2DeleteS1S2BlobsDigest(ctx echo.Context, s1 string, s2 string, digest string) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2DeleteS1BlobsDigest(ctx echo.Context, s1 string, digest string) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2DeleteS1S2S3S4ManifestsReference(ctx echo.Context, s1 string, s2 string, s3 string, s4 string, reference string) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2DeleteS1S2S3ManifestsReference(ctx echo.Context, s1 string, s2 string, s3 string, reference string) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2DeleteS1S2ManifestsReference(ctx echo.Context, s1 string, s2 string, reference string) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2DeleteS1ManifestsReference(ctx echo.Context, s1 string, reference string) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2PutS1ManifestsReference(ctx echo.Context, s1 string, reference string) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}
