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
