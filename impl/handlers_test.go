package impl

import (
	"io"
	"net/http"
	"net/http/httptest"
	"ociregistry/mock"
	"os"
	"testing"

	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
)

func init() {
	log.SetOutput(io.Discard)
}

// Starts the mock OCI distribution server then runs the ociregistry server
// and gets a manifest from the ociregistry server with the mock distribution
// server as the upstream to pull from. Ensures the default behavior which is
// that the first pull talks to the upstream (the mock distribution server
// in this case) and all other pulls get from the ociregistry server cache.
func TestManifestGetWithNs(t *testing.T) {
	cnt := 0
	callback := func(url string) {
		if url == "/v2/hello-world/manifests/latest" {
			cnt++
		}
	}
	server, url := mock.ServerWithCallback(mock.NewMockParams(mock.NONE, mock.HTTP), &callback)
	defer server.Close()
	d, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fail()
	}
	defer os.RemoveAll(d)

	r := NewOciRegistry(d, 1000, false)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	getCnt := 5
	for i := 0; i < getCnt; i++ {
		err = r.handleV2OrgImageManifestsReference(ctx, "", "hello-world", "latest", http.MethodGet, &url)
		if err != nil {
			t.Fail()
		}
	}
	if cnt != 1 {
		t.Fail()
	}
}

// Test proxy mode for "latest". In this mode, all pulls of "latest" go to the
// upstream.
func TestNeverCacheLatest(t *testing.T) {
	cnt := 0
	callback := func(url string) {
		if url == "/v2/hello-world/manifests/latest" {
			cnt++
		}
	}
	server, url := mock.ServerWithCallback(mock.NewMockParams(mock.NONE, mock.HTTP), &callback)
	defer server.Close()
	d, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fail()
	}
	defer os.RemoveAll(d)

	r := NewOciRegistry(d, 1000, true)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	getCnt := 5
	for i := 0; i < getCnt; i++ {
		err = r.handleV2OrgImageManifestsReference(ctx, "", "hello-world", "latest", http.MethodGet, &url)
		if err != nil {
			t.Fail()
		}
	}
	if cnt != getCnt {
		t.Fail()
	}
}

func TestParseNamespace(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	namespace := "docker.io"
	remote := parseRemote(ctx, &namespace)
	if remote != namespace {
		t.Fail()
	}
	ctx.Request().Header.Add("X-Registry", "quay.io")
	remote = parseRemote(ctx, nil)
	if remote != "quay.io" {
		t.Fail()
	}
}
