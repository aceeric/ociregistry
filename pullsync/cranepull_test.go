package pullsync

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestOptParse(t *testing.T) {
	var cfg = `
    ---
    - name: foobar
      description: frobozz
      auth:
        user: foobar
        password: frobozz`

	parseConfig([]byte(sl(cfg)))
	_, err := configFor("foobar")
	if err != nil {
		t.Errorf(err.Error())
	}
	// just have to visually confirm entry was re-used
	configFor("foobar")
}

func TestNoAuthHttp(t *testing.T) {
	var cfg = `
    ---
    - name: localhost:5000
      description: No auth (anonymous), HTTP
      auth: {}
      tls: {}`

	parseConfig([]byte(sl(cfg)))
	os.Remove("/tmp/deleteme.tar")
	err := cranePull("localhost:5000/hello-world:latest", "/tmp/deleteme.tar")
	if err != nil {
		t.Errorf(err.Error())
	}
}

func TestBasicAuthHttp(t *testing.T) {
	var cfg = `
    ---
    - name: localhost:5001
      description: Basic auth, HTTP
      auth:
        user: ericace
        password: ericace`

	parseConfig([]byte(sl(cfg)))
	os.Remove("/tmp/deleteme.tar")
	err := cranePull("localhost:5001/hello-world:latest", "/tmp/deleteme.tar")
	if err != nil {
		t.Errorf(err.Error())
	}
}

func TestNoAuthHttps1WayNoTlsConfig(t *testing.T) {
	var cfg = `
    ---
    - name: localhost:8443
      description: No auth, HTTPS, no TLS config`

	parseConfig([]byte(sl(cfg)))
	err := cranePull("localhost:8443/hello-world:latest", "/tmp/deleteme.tar")
	if err == nil {
		t.Errorf("Expected TLS error because insecure not specified and server cert not in trust store")
	}
}

func TestNoAuthHttps1WayInsecure(t *testing.T) {
	var cfg = `
    ---
    - name: localhost:8443
      description: No auth, HTTPS, 1-way, insecure
      tls:
        insecure_skip_verify: true`

	parseConfig([]byte(sl(cfg)))
	os.Remove("/tmp/deleteme.tar")
	err := cranePull("localhost:8443/hello-world:latest", "/tmp/deleteme.tar")
	if err != nil {
		t.Errorf(err.Error())
	}
}

func TestNoAuthHttps1WaySecure(t *testing.T) {
	var cfg = `
    ---
    - name: localhost:8443
      description: No auth, HTTPS, 1-way, verify server cert
      tls:
        ca: /home/eace/projects/ociregistry/test-tegistry-servers/no-auth-one-way-tls/certs/ca.crt`

	parseConfig([]byte(sl(cfg)))
	os.Remove("/tmp/deleteme.tar")
	err := cranePull("localhost:8443/hello-world:latest", "/tmp/deleteme.tar")
	if err != nil {
		t.Errorf(err.Error())
	}
}

func TestNoAuthHttps2Way(t *testing.T) {
	var cfg = `
    ---
    - name: localhost:8444
      description: No auth, HTTPS, 2-way
      tls:
        ca: /home/eace/projects/ociregistry/test-tegistry-servers/no-auth-one-way-tls/certs/ca.crt
        cert: /home/eace/projects/ociregistry/test-tegistry-servers/no-auth-one-way-tls/certs/localhost.crt
        key: /home/eace/projects/ociregistry/test-tegistry-servers/no-auth-one-way-tls/certs/localhost.key`

	parseConfig([]byte(sl(cfg)))
	os.Remove("/tmp/deleteme.tar")
	err := cranePull("localhost:8444/hello-world:latest", "/tmp/deleteme.tar")
	if err != nil {
		t.Errorf(err.Error())
	}
}

// sl Strips Leading spaces from each line so the inlined config yaml
// can be enclosed within each testing function and the indentation doesn't
// cause yaml parse errors.
func sl(s string) string {
	var ret = ""
	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		ret = fmt.Sprintf("%s%s\n", ret, strings.TrimPrefix(scanner.Text(), "    "))
	}
	return ret
}
