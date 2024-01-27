package apiimpl

import (
	"encoding/json"
	"fmt"
	"net/http"
	. "ociregistry/api/models"
	"ociregistry/helpers"
	"ociregistry/pullsync"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "crypto/sha256"
	_ "crypto/sha512"

	"github.com/labstack/echo/v4"
	digest "github.com/opencontainers/go-digest"
)

// everyone authenticates successfully and gets the same token which is
// subsequently ignored by the server
func handleV2Auth(r *OciRegistry, ctx echo.Context, params V2AuthParams) error {
	body := &Token{Token: "FROBOZZ"}
	ctx.Response().Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
	ctx.Response().Header().Add("Vary", "Cookie")
	ctx.Response().Header().Add("Strict-Transport-Security", "max-age=63072000; preload")
	return ctx.JSON(http.StatusOK, body)
}

func handleV2Default(r *OciRegistry, ctx echo.Context) error {
	var scheme string
	if ctx.Request().URL.Scheme == "" {
		scheme = "http"
	} else {
		scheme = ctx.Request().URL.Scheme
	}
	svc := ctx.Request().Host
	url := svc + "/v2/auth"
	realm := scheme + "://" + url

	ctx.Response().Header().Add("Content-Type", "text/html; charset=utf-8")
	ctx.Response().Header().Add("www-authenticate", "Bearer realm=\""+realm+"\",service=\""+svc+"\"")
	ctx.Response().Header().Add("Vary", "Cookie")
	return ctx.JSON(http.StatusOK, "true")
}

// client can ask for the manifest using the /blobs/ endpoint using the Docker-Content-Digest value
// provided by a prior call to the /manifests/reference endpoint
func handleV2GetOrgImageBlobsDigest(r *OciRegistry, ctx echo.Context, org string, image string, digest string) error {
	ctx.Logger().Info(fmt.Sprintf("get blob - org: %s, image: %s, digest: %s", org, image, digest))

	if strings.HasPrefix(digest, "sha256:") {
		manifest_ref := xlatManifestDigest(image_path, digest)
		if manifest_ref != "" {
			return handleOrgImageManifestsReference(r, ctx, org, image, manifest_ref, http.MethodGet)
		}
	}

	blob_file := getArtifactPath(filepath.Join(image_path, "blobs"), digest)
	if blob_file == "" {
		return ctx.JSON(http.StatusNotFound, "")
	}
	SHA, err := computeMd5Sum(blob_file)
	if err != nil {
		return ctx.JSON(http.StatusNotFound, "")
	}
	ctx.Logger().Info(fmt.Sprintf("found blob - %s", blob_file))
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

func handleOrgImageManifestsReference(r *OciRegistry, ctx echo.Context, org string, image string, reference string, verb string) error {
	ctx.Logger().Info(fmt.Sprintf("%s manifest - org: %s, image: %s, ref: %s", verb, org, image, reference))

	if strings.HasPrefix(reference, "sha256:") {
		// test - might never get here now...
		reference = xlatManifestDigest(image_path, reference)
		if reference == "" {
			return ctx.JSON(http.StatusNotFound, "")
		}
	}

	var manifest_path string = ""
	for i := 0; i < 2; i++ {
		manifest_path = getArtifactPath(filepath.Join(image_path, org, image, reference, "manifest.json"), "")
		if manifest_path == "" {
			pullsync.PullImage(fmt.Sprintf("%s/%s:%s", org, image, reference), 60000, ctx.Logger())
		}
	}
	if manifest_path == "" {
		return ctx.JSON(http.StatusNotFound, "")
	}
	b, err := os.ReadFile(manifest_path)
	if err != nil {
		return ctx.JSON(http.StatusNotFound, "")
	}
	ctx.Logger().Info(fmt.Sprintf("found manifest - %s", manifest_path))

	var m []ManifestJson
	jerr := json.Unmarshal(b, &m)
	if jerr != nil {
		return ctx.JSON(http.StatusInternalServerError, "")
	}
	config_path := getArtifactPath(filepath.Join(image_path, "blobs"), m[0].Config)
	if config_path == "" {
		return ctx.JSON(http.StatusNotFound, "")
	}
	fi, _ := os.Stat(config_path)

	var tmpdgst string
	tmpdgst = helpers.GetSHAfromPath(m[0].Config)
	if tmpdgst == "" {
		return ctx.JSON(http.StatusNotFound, "")
	}
	manifest := ImageManifest{
		SchemaVersion: 2,
		MediaType:     "application/vnd.oci.image.manifest.v1+json",
		Config: ManifestConfig{
			MediaType: "application/vnd.oci.image.config.v1+json",
			Size:      int(fi.Size()),
			Digest:    "sha256:" + tmpdgst,
		},
	}
	for i := 0; i < len(m[0].Layers); i++ {
		ctx.Logger().Info(fmt.Sprintf("get layer - %s", m[0].Layers[i]))
		layer_path := getArtifactPath(filepath.Join(image_path, "blobs"), m[0].Layers[i])
		if layer_path == "" {
			return ctx.JSON(http.StatusNotFound, "")
		}
		ctx.Logger().Info(fmt.Sprintf("found layer - %s", layer_path))
		fi, _ := os.Stat(layer_path)
		tmpdgst = helpers.GetSHAfromPath(m[0].Layers[i])
		if tmpdgst == "" {
			return ctx.JSON(http.StatusNotFound, "")
		}
		new_layer := ManifestLayer{
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
	ctx.Logger().Info(fmt.Sprintf("computed digest for ref %s = sha256:%s (cnt: %d / mblen:%d)", reference, dgst.Hex(), cnt, mblen))

	saveManifestDigest(image_path, reference, dgst.Hex())

	ctx.Response().Header().Add("Content-Length", strconv.Itoa(mblen))
	ctx.Response().Header().Add("Docker-Content-Digest", fmt.Sprintf("sha256:%s", dgst.Hex()))
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