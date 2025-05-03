package impl

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/aceeric/ociregistry/api/models"
	"github.com/aceeric/ociregistry/impl/cache"

	"github.com/aceeric/imgpull/pkg/imgpull"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
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

// GET /cmd/manifest/list?pattern=...
func (r *OciRegistry) CmdManifestlist(ctx echo.Context, params models.CmdManifestlistParams) error {
	defer func() {
		if err := recover(); err != nil {
			log.Errorf("handled panic: %s", err)
		}
	}()
	comparer, err := makeComparer(params.Pattern, nil)
	if err != nil {
		return ctx.String(http.StatusBadRequest, "invalid parameters\n")
	}
	manifests := cache.GetManifestsCompare(comparer, count(params.Count))
	if len(manifests) == 0 {
		return ctx.String(http.StatusOK, "no manifests found\n")
	}
	return ctx.Stream(http.StatusOK, "text/plain", cache.NewMFReader(manifests))
}

// GET /cmd/blob/list?substr=...
func (r *OciRegistry) CmdBloblist(ctx echo.Context, params models.CmdBloblistParams) error {
	defer func() {
		if err := recover(); err != nil {
			log.Errorf("handled panic: %s", err)
		}
	}()
	substr := ""
	if params.Substr != nil {
		substr = *params.Substr
	}
	blobs := cache.GetBlobsSubstr(substr, count(params.Count))
	if len(blobs) == 0 {
		return ctx.String(http.StatusOK, "no manifests found\n")
	}
	return ctx.Stream(http.StatusOK, "text/plain", cache.NewBlobReader(blobs))
}

// GET cmd/image/list?digest=... or cmd/image/list?pattern=...
func (r *OciRegistry) CmdImagelist(ctx echo.Context, params models.CmdImagelistParams) error {
	defer func() {
		if err := recover(); err != nil {
			log.Errorf("handled panic: %s", err)
		}
	}()
	comparer, err := makeComparer(params.Pattern, params.Digest)
	if err != nil {
		return ctx.String(http.StatusBadRequest, "invalid parameters\n")
	}

	manifests := cache.GetManifestsCompare(comparer, count(params.Count))
	if len(manifests) == 0 {
		return ctx.String(http.StatusOK, "no manifests found\n")
	}
	return ctx.Stream(http.StatusOK, "text/plain", cache.NewMFReaderWithBlobs(manifests))
}

// DELETE /cmd/prune?type=...&dur=...&expr=...&dryRun=
func (r *OciRegistry) CmdPrune(ctx echo.Context, params models.CmdPruneParams) error {
	defer func() {
		if err := recover(); err != nil {
			log.Errorf("handled panic: %s", err)
		}
	}()
	logs, err := cache.Prune(params.Type, params.Dur, params.Expr, params.DryRun, params.Count)
	if err != nil {
		return ctx.String(http.StatusBadRequest, err.Error())
	}
	return ctx.Stream(http.StatusOK, "text/plain", strings.NewReader(logs))
}

// makeComparer makes a comparer. If pattern is non-nil, it is used, else if digest is
// non-nil, it is used, else a comparer that always returns true is returned.
func makeComparer(pattern *string, digest *string) (cache.ManifestComparer, error) {
	if pattern != nil {
		srchs := []*regexp.Regexp{}
		for _, ref := range strings.Split(*pattern, ",") {
			if exp, err := regexp.Compile(ref); err == nil {
				srchs = append(srchs, exp)
			} else {
				return nil, fmt.Errorf("regex did not compile: %q", ref)
			}
		}
		return func(mh imgpull.ManifestHolder) bool {
			if len(srchs) != 0 {
				for _, srch := range srchs {
					if srch.MatchString(mh.ImageUrl) {
						return true
					}
				}
			}
			return false
		}, nil
	} else if digest != nil {
		substr := *digest
		return func(mh imgpull.ManifestHolder) bool {
			for _, layer := range mh.Layers() {
				if strings.Contains(layer.Digest, substr) {
					return true
				}
			}
			return false
		}, nil
	} else {
		return func(imgpull.ManifestHolder) bool {
			return true
		}, nil
	}
}

// count supports a default throttle for all commands unless explicitly
// overridden with ...?count=X
func count(count *int) int {
	if count != nil {
		return *count
	}
	return 50
}
