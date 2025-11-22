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
func (r *OciRegistry) handleV2OrgImageManifestsReference(ctx echo.Context, org string, image string, ref string, verb string, ns *string) error {
	metrics.IncV2ApiEndpointHits()
	metrics.IncManifestPulls()
	pr := pullrequest.NewPullRequest(org, image, ref, parseRemote(ctx, ns, r.defaultNs))
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

// GET /v2/org/image/blobs/digest
func (r *OciRegistry) handleV2GetOrgImageBlobsDigest(ctx echo.Context, org string, image string, digest string) error {
	metrics.IncV2ApiEndpointHits()
	metrics.IncBlobPulls()
	digest = helpers.GetDigestFrom(digest)
	if refCnt := cache.GetBlob(digest); refCnt <= 0 {
		log.Errorf("blob not in cache for org %q, image %q, digest %q", org, image, digest)
		metrics.IncApiErrorResults()
		return ctx.JSON(http.StatusNotFound, "")
	}
	blob_file := helpers.GetBlobPath(r.imagePath, digest)
	fi, err := os.Stat(blob_file)
	if err != nil {
		log.Errorf("blob not on the file system for org %q, image %q, digest %q", org, image, digest)
		metrics.IncApiErrorResults()
		return ctx.JSON(http.StatusInternalServerError, "")
	}
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
	log.Info("get /v2/")
	return ctx.JSON(http.StatusOK, "true")
}

// HEAD /v2/
func (r *OciRegistry) handleV2HeadDefault(ctx echo.Context) error {
	metrics.IncV2ApiEndpointHits()
	log.Info("head /v2/")
	return ctx.JSON(http.StatusOK, "true")
}

// GET /v2/auth. The server doesn't do anything with tokens but if the client wants a token
// it gets one.
func (r *OciRegistry) handleV2Auth(ctx echo.Context, params models.V2AuthParams) error {
	metrics.IncV2ApiEndpointHits()
	log.Infof("get auth scope: %s, service: %s, auth: %s", *params.Scope, *params.Service, params.Authorization)
	body := struct {
		Token string `json:"token"`
	}{
		Token: "FROBOZZ",
	}
	ctx.Response().Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
	ctx.Response().Header().Add("Vary", "Cookie")
	return ctx.JSON(http.StatusOK, body)
}

// parseRemote looks in the passed echo context for header 'X-Registry' and if
// it exists, returns the header value. Else looks at the passed namespace arg and if
// non-nil, returns the value from the pointer. Background: if containerd is configured
// to mirror, then when it pulls from the mirror it passes the registry being mirrored
// as a query param like so:
//
//	https://mymirror.io/v2/image-name/manifests/tag-name?ns=myregistry.io:5000.
//
// This query param is passed through to the API handlers so they can know which upstream
// registry to pull from. If neither the header nor the query param are set then the
// function returns the empty string.
func parseRemote(ctx echo.Context, namespace *string, defaultNs string) string {
	if hdr, exists := ctx.Request().Header["X-Registry"]; exists && len(hdr) == 1 {
		return hdr[0]
	} else if namespace != nil {
		return *namespace
	} else if defaultNs != "" {
		return defaultNs
	}
	return ""
}
