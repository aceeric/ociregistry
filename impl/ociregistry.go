// implements the "pull-only" registry server. Provides implementations for methods
// required to pull an image. This file is lean to simplify handling any changes to
// the API - each function simply calls a handler in 'handlers.go' if one is defined
// otherwise returns 405 not allowed.
package impl

import (
	"net/http"
	"ociregistry/api/models"
	"ociregistry/impl/config"
	"strings"

	_ "crypto/sha256"
	_ "crypto/sha512"

	"github.com/labstack/echo/v4"
)

// OciRegistry implements the OCI Distribution REST API.
type OciRegistry struct {
	// base location of the image and metadata cache
	imagePath string
	// timeout in milliseconds for pulling from upstreams
	pullTimeout int
	// if true then always pull images with tag 'latest' (act like a simple proxy)
	alwaysPullLatest bool
	// if air-gapped, we can't pull so don't try just return 404
	airGapped bool
	// allows to shut down the echo server
	shutdownCh chan bool
}

// NewOciRegistry creates and returns an OciRegistry struct from global configuration. The
// passed channel allows the /cmd/stop endpoint to signal the REST server to shut down.
// The OciRegistry struct returned by the function implements the api.ServerInterface interface,
// which is generated from the api/ociregistry.yaml openapi spec for the distribution server.
func NewOciRegistry(ch chan bool) *OciRegistry {
	return &OciRegistry{
		imagePath:        config.GetImagePath(),
		pullTimeout:      int(config.GetPullTimeout()),
		alwaysPullLatest: config.GetAlwaysPullLatest(),
		airGapped:        config.GetAirGapped(),
		shutdownCh:       ch,
	}
}

// CONNECT
func (r *OciRegistry) Connect(ctx echo.Context) error {
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

// note regarding these blob getters: in the handler everything except the digest is ignored because
// since the blob is content addressable storage the only thing that is needed is the digest. The other
// segments are just in the API because clients will expect those endpoints

// GET /v2/{image}/blobs/{digest}
func (r *OciRegistry) V2GetImageBlobsDigest(ctx echo.Context, image string, digest string) error {
	return r.handleV2GetOrgImageBlobsDigest(ctx, "", image, digest)
}

// GET /v2/{org}/{image}/blobs/{digest}
func (r *OciRegistry) V2GetOrgImageBlobsDigest(ctx echo.Context, org string, image string, digest string) error {
	return r.handleV2GetOrgImageBlobsDigest(ctx, org, image, digest)
}

// GET /v2/{ns}/{org}/{image}/blobs/{digest}
func (r *OciRegistry) V2GetNsOrgImageBlobsDigest(ctx echo.Context, ns string, org string, image string, digest string) error {
	return r.handleV2GetOrgImageBlobsDigest(ctx, org, image, digest)
}

// HEAD /v2/{image}/manifests/{reference}
func (r *OciRegistry) V2HeadImageManifestsReference(ctx echo.Context, image string, reference string, params models.V2HeadImageManifestsReferenceParams) error {
	return r.handleV2OrgImageManifestsReference(ctx, "", image, reference, http.MethodHead, params.Ns)
}

// HEAD /v2/{org}/{image}/manifests/{reference}
func (r *OciRegistry) V2HeadOrgImageManifestsReference(ctx echo.Context, org string, image string, reference string, params models.V2HeadOrgImageManifestsReferenceParams) error {
	if strings.Contains(org, ".") {
		// if /v2/docker.io/hello-world/manifests/latest then org is a namespace
		ns := org
		return r.handleV2OrgImageManifestsReference(ctx, "", image, reference, http.MethodHead, &ns)
	}
	return r.handleV2OrgImageManifestsReference(ctx, org, image, reference, http.MethodHead, params.Ns)
}

// HEAD /v2/{ns}/{org}/{image}/manifests/{reference}
func (r *OciRegistry) V2HeadNsOrgImageManifestsReference(ctx echo.Context, ns string, org string, image string, reference string) error {
	_ns := ns
	return r.handleV2OrgImageManifestsReference(ctx, org, image, reference, http.MethodHead, &_ns)
}

// GET /v2/{image}/manifests/{reference}
func (r *OciRegistry) V2GetImageManifestsReference(ctx echo.Context, image string, reference string, params models.V2GetImageManifestsReferenceParams) error {
	return r.handleV2OrgImageManifestsReference(ctx, "", image, reference, http.MethodGet, params.Ns)
}

// GET /v2/{org}/{image}/manifests/{reference}
func (r *OciRegistry) V2GetOrgImageManifestsReference(ctx echo.Context, org string, image string, reference string, params models.V2GetOrgImageManifestsReferenceParams) error {
	if strings.Contains(org, ".") {
		// if /v2/docker.io/hello-world/manifests/latest then org is a namespace
		ns := org
		return r.handleV2OrgImageManifestsReference(ctx, "", image, reference, http.MethodGet, &ns)
	}
	return r.handleV2OrgImageManifestsReference(ctx, org, image, reference, http.MethodGet, params.Ns)
}

// GET /v2/{ns}/{org}/{image}/manifests/{reference}
func (r *OciRegistry) V2GetNsOrgImageManifestsReference(ctx echo.Context, ns string, org string, image string, reference string) error {
	_ns := ns
	return r.handleV2OrgImageManifestsReference(ctx, org, image, reference, http.MethodGet, &_ns)
}

// unimplemented methods of the OCI distribution spec

func (r *OciRegistry) V2HeadNsOrgImageBlobsDigest(ctx echo.Context, ns string, org string, image string, digest string) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2HeadOrgImageBlobsDigest(ctx echo.Context, org string, image string, digest string) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2HeadImageBlobsDigest(ctx echo.Context, image string, digest string) error {
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

func (r *OciRegistry) V2PutNsOrgImageManifestsReference(ctx echo.Context, ns string, org string, image string, reference string) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2PutOrgImageManifestsReference(ctx echo.Context, org string, image string, reference string) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2GetNameReferrersDigest(ctx echo.Context, name string, digest string, params models.V2GetNameReferrersDigestParams) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2GetNameTagsList(ctx echo.Context, name string, params models.V2GetNameTagsListParams) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2DeleteNsOrgImageBlobsDigest(ctx echo.Context, ns string, org string, image string, digest string) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2DeleteOrgImageBlobsDigest(ctx echo.Context, org string, image string, digest string) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2DeleteImageBlobsDigest(ctx echo.Context, image string, digest string) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2DeleteNsOrgImageManifestsReference(ctx echo.Context, ns string, org string, image string, reference string) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2DeleteOrgImageManifestsReference(ctx echo.Context, org string, image string, reference string) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2DeleteImageManifestsReference(ctx echo.Context, image string, reference string) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2PutImageManifestsReference(ctx echo.Context, image string, reference string) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}
