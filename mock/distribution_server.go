package mock

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	manifestList  []byte
	imageManifest []byte
	d2c9          []byte
	c1ec          []byte
	re            = regexp.MustCompile(`https://|http://`)
)

// MockParams supports different configurations for the mock OCI
// Distribution Server
type MockParams struct {
	Auth      AuthType
	Scheme    SchemeType
	TlsConfig *tls.Config
	CliAuth   tls.ClientAuthType
	DelayMs   int
}

// SchemeType specifies http or https
type SchemeType string

const (
	HTTP  SchemeType = "http"
	HTTPS SchemeType = "https"
)

type AuthType string

const (
	BASIC AuthType = "basic auth"
	NONE  AuthType = "no auth"
)

// fileToLoad has a test file to load and the pointer of the variable to load it in to.
type fileToLoad struct {
	fname string
	vname *[]byte
	strip bool
}

// NewMockParams returns a 'MockParams' instance from the passed args.
func NewMockParams(auth AuthType, scheme SchemeType) MockParams {
	return MockParams{
		Auth:   auth,
		Scheme: scheme,
	}
}

// Server simply calls ServerWithCallback with no callback function
func Server(params MockParams) (*httptest.Server, string) {
	return ServerWithCallback(params, nil)
}

// ServerWithCallback runs the mock OCI distribution server. It returns a ref to the server, and a
// server url (without the scheme). If a callback function is passed, it is called on each
// method invocation of the server's 'HandlerFunc'.
func ServerWithCallback(params MockParams, callback *func(string)) (*httptest.Server, string) {
	var err error
	testFilesDir := getTestFilesDir()

	filesToLoad := []fileToLoad{
		{fname: "manifestList.json", vname: &manifestList, strip: true},
		{fname: "imageManifest.json", vname: &imageManifest, strip: false},
		{fname: "d2c9.json", vname: &d2c9, strip: false},
		{fname: "c1ec31eb59444d78df06a974d155e597c894ab4cda84f08294145e845394988e.tar.gz", vname: &c1ec, strip: false},
	}

	m1 := regexp.MustCompile(`[\r\n\t ]{1}`)
	for _, testFile := range filesToLoad {
		*testFile.vname, err = os.ReadFile(filepath.Join(testFilesDir, testFile.fname))
		if err != nil {
			panic(err)
		}
		if testFile.strip {
			*testFile.vname = []byte(m1.ReplaceAllString(string(*testFile.vname), ""))
		}
	}

	gmtTimeLoc := time.FixedZone("GMT", 0)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if callback != nil {
			(*callback)(r.URL.Path)
		}
		// delayMs supports simulating slow links or large images
		if params.DelayMs != 0 {
			time.Sleep(time.Duration(params.DelayMs) * time.Millisecond)
		}
		p := strings.Replace(r.URL.Path, "/library/", "/", 1)
		if p == "/v2/" {
			if params.Auth == NONE {
				w.WriteHeader(http.StatusOK)
			} else {
				authUrl := `Bearer realm="%s://%s/v2/auth",service="registry.docker.io"`
				w.Header().Set("Content-Length", "87")
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Date", time.Now().In(gmtTimeLoc).Format(http.TimeFormat))
				w.Header().Set("Docker-Distribution-Api-Version", "registry/2.0")
				w.Header().Set("Www-Authenticate", fmt.Sprintf(authUrl, params.Scheme, r.Host))
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"errors":[{"code":"UNAUTHORIZED","message":"authentication required","detail":null}]}`))
			}
		} else if p == "/v2/auth" {
			if params.Auth == BASIC {
				if r.Header.Get("Authorization") == "" {
					w.WriteHeader(http.StatusUnauthorized)
				}
			}
			w.Header().Set("Content-Length", "19")
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"token":"FROBOZZ"}`))
		} else if p == "/v2/hello-world/manifests/latest" {
			w.Header().Set("Content-Length", strconv.Itoa(len(manifestList))) // 9125
			w.Header().Set("Content-Type", "application/vnd.oci.image.index.v1+json")
			w.Header().Set("Date", time.Now().In(gmtTimeLoc).Format(http.TimeFormat))
			w.Header().Set("Docker-Content-Digest", "sha256:e4ccfd825622441dcee5123f9d4a48b2eb8787d858de346106a83f0c745cc255")
			w.Header().Set("Docker-Distribution-Api-Version", "registry/2.0")
			w.Write([]byte(manifestList))
		} else if p == "/v2/hello-world/manifests/sha256:e2fc4e5012d16e7fe466f5291c476431beaa1f9b90a5c2125b493ed28e2aba57" {
			w.Header().Add("Content-Length", strconv.Itoa(len(imageManifest)))
			w.Header().Add("Content-Type", "application/vnd.oci.image.manifest.v1+json")
			w.Header().Add("Date", time.Now().In(gmtTimeLoc).Format(http.TimeFormat))
			w.Header().Add("Docker-Content-Digest", "sha256:e2fc4e5012d16e7fe466f5291c476431beaa1f9b90a5c2125b493ed28e2aba57")
			w.Header().Add("Docker-Distribution-Api-Version", "registry/2.0")
			w.Write([]byte(imageManifest))
		} else if p == "/v2/hello-world/blobs/sha256:d2c94e258dcb3c5ac2798d32e1249e42ef01cba4841c2234249495f87264ac5a" {
			w.Header().Add("Content-Length", strconv.Itoa(len(d2c9)))
			w.Header().Add("Content-Type", "application/octet-stream")
			w.Header().Add("Date", time.Now().In(gmtTimeLoc).Format(http.TimeFormat))
			w.Write([]byte(d2c9))
		} else if p == "/v2/hello-world/blobs/sha256:c1ec31eb59444d78df06a974d155e597c894ab4cda84f08294145e845394988e" {
			w.Header().Add("Content-Length", strconv.Itoa(len(c1ec)))
			w.Header().Add("Content-Type", "application/octet-stream")
			w.Header().Add("Date", time.Now().In(gmtTimeLoc).Format(http.TimeFormat))
			w.Write([]byte(c1ec))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	if params.Scheme == HTTPS {
		server.TLS = params.TlsConfig
		server.TLS.ClientAuth = params.CliAuth
		server.StartTLS()
	} else {
		server.Start()
	}
	return server, re.ReplaceAllString(server.URL, "")
}

// getTestFilesDir finds the directory that this file is in becuase the
// mock registry server could be used from other test directories but it
// needs files in this directory.
func getTestFilesDir() string {
	for d, _ := os.Getwd(); d != "/"; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return filepath.Join(d, "mock/testfiles")
		}
	}
	panic(errors.New("no go.mod?"))
}
