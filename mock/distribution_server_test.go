package mock

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"ociregistry/impl/helpers"
	"testing"
)

// Sanity check the mock OCI distribution server
func TestServer(t *testing.T) {
	server, url := Server(NewMockParams(NONE, HTTP))
	defer server.Close()

	URLs := make([]string, 4)
	URLs[0] = fmt.Sprintf("http://%s/v2/hello-world/manifests/latest", url)
	URLs[1] = fmt.Sprintf("http://%s/v2/hello-world/manifests/sha256:e2fc4e5012d16e7fe466f5291c476431beaa1f9b90a5c2125b493ed28e2aba57", url)
	URLs[2] = fmt.Sprintf("http://%s/v2/hello-world/blobs/sha256:d2c94e258dcb3c5ac2798d32e1249e42ef01cba4841c2234249495f87264ac5a", url)
	URLs[3] = fmt.Sprintf("http://%s/v2/hello-world/blobs/sha256:c1ec31eb59444d78df06a974d155e597c894ab4cda84f08294145e845394988e", url)

	for _, url := range URLs {
		resp, err := http.Get(url)
		if err != nil && resp.StatusCode != 200 {
			t.Fail()
		}
		if resp.Header["Docker-Content-Digest"] != nil {
			expectedHash := helpers.GetDigestFrom(resp.Header["Docker-Content-Digest"][0])
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fail()
			}
			hasher := sha256.New()
			hasher.Write(body)
			computedHash := fmt.Sprintf("%x", hasher.Sum(nil))
			if computedHash != expectedHash {
				t.Fail()
			}
		}
	}
}
