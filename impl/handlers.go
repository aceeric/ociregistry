package impl

import (
	"encoding/json"
	"net/http"
	. "ociregistry/api/models"
	"ociregistry/impl/cache"
	"ociregistry/impl/helpers"
	"ociregistry/impl/memcache"
	"ociregistry/impl/pullrequest"
	"ociregistry/impl/serialize"
	"ociregistry/impl/upstream"
	"os"
	"strconv"

	"github.com/aceeric/imgpull/pkg/imgpull"
	log "github.com/sirupsen/logrus"

	"github.com/labstack/echo/v4"
)

// TODO BEGIN MOVE TO IMGPULL

// func (mh *imgpull.ManifestHolder) Bytes() ([]byte, error) {
func Bytes(mh imgpull.ManifestHolder) ([]byte, error) {
	var err error
	var marshalled []byte
	switch mh.Type {
	case imgpull.V2dockerManifestList:
		marshalled, err = json.Marshal(mh.V2dockerManifestList)
	case imgpull.V2dockerManifest:
		marshalled, err = json.Marshal(mh.V2dockerManifest)
	case imgpull.V1ociIndex:
		marshalled, err = json.Marshal(mh.V1ociIndex)
	case imgpull.V1ociManifest:
		marshalled, err = json.Marshal(mh.V1ociManifest)
	}
	return marshalled, err
}

// get rid of upstream.
func ToMediaType(mh imgpull.ManifestHolder) string {
	switch mh.Type {
	case imgpull.V2dockerManifestList:
		return upstream.V2dockerManifestListMt
	case imgpull.V2dockerManifest:
		return upstream.V2dockerManifestMt
	case imgpull.V1ociIndex:
		return upstream.V1ociIndexMt
	case imgpull.V1ociManifest:
		return upstream.V1ociManifestMt
	default:
		return ""
	}
}

// TODO END MOVE TO IMGPULL

// TODO HANDLE ALWAYS PULL LATEST
// TODO LOGGING MOVES TO CACHE?

// HEAD or GET /v2/.../manifests/ref
func (r *OciRegistry) handleV2OrgImageManifestsReference(ctx echo.Context, org string, image string, reference string, verb string, namespace *string) error {
	pr := pullrequest.NewPullRequest(org, image, reference, parseRemote(ctx, namespace))
	mh, err := cache.GetManifest(pr)
	if err != nil {
		return ctx.NoContent(http.StatusInternalServerError)
	}
	manifestBytes, err := Bytes(mh) //mh.Bytes()
	if err != nil {
		return ctx.NoContent(http.StatusInternalServerError)
	}
	mt := ToMediaType(mh)
	if mt == "" {
		return ctx.NoContent(http.StatusInternalServerError)
	}
	//remote := parseRemote(ctx, namespace)
	//pr := pullrequest.NewPullRequest(org, image, reference, remote)
	//mh := upstream.ManifestHolder{}
	//exists := false
	//shouldCache := false
	//if pr.Reference == "latest" && r.alwaysPullLatest {
	//	log.Debugf("ignoring cache for: %s", pr.Url())
	//} else {
	//	mh, exists = memcache.IsCached(pr)
	//	shouldCache = true
	//}
	//if exists {
	//	log.Debugf("serving manifest from cache: %s", pr.Url())
	//} else if remote == "" {
	//	return ctx.NoContent(http.StatusNotFound)
	//} else {
	//	log.Debugf("will pull for pr id: %s", pr.Id())
	//	imh, err := r.pullAndCache(pr, shouldCache)
	//	if err != nil {
	//		return ctx.NoContent(http.StatusInternalServerError)
	//	}
	//	mh = imh
	//}
	ctx.Response().Header().Add("Content-Length", strconv.Itoa(len(manifestBytes)))
	ctx.Response().Header().Add("Docker-Content-Digest", "sha256:"+mh.Digest)
	ctx.Response().Header().Add("Docker-Distribution-Api-Version", "registry/2.0")
	ctx.Response().Header().Add("Content-Type", mt)

	if verb == http.MethodGet {
		return ctx.Blob(http.StatusOK, mt, manifestBytes) //mh.Bytes())
	} else {
		return ctx.NoContent(http.StatusOK)
	}
}

// GET /v2/auth
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

// HEAD /v2/
func (r *OciRegistry) handleV2HeadDefault(ctx echo.Context) error {
	log.Info("head /v2/")
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

// pullAndCache pulls a manifest represented in the passed 'PullRequest'. If the
// manifest is an image manifest then the blobs are also downloaded. If 'shouldCache'
// is true, the image is cached. Otherwise the image is not cached. If cached, then
// subsequent requests for the image will serve from cache rather than making a trip
// to the upstream.
func (r *OciRegistry) pullAndCache(pr pullrequest.PullRequest, shouldCache bool) (upstream.ManifestHolder, error) {
	mh, err := upstream.Get(pr, r.imagePath, r.pullTimeout)
	if err != nil {
		return mh, err
	}
	if shouldCache {
		memcache.AddToCache(pr, mh, true)
		go serialize.ToFilesystem(mh, r.imagePath)
	}
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
