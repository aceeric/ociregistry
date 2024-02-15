package apiimpl

import (
	"encoding/json"
	"fmt"
	"net/http"
	. "ociregistry/api/models"
	"ociregistry/helpers"
	"ociregistry/pullsync"
	"ociregistry/types"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "crypto/sha256"
	_ "crypto/sha512"

	"github.com/labstack/echo/v4"
	digest "github.com/opencontainers/go-digest"
	log "github.com/sirupsen/logrus"
)

// manifestWithDigest pairs a manifest with its digest
type manifestWithDigest struct {
	mb   []byte
	dgst digest.Digest
}

// in-mem cache of manifests becaues calculating a manifest digest takes
// CPU cycles and we can avoid repetitively doing it by saving it the
// first time and re-using it
var (
	mu          sync.Mutex
	manifestMap = make(map[string]manifestWithDigest)
)

// GET /v2/auth
// everyone authenticates successfully and gets the same token which is
// subsequently ignored by the server
func handleV2Auth(r *OciRegistry, ctx echo.Context, params V2AuthParams) error {
	log.Infof("get auth scope: %s, service: %s, auth: %s", *params.Scope, *params.Service, params.Authorization)
	logRequestHeaders(ctx)
	body := &types.Token{Token: "FROBOZZ"}
	ctx.Response().Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
	ctx.Response().Header().Add("Vary", "Cookie")
	ctx.Response().Header().Add("Strict-Transport-Security", "max-age=63072000; preload")
	return ctx.JSON(http.StatusOK, body)
}

// GET /v2/
// does not require authentication (would return 401 with Www-Authenticate hdr
// to force authentication)
func handleV2Default(r *OciRegistry, ctx echo.Context) error {
	log.Info("get /v2/")
	logRequestHeaders(ctx)
	return ctx.JSON(http.StatusOK, "true")
}

