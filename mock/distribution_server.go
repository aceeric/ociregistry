package mock

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	manifestList  = `{"manifests":[{"annotations":{"org.opencontainers.image.revision":"3fb6ebca4163bf5b9cc496ac3e8f11cb1e754aee","org.opencontainers.image.source":"https:\/\/github.com\/docker-library\/hello-world.git#3fb6ebca4163bf5b9cc496ac3e8f11cb1e754aee:amd64\/hello-world","org.opencontainers.image.url":"https:\/\/hub.docker.com\/_\/hello-world","org.opencontainers.image.version":"linux"},"digest":"sha256:e2fc4e5012d16e7fe466f5291c476431beaa1f9b90a5c2125b493ed28e2aba57","mediaType":"application\/vnd.oci.image.manifest.v1+json","platform":{"architecture":"amd64","os":"linux"},"size":861},{"annotations":{"vnd.docker.reference.digest":"sha256:e2fc4e5012d16e7fe466f5291c476431beaa1f9b90a5c2125b493ed28e2aba57","vnd.docker.reference.type":"attestation-manifest"},"digest":"sha256:579b3724a7b189f6dca599a46f16d801a43d5def185de0b7bcd5fb9d1e312c27","mediaType":"application\/vnd.oci.image.manifest.v1+json","platform":{"architecture":"unknown","os":"unknown"},"size":837},{"annotations":{"org.opencontainers.image.revision":"3fb6ebca4163bf5b9cc496ac3e8f11cb1e754aee","org.opencontainers.image.source":"https:\/\/github.com\/docker-library\/hello-world.git#3fb6ebca4163bf5b9cc496ac3e8f11cb1e754aee:arm32v5\/hello-world","org.opencontainers.image.url":"https:\/\/hub.docker.com\/_\/hello-world","org.opencontainers.image.version":"linux"},"digest":"sha256:c2d891e5c2fb4c723efb72b064be3351189f62222bd3681ce7e57f2a1527362c","mediaType":"application\/vnd.oci.image.manifest.v1+json","platform":{"architecture":"arm","os":"linux","variant":"v5"},"size":863},{"annotations":{"vnd.docker.reference.digest":"sha256:c2d891e5c2fb4c723efb72b064be3351189f62222bd3681ce7e57f2a1527362c","vnd.docker.reference.type":"attestation-manifest"},"digest":"sha256:6901d6a88eee6e90f0baa62b020bb61c4f13194cbcd9bf568ab66e8cc3f940dd","mediaType":"application\/vnd.oci.image.manifest.v1+json","platform":{"architecture":"unknown","os":"unknown"},"size":566},{"annotations":{"org.opencontainers.image.revision":"3fb6ebca4163bf5b9cc496ac3e8f11cb1e754aee","org.opencontainers.image.source":"https:\/\/github.com\/docker-library\/hello-world.git#3fb6ebca4163bf5b9cc496ac3e8f11cb1e754aee:arm32v7\/hello-world","org.opencontainers.image.url":"https:\/\/hub.docker.com\/_\/hello-world","org.opencontainers.image.version":"linux"},"digest":"sha256:20aea1c63c90d5e117db787c9fe1a8cd0ad98bedb5fd711273ffe05c084ff18a","mediaType":"application\/vnd.oci.image.manifest.v1+json","platform":{"architecture":"arm","os":"linux","variant":"v7"},"size":863},{"annotations":{"vnd.docker.reference.digest":"sha256:20aea1c63c90d5e117db787c9fe1a8cd0ad98bedb5fd711273ffe05c084ff18a","vnd.docker.reference.type":"attestation-manifest"},"digest":"sha256:70304c314d8a61ba1b36518624bb00bfff8d4b6016153792042de43f0453ca61","mediaType":"application\/vnd.oci.image.manifest.v1+json","platform":{"architecture":"unknown","os":"unknown"},"size":837},{"annotations":{"org.opencontainers.image.revision":"3fb6ebca4163bf5b9cc496ac3e8f11cb1e754aee","org.opencontainers.image.source":"https:\/\/github.com\/docker-library\/hello-world.git#3fb6ebca4163bf5b9cc496ac3e8f11cb1e754aee:arm64v8\/hello-world","org.opencontainers.image.url":"https:\/\/hub.docker.com\/_\/hello-world","org.opencontainers.image.version":"linux"},"digest":"sha256:2d4e459f4ecb5329407ae3e47cbc107a2fbace221354ca75960af4c047b3cb13","mediaType":"application\/vnd.oci.image.manifest.v1+json","platform":{"architecture":"arm64","os":"linux","variant":"v8"},"size":863},{"annotations":{"vnd.docker.reference.digest":"sha256:2d4e459f4ecb5329407ae3e47cbc107a2fbace221354ca75960af4c047b3cb13","vnd.docker.reference.type":"attestation-manifest"},"digest":"sha256:1f11fbd1720fcae3e402fc3eecb7d57c67023d2d1e11becc99ad9c7fe97d65ca","mediaType":"application\/vnd.oci.image.manifest.v1+json","platform":{"architecture":"unknown","os":"unknown"},"size":837},{"annotations":{"org.opencontainers.image.revision":"3fb6ebca4163bf5b9cc496ac3e8f11cb1e754aee","org.opencontainers.image.source":"https:\/\/github.com\/docker-library\/hello-world.git#3fb6ebca4163bf5b9cc496ac3e8f11cb1e754aee:i386\/hello-world","org.opencontainers.image.url":"https:\/\/hub.docker.com\/_\/hello-world","org.opencontainers.image.version":"linux"},"digest":"sha256:dbbd3cf666311ad526fad9d1746177469268f32fd91b371df2ebd1c84eb22f23","mediaType":"application\/vnd.oci.image.manifest.v1+json","platform":{"architecture":"386","os":"linux"},"size":860},{"annotations":{"vnd.docker.reference.digest":"sha256:dbbd3cf666311ad526fad9d1746177469268f32fd91b371df2ebd1c84eb22f23","vnd.docker.reference.type":"attestation-manifest"},"digest":"sha256:18b1c92de36d42c75440c6fd6b25605cc91709d176faaccca8afe58b317bc33a","mediaType":"application\/vnd.oci.image.manifest.v1+json","platform":{"architecture":"unknown","os":"unknown"},"size":566},{"annotations":{"org.opencontainers.image.revision":"3fb6ebca4163bf5b9cc496ac3e8f11cb1e754aee","org.opencontainers.image.source":"https:\/\/github.com\/docker-library\/hello-world.git#3fb6ebca4163bf5b9cc496ac3e8f11cb1e754aee:mips64le\/hello-world","org.opencontainers.image.url":"https:\/\/hub.docker.com\/_\/hello-world","org.opencontainers.image.version":"linux"},"digest":"sha256:c19784034d46da48550487c5c44639f5f92d48be7b9baf4d67b5377a454d92af","mediaType":"application\/vnd.oci.image.manifest.v1+json","platform":{"architecture":"mips64le","os":"linux"},"size":864},{"annotations":{"vnd.docker.reference.digest":"sha256:c19784034d46da48550487c5c44639f5f92d48be7b9baf4d67b5377a454d92af","vnd.docker.reference.type":"attestation-manifest"},"digest":"sha256:951bcd144ddccd1ee902dc180b435faabaaa6a8747e70cbc893f2dca16badb94","mediaType":"application\/vnd.oci.image.manifest.v1+json","platform":{"architecture":"unknown","os":"unknown"},"size":566},{"annotations":{"org.opencontainers.image.revision":"3fb6ebca4163bf5b9cc496ac3e8f11cb1e754aee","org.opencontainers.image.source":"https:\/\/github.com\/docker-library\/hello-world.git#3fb6ebca4163bf5b9cc496ac3e8f11cb1e754aee:ppc64le\/hello-world","org.opencontainers.image.url":"https:\/\/hub.docker.com\/_\/hello-world","org.opencontainers.image.version":"linux"},"digest":"sha256:f0c95f1ebb50c9b0b3e3416fb9dd4d1d197386a076c464cceea3d1f94c321b8f","mediaType":"application\/vnd.oci.image.manifest.v1+json","platform":{"architecture":"ppc64le","os":"linux"},"size":863},{"annotations":{"vnd.docker.reference.digest":"sha256:f0c95f1ebb50c9b0b3e3416fb9dd4d1d197386a076c464cceea3d1f94c321b8f","vnd.docker.reference.type":"attestation-manifest"},"digest":"sha256:838d191bca398e46cddebc48e816da83b0389d4ed2d64f408d618521b8fd1a57","mediaType":"application\/vnd.oci.image.manifest.v1+json","platform":{"architecture":"unknown","os":"unknown"},"size":837},{"annotations":{"org.opencontainers.image.revision":"3fb6ebca4163bf5b9cc496ac3e8f11cb1e754aee","org.opencontainers.image.source":"https:\/\/github.com\/docker-library\/hello-world.git#3fb6ebca4163bf5b9cc496ac3e8f11cb1e754aee:riscv64\/hello-world","org.opencontainers.image.url":"https:\/\/hub.docker.com\/_\/hello-world","org.opencontainers.image.version":"linux"},"digest":"sha256:8d064a6fc27fd5e97fa8225994a1addd872396236367745bea30c92d6c032fa3","mediaType":"application\/vnd.oci.image.manifest.v1+json","platform":{"architecture":"riscv64","os":"linux"},"size":863},{"annotations":{"vnd.docker.reference.digest":"sha256:8d064a6fc27fd5e97fa8225994a1addd872396236367745bea30c92d6c032fa3","vnd.docker.reference.type":"attestation-manifest"},"digest":"sha256:48147407c4594e45b7c3f0be1019bb0f44d78d7f037ce63e0e3da75b256f849e","mediaType":"application\/vnd.oci.image.manifest.v1+json","platform":{"architecture":"unknown","os":"unknown"},"size":837},{"annotations":{"org.opencontainers.image.revision":"3fb6ebca4163bf5b9cc496ac3e8f11cb1e754aee","org.opencontainers.image.source":"https:\/\/github.com\/docker-library\/hello-world.git#3fb6ebca4163bf5b9cc496ac3e8f11cb1e754aee:s390x\/hello-world","org.opencontainers.image.url":"https:\/\/hub.docker.com\/_\/hello-world","org.opencontainers.image.version":"linux"},"digest":"sha256:65f4b0d1802589b418bb6774d85de3d1a11d5bd971ee73cb8569504d928bb5d9","mediaType":"application\/vnd.oci.image.manifest.v1+json","platform":{"architecture":"s390x","os":"linux"},"size":861},{"annotations":{"vnd.docker.reference.digest":"sha256:65f4b0d1802589b418bb6774d85de3d1a11d5bd971ee73cb8569504d928bb5d9","vnd.docker.reference.type":"attestation-manifest"},"digest":"sha256:50f420e8710676da03668e446f1f51097b745e3e2c9807b018e569d26d4f65f7","mediaType":"application\/vnd.oci.image.manifest.v1+json","platform":{"architecture":"unknown","os":"unknown"},"size":837},{"digest":"sha256:245fe15fbb8f72b1988e35debf9172dedde4ec794de307633c5fb38c96ded61a","mediaType":"application\/vnd.docker.distribution.manifest.v2+json","platform":{"architecture":"amd64","os":"windows","os.version":"10.0.20348.2322"},"size":946},{"digest":"sha256:088bdbea94d5c8fe3eb9f3cec836c3f7ea82923e7d0d3a4f1146ef0f860f5a93","mediaType":"application\/vnd.docker.distribution.manifest.v2+json","platform":{"architecture":"amd64","os":"windows","os.version":"10.0.17763.5458"},"size":946}],"mediaType":"application\/vnd.oci.image.index.v1+json","schemaVersion":2}`
	imageManifest []byte
	d2c9          = `{"architecture":"amd64","config":{"Env":["PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"],"Cmd":["/hello"],"WorkingDir":"/","ArgsEscaped":true,"OnBuild":null},"created":"2023-05-02T16:49:27Z","history":[{"created":"2023-05-02T16:49:27Z","created_by":"COPY hello / # buildkit","comment":"buildkit.dockerfile.v0"},{"created":"2023-05-02T16:49:27Z","created_by":"CMD [\"/hello\"]","comment":"buildkit.dockerfile.v0","empty_layer":true}],"os":"linux","rootfs":{"type":"layers","diff_ids":["sha256:ac28800ec8bb38d5c35b49d45a6ac4777544941199075dff8c4eb63e093aa81e"]}}`
	c1ec          []byte
)

