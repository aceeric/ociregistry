package impl

import (
	"net/http"
	. "ociregistry/api/models"
	"ociregistry/impl/pullrequest"
	"ociregistry/impl/upstream"
	"strconv"

	"github.com/labstack/echo/v4"
)

func (r *OciRegistry) handleV2OrgImageManifestsReference(ctx echo.Context, org string, image string, reference string, verb string, namespace *string) error {
	remote := parseRemote(ctx, namespace)
	pr := pullrequest.NewPullRequest(org, image, reference, remote)
	mh, exists := isCached(pr)
	if !exists {
		imh, err := r.pullAndCache(pr)
		if err != nil {
			return ctx.NoContent(http.StatusInternalServerError)
		}
		mh = imh
	}
	return sendManifest(ctx, mh, verb)
}

// parseRemoteNamespace accepts the remote registry to pull from in either the X-Registry header,
// or a query param 'ns' - such as is passed by containerd. E.g. if containerd is configured
// to mirror, then when it pulls from the mirror it passes the registry being mirrored like so:
// https://mymirror.io/v2/image-name/manifests/tag-name?ns=myregistry.io:5000.
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

func (r *OciRegistry) pullAndCache(pr pullrequest.PullRequest) (upstream.ManifestHolder, error) {
	url := pr.Url()
	mh, err := upstream.Get(url, r.imagePath, 60000)
	if err != nil {
		return mh, err
	}
	addToCache(pr, mh)
	return mh, nil
}

func sendManifest(ctx echo.Context, mh upstream.ManifestHolder, verb string) error {
	ctx.Response().Header().Add("Content-Length", strconv.Itoa(mh.Size))
	ctx.Response().Header().Add("Docker-Content-Digest", mh.Digest)
	ctx.Response().Header().Add("Vary", "Cookie")
	ctx.Response().Header().Add("Strict-Transport-Security", "max-age=63072000; preload")
	ctx.Response().Header().Add("X-Frame-Options", "DENY")
	ctx.Response().Header().Add("Docker-Distribution-Api-Version", "registry/2.0")
	ctx.Response().Header().Add("Content-Type", mh.MediaType)

	if verb == http.MethodGet {
		return ctx.Blob(http.StatusOK, mh.MediaType, mh.Bytes)
	} else {
		return ctx.NoContent(http.StatusOK)
	}
}

func (r *OciRegistry) handleV2Auth(ctx echo.Context, params V2AuthParams) error {
	return nil
}

func (r *OciRegistry) handleV2Default(ctx echo.Context) error {
	return nil
}

func (r *OciRegistry) handleV2GetOrgImageBlobsDigest(ctx echo.Context, org string, image string, digest string) error {
	return nil
}
