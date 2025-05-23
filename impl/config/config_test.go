package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/aceeric/imgpull/pkg/imgpull"
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
