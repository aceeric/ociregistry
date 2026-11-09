package impl

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aceeric/ociregistry/api"
)

// Tests that the generated routes register (http.ServeMux panics at registration on
// ambiguous patterns) and that requests are routed to the right handlers.
func TestRoutes(t *testing.T) {
	h := api.Handler(NewOciRegistry(nil))
	const digest = "sha256:d2c94e258dcb3c5ac2798d32e1249e42ef01cba4841c2234249495f87264ac5a"
	tests := []struct {
		method string
		path   string
		hdrs   map[string]string
		status int
	}{
		{http.MethodGet, "/v2/", nil, http.StatusOK},
		{http.MethodHead, "/v2/", nil, http.StatusOK},
		{http.MethodGet, "/v2/auth", map[string]string{"Authorization": "x"}, http.StatusOK},
		// required header missing: rejected by the generated parameter binding
		{http.MethodGet, "/v2/auth", nil, http.StatusBadRequest},
		// unimplemented operations reach their handlers, which return 405
		{http.MethodHead, "/v2/hello-world/blobs/" + digest, nil, http.StatusMethodNotAllowed},
		{http.MethodDelete, "/v2/a/b/c/d/blobs/" + digest, nil, http.StatusMethodNotAllowed},
		{http.MethodPut, "/v2/library/hello-world/manifests/latest", nil, http.StatusMethodNotAllowed},
		{http.MethodPost, "/v2/hello-world/blobs/uploads", nil, http.StatusMethodNotAllowed},
		{http.MethodGet, "/v2/hello-world/tags/list", nil, http.StatusMethodNotAllowed},
		// more than four repository segments isn't in the spec
		{http.MethodGet, "/v2/a/b/c/d/e/manifests/latest", nil, http.StatusNotFound},
	}
	for _, tst := range tests {
		req := httptest.NewRequest(tst.method, tst.path, nil)
		for k, v := range tst.hdrs {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != tst.status {
			t.Errorf("%s %s: expected %d, got %d", tst.method, tst.path, tst.status, rec.Code)
		}
	}
}
