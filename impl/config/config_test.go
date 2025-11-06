package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/aceeric/imgpull/pkg/imgpull"
	"github.com/aceeric/ociregistry/mock"
)

var testCfg = `
---
imagePath: /var/lib/ociregistry
logLevel: error
logFile: /foo/bar/baz.log
preloadImages: /foo/bar
imageFile: /bar/baz
port: 8080
os: linux
arch: amd64
pullTimeout: 60000
health: 6543
alwaysPullLatest: false
airGapped: false
helloWorld: false
registries:
  - name: localhost:8080
    description: server running on the desktop
    scheme: http
pruneConfig:
  enabled: false
  duration: 30d
  type: accessed
  frequency: 1d
  count: -1
  dryRun: false
`

var expectConfig = Configuration{
	ImagePath:        "/var/lib/ociregistry",
	LogLevel:         "error",
	LogFile:          "/foo/bar/baz.log",
	PreloadImages:    "/foo/bar",
	ImageFile:        "/bar/baz",
	Port:             8080,
	Os:               "linux",
	Arch:             "amd64",
	PullTimeout:      60000,
	Health:           6543,
	AlwaysPullLatest: false,
	AirGapped:        false,
	HelloWorld:       false,
	Registries: []RegistryConfig{
		{
			Name:        "localhost:8080",
			Description: "server running on the desktop",
			Scheme:      "http",
			Opts:        imgpull.PullerOpts{},
		},
	},
	PruneConfig: PruneConfig{
		Enabled:  false,
		Duration: "30d",
		Type:     "accessed",
		Freq:     "1d",
		Count:    -1,
		DryRun:   false,
	},
}

// Test loading and parsing a configuration file
func TestLoadConfigFile(t *testing.T) {
	td, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fail()
	}
	defer os.RemoveAll(td)
	cfgFile := filepath.Join(td, "testcfg.yaml")
	os.WriteFile(cfgFile, []byte(testCfg), 0700)
	if Load(cfgFile) != nil {
		t.Fail()
	}
	if !reflect.DeepEqual(config, expectConfig) {
		t.Fail()
	}
}

var testCfgTls = `
---
registries:
  - name: %[1]s
    description: tls config
    scheme: https
    tls:
      ca:   %[2]s/ca.pem
      cert: %[2]s/cert.pem
      key:  %[2]s/key.pem
      insecureSkipVerify: true
`

// Test that a registry with TLS configuration is parsed
func TestTlsConfig(t *testing.T) {
	td, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fail()
	}
	defer os.RemoveAll(td)

	certSetup, err := mock.NewCertSetup()
	if err != nil {
		t.FailNow()
	}
	certSetup.ServerCertToFile(td, "cert.pem")
	certSetup.ServerCertPrivKeyToFile(td, "key.pem")
	certSetup.CaToFile(td, "ca.pem")

	registry := "tls.io"
	cfgFile := filepath.Join(td, "testcfg.yaml")
	cfgWithPath := fmt.Sprintf(testCfgTls, registry, td)
	os.WriteFile(cfgFile, []byte(cfgWithPath), 0700)
	if Load(cfgFile) != nil {
		t.Fail()
	}
	opts, err := ConfigFor(registry)
	if err != nil {
		t.Fail()
	}
	if opts.Scheme != "https" || opts.TlsCfg == nil || !opts.TlsCfg.InsecureSkipVerify {
		t.FailNow()
	}
	if !reflect.DeepEqual(opts.TlsCfg.Certificates[0].Leaf.Subject, certSetup.ServerCert.Leaf.Subject) {
		t.FailNow()
	}
}

var testEnvPass = `
---
registries:
  - name: testme
    auth:
      user: testme
      passwordFromEnv: TESTME
`

// Test getting a password from an environment variable
func TestPassFromEnv(t *testing.T) {
	expPass := "1234567890"
	td, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fail()
	}
	defer os.RemoveAll(td)
	cfgFile := filepath.Join(td, "testcfg.yaml")
	os.WriteFile(cfgFile, []byte(testEnvPass), 0700)
	if Load(cfgFile) != nil {
		t.Fail()
	}
	t.Setenv("TESTME", expPass)
	opts, err := ConfigFor("testme")
	if err != nil {
		t.Fail()
	}
	if opts.Password != expPass {
		t.Fail()
	}
}