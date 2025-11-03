package impl

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/aceeric/ociregistry/api/models"
	"github.com/aceeric/ociregistry/impl/config"
	"github.com/aceeric/ociregistry/mock"

	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
)

func init() {
	log.SetOutput(io.Discard)
}

// configures the OCI distribution server's connection to the upstream
// mock server.
var serverCfg = `
---
imagePath: %s
pullTimeout: %d
alwaysPullLatest: %t
registries:
  - name: %s
    scheme: http
`

// Gets a manifest from the ociregistry server with the mock distribution
// server as the upstream to pull from. Ensures the default behavior which is
// that the first pull talks to the upstream (the mock distribution server
// in this case) and all other pulls get from the ociregistry server cache.
func TestManifestGetWithNs(t *testing.T) {
	td, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fail()
	}
	defer os.RemoveAll(td)
	cnt := 0
	expectCnt := 2
	callback := func(url string) {
		// one call to HEAD the server to check and see if auth is required and
		// a second call to get the manifest
		if url == "/v2/hello-world/manifests/latest" {
			cnt++
		}
	}
	server, url := mock.ServerWithCallback(mock.NewMockParams(mock.NONE, mock.HTTP), &callback)
	cfg := fmt.Sprintf(serverCfg, td, 1000, false, url)
	if err := config.SetConfigFromStr([]byte(cfg)); err != nil {
		t.Fail()
	}
	defer server.Close()

	r := NewOciRegistry(nil)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	getCnt := 5
	for i := 0; i < getCnt; i++ {
		r.handleV2OrgImageManifestsReference(ctx, "", "hello-world", "latest", http.MethodGet, &url)
		if ctx.Response().Status != 200 {
			t.Fail()
		}
	}
	if cnt != expectCnt {
		t.Fail()
	}
}

// Test proxy mode for "latest". In this mode, all pulls of "latest" go to the
// upstream.
func TestNeverCacheLatest(t *testing.T) {
	td, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fail()
	}
	defer os.RemoveAll(td)
	cnt := 0
	callback := func(url string) {
		// since no images are cached each image pull will access this url two times:
		// once to HEAD for auth check and a second to pull the manifest
		if url == "/v2/hello-world/manifests/latest" {
			cnt++
		}
	}
	server, url := mock.ServerWithCallback(mock.NewMockParams(mock.NONE, mock.HTTP), &callback)
	cfg := fmt.Sprintf(serverCfg, td, 1000, true, url)
	if err := config.SetConfigFromStr([]byte(cfg)); err != nil {
		t.Fail()
	}
	defer server.Close()

	r := NewOciRegistry(nil)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	getCnt := 5
	expectCnt := getCnt * 2
	for i := 0; i < getCnt; i++ {
		r.handleV2OrgImageManifestsReference(ctx, "", "hello-world", "latest", http.MethodGet, &url)
		if ctx.Response().Status != 200 {
			t.Fail()
		}
	}
	if cnt != expectCnt {
		t.Fail()
	}
}

func TestParseNamespace(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	remote := parseRemote(ctx, nil)
	if remote != "" {
		t.Fail()
	}

	namespace := "docker.io"
	remote = parseRemote(ctx, &namespace)
	if remote != namespace {
		t.Fail()
	}

	ctx.Request().Header.Add("X-Registry", "quay.io")
	remote = parseRemote(ctx, nil)
	if remote != "quay.io" {
		t.Fail()
	}

	remote = parseRemote(ctx, &namespace)
	// header has higher precedence than explicit namespace arg
	if remote != "quay.io" {
		t.Fail()
	}
}

