// implements the "pull-only" registry server. Provides implementations for methods
// required to pull an image.
package apiimpl

import (
	"net/http"
	. "ociregistry/api/models"
	"os"

	_ "crypto/sha256"
	_ "crypto/sha512"

	"github.com/labstack/echo/v4"
)

type OciRegistry struct{}

const library = "library"

// where image tarballs are unarchived to
var imagePath string

func SetImagePath(imagePathArg string) error {
	imagePath = imagePathArg
	return os.MkdirAll(imagePath, 0755)
}

func NewOciRegistry() *OciRegistry {
	return &OciRegistry{}
}

// CONNECT
func (r *OciRegistry) Connect(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, nil)
}

// GET /v2/auth
func (r *OciRegistry) V2Auth(ctx echo.Context, params V2AuthParams) error {
	return handleV2Auth(r, ctx, params)
}

// GET /v2/
func (r *OciRegistry) V2Default(ctx echo.Context) error {
	return handleV2Default(r, ctx)
}

// GET /v2/{image}/blobs/{digest}
func (r *OciRegistry) V2GetImageBlobsDigest(ctx echo.Context, image string, digest string) error {
	return handleV2GetOrgImageBlobsDigest(r, ctx, "", image, digest)
}

// GET /v2/{org}/{image}/blobs/{digest}
func (r *OciRegistry) V2GetOrgImageBlobsDigest(ctx echo.Context, org string, image string, digest string) error {
	return handleV2GetOrgImageBlobsDigest(r, ctx, org, image, digest)
}

// HEAD /v2/{image}/manifests/{reference}
func (r *OciRegistry) V2HeadImageManifestsReference(ctx echo.Context, image string, reference string, params V2HeadImageManifestsReferenceParams) error {
	return handleOrgImageManifestsReference(r, ctx, "", image, reference, http.MethodHead, params.Ns)
}

// HEAD /v2/{org}/{image}/manifests/{reference}
func (r *OciRegistry) V2HeadOrgImageManifestsReference(ctx echo.Context, org string, image string, reference string, params V2HeadOrgImageManifestsReferenceParams) error {
	return handleOrgImageManifestsReference(r, ctx, org, image, reference, http.MethodHead, params.Ns)
}

// GET /v2/{image}/manifests/{reference}
func (r *OciRegistry) V2GetImageManifestsReference(ctx echo.Context, image string, reference string, params V2GetImageManifestsReferenceParams) error {
	return handleOrgImageManifestsReference(r, ctx, "", image, reference, http.MethodGet, params.Ns)
}

// GET /v2/{org}/{image}/manifests/{reference}
func (r *OciRegistry) V2GetOrgImageManifestsReference(ctx echo.Context, org string, image string, reference string, params V2GetOrgImageManifestsReferenceParams) error {
	return handleOrgImageManifestsReference(r, ctx, org, image, reference, http.MethodGet, params.Ns)
}

// unimplemented methods of the OCI distribution spec

func (r *OciRegistry) V2HeadOrgImageBlobsDigest(ctx echo.Context, org string, image string, digest string) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2PostNameBlobsUploads(ctx echo.Context, name string, params V2PostNameBlobsUploadsParams) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2GetNameBlobsUploadsReference(ctx echo.Context, name string, reference string) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2PatchNameBlobsUploadsReference(ctx echo.Context, name string, reference string) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2PutNameBlobsUploadsReference(ctx echo.Context, name string, reference string, params V2PutNameBlobsUploadsReferenceParams) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2PutOrgImageManifestsReference(ctx echo.Context, org string, image string, reference string) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2GetNameReferrersDigest(ctx echo.Context, name string, digest string, params V2GetNameReferrersDigestParams) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2GetNameTagsList(ctx echo.Context, name string, params V2GetNameTagsListParams) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2DeleteImageBlobsDigest(ctx echo.Context, image string, digest string) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2DeleteImageManifestsReference(ctx echo.Context, image string, reference string) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2DeleteOrgImageBlobsDigest(ctx echo.Context, org string, image string, digest string) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2DeleteOrgImageManifestsReference(ctx echo.Context, org string, image string, reference string) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2HeadImageBlobsDigest(ctx echo.Context, image string, digest string) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}

func (r *OciRegistry) V2PutImageManifestsReference(ctx echo.Context, image string, reference string) error {
	return ctx.NoContent(http.StatusMethodNotAllowed)
}
