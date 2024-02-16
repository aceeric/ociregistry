package impl

import (
	"ociregistry/impl/pullrequest"
	"ociregistry/impl/upstream"

	"github.com/labstack/echo/v4"
)

type OciRegistry struct {
	imagePath string
}

func NewOciRegistry(imagePath string) OciRegistry {
	return OciRegistry{
		imagePath: imagePath,
	}
}

func (r *OciRegistry) handleV2OrgImageManifestsReference(ctx echo.Context, org string, image string, reference string, verb string, namespace *string) error {
	remote := parseRemote(ctx, namespace)
	pr := pullrequest.NewPullRequest(org, image, reference, remote)
	mh, exists := isCached(pr)
	if exists {
		return r.fromCache(mh)
	}
	return r.addToCache(pr)
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

func (r *OciRegistry) fromCache(mh upstream.ManifestHolder) error {
	// get from cache
	// return
	return nil
}

func (r *OciRegistry) addToCache(pr pullrequest.PullRequest) error {
	url := pr.Url()
	upstream.Get(url, r.imagePath, 60000)
	return nil
}