// Tests getting a blob. Since no manifests have been pulled thru a 404
// should be returned.
func TestBlobGetFails(t *testing.T) {
	td, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fail()
	}
	defer os.RemoveAll(td)
	server, url := mock.Server(mock.NewMockParams(mock.NONE, mock.HTTP))
	cfg := fmt.Sprintf(serverCfg, td, 1000, false, url)
	if err := config.SetConfigFromStr([]byte(cfg)); err != nil {
		t.Fail()
	}
	defer server.Close()

	r := NewOciRegistry(nil)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	r.handleV2GetOrgImageBlobsDigest(ctx, "", "hello-world", "d2c94e258dcb3c5ac2798d32e1249e42ef01cba4841c2234249495f87264ac5a")
	if ctx.Response().Status != 404 {
		t.Fail()
	}
}

// gets an image manifest which triggers also pulling the blobs.
func TestPullImageAndBlob(t *testing.T) {
	td, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fail()
	}
	defer os.RemoveAll(td)
	server, url := mock.Server(mock.NewMockParams(mock.NONE, mock.HTTP))
	cfg := fmt.Sprintf(serverCfg, td, 1000, false, url)
	if err := config.SetConfigFromStr([]byte(cfg)); err != nil {
		t.Fail()
	}
	defer server.Close()

	r := NewOciRegistry(nil)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	r.handleV2OrgImageManifestsReference(ctx, "", "hello-world", "sha256:e2fc4e5012d16e7fe466f5291c476431beaa1f9b90a5c2125b493ed28e2aba57", http.MethodGet, &url)
	if ctx.Response().Status != 200 {
		t.Fail()
	}
	r.handleV2GetOrgImageBlobsDigest(ctx, "", "hello-world", "d2c94e258dcb3c5ac2798d32e1249e42ef01cba4841c2234249495f87264ac5a")
	if ctx.Response().Status != 200 {
		t.Fail()
	}
}

// Tests Range header support for chunked blob downloads
func TestBlobRangeRequest(t *testing.T) {
	td, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fail()
	}
	defer os.RemoveAll(td)
	server, url := mock.Server(mock.NewMockParams(mock.NONE, mock.HTTP))
	cfg := fmt.Sprintf(serverCfg, td, 1000, false, url)
	if err := config.SetConfigFromStr([]byte(cfg)); err != nil {
		t.Fail()
	}
	defer server.Close()

	r := NewOciRegistry(nil)
	e := echo.New()

	// First pull the manifest and blob
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	r.handleV2OrgImageManifestsReference(ctx, "", "hello-world", "sha256:e2fc4e5012d16e7fe466f5291c476431beaa1f9b90a5c2125b493ed28e2aba57", http.MethodGet, &url)
	if ctx.Response().Status != 200 {
		t.Fail()
	}

	// Test 1: Request with Range header for first 100 bytes
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Range", "bytes=0-99")
	rec = httptest.NewRecorder()
	ctx = e.NewContext(req, rec)
	r.handleV2GetOrgImageBlobsDigest(ctx, "", "hello-world", "d2c94e258dcb3c5ac2798d32e1249e42ef01cba4841c2234249495f87264ac5a")
	if ctx.Response().Status != http.StatusPartialContent {
		t.Errorf("expected status %d, got %d", http.StatusPartialContent, ctx.Response().Status)
	}
	if ctx.Response().Header().Get("Content-Range") == "" {
		t.Error("expected Content-Range header to be set")
	}
	if ctx.Response().Header().Get("Accept-Ranges") != "bytes" {
		t.Error("expected Accept-Ranges header to be 'bytes'")
	}
	contentLength := ctx.Response().Header().Get("Content-Length")
	if contentLength != "100" {
		t.Errorf("expected Content-Length to be 100, got %s", contentLength)
	}

	// Test 2: Request with open-ended Range (from byte 50 to end)
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Range", "bytes=50-")
	rec = httptest.NewRecorder()
	ctx = e.NewContext(req, rec)
	r.handleV2GetOrgImageBlobsDigest(ctx, "", "hello-world", "d2c94e258dcb3c5ac2798d32e1249e42ef01cba4841c2234249495f87264ac5a")
	if ctx.Response().Status != http.StatusPartialContent {
		t.Errorf("expected status %d, got %d", http.StatusPartialContent, ctx.Response().Status)
	}

	// Test 3: Request without Range header should return 200 OK
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = httptest.NewRecorder()
	ctx = e.NewContext(req, rec)
	r.handleV2GetOrgImageBlobsDigest(ctx, "", "hello-world", "d2c94e258dcb3c5ac2798d32e1249e42ef01cba4841c2234249495f87264ac5a")
	if ctx.Response().Status != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, ctx.Response().Status)
	}
	if ctx.Response().Header().Get("Accept-Ranges") != "bytes" {
		t.Error("expected Accept-Ranges header to be 'bytes' even without Range request")
	}

	// Test 4: Invalid Range header should return 416 Range Not Satisfiable
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Range", "bytes=999999999-")
	rec = httptest.NewRecorder()
	ctx = e.NewContext(req, rec)
	r.handleV2GetOrgImageBlobsDigest(ctx, "", "hello-world", "d2c94e258dcb3c5ac2798d32e1249e42ef01cba4841c2234249495f87264ac5a")
	if ctx.Response().Status != http.StatusRequestedRangeNotSatisfiable {
		t.Errorf("expected status %d, got %d", http.StatusRequestedRangeNotSatisfiable, ctx.Response().Status)
	}
}

