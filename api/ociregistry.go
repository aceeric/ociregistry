// implements the "pull-only" registry server. Provides implementations for methods
// required to pull an image.
package api

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	. "ociregistry/api/models"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

type OciRegistry struct{}

func NewOciRegistry() *OciRegistry {
	return &OciRegistry{}
}

// CONNECT /
func (r *OciRegistry) Connect(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, nil)
}

// GET /v2/auth
// everyone authenticates successfully and gets the same token which is subsequently ignored by the server
func (r *OciRegistry) V2Auth(ctx echo.Context, params V2AuthParams) error {
	body := &Token{Token: "FROBOZZ"}
	ctx.Response().Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
	ctx.Response().Header().Add("Vary", "Cookie")
	ctx.Response().Header().Add("Strict-Transport-Security", "max-age=63072000; preload")
	return ctx.JSON(http.StatusOK, body)
}

// GET /v2/
func (r *OciRegistry) V2Default(ctx echo.Context) error {
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

// GET /v2/{image}/blobs/{digest}
func (r *OciRegistry) V2GetImageBlobsDigest(ctx echo.Context, image string, digest string) error {
	return r.V2GetOrgImageBlobsDigest(ctx, "library", image, digest)
}

// GET /v2/{org}/{image}/blobs/{digest}
func (r *OciRegistry) V2GetOrgImageBlobsDigest(ctx echo.Context, org string, image string, digest string) error {
	ctx.Logger().Info(fmt.Sprintf("get blob - org: %s, image: %s, digest: %s", org, image, digest))
	blob_file := getArtifactPath(filepath.Join(image_path, org, image), digest)
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
	f, err := os.Open(blob_file)
	if err != nil {
		return err
	}
	return ctx.Stream(http.StatusOK, "binary/octet-stream", f)
}

// HEAD /v2/{image}/manifests/{reference}
func (r *OciRegistry) V2HeadImageManifestsReference(ctx echo.Context, image string, reference string) error {
	return r.handleOrgImageManifestsReference(ctx, "library", image, reference, false)
}

// HEAD /v2/{org}/{image}/manifests/{reference}
func (r *OciRegistry) V2HeadOrgImageManifestsReference(ctx echo.Context, org string, image string, reference string) error {
	return r.handleOrgImageManifestsReference(ctx, org, image, reference, false)
}

// GET /v2/{image}/manifests/{reference}
func (r *OciRegistry) V2GetImageManifestsReference(ctx echo.Context, image string, reference string) error {
	//return r.V2GetOrgImageManifestsReference(ctx, "library", image, reference)
	return r.handleOrgImageManifestsReference(ctx, "library", image, reference, true)
}

// GET /v2/{org}/{image}/manifests/{reference}
func (r *OciRegistry) V2GetOrgImageManifestsReference(ctx echo.Context, org string, image string, reference string) error {
	return r.handleOrgImageManifestsReference(ctx, org, image, reference, true)
}

func (r *OciRegistry) handleOrgImageManifestsReference(ctx echo.Context, org string, image string, reference string, isGet bool) error {
	ctx.Logger().Info(fmt.Sprintf("get manifest - org: %s, image: %s, ref: %s", org, image, reference))

	if strings.HasPrefix(reference, "sha256:") {
		reference = xlatManifestDigest(image_path, org, image, reference)
		if reference == "" {
			return ctx.JSON(http.StatusNotFound, "")
		}
	}

	manifest_path := getArtifactPath(filepath.Join(image_path, org, image, reference, "manifest.json"), "")
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
	config_path := getArtifactPath(filepath.Join(image_path, org, image, reference, m[0].Config), "")
	if config_path == "" {
		return ctx.JSON(http.StatusNotFound, "")
	}
	fi, _ := os.Stat(config_path)

	var digest string
	digest = strings.Replace(m[0].Config, ".json", "", 1)
	if !strings.HasPrefix(digest, "sha256:") {
		digest = "sha256:" + digest
	}
	manifest := ImageManifest{
		SchemaVersion: 2,
		MediaType:     "application/vnd.docker.distribution.manifest.v2+json",
		Config: ManifestConfig{
			MediaType: "application/vnd.docker.container.image.v1+json",
			Size:      int(fi.Size()),
			Digest:    digest,
		},
	}
	for i := 0; i < len(m[0].Layers); i++ {
		ctx.Logger().Info(fmt.Sprintf("get layer - %s", m[0].Layers[i]))
		layer_path := getArtifactPath(filepath.Join(image_path, org, image, reference, m[0].Layers[i]), "")
		if layer_path == "" {
			return ctx.JSON(http.StatusNotFound, "")
		}
		ctx.Logger().Info(fmt.Sprintf("found layer - %s", layer_path))
		fi, _ := os.Stat(layer_path)
		digest = strings.Replace(m[0].Layers[i], ".tar.gz", "", 1)
		digest = strings.Replace(digest, "/layer.tar", "", 1)
		new_layer := ManifestLayer{
			MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
			Size:      int(fi.Size()),
			Digest:    "sha256:" + digest,
		}
		manifest.Layers = append(manifest.Layers, new_layer)
	}
	ctx.Response().Header().Add("Content-Type", " application/vnd.docker.distribution.manifest.v2+json")

	// compute manifest digest
	mb, err := json.Marshal(manifest)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, "")
	}
	sha := sha256.New()
	sha.Write(mb)
	saveManifestDigest(manifest_path, sha)

	ctx.Response().Header().Add("Docker-Content-Digest", "sha256:"+fmt.Sprintf("%x", sha.Sum(nil)))
	ctx.Response().Header().Add("Vary", "Cookie")
	ctx.Response().Header().Add("Strict-Transport-Security", "max-age=63072000; preload")
	ctx.Response().Header().Add("X-Frame-Options", "DENY")

	if isGet {
		return ctx.JSON(http.StatusOK, manifest)
	} else {
		mstr := string(mb)
		mlen := len(mstr) + 1
		ctx.Response().Header().Add("Content-Length", strconv.Itoa(mlen))
		return ctx.NoContent(http.StatusOK)
	}
}

