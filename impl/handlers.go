package impl

import (
	"net/http"
	. "ociregistry/api/models"
	"ociregistry/impl/helpers"
	"ociregistry/impl/memcache"
	"ociregistry/impl/pullrequest"
	"ociregistry/impl/serialize"
	"ociregistry/impl/upstream"
	"os"
	"strconv"

	log "github.com/sirupsen/logrus"

	"github.com/labstack/echo/v4"
)

// HEAD or GET /v2/.../manifests/ref
func (r *OciRegistry) handleV2OrgImageManifestsReference(ctx echo.Context, org string, image string, reference string, verb string, namespace *string) error {
	remote := parseRemote(ctx, namespace)
	pr := pullrequest.NewPullRequest(org, image, reference, remote)
	mh, exists := memcache.IsCached(pr)
	if exists {
		log.Debugf("serving manifest from cache: %s", pr.Url())
	} else if remote == "" {
		return ctx.NoContent(http.StatusNotFound)
	} else {
		log.Debugf("will pull and cache for pr id: %s", pr.Id())
		imh, err := r.pullAndCache(pr)
		if err != nil {
			return ctx.NoContent(http.StatusInternalServerError)
		}
		mh = imh
	}
	ctx.Response().Header().Add("Content-Length", strconv.Itoa(mh.Size))
	ctx.Response().Header().Add("Docker-Content-Digest", "sha256:"+mh.Digest)
	ctx.Response().Header().Add("Docker-Distribution-Api-Version", "registry/2.0")
	ctx.Response().Header().Add("Content-Type", mh.MediaType)

	if verb == http.MethodGet {
		return ctx.Blob(http.StatusOK, mh.MediaType, mh.Bytes)
	} else {
		return ctx.NoContent(http.StatusOK)
	}
}

// GET /v2/auth (never called)
func (r *OciRegistry) handleV2Auth(ctx echo.Context, params V2AuthParams) error {
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

// GET /v2/
func (r *OciRegistry) handleV2Default(ctx echo.Context) error {
	log.Info("get /v2/")
	return ctx.JSON(http.StatusOK, "true")
}

// GET blob
func (r *OciRegistry) handleV2GetOrgImageBlobsDigest(ctx echo.Context, org string, image string, digest string) error {
	log.Infof("get blob org: %s, image: %s, digest: %s", org, image, digest)

	blob_file := helpers.GetBlobPath(r.imagePath, digest)
	if blob_file == "" {
		log.Errorf("blob not in cache for org: %s, image: %s, digest: %s", org, image, digest)
		return ctx.JSON(http.StatusNotFound, "")
	}
	fi, _ := os.Stat(blob_file)

	ctx.Response().Header().Add("Content-Length", strconv.Itoa(int(fi.Size())))
	ctx.Response().Header().Add("Docker-Distribution-Api-Version", "registry/2.0")
	f, err := os.Open(blob_file)
	if err != nil {
		return err
	}
	return ctx.Stream(http.StatusOK, "binary/octet-stream", f)
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
func parseRemote(ctx echo.Context, namespace *string) string {
	hdr, exists := ctx.Request().Header["X-Registry"]
	if exists && len(hdr) == 1 {
		return hdr[0]
	}
	if namespace != nil {
		return *namespace
	}
	return ""
}

// pullAndCache pulls a manifest represented in the passed 'PullRequest' and caches it.
// If the manifest is an image manifest then the blobs are also downloaded and cached. Upon
// return from this function, if the manifest is an image manifest, then the server is able
// to serve the image from cache.
func (r *OciRegistry) pullAndCache(pr pullrequest.PullRequest) (upstream.ManifestHolder, error) {
	mh, err := upstream.Get(pr, r.imagePath, r.pullTimeout)
	if err != nil {
		return mh, err
	}
	memcache.AddToCache(pr, mh, true)
	go serialize.ToFilesystem(mh, r.imagePath)
	return mh, nil
}

//// logRequestHeaders emanates the request headers to the logger
//func logRequestHeaders(ctx echo.Context) {
//	if !log.IsLevelEnabled(log.DebugLevel) {
//		return
//	}
//	hdrs := ctx.Request().Header
//	for h := range hdrs {
//		v := strings.Join(hdrs[h], ",")
//		log.Debugf("HDR: %s=%s", h, v)
//	}
//}