// Tests parseRangeHeader helper function
func TestParseRangeHeader(t *testing.T) {
	fileSize := int64(1000)

	// Test 1: Normal range
	start, end, err := parseRangeHeader("bytes=0-99", fileSize)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if start != 0 || end != 99 {
		t.Errorf("expected range 0-99, got %d-%d", start, end)
	}

	// Test 2: Open-ended range
	start, end, err = parseRangeHeader("bytes=500-", fileSize)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if start != 500 || end != 999 {
		t.Errorf("expected range 500-999, got %d-%d", start, end)
	}

	// Test 3: Suffix range (last N bytes)
	start, end, err = parseRangeHeader("bytes=-100", fileSize)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if start != 900 || end != 999 {
		t.Errorf("expected range 900-999, got %d-%d", start, end)
	}

	// Test 4: Range beyond file size should be adjusted
	start, end, err = parseRangeHeader("bytes=0-9999", fileSize)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if start != 0 || end != 999 {
		t.Errorf("expected range 0-999 (adjusted), got %d-%d", start, end)
	}

	// Test 5: Invalid range (start beyond file size)
	_, _, err = parseRangeHeader("bytes=1001-", fileSize)
	if err == nil {
		t.Error("expected error for start position beyond file size")
	}

	// Test 6: Invalid range format
	_, _, err = parseRangeHeader("invalid", fileSize)
	if err == nil {
		t.Error("expected error for invalid range format")
	}

	// Test 7: Multiple ranges not supported
	_, _, err = parseRangeHeader("bytes=0-99,200-299", fileSize)
	if err == nil {
		t.Error("expected error for multiple ranges")
	}

	// Test 8: Start > End
	_, _, err = parseRangeHeader("bytes=100-50", fileSize)
	if err == nil {
		t.Error("expected error for start > end")
	}
}

type TestAuthToken struct {
	Token string `json:"token"`
}

func TestHandleV2Auth(t *testing.T) {
	r := NewOciRegistry(nil)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	scope := "SCOPE"
	service := "SERVICE"
	params := models.V2AuthParams{
		Scope:         &scope,
		Service:       &service,
		Authorization: "AUTHORIZATION",
	}
	r.handleV2Auth(ctx, params)
	if ctx.Response().Status != 200 {
		t.Fail()
	}
	token, err := io.ReadAll(rec.Body)
	if err != nil {
		t.FailNow()
	}
	parsedToken := TestAuthToken{}
	err = json.Unmarshal(token, &parsedToken)
	if err != nil {
		t.FailNow()
	}
	if parsedToken.Token != "FROBOZZ" {
		t.FailNow()
	}
}
