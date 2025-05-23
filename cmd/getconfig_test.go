package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/aceeric/ociregistry/impl/cmdline"
	"github.com/aceeric/ociregistry/impl/config"
)

var cfgYaml = `
---
imagePath: /var/lib/ociregistry
logLevel: error
preloadImages: /foo/bar
port: 8080
os: linux
arch: amd64
pullTimeout: 60000
alwaysPullLatest: false
airGapped: false
helloWorld: false
registries:
  - name: registry.one
    description: A description
  - name: registry.two
    description: Another description
pruneConfig:
  enabled: false
  duration: 30d
  type: accessed
  frequency: 1d
  count: -1
  dryRun: false
`

// setup clears globals: configuration and the parsed command line
func setup() {
	config.Set(config.Configuration{})
	cmdline.ClearParse()
}

// Test that lower-level command line parse failures are returned from the config function
func TestFails(t *testing.T) {
	setup()
	os.Args = []string{"bin/ociregistry", "serve", "--no-such-arg"}
	if _, err := getCfg(); err == nil {
		t.Fail()
	}
}

// Test that the command line configuration is correctly merged into config from
// a file.
func TestCmdlineOverridesConfig(t *testing.T) {
	setup()
	td, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fail()
	}
	defer os.RemoveAll(td)
	dummyFile := filepath.Join(td, "foo")
	os.WriteFile(dummyFile, []byte("foo"), 0755)
	cfgFile := filepath.Join(td, "testcfg.yaml")
	os.WriteFile(cfgFile, []byte(cfgYaml), 0700)
	os.Args = []string{"bin/ociregistry", "--image-path", td, "--log-level", "info", "--config-file", cfgFile, "serve", "--port", "22", "--os", "foobar", "--arch", "frobozz", "--preload-images", dummyFile, "--pull-timeout", "123", "--air-gapped", "--hello-world", "--always-pull-latest"}

	command, err := getCfg()
	if err != nil {
		t.Fail()
	}
	switch {
	case command != "serve":
		t.Fail()
	case config.GetLogLevel() != "info":
		t.Fail()
	case config.GetConfigFile() != cfgFile:
		t.Fail()
	case config.GetImagePath() != td:
		t.Fail()
	case config.GetPreloadImages() != dummyFile:
		t.Fail()
	case config.GetPort() != 22:
		t.Fail()
	case config.GetOs() != "foobar":
		t.Fail()
	case config.GetArch() != "frobozz":
		t.Fail()
	case config.GetPullTimeout() != 123:
		t.Fail()
	case !config.GetAlwaysPullLatest():
		t.Fail()
	case !config.GetAirGapped():
		t.Fail()
	case !config.GetHelloWorld():
		t.Fail()
	case len(config.GetRegistries()) != 2:
		t.Fail()
	case config.GetPruneConfig().Duration != "30d":
		t.Fail()
	}
}

// Test that the command line configuration is correctly handled when no
// config file is specified
func TestCmdlineNoConfigFile(t *testing.T) {
	setup()
	td, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fail()
	}
	defer os.RemoveAll(td)
	dummyFile := filepath.Join(td, "foo")
	os.WriteFile(dummyFile, []byte("foo"), 0755)
	os.Args = []string{"bin/ociregistry", "--image-path", td, "--log-level", "info", "serve", "--port", "22", "--os", "foobar", "--arch", "frobozz", "--preload-images", dummyFile, "--pull-timeout", "123", "--air-gapped", "--hello-world", "--always-pull-latest"}

	command, err := getCfg()
	if err != nil {
		t.Fail()
	}
	switch {
	case command != "serve":
		t.Fail()
	case config.GetLogLevel() != "info":
		t.Fail()
	case config.GetConfigFile() != "":
		t.Fail()
	case config.GetImagePath() != td:
		t.Fail()
	case config.GetPreloadImages() != dummyFile:
		t.Fail()
	case config.GetPort() != 22:
		t.Fail()
	case config.GetOs() != "foobar":
		t.Fail()
	case config.GetArch() != "frobozz":
		t.Fail()
	case config.GetPullTimeout() != 123:
		t.Fail()
	case !config.GetAlwaysPullLatest():
		t.Fail()
	case !config.GetAirGapped():
		t.Fail()
	case !config.GetHelloWorld():
		t.Fail()
	case len(config.GetRegistries()) != 0:
		t.Fail()
	case config.GetPruneConfig() != config.PruneConfig{}:
		t.Fail()
	}
}

var pruneCfg = `
---
pruneConfig:
  enabled: true
  duration: 30d
  type: accessed
  frequency: 1d
  count: -1
  expr: testing-123
  dryRun: false
`

// Test that prune configuration is parsed correctly from the config file
func TestPruneCfgEnabled(t *testing.T) {
	setup()
	td, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fail()
	}
	defer os.RemoveAll(td)
	cfgFile := filepath.Join(td, "testcfg.yaml")
	os.WriteFile(cfgFile, []byte(pruneCfg), 0700)
	os.Args = []string{"bin/ociregistry", "--config-file", cfgFile, "serve"}

	_, err = getCfg()
	if err != nil {
		t.Fail()
	}
	parsedCfg := config.GetPruneConfig()
	expectCfg := config.PruneConfig{
		Enabled:  true,
		Duration: "30d",
		Type:     "accessed",
		Freq:     "1d",
		Count:    -1,
		Expr:     "testing-123",
		DryRun:   false,
	}
	if !reflect.DeepEqual(parsedCfg, expectCfg) {
		t.Fail()
	}
}