// unimplemented methods of the OCI distribution spec

func (r *OciRegistry) V2HeadOrgImageBlobsDigest(ctx echo.Context, org string, image string, digest string) error {
	return ctx.JSON(http.StatusOK, "V2HeadNameBlobsDigest")
}

func (r *OciRegistry) V2PostNameBlobsUploads(ctx echo.Context, name string, params V2PostNameBlobsUploadsParams) error {
	return ctx.JSON(http.StatusOK, "V2PostNameBlobsUploads")
}

func (r *OciRegistry) V2GetNameBlobsUploadsReference(ctx echo.Context, name string, reference string) error {
	return ctx.JSON(http.StatusOK, "V2GetNameBlobsUploadsReference")
}

func (r *OciRegistry) V2PatchNameBlobsUploadsReference(ctx echo.Context, name string, reference string) error {
	return ctx.JSON(http.StatusOK, "V2PatchNameBlobsUploadsReference")
}

func (r *OciRegistry) V2PutNameBlobsUploadsReference(ctx echo.Context, name string, reference string, params V2PutNameBlobsUploadsReferenceParams) error {
	return ctx.JSON(http.StatusOK, "")
}

func (r *OciRegistry) V2PutOrgImageManifestsReference(ctx echo.Context, org string, image string, reference string) error {
	return ctx.JSON(http.StatusOK, "V2PutNameManifestsReference")
}

func (r *OciRegistry) V2GetNameReferrersDigest(ctx echo.Context, name string, digest string, params V2GetNameReferrersDigestParams) error {
	return ctx.JSON(http.StatusOK, "V2GetNameReferrersDigest")
}

func (r *OciRegistry) V2GetNameTagsList(ctx echo.Context, name string, params V2GetNameTagsListParams) error {
	return ctx.JSON(http.StatusOK, "V2GetNameTagsList")
}

func (r *OciRegistry) V2DeleteImageBlobsDigest(ctx echo.Context, image string, digest string) error {
	return ctx.JSON(http.StatusOK, "V2GetNameTagsList")
}

func (r *OciRegistry) V2DeleteImageManifestsReference(ctx echo.Context, image string, reference string) error {
	return ctx.JSON(http.StatusOK, "V2GetNameTagsList")
}

func (r *OciRegistry) V2DeleteOrgImageBlobsDigest(ctx echo.Context, org string, image string, digest string) error {
	return ctx.JSON(http.StatusOK, "V2GetNameTagsList")
}

func (r *OciRegistry) V2DeleteOrgImageManifestsReference(ctx echo.Context, org string, image string, reference string) error {
	return ctx.JSON(http.StatusOK, "V2GetNameTagsList")
}

func (r *OciRegistry) V2HeadImageBlobsDigest(ctx echo.Context, image string, digest string) error {
	return ctx.JSON(http.StatusOK, "V2GetNameTagsList")
}

func (r *OciRegistry) V2PutImageManifestsReference(ctx echo.Context, image string, reference string) error {
	return ctx.JSON(http.StatusOK, "V2GetNameTagsList")
}
