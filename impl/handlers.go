package impl

import "github.com/labstack/echo/v4"

type OciRegistry struct{}

func (r *OciRegistry) handleV2OrgImageManifestsReference(ctx echo.Context, org string, image string, reference string, verb string, namespace *string) error {
	remote := parseRemote(ctx, namespace)
	pr := NewPullRequest(org, image, reference, remote)
	if isCached(pr) {
		return doIsCached(pr)
	}
	return doIsNotCached(pr)
}

// parseRemoteNamespace accepts the remote registry to pull from in either the X-Registry header,
// or a query param 'ns' - such as is passed by containerd. E.g. if containerd is configured
// to mirror, then when it pull from the mirror is passes the regstry being mirrored like so:
// https://mymirror.io/v2/image_name/manifests/tag_name?ns=myregistry.io:5000.
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

func doIsCached(pr pullRequest) error {
	return nil

}

func doIsNotCached(pr pullRequest) error {
	return nil
}
