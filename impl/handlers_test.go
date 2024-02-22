package impl

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func Test1(t *testing.T) {
	r := NewOciRegistry("/tmp", 1000)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	remote := "docker.io"
	err := r.handleV2OrgImageManifestsReference(ctx, "", "hello-world", "latest", http.MethodGet, &remote)
	fmt.Print(err)
	r.handleV2OrgImageManifestsReference(ctx, "", "hello-world", "latest", http.MethodGet, &remote)
	fmt.Print(err)
}

func Test2(t *testing.T) {
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
