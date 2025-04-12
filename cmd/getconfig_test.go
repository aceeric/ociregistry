package main

import (
	"ociregistry/impl/config"
	"os"
	"path/filepath"
	"testing"
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

func TestCmdlineOverridesConfig(t *testing.T) {
	td, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fail()
	}
	defer os.Remove(td)
	dummyFile := filepath.Join(td, "foo")
	os.WriteFile(dummyFile, []byte("foo"), 0755)
	cfgFile := filepath.Join(td, "testcfg.yaml")
	os.WriteFile(cfgFile, []byte(cfgYaml), 0700)
	os.Args = []string{"bin/server", "--image-path", td, "--log-level", "info", "--config-file", cfgFile, "serve", "--port", "22", "--os", "foobar", "--arch", "frobozz", "--preload-images", dummyFile, "--pull-timeout", "123", "--air-gapped", "--hello-world", "--always-pull-latest"}

	command, err := getCfg()
	if err != nil {
		t.Fail()
	}
	switch {
	case command != "serve":
		t.Fail()
	case config.NewConfig.LogLevel != "info":
		t.Fail()
	case config.NewConfig.ConfigFile != cfgFile:
		t.Fail()
	case config.NewConfig.ImagePath != td:
		t.Fail()
	case config.NewConfig.PreloadImages != dummyFile:
		t.Fail()
	case config.NewConfig.Port != 22:
		t.Fail()
	case config.NewConfig.Os != "foobar":
		t.Fail()
	case config.NewConfig.Arch != "frobozz":
		t.Fail()
	case config.NewConfig.PullTimeout != 123:
		t.Fail()
	case !config.NewConfig.AlwaysPullLatest:
		t.Fail()
	case !config.NewConfig.AirGapped:
		t.Fail()
	case !config.NewConfig.HelloWorld:
		t.Fail()
	case len(config.NewConfig.Registries) != 2:
		t.Fail()
	case config.NewConfig.PruneConfig.Duration != "30d":
		t.Fail()
	}
}
