package impl

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"strings"

	"github.com/aceeric/ociregistry/api/models"
	"github.com/aceeric/ociregistry/impl/cache"
	"github.com/aceeric/ociregistry/impl/config"

	"github.com/aceeric/imgpull/pkg/imgpull"
	log "github.com/sirupsen/logrus"
)

// GET /cmd/stop
func (r *OciRegistry) CmdStop(w http.ResponseWriter, req *http.Request) {
	r.shutdownCh <- true
}

// GET /cmd/manifest/list?pattern=...
func (r *OciRegistry) CmdManifestlist(w http.ResponseWriter, req *http.Request, params models.CmdManifestlistParams) {
	defer func() {
		if err := recover(); err != nil {
			log.Errorf("handled panic: %s", err)
		}
	}()
	comparer, err := makeComparer(params.Pattern, nil)
	if err != nil {
		writeString(w, http.StatusBadRequest, "invalid parameters\n")
		return
	}
	manifests := cache.GetManifestsCompare(comparer, count(params.Count))
	if len(manifests) == 0 {
		writeString(w, http.StatusOK, "no manifests found\n")
		return
	}
	writeStream(w, http.StatusOK, "text/plain", cache.NewMFReader(manifests))
}

// GET /cmd/blob/list?substr=...
func (r *OciRegistry) CmdBloblist(w http.ResponseWriter, req *http.Request, params models.CmdBloblistParams) {
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
		writeString(w, http.StatusOK, "no manifests found\n")
		return
	}
	writeStream(w, http.StatusOK, "text/plain", cache.NewBlobReader(blobs))
}

// GET cmd/image/list?digest=... or cmd/image/list?pattern=...
// If digest, then manifests with layers matching the digest substr are returned
func (r *OciRegistry) CmdImagelist(w http.ResponseWriter, req *http.Request, params models.CmdImagelistParams) {
	defer func() {
		if err := recover(); err != nil {
			log.Errorf("handled panic: %s", err)
		}
	}()
	comparer, err := makeComparer(params.Pattern, params.Digest)
	if err != nil {
		writeString(w, http.StatusBadRequest, "invalid parameters\n")
		return
	}

	manifests := cache.GetManifestsCompare(comparer, count(params.Count))
	if len(manifests) == 0 {
		writeString(w, http.StatusOK, "no manifests found\n")
		return
	}
	writeStream(w, http.StatusOK, "text/plain", cache.NewMFReaderWithBlobs(manifests))
}

// GET /cmd/pulldate/list?count=n
// Shows the pull dates recorded since the server started, which the server journals to the
// file system against a crash and writes to the manifests when it shuts down: url and pull date,
// sorted by url. count defaults to 50, and -1 shows all.
func (r *OciRegistry) CmdPulldatelist(w http.ResponseWriter, req *http.Request, params models.CmdPulldatelistParams) {
	defer func() {
		if err := recover(); err != nil {
			log.Errorf("handled panic: %s", err)
		}
	}()
	if out := cache.GetPullDates(count(params.Count)); out != "" {
		writeString(w, http.StatusOK, out)
	} else {
		writeString(w, http.StatusOK, "no pull dates found\n")
	}
}

// GET /cmd/config?format=yaml|json
// Shows the configuration in effect, from the config file and the command line, with secrets
// redacted (see config.Redacted). format defaults to yaml.
func (r *OciRegistry) CmdConfig(w http.ResponseWriter, req *http.Request, params models.CmdConfigParams) {
	defer func() {
		if err := recover(); err != nil {
			log.Errorf("handled panic: %s", err)
		}
	}()
	format := models.Yaml
	if params.Format != nil {
		format = *params.Format
	}
	if !format.Valid() {
		writeString(w, http.StatusBadRequest, "invalid parameters\n")
		return
	}
	b, err := config.Marshal(string(format))
	if err != nil {
		log.Errorf("error marshaling the configuration: %s", err)
		writeString(w, http.StatusInternalServerError, "unable to show the configuration\n")
		return
	}
	contentType := "application/yaml"
	if format == models.Json {
		contentType = "application/json"
	}
	writeBlob(w, http.StatusOK, contentType, b)
}

// GET /cmd/image/cache?digestLen=n
// Shows the in-memory manifest cache: every entry, then every digest alias with its tags.
// Digests are shortened to digestLen hex characters; the default, 64, shows them in full.
func (r *OciRegistry) CmdImagecache(w http.ResponseWriter, req *http.Request, params models.CmdImagecacheParams) {
	defer func() {
		if err := recover(); err != nil {
			log.Errorf("handled panic: %s", err)
		}
	}()
	digestLen := 64
	if params.DigestLen != nil {
		digestLen = *params.DigestLen
	}
	if digestLen < 1 {
		writeString(w, http.StatusBadRequest, "invalid parameters\n")
		return
	}
	writeString(w, http.StatusOK, cache.DumpCache(digestLen))
}

// DELETE /cmd/prune?type=...&dur=...&expr=...&dryRun=
func (r *OciRegistry) CmdPrune(w http.ResponseWriter, req *http.Request, params models.CmdPruneParams) {
	defer func() {
		if err := recover(); err != nil {
			log.Errorf("handled panic: %s", err)
		}
	}()
	cnt := count(params.Count)
	logs, err := cache.Prune(params.Type, params.Dur, params.Expr, params.DryRun, &cnt)
	if errors.Is(err, cache.ErrPruneBusy) {
		writeString(w, http.StatusServiceUnavailable, err.Error())
		return
	} else if err != nil {
		writeString(w, http.StatusBadRequest, err.Error())
		return
	}
	writeStream(w, http.StatusOK, "text/plain", strings.NewReader(logs))
}

// makeComparer makes a comparer. If pattern is non-nil, it is used, else if digest is
// non-nil, it is used, else a comparer that always returns true is returned.
func makeComparer(pattern *string, digest *string) (cache.ManifestComparer, error) {
	if pattern != nil {
		srchs := []*regexp.Regexp{}
		for ref := range strings.SplitSeq(*pattern, ",") {
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
// overridden with ...?count=X. If -1 is passed, then max int is returned since
// it's unlikely that the number of cached images would ever exceeed that.
func count(count *int) int {
	if count != nil {
		if *count == -1 {
			return math.MaxInt32
		}
		return *count
	}
	return 50
}
