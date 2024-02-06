package pullsync

import "testing"

var testConfigNoAuth = `
---
- name: localhost:5001
  description: No auth, TLS if required by the server using OS trust store
  auth: {}
  tls: {}`

func TestCranePullNoAuth(t *testing.T) {
	parseConfig([]byte(testConfigNoAuth))
	cranePull("localhost:5001/infoblox/dnstools:latest", "/tmp/deleteme.tar")
}

var testlocalBasic = `
---
- name: localhost:5001
  description:
  auth:
    user: ericace
    password: ericace`

func TestCranePullBasicAuthLocalReg(t *testing.T) {
	parseConfig([]byte(testlocalBasic))
	cranePull("localhost:5001/infoblox/dnstools:latest", "/tmp/deleteme.tar")
}

var testTlsNoAuthNoTlsConfig = `
---
- name: localhost:8443
  description: Nginx 8443, Docker 5001, No Auth, TLS, No TLS Config`

func TestCraneTlsNoAuthNoTlsConfig(t *testing.T) {
	parseConfig([]byte(testTlsNoAuthNoTlsConfig))
	err := cranePull("localhost:8443/hello-world:latest", "/tmp/deleteme.tar")
	if err == nil {
		t.Errorf("Expected TLS error because insecure not specified")
	}
}

var testTlsNoAuthTlsConfigInsecure = `
---
- name: localhost:8443
  description: Nginx 8443, Docker 5001, No Auth, TLS, Insecure
  tls:
    insecure_skip_verify: true`

func TestTlsNoAuthTlsConfigInsecure(t *testing.T) {
	parseConfig([]byte(testTlsNoAuthTlsConfigInsecure))
	err := cranePull("localhost:8443/hello-world:latest", "/tmp/deleteme.tar")
	if err != nil {
		t.Errorf(err.Error())
	}
}

var testTlsNoAuthTlsConfigSecure = `
---
- name: localhost:8443
  description: Nginx 8443, Docker 5001, No Auth, TLS, Insecure
  tls:
    ca: /home/eace/projects/ociregistry/test-tegistry-servers/no-auth-one-way-tls/certs/ca.crt`

func TestTlsNoAuthTlsConfigSecure(t *testing.T) {
	parseConfig([]byte(testTlsNoAuthTlsConfigSecure))
	err := cranePull("localhost:8443/hello-world:latest", "/tmp/deleteme.tar")
	if err != nil {
		t.Errorf(err.Error())
	}
}
