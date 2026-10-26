package impl

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/aceeric/ociregistry/api/models"
	"github.com/aceeric/ociregistry/impl/cache"
	"github.com/aceeric/ociregistry/impl/config"
	"github.com/aceeric/ociregistry/impl/serialize"
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
	serialize.CreateDirs(td, true)
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
	for range getCnt {
		r.handleV2ManifestsReference(ctx, "latest", &url, http.MethodGet, "hello-world")
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
	serialize.CreateDirs(td, true)
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
	for range getCnt {
		r.handleV2ManifestsReference(ctx, "latest", &url, http.MethodGet, "hello-world")
		if ctx.Response().Status != 200 {
			t.Fail()
		}
	}
	if cnt != expectCnt {
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
	r.handleV2BlobsDigest(ctx, "d2c94e258dcb3c5ac2798d32e1249e42ef01cba4841c2234249495f87264ac5a", "hello-world")
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
	serialize.CreateDirs(td, true)
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
	r.handleV2ManifestsReference(ctx, "sha256:e2fc4e5012d16e7fe466f5291c476431beaa1f9b90a5c2125b493ed28e2aba57", &url, http.MethodGet, "hello-world")
	if ctx.Response().Status != 200 {
		t.Fail()
	}
	r.handleV2BlobsDigest(ctx, "d2c94e258dcb3c5ac2798d32e1249e42ef01cba4841c2234249495f87264ac5a", "hello-world")
	if ctx.Response().Status != 200 {
		t.Fail()
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

// configures the server with --no-cache patterns, an upstream (the mock server) and,
// optionally, air-gapped mode
var noCacheCfg = `
---
imagePath: %s
pullTimeout: 1000
airGapped: %t
noCache:
%s
registries:
  - name: %s
    scheme: http
`

const helloImageDigest = "sha256:e2fc4e5012d16e7fe466f5291c476431beaa1f9b90a5c2125b493ed28e2aba57"

// noCacheServer starts the mock upstream counting manifest requests per path, configures the
// server with the passed --no-cache patterns, and returns a registry plus the counts.
func noCacheServer(t *testing.T, airGapped bool, patterns ...string) (*OciRegistry, string, map[string]int, func()) {
	t.Helper()
	cache.ResetCache()
	td := t.TempDir()
	serialize.CreateDirs(td, true)
	var mu sync.Mutex
	counts := map[string]int{}
	callback := func(url string) {
		mu.Lock()
		counts[url]++
		mu.Unlock()
	}
	server, url := mock.ServerWithCallback(mock.NewMockParams(mock.NONE, mock.HTTP), &callback)
	list := ""
	for _, p := range patterns {
		list += fmt.Sprintf("  - '%s'\n", p)
	}
	if err := config.SetConfigFromStr([]byte(fmt.Sprintf(noCacheCfg, td, airGapped, list, url))); err != nil {
		t.Fatal(err)
	}
	return NewOciRegistry(nil), url, counts, server.Close
}

// getManifest gets a manifest through the handler and returns the response status.
func getManifest(r *OciRegistry, ns string, ref string) int {
	ctx := echo.New().NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())
	r.handleV2ManifestsReference(ctx, ref, &ns, http.MethodGet, "hello-world")
	return ctx.Response().Status
}

// Tests that a tag matching a --no-cache pattern is pulled from the upstream every time,
// and that a digest request matching a pattern is too.
func TestNoCachePattern(t *testing.T) {
	r, url, counts, stop := noCacheServer(t, false, ":latest$", "@sha256:e2fc")
	defer stop()
	for range 3 {
		if status := getManifest(r, url, "latest"); status != http.StatusOK {
			t.Fatalf("expected 200, got %d", status)
		}
		if status := getManifest(r, url, helloImageDigest); status != http.StatusOK {
			t.Fatalf("expected 200, got %d", status)
		}
	}
	// two upstream requests per pull: a HEAD for the auth check, then the GET
	if n := counts["/v2/hello-world/manifests/latest"]; n != 6 {
		t.Errorf("expected every tag request to go upstream (6), got %d", n)
	}
	if n := counts["/v2/hello-world/manifests/"+helloImageDigest]; n != 6 {
		t.Errorf("expected every digest request to go upstream (6), got %d", n)
	}
}

// Tests that without a matching pattern, only the first request goes upstream.
func TestNoCachePatternNoMatch(t *testing.T) {
	r, url, counts, stop := noCacheServer(t, false, ":nightly$")
	defer stop()
	for range 3 {
		if status := getManifest(r, url, "latest"); status != http.StatusOK {
			t.Fatalf("expected 200, got %d", status)
		}
	}
	if n := counts["/v2/hello-world/manifests/latest"]; n != 2 {
		t.Errorf("expected one upstream pull (2 requests), got %d", n)
	}
}

// Tests that when the upstream can't be reached, a --no-cache image is served from the cache,
// and one that was never cached fails.
func TestNoCacheUpstreamDown(t *testing.T) {
	r, url, _, stop := noCacheServer(t, false, ":latest$", "@sha256:")
	if status := getManifest(r, url, "latest"); status != http.StatusOK {
		t.Fatalf("expected 200 while the upstream is up, got %d", status)
	}
	stop()
	if status := getManifest(r, url, "latest"); status != http.StatusOK {
		t.Fatalf("expected the cached copy (200) with the upstream down, got %d", status)
	}
	if status := getManifest(r, url, helloImageDigest); status != http.StatusNotFound {
		t.Fatalf("expected 404 for a never-cached image with the upstream down, got %d", status)
	}
}

// Tests that air-gapped mode ignores --no-cache: a cached image is served without contacting
// the upstream.
func TestNoCacheAirGapped(t *testing.T) {
	r, url, counts, stop := noCacheServer(t, false)
	defer stop()
	if status := getManifest(r, url, "latest"); status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	before := counts["/v2/hello-world/manifests/latest"]
	td := config.GetImagePath()
	if err := config.SetConfigFromStr([]byte(fmt.Sprintf(noCacheCfg, td, true, "  - ':latest$'\n", url))); err != nil {
		t.Fatal(err)
	}
	airGapped := NewOciRegistry(nil)
	for range 3 {
		if status := getManifest(airGapped, url, "latest"); status != http.StatusOK {
			t.Fatalf("expected 200, got %d", status)
		}
	}
	if n := counts["/v2/hello-world/manifests/latest"]; n != before {
		t.Errorf("air-gapped mode must not contact the upstream: %d requests, expected %d", n, before)
	}
}

// Tests that alwaysPullLatest means the same as --no-cache ':latest$'.
func TestAlwaysPullLatestIsNoCacheLatest(t *testing.T) {
	cache.ResetCache()
	config.SetConfigFromStr([]byte("alwaysPullLatest: true\n"))
	r := NewOciRegistry(nil)
	if !r.alwaysPull("docker.io/library/hello-world:latest") || r.alwaysPull("docker.io/library/hello-world:v1") {
		t.Fatal("alwaysPullLatest should match exactly the latest tag")
	}
}
