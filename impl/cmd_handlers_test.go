package impl

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aceeric/ociregistry/api/models"
	"github.com/aceeric/ociregistry/impl/cache"
	"github.com/aceeric/ociregistry/impl/config"
	"github.com/aceeric/ociregistry/impl/globals"
	"github.com/aceeric/ociregistry/impl/serialize"
	"github.com/labstack/echo/v4"
)

var orgs = []string{"foo", "bar", "baz"}

// note %d at end
var manifestDigest = "03b62250a3cb1abd125271d393fc08bf0cc713391eda6b57c02d1ef85efcc25%d"

var manifest = `{
  "type": 3,
  "digest": "%s",
  "imageUrl": "docker.io/library/%s:v1.2.3",
  "bytes": "",
  "v1.oci.index": {},
  "v1.oci.manifest": {
    "schemaVersion": 2,
    "mediaType": "application/vnd.oci.image.manifest.v1+json",
    "config": {
      "mediaType": "application/vnd.oci.image.config.v1+json",
      "digest": "sha256:74cc54e27dc41bb10dc4b2226072d469509f2f22f1a3ce74f4a59661a1d44602"
    },
    "layers": [
      {
        "mediaType": "application/vnd.oci.image.layer.v1.tar+gzip",
        "digest": "sha256:e6590344b1a5dc518829d6ea1524fc12f8bcd14ee9a02aa6ad8360cce3a9a9e9"
      }
    ]
  },
  "v2.docker.manifestList": {},
  "v2.docker.manifest": {},
  "created": "",
  "pulled": ""
}
`

func TestCmdApi(t *testing.T) {
	cache.ResetCache()
	td, err := setupTests()
	if td != "" {
		defer os.RemoveAll(td)
	}
	if err != nil {
		t.FailNow()
	}
	r := NewOciRegistry(nil)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	tests := []struct {
		testFun      func(*OciRegistry, echo.Context, *httptest.ResponseRecorder) bool
		expRespLines int
	}{
		{testFun: testGetManifestList, expRespLines: 2},
		{testFun: testGetBlobList, expRespLines: 2},
		{testFun: testGetImgList, expRespLines: 10},
		{testFun: testPrune, expRespLines: 2},
	}
	for _, tst := range tests {
		rec := httptest.NewRecorder()
		ctx := e.NewContext(req, rec)
		if !tst.testFun(r, ctx, rec) {
			t.FailNow()
		}
		res := rec.Result()
		body, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			t.FailNow()
		}
		fmt.Print(string(body))
		if strings.Count(string(body), "\n") != tst.expRespLines {
			t.FailNow()
		}
	}
}

// GET /cmd/manifest/list?pattern=...
// returns header line, one manifest line
func testGetManifestList(r *OciRegistry, ctx echo.Context, rec *httptest.ResponseRecorder) bool {
	r.CmdManifestlist(ctx, models.CmdManifestlistParams{Pattern: &orgs[1]})
	return ctx.Response().Status == 200
}

// GET /cmd/blob/list?substr=...
// returns header line, one blob line
func testGetBlobList(r *OciRegistry, ctx echo.Context, rec *httptest.ResponseRecorder) bool {
	substr := "74cc54e27dc"
	r.CmdBloblist(ctx, models.CmdBloblistParams{Substr: &substr})
	return ctx.Response().Status == 200
}

// GET cmd/image/list?digest=... or cmd/image/list?pattern=...
// since for the test data all the manifests have the same blobs all manifests are returned so
// returns header line and 3X: manifest line, two blob lines
func testGetImgList(r *OciRegistry, ctx echo.Context, rec *httptest.ResponseRecorder) bool {
	md := "e6590344b1a5dc518829d"
	r.CmdImagelist(ctx, models.CmdImagelistParams{Digest: &md})
	return ctx.Response().Status == 200
}

// DELETE /cmd/prune?type=...&dur=...&expr=...&dryRun=
// returns two log entries:
// time=... level=info msg="begin prune..."
// time=... level=info msg="doPrune..."
func testPrune(r *OciRegistry, ctx echo.Context, rec *httptest.ResponseRecorder) bool {
	expr := "docker.io/library/" + orgs[2]
	r.CmdPrune(ctx, models.CmdPruneParams{
		Type: "pattern",
		Expr: &expr,
	})
	return ctx.Response().Status == 200
}

// setupTests create three manifests each with the same single blob. Create an orphaned
// blob just so there are two for the blob list.
func setupTests() (string, error) {
	td, err := os.MkdirTemp("", "")
	blobDigests := []string{
		"e6590344b1a5dc518829d6ea1524fc12f8bcd14ee9a02aa6ad8360cce3a9a9e9",
		"74cc54e27dc41bb10dc4b2226072d469509f2f22f1a3ce74f4a59661a1d44602",
	}
	if err != nil {
		return "", err
	}
	serialize.CreateDirs(td, true)
	for _, blobDigest := range blobDigests {
		if err = os.WriteFile(filepath.Join(td, globals.BlobPath, blobDigest), []byte(blobDigest), 0777); err != nil {
			return td, err
		}
	}
	config.Set(config.Configuration{
		ImagePath:   td,
		PullTimeout: 1000,
	})
	for i := range 3 {
		md := fmt.Sprintf(manifestDigest, i)
		mfst := fmt.Sprintf(manifest, md, orgs[i])
		if err = os.WriteFile(filepath.Join(td, globals.ImgPath, md), []byte(mfst), 0777); err != nil {
			return td, err
		}
	}
	if err = cache.Load(td); err != nil {
		return td, err
	}
	return td, nil
}