// ManifestInfo hold some info that supports the unit tests
type ManifestInfo struct {
	Url                 string
	ImageManifestDigest string
}

// Server runs an OCI distribution server that only allows pulling and
// only serves docker.io/hello-world:latest. Built by running 'crane pull -v'
// and transcribing the log into the handler function along with the files
// in the 'testfiles' dir and the variable values above.
func Server() (*httptest.Server, ManifestInfo) {
	var err error
	testFiles := testFilesDir()
	imageManifest, err = os.ReadFile(filepath.Join(testFiles, "imageManifest"))
	if err != nil {
		panic(err)
	}
	c1ec, err = os.ReadFile(filepath.Join(testFiles, "c1ec31eb59444d78df06a974d155e597c894ab4cda84f08294145e845394988e.tar.gz"))
	if err != nil {
		panic(err)
	}
	gmtTimeLoc := time.FixedZone("GMT", 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// support docker.io/library/hello-world:latest and docker.io/hello-world:latest the same way
		p := strings.Replace(r.URL.Path, "/library/", "/", 1)
		if p == "/v2/" {
			w.WriteHeader(http.StatusOK)
		} else if p == "/v2/hello-world/manifests/latest" {
			w.Header().Set("Content-Length", "9125")
			w.Header().Set("Content-Type", "application/vnd.oci.image.index.v1+json")
			w.Header().Set("Date", time.Now().In(gmtTimeLoc).Format(http.TimeFormat))
			w.Header().Set("Docker-Content-Digest", "sha256:d000bc569937abbe195e20322a0bde6b2922d805332fd6d8a68b19f524b7d21d")
			w.Header().Set("Docker-Distribution-Api-Version", "registry/2.0")
			w.Write([]byte(manifestList))
		} else if p == "/v2/hello-world/manifests/sha256:e2fc4e5012d16e7fe466f5291c476431beaa1f9b90a5c2125b493ed28e2aba57" {
			w.Header().Add("Content-Length", "861")
			w.Header().Add("Content-Type", "application/vnd.oci.image.manifest.v1+json")
			w.Header().Add("Date", time.Now().In(gmtTimeLoc).Format(http.TimeFormat))
			w.Header().Add("Docker-Content-Digest", "sha256:e2fc4e5012d16e7fe466f5291c476431beaa1f9b90a5c2125b493ed28e2aba57")
			w.Header().Add("Docker-Distribution-Api-Version", "registry/2.0")
			w.Write([]byte(imageManifest))
		} else if p == "/v2/hello-world/blobs/sha256:d2c94e258dcb3c5ac2798d32e1249e42ef01cba4841c2234249495f87264ac5a" {
			w.Header().Add("Content-Length", "581")
			w.Header().Add("Content-Type", "application/octet-stream")
			w.Header().Add("Date", time.Now().In(gmtTimeLoc).Format(http.TimeFormat))
			w.Write([]byte(d2c9))
		} else if p == "/v2/hello-world/blobs/sha256:c1ec31eb59444d78df06a974d155e597c894ab4cda84f08294145e845394988e" {
			w.Header().Add("Content-Length", "2459")
			w.Header().Add("Content-Type", "application/octet-stream")
			w.Header().Add("Date", time.Now().In(gmtTimeLoc).Format(http.TimeFormat))
			w.Write([]byte(c1ec))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	return server, ManifestInfo{
		Url:                 strings.Replace(server.URL, "http://", "", 1),
		ImageManifestDigest: "sha256:e2fc4e5012d16e7fe466f5291c476431beaa1f9b90a5c2125b493ed28e2aba57",
	}
}

// testFilesDir finds the directory that this file is in becuase the
// mock registry server could be used from other test directories.
func testFilesDir() string {

	for d, _ := os.Getwd(); d != "/"; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return filepath.Join(d, "mock/testfiles")
		}
	}
	panic(errors.New("no go.mod?"))
}
