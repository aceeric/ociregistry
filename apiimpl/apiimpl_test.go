package apiimpl

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/labstack/echo/v4"
	digest "github.com/opencontainers/go-digest"
)

func Test(t *testing.T) {
	digest := "c505b92c0b63dffe1f09ce64ae9d99cddefb01aafbb2a51d8531f44b0998f248"
	dir := filepath.Join("/tmp/docker.io/calico/node", digest)
	os.MkdirAll(dir, 0755)
	manifest := filepath.Join(dir, "manifest.json")
	os.Create(manifest)

	exists, sha := manifestIsUnderDigest("/tmp", "calico", "node", "sha256:c505b92c0b63dffe1f09ce64ae9d99cddefb01aafbb2a51d8531f44b0998f248")
	if exists {
		fmt.Println(sha)
	} else {
		t.Fail()
	}
}

func TestGetManifest(t *testing.T) {
	SetImagePath("/home/eace/projects/ociregistry/images")
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ref := "58cd8f6547b4f438f36a1cd7030985f153eca22d78d86dc9cd6f0fe4f32d01bf"
	handleOrgImageManifestsReference(nil, ctx, "calico", "node", ref, http.MethodGet, nil)
	handleOrgImageManifestsReference(nil, ctx, "calico", "node", ref, http.MethodGet, nil)
}

func TestGetManifestByDigest(t *testing.T) {
	SetImagePath("/home/eace/projects/ociregistry/images")
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ref := "58cd8f6547b4f438f36a1cd7030985f153eca22d78d86dc9cd6f0fe4f32d01bf"
	handleOrgImageManifestsReference(nil, ctx, "calico", "node", ref, http.MethodGet, nil)
	handleOrgImageManifestsReference(nil, ctx, "calico", "node", ref, http.MethodGet, nil)
}

func TestDigest(t *testing.T) {
	manifest := `{"schemaVersion":2,"mediaType":"application/vnd.docker.distribution.manifest.list.v2+json","manifests":[{"mediaType":"application/vnd.docker.distribution.manifest.v2+json","size":2214,"digest":"sha256:21498c24d6e850a70a6d68362ebb1b5354fb5894b7c09b0a7085ed63227a72f5","platform":{"architecture":"arm","os":"linux","variant":"v7"}},{"mediaType":"application/vnd.docker.distribution.manifest.v2+json","size":945,"digest":"sha256:fd23b0abad4afbc5abbc83de79ea13a3ea269ec3b348673a9186d13da96cfe6b","platform":{"architecture":"amd64","os":"linux"}},{"mediaType":"application/vnd.docker.distribution.manifest.v2+json","size":2425,"digest":"sha256:00b79cecd9036207c019fff0c5d4f509bdfa63e71bfe35abaa725ff092990f0d","platform":{"architecture":"s390x","os":"linux"}},{"mediaType":"application/vnd.docker.distribution.manifest.v2+json","size":945,"digest":"sha256:9d2e575c5958558b7472d671fe47703d414970fca0e980cc756a1424bdb43751","platform":{"architecture":"arm64","os":"linux"}},{"mediaType":"application/vnd.docker.distribution.manifest.v2+json","size":2424,"digest":"sha256:175f4862c61e2d84a81fee32255b24f28540bf34c783880feda0cc352746ef45","platform":{"architecture":"ppc64le","os":"linux"}}]}`
	var digester = digest.Canonical.Digester()
	cnt, _ := digester.Hash().Write([]byte(manifest))
	dgst := digester.Digest()
	fmt.Printf("digest: %s, len: %d", dgst.Hex(), cnt)
}

func TestDigest2(t *testing.T) {
	manifest := `{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","config":{"mediaType":"application/vnd.oci.image.config.v1+json","digest":"sha256:1843802b91be8ff1c1d35ee08461ebe909e7a2199e59396f69886439a372312c","size":2027},"layers":[{"mediaType":"application/vnd.oci.image.layer.v1.tar+gzip","digest":"sha256:40019c15ac2bad2a1e6087434d6db4bfd46933cebd154e20145ee678970284f6","size":116694148},{"mediaType":"application/vnd.oci.image.layer.v1.tar+gzip","digest":"sha256:c505b92c0b63dffe1f09ce64ae9d99cddefb01aafbb2a51d8531f44b0998f248","size":4061},{"mediaType":"application/vnd.oci.image.layer.v1.tar+gzip","digest":"sha256:e45dcd3a4b7f800f0bc75952e03999896f4752fd74a4f5d662695e1e503a5fca","size":162}]}`
	var digester = digest.Canonical.Digester()
	cnt, _ := digester.Hash().Write([]byte(manifest))
	dgst := digester.Digest()
	fmt.Printf("digest: %s, len: %d", dgst.Hex(), cnt)
}
