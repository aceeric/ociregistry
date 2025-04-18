package impl

import (
	"net/http"
	"ociregistry/api/models"
	"ociregistry/impl/cache"

	"github.com/aceeric/imgpull/pkg/imgpull"
	"github.com/labstack/echo/v4"
)

// GET /cmd/stop
func (r *OciRegistry) CmdStop(ctx echo.Context) error {
	r.shutdownCh <- true
	return nil
}

// GET /health
func (r *OciRegistry) CmdHealth(ctx echo.Context) error {
	return ctx.NoContent(http.StatusOK)
}

// GET /cmd/manifest/list
func (r *OciRegistry) CmdManifestlist(ctx echo.Context, params models.CmdManifestlistParams) error {
	comparer := func(imgpull.ManifestHolder) bool {
		return true
	}
	manifests := cache.GetManifestsCompare(comparer, -1)
	if len(manifests) == 0 {
		return ctx.String(http.StatusOK, "no manifests found")
	}
	return ctx.Stream(http.StatusOK, "text/plain", cache.NewMFReader(manifests))
}
