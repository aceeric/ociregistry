package impl

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/aceeric/ociregistry/api/models"
	"github.com/aceeric/ociregistry/impl/cache"
	"github.com/aceeric/ociregistry/impl/helpers"
	"github.com/aceeric/ociregistry/impl/metrics"
	"github.com/aceeric/ociregistry/impl/pullrequest"

	log "github.com/sirupsen/logrus"
)

// HEAD or GET /v2/.../manifests/ref
func (r *OciRegistry) handleV2ManifestsReference(w http.ResponseWriter, req *http.Request, reference string, namespace *string, verb string, repoSegments ...string) {
	metrics.IncV2ApiEndpointHits()
	metrics.IncManifestPulls()
	pr, err := pullrequest.NewPullRequest(r.x_registry_hdr(req), namespace, r.defaultNs, reference, repoSegments...)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, err)
		return
	}
	if r.airGapped && !cache.IsCached(pr) {
		log.Debugf("request for un-cached manifest %q in air-gapped mode - returning 404", pr.Url())
		metrics.IncApiErrorResults()
		writeJSON(w, http.StatusNotFound, "")
		return
	}
	mh, err := cache.GetManifest(pr, r.imagePath, r.pullTimeout, r.alwaysPull(pr.Url()))
	if err != nil {
		log.Errorf("error getting manifest for %q: %s", pr.Url(), err)
		metrics.IncApiErrorResults()
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Add("Content-Length", strconv.Itoa(len(mh.Bytes)))
	w.Header().Add("Docker-Content-Digest", "sha256:"+mh.Digest)
	w.Header().Add("Docker-Distribution-Api-Version", "registry/2.0")
	w.Header().Add("Content-Type", mh.MediaType())

	if verb == http.MethodGet {
		writeBlob(w, http.StatusOK, mh.MediaType(), mh.Bytes)
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

// GET /v2/.../blobs/digest
func (r *OciRegistry) handleV2BlobsDigest(w http.ResponseWriter, req *http.Request, digest string, repoSegments ...string) {
	metrics.IncV2ApiEndpointHits()
	metrics.IncBlobPulls()
	digest = helpers.GetDigestFrom(digest)
	if refCnt := cache.GetBlob(digest); refCnt <= 0 {
		log.Errorf("blob not in cache for %q, digest %q", strings.Join(repoSegments, "/"), digest)
		metrics.IncApiErrorResults()
		writeJSON(w, http.StatusNotFound, "")
		return
	}
	fi, err, blob_file := helpers.GetBlob(r.imagePath, digest)
	if err != nil {
		log.Errorf("blob not on the file system for %q, digest %q", strings.Join(repoSegments, "/"), digest)
		metrics.IncApiErrorResults()
		writeJSON(w, http.StatusNotFound, "")
		return
	}
	// open the blob before setting any headers so an error response isn't sent with
	// this blob's Content-Length
	f, err := os.Open(blob_file)
	if err != nil {
		log.Errorf("error opening blob file %q: %s", blob_file, err)
		metrics.IncApiErrorResults()
		writeJSON(w, http.StatusInternalServerError, map[string]string{"message": http.StatusText(http.StatusInternalServerError)})
		return
	}
	defer f.Close()
	// if the Range header is set in the request then by omitting the Content-Length header
	// in the response, the underlying http library automatically supports chunked transfer
	if req.Header.Get("Range") == "" {
		w.Header().Add("Content-Length", strconv.Itoa(int(fi.Size())))
	}
	w.Header().Add("Docker-Distribution-Api-Version", "registry/2.0")
	writeStream(w, http.StatusOK, "binary/octet-stream", f)
}

// GET /v2/
func (r *OciRegistry) handleV2Default(w http.ResponseWriter, _ *http.Request) {
	metrics.IncV2ApiEndpointHits()
	writeJSON(w, http.StatusOK, "true")
}

// HEAD /v2/
func (r *OciRegistry) handleV2HeadDefault(w http.ResponseWriter, _ *http.Request) {
	metrics.IncV2ApiEndpointHits()
	writeJSON(w, http.StatusOK, "true")
}

// GET /v2/auth. The server doesn't do anything with tokens but if the client wants a token
// it gets one.
func (r *OciRegistry) handleV2Auth(w http.ResponseWriter, _ *http.Request, _ models.V2AuthParams) {
	metrics.IncV2ApiEndpointHits()
	body := struct {
		Token string `json:"token"`
	}{
		Token: "FROBOZZ",
	}
	w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Add("Vary", "Cookie")
	writeJSON(w, http.StatusOK, body)
}

// x_registry_hdr returns the X-Registry header from the passed request or the empty
// string if the header is not present.
func (r *OciRegistry) x_registry_hdr(req *http.Request) string {
	if hdr, exists := req.Header["X-Registry"]; exists && len(hdr) == 1 {
		return hdr[0]
	}
	return ""
}

// alwaysPull returns true if the passed canonical image url must be pulled from the upstream
// on every request because it matches a --no-cache pattern. Never true when air-gapped.
func (r *OciRegistry) alwaysPull(url string) bool {
	if r.airGapped {
		return false
	}
	for _, re := range r.noCache {
		if re.MatchString(url) {
			return true
		}
	}
	return false
}
