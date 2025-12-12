package impl

import (
	"net/http"
	"os"
	"strconv"

	"github.com/aceeric/ociregistry/api/models"
	"github.com/aceeric/ociregistry/impl/cache"
	"github.com/aceeric/ociregistry/impl/helpers"
	"github.com/aceeric/ociregistry/impl/metrics"
	"github.com/aceeric/ociregistry/impl/pullrequest"

	log "github.com/sirupsen/logrus"

	"github.com/labstack/echo/v4"
)

// HEAD or GET /v2/.../manifests/ref
func (r *OciRegistry) handleV2ManifestsReference(ctx echo.Context, pr pullrequest.PullRequest, verb string) error {
	metrics.IncV2ApiEndpointHits()
	metrics.IncManifestPulls()
	if r.airGapped && !cache.IsCached(pr) {
		log.Debugf("request for un-cached manifest %q in air-gapped mode - returning 404", pr.Url())
		metrics.IncApiErrorResults()
		return ctx.JSON(http.StatusNotFound, "")
	}
	forcePull := r.alwaysPullLatest && pr.Reference == "latest"
	mh, err := cache.GetManifest(pr, r.imagePath, r.pullTimeout, forcePull)
	if err != nil {
		log.Errorf("error getting manifest for %q: %s", pr.Url(), err)
		metrics.IncApiErrorResults()
		return ctx.NoContent(http.StatusInternalServerError)
	}
	ctx.Response().Header().Add("Content-Length", strconv.Itoa(len(mh.Bytes)))
	ctx.Response().Header().Add("Docker-Content-Digest", "sha256:"+mh.Digest)
	ctx.Response().Header().Add("Docker-Distribution-Api-Version", "registry/2.0")
	ctx.Response().Header().Add("Content-Type", mh.MediaType())

	if verb == http.MethodGet {
		return ctx.Blob(http.StatusOK, mh.MediaType(), mh.Bytes)
	} else {
		return ctx.NoContent(http.StatusOK)
	}
}

// GET /v2/.../blobs/digest
func (r *OciRegistry) handleV2BlobsDigest(ctx echo.Context, repository string, digest string) error {
	metrics.IncV2ApiEndpointHits()
	metrics.IncBlobPulls()
	digest = helpers.GetDigestFrom(digest)
	if refCnt := cache.GetBlob(digest); refCnt <= 0 {
		log.Errorf("blob not in cache for %q, digest %q", repository, digest)
		metrics.IncApiErrorResults()
		return ctx.JSON(http.StatusNotFound, "")
	}
	blob_file := helpers.GetBlobPath(r.imagePath, digest)
	fi, err := os.Stat(blob_file)
	if err != nil {
		log.Errorf("blob not on the file system for %q, digest %q", repository, digest)
		metrics.IncApiErrorResults()
		return ctx.JSON(http.StatusInternalServerError, "")
	}
	// if the Range header is set in the request then by omitting the Content-Length header
	// in the response, the underlying http library automatically supports chunked transfer
	if ctx.Request().Header.Get("Range") == "" {
		ctx.Response().Header().Add("Content-Length", strconv.Itoa(int(fi.Size())))
	}
	ctx.Response().Header().Add("Docker-Distribution-Api-Version", "registry/2.0")
	f, err := os.Open(blob_file)
	if err != nil {
		return err
	}
	return ctx.Stream(http.StatusOK, "binary/octet-stream", f)
}

// GET /v2/
func (r *OciRegistry) handleV2Default(ctx echo.Context) error {
	metrics.IncV2ApiEndpointHits()
	return ctx.JSON(http.StatusOK, "true")
}

// HEAD /v2/
func (r *OciRegistry) handleV2HeadDefault(ctx echo.Context) error {
	metrics.IncV2ApiEndpointHits()
	return ctx.JSON(http.StatusOK, "true")
}

// GET /v2/auth. The server doesn't do anything with tokens but if the client wants a token
// it gets one.
func (r *OciRegistry) handleV2Auth(ctx echo.Context, params models.V2AuthParams) error {
	metrics.IncV2ApiEndpointHits()
	body := struct {
		Token string `json:"token"`
	}{
		Token: "FROBOZZ",
	}
	ctx.Response().Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
	ctx.Response().Header().Add("Vary", "Cookie")
	return ctx.JSON(http.StatusOK, body)
}

// x_registry_hdr returns the X-Registry header from the passed context or the empty
// string if the header is not present.
func (r *OciRegistry) x_registry_hdr(ctx echo.Context) string {
	if hdr, exists := ctx.Request().Header["X-Registry"]; exists && len(hdr) == 1 {
		return hdr[0]
	}
	return ""
}
