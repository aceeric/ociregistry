package impl

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/aceeric/ociregistry/api/models"
	"github.com/aceeric/ociregistry/impl/cache"
	"github.com/aceeric/ociregistry/impl/helpers"
	"github.com/aceeric/ociregistry/impl/pullrequest"

	log "github.com/sirupsen/logrus"

	"github.com/labstack/echo/v4"
)

// HEAD or GET /v2/.../manifests/ref
func (r *OciRegistry) handleV2OrgImageManifestsReference(ctx echo.Context, org string, image string, ref string, verb string, ns *string) error {
	pr := pullrequest.NewPullRequest(org, image, ref, parseRemote(ctx, ns))
	if r.airGapped && !cache.IsCached(pr) {
		log.Debugf("request for un-cached manifest %q in air-gapped mode - returning 404", pr.Url())
		return ctx.JSON(http.StatusNotFound, "")
	}
	forcePull := r.alwaysPullLatest && pr.Reference == "latest"
	mh, err := cache.GetManifest(pr, r.imagePath, r.pullTimeout, forcePull)
	if err != nil {
		log.Errorf("error getting manifest for %q: %s", pr.Url(), err)
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
	digest = helpers.GetDigestFrom(digest)
	if refCnt := cache.GetBlob(digest); refCnt <= 0 {
		log.Errorf("blob not in cache for org %q, image %q, digest %q", org, image, digest)
		return ctx.JSON(http.StatusNotFound, "")
	}
	blob_file := helpers.GetBlobPath(r.imagePath, digest)
	fi, err := os.Stat(blob_file)
	if err != nil {
		log.Errorf("blob not on the file system for org %q, image %q, digest %q", org, image, digest)
		return ctx.JSON(http.StatusInternalServerError, "")
	}

	f, err := os.Open(blob_file)
	if err != nil {
		return err
	}
	defer f.Close()

	fileSize := fi.Size()

	// Check for Range header
	rangeHeader := ctx.Request().Header.Get("Range")
	if rangeHeader == "" {
		// No Range header - serve entire file
		ctx.Response().Header().Add("Content-Length", strconv.FormatInt(fileSize, 10))
		ctx.Response().Header().Add("Docker-Distribution-Api-Version", "registry/2.0")
		ctx.Response().Header().Add("Accept-Ranges", "bytes")
		return ctx.Stream(http.StatusOK, "binary/octet-stream", f)
	}

	// Parse Range header (format: "bytes=start-end")
	start, end, err := parseRangeHeader(rangeHeader, fileSize)
	if err != nil {
		log.Warnf("invalid Range header %q for digest %q: %v", rangeHeader, digest, err)
		ctx.Response().Header().Add("Content-Range", fmt.Sprintf("bytes */%d", fileSize))
		return ctx.NoContent(http.StatusRequestedRangeNotSatisfiable)
	}

	// Seek to start position
	_, err = f.Seek(start, io.SeekStart)
	if err != nil {
		log.Errorf("failed to seek to position %d for digest %q: %v", start, digest, err)
		return ctx.JSON(http.StatusInternalServerError, "")
	}

	// Calculate content length for this range
	contentLength := end - start + 1

	// Set response headers for partial content
	ctx.Response().Header().Add("Content-Length", strconv.FormatInt(contentLength, 10))
	ctx.Response().Header().Add("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
	ctx.Response().Header().Add("Docker-Distribution-Api-Version", "registry/2.0")
	ctx.Response().Header().Add("Accept-Ranges", "bytes")

	// Stream the requested range
	return ctx.Stream(http.StatusPartialContent, "binary/octet-stream", io.LimitReader(f, contentLength))
}

// parseRangeHeader parses an HTTP Range header and returns start and end positions.
// Returns an error if the range is invalid or unsatisfiable.
func parseRangeHeader(rangeHeader string, fileSize int64) (start, end int64, err error) {
	// Expected format: "bytes=start-end" or "bytes=start-" or "bytes=-suffix"
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		return 0, 0, fmt.Errorf("range header must start with 'bytes='")
	}

	rangeSpec := strings.TrimPrefix(rangeHeader, "bytes=")

	// Handle multiple ranges (we only support single range)
	if strings.Contains(rangeSpec, ",") {
		return 0, 0, fmt.Errorf("multiple ranges not supported")
	}

	parts := strings.Split(rangeSpec, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid range format")
	}

	startStr := strings.TrimSpace(parts[0])
	endStr := strings.TrimSpace(parts[1])

	if startStr == "" && endStr == "" {
		return 0, 0, fmt.Errorf("both start and end cannot be empty")
	}

	if startStr == "" {
		// Suffix range: "bytes=-500" means last 500 bytes
		suffix, err := strconv.ParseInt(endStr, 10, 64)
		if err != nil || suffix <= 0 {
			return 0, 0, fmt.Errorf("invalid suffix length")
		}
		if suffix > fileSize {
			suffix = fileSize
		}
		return fileSize - suffix, fileSize - 1, nil
	}

	start, err = strconv.ParseInt(startStr, 10, 64)
	if err != nil || start < 0 {
		return 0, 0, fmt.Errorf("invalid start position")
	}

	if endStr == "" {
		// Open-ended range: "bytes=100-" means from byte 100 to end
		end = fileSize - 1
	} else {
		end, err = strconv.ParseInt(endStr, 10, 64)
		if err != nil || end < 0 {
			return 0, 0, fmt.Errorf("invalid end position")
		}
	}

	// Validate range
	if start > end {
		return 0, 0, fmt.Errorf("start position %d is greater than end position %d", start, end)
	}

	if start >= fileSize {
		return 0, 0, fmt.Errorf("start position %d is beyond file size %d", start, fileSize)
	}

	// Adjust end if it's beyond file size
	if end >= fileSize {
		end = fileSize - 1
	}

	return start, end, nil
}

// GET /v2/
func (r *OciRegistry) handleV2Default(ctx echo.Context) error {
	log.Info("get /v2/")
	return ctx.JSON(http.StatusOK, "true")
}

// HEAD /v2/
func (r *OciRegistry) handleV2HeadDefault(ctx echo.Context) error {
	log.Info("head /v2/")
	return ctx.JSON(http.StatusOK, "true")
}

// GET /v2/auth. The server doesn't do anything with tokens but if the client wants a token
// it gets one.
func (r *OciRegistry) handleV2Auth(ctx echo.Context, params models.V2AuthParams) error {
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
func parseRemote(ctx echo.Context, namespace *string) string {
	if hdr, exists := ctx.Request().Header["X-Registry"]; exists && len(hdr) == 1 {
		return hdr[0]
	} else if namespace != nil {
		return *namespace
	}
	return ""
}