// GET /v2/{org}/{image}/blobs/{digest}
func handleV2GetOrgImageBlobsDigest(r *OciRegistry, ctx echo.Context, org string, image string, digest string) error {
	log.Debugf("get blob org: %s, image: %s, digest: %s", org, image, digest)
	logRequestHeaders(ctx)

	if strings.HasPrefix(digest, "sha256:") {
		_, manifest_ref := xlatManifestDigest(imagePath, digest)
		if manifest_ref != "" {
			return handleOrgImageManifestsReference(r, ctx, org, image, manifest_ref, http.MethodGet, nil)
		}
	}

	blob_file := getBlobPath(imagePath, digest)
	if blob_file == "" {
		return ctx.JSON(http.StatusNotFound, "")
	}
	SHA, err := computeMd5Sum(blob_file)
	if err != nil {
		return ctx.JSON(http.StatusNotFound, "")
	}
	log.Debugf("found blob %s", blob_file)
	fi, _ := os.Stat(blob_file)

	now := time.Now()
	ctx.Response().Header().Add("Content-Length", strconv.Itoa(int(fi.Size())))
	ctx.Response().Header().Add("Accept-Ranges", "bytes")
	ctx.Response().Header().Add("Access-Control-Allow-Origin", "*")
	ctx.Response().Header().Add("Cache-Control", "max-age=1500")
	ctx.Response().Header().Add("Etag", SHA)
	ctx.Response().Header().Add("Expires", now.Add(time.Hour*24).Format(time.RFC1123))
	ctx.Response().Header().Add("Last-Modified", now.Format(time.RFC1123))
	ctx.Response().Header().Add("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	ctx.Response().Header().Add("Docker-Distribution-Api-Version", "registry/2.0")
	f, err := os.Open(blob_file)
	if err != nil {
		return err
	}
	return ctx.Stream(http.StatusOK, "binary/octet-stream", f)
}

// GET or HEAD /v2/{image}/manifests/{reference} or /v2/{org}/{image}/manifests/{reference}
// {org} can be empty string (like pull hello-world:latest)
func handleOrgImageManifestsReference(r *OciRegistry, ctx echo.Context, org string, image string, reference string, verb string, namespace *string) error {
	log.Infof("%s manifest org: %s, image: %s, ref: %s", verb, org, image, reference)
	logRequestHeaders(ctx)

	if strings.HasPrefix(reference, "sha256:") {
		_, reference = xlatManifestDigest(imagePath, reference)
		if reference == "" {
			return ctx.JSON(http.StatusNotFound, "")
		}
	}
	manifestRef := filepath.Join(org, image, reference)

	mu.Lock()
	mfst, exists := manifestMap[manifestRef]
	mu.Unlock()
	if exists {
		return sendManifest(ctx, mfst.mb, mfst.dgst, verb)
	}

	var manifestPath string = ""
	remote := parseRemoteNamespace(ctx, namespace)

	// try once to get the manifest from cache and - failing that - once from the remote
	manifestPath = getManifestPath(imagePath, manifestRef)
	if manifestPath == "" && remote != "" {
		var imageToPull = ""
		if org == "" {
			imageToPull = fmt.Sprintf("%s/%s:%s", remote, image, reference)
		} else {
			imageToPull = fmt.Sprintf("%s/%s/%s:%s", remote, org, image, reference)
		}
		pullsync.PullImage(imageToPull, imagePath, 60000)
		manifestPath = getManifestPath(imagePath, manifestRef)
	}
	if manifestPath == "" {
		return ctx.JSON(http.StatusNotFound, "")
	}
	b, err := os.ReadFile(manifestPath)
	if err != nil {
		return ctx.JSON(http.StatusNotFound, "")
	}
	log.Debugf("found manifest %s", manifestPath)

	var mjson []types.ManifestJson
	jerr := json.Unmarshal(b, &mjson)
	if jerr != nil {
		return ctx.JSON(http.StatusInternalServerError, "")
	}
	config_path := getBlobPath(imagePath, mjson[0].Config)
	if config_path == "" {
		return ctx.JSON(http.StatusNotFound, "")
	}
	fi, _ := os.Stat(config_path)

	var tmpdgst string
	tmpdgst = helpers.GetSHAfromPath(mjson[0].Config)
	if tmpdgst == "" {
		return ctx.JSON(http.StatusNotFound, "")
	}
	manifest := types.ImageManifest{
		SchemaVersion: 2,
		MediaType:     "application/vnd.oci.image.manifest.v1+json",
		Config: types.ManifestConfig{
			MediaType: "application/vnd.oci.image.config.v1+json",
			Size:      int(fi.Size()),
			Digest:    "sha256:" + tmpdgst,
		},
	}
	for i := 0; i < len(mjson[0].Layers); i++ {
		log.Debugf("get layer %s", mjson[0].Layers[i])
		layer_path := getBlobPath(imagePath, mjson[0].Layers[i])
		if layer_path == "" {
			return ctx.JSON(http.StatusNotFound, "")
		}
		log.Debugf("found layer %s", layer_path)
		fi, _ := os.Stat(layer_path)
		tmpdgst = helpers.GetSHAfromPath(mjson[0].Layers[i])
		if tmpdgst == "" {
			return ctx.JSON(http.StatusNotFound, "")
		}
		new_layer := types.ManifestLayer{
			MediaType: "application/vnd.oci.image.layer.v1.tar+gzip",
			Size:      int(fi.Size()),
			Digest:    "sha256:" + tmpdgst,
		}
		manifest.Layers = append(manifest.Layers, new_layer)
	}

	// compute manifest digest
	mb, err := json.Marshal(manifest)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, "")
	}

	var digester = digest.Canonical.Digester()
	mblen := len(mb)
	cnt, _ := digester.Hash().Write(mb)
	dgst := digester.Digest()
	log.Debugf("computed digest for ref %s = sha256:%s (cnt: %d / mblen:%d)", reference, dgst.Hex(), cnt, mblen)

	// in case a client asks for the manifest in the future by "sha256:..."
	saveManifestDigest(imagePath, reference, dgst.Hex())

	// in-mem cache for faster lookup next time through
	mu.Lock()
	manifestMap[manifestRef] = manifestWithDigest{mb, dgst}
	mu.Unlock()

	return sendManifest(ctx, mb, dgst, verb)
}

// sendManifest returns an image manifest to the caller with headers for a GET, and
// just returns HTTP 200 for a HEAD.
func sendManifest(ctx echo.Context, mb []byte, dgst digest.Digest, verb string) error {
	ctx.Response().Header().Add("Content-Length", strconv.Itoa(len(mb)))
	ctx.Response().Header().Add("Docker-Content-Digest", "sha256:"+dgst.Hex())
	ctx.Response().Header().Add("Vary", "Cookie")
	ctx.Response().Header().Add("Strict-Transport-Security", "max-age=63072000; preload")
	ctx.Response().Header().Add("X-Frame-Options", "DENY")
	ctx.Response().Header().Add("Docker-Distribution-Api-Version", "registry/2.0")
	ctx.Response().Header().Add("Content-Type", "application/vnd.oci.image.manifest.v1+json")

	if verb == http.MethodGet {
		return ctx.Blob(http.StatusOK, "application/vnd.oci.image.manifest.v1+json", mb)
	} else {
		return ctx.NoContent(http.StatusOK)
	}
}

// parseRemoteNamespace accepts the remote registry to pull from in either the X-Registry header,
// or a query param 'ns' - such as is passed by containerd. E.g. if containerd is configured
// to mirror, then when it pull from the mirror is passes the regstry being mirrored like so:
// https://mymirror.io/v2/image_name/manifests/tag_name?ns=myregistry.io:5000.
func parseRemoteNamespace(ctx echo.Context, namespace *string) string {
	hdr, exists := ctx.Request().Header["X-Registry"]
	if exists && len(hdr) == 1 {
		return hdr[0]
	}
	if namespace != nil {
		return *namespace
	}
	return ""
}
