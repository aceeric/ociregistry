package impl

import (
	"fmt"
	"net/http"
	"ociregistry/api/models"
	"ociregistry/impl/cache"
	"regexp"
	"strings"

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
			log.Errorf("handled panic:", err)
		}
	}()
	comparer := func(imgpull.ManifestHolder) bool {
		return true
	}
	if params.Pattern != nil {
		srchs := []*regexp.Regexp{}
		for _, ref := range strings.Split(*params.Pattern, ",") {
			if exp, err := regexp.Compile(ref); err == nil {
				srchs = append(srchs, exp)
			} else {
				return ctx.String(http.StatusBadRequest, fmt.Sprintf("regex did not compile: %q", ref))
			}
		}
		comparer = func(mh imgpull.ManifestHolder) bool {
			if len(srchs) != 0 {
				for _, srch := range srchs {
					if srch.MatchString(mh.ImageUrl) {
						return true
					}
				}
			}
			return false
		}
	}
	manifests := cache.GetManifestsCompare(comparer, -1)
	if len(manifests) == 0 {
		return ctx.String(http.StatusOK, "no manifests found")
	}
	return ctx.Stream(http.StatusOK, "text/plain", cache.NewMFReader(manifests))
}

// GET /cmd/manifest/list?pattern=...
func (r *OciRegistry) CmdBloblist(ctx echo.Context, params models.CmdBloblistParams) error {
	defer func() {
		if err := recover(); err != nil {
			log.Errorf("handled panic:", err)
		}
	}()
	substr := ""
	if params.Substr != nil {
		substr = *params.Substr
	}
	blobs := cache.GetBlobsSubstr(substr, 0)
	if len(blobs) == 0 {
		return ctx.String(http.StatusOK, "no manifests found")
	}
	return ctx.Stream(http.StatusOK, "text/plain", cache.NewBlobReader(blobs))
}

func (r *OciRegistry) CmdImagelist(ctx echo.Context, params models.CmdImagelistParams) error {
	defer func() {
		if err := recover(); err != nil {
			log.Errorf("handled panic:", err)
		}
	}()
	comparer := func(imgpull.ManifestHolder) bool {
		return true
	}
	var expr *regexp.Regexp
	var err error
	digest := ""
	if params.Pattern != nil {
		// only one pattern allowed here
		expr, err = regexp.Compile(*params.Pattern)
		if err != nil {
			return ctx.String(http.StatusBadRequest, fmt.Sprintf("regex did not compile: %q", *params.Pattern))
		}
	}
	// can be substring
	if params.Digest != nil {
		digest = *params.Digest
	}
	if params.Pattern != nil || params.Digest != nil {
		comparer = func(mh imgpull.ManifestHolder) bool {
			if expr != nil && expr.MatchString(mh.ImageUrl) {
				return true
			} else if digest != "" {
				for _, layer := range mh.Layers() {
					if strings.Contains(layer.Digest, digest) {
						return true
					}
				}
			}
			return false
		}
	}

	manifests := cache.GetManifestsCompare(comparer, -1)
	if len(manifests) == 0 {
		return ctx.String(http.StatusOK, "no manifests found")
	}
	return ctx.Stream(http.StatusOK, "text/plain", cache.NewMFReaderWithBlobs(manifests))
}
