package cmdline

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/aceeric/ociregistry/impl/config"
)

// Test that the parser detects when defaults are overridden on the command line for the serve command
func TestParseServe(t *testing.T) {
	ClearParse()
	td, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fail()
	}
	defer os.RemoveAll(td)
	afile := filepath.Join(td, "foo")
	os.WriteFile(afile, []byte("foo"), 0755)

	os.Args = []string{"bin/ociregistry", "--image-path", td, "--log-level", "info", "--config-file", afile, "serve", "--port", "22", "--os", "linux", "--arch", "amd64", "--preload-images", afile, "--pull-timeout", "123", "--air-gapped", "--hello-world", "--always-pull-latest", "--health", "9876", "--default-ns", "abc.io"}
	fromCmdline, _, err := Parse()
	if err != nil {
		t.Fail()
	}
	if fromCmdline.Command != "serve" {
		t.Fail()
	}
	switch {
	case !fromCmdline.LogLevel:
		t.Fail()
	case !fromCmdline.ConfigFile:
		t.Fail()
	case !fromCmdline.ImagePath:
		t.Fail()
	case !fromCmdline.PreloadImages:
		t.Fail()
	case !fromCmdline.Port:
		t.Fail()
	case !fromCmdline.Os:
		t.Fail()
	case !fromCmdline.Arch:
		t.Fail()
	case !fromCmdline.PullTimeout:
		t.Fail()
	case !fromCmdline.Health:
		t.Fail()
	case !fromCmdline.AlwaysPullLatest:
		t.Fail()
	case !fromCmdline.AirGapped:
		t.Fail()
	case !fromCmdline.HelloWorld:
		t.Fail()
	case !fromCmdline.DefaultNs:
		t.Fail()
	}
}

// Test that the parser detects when defaults are overridden on the command line for the prune command
func TestParsePrune(t *testing.T) {
	ClearParse()
	td, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fail()
	}
	defer os.RemoveAll(td)
	afile := filepath.Join(td, "foo")
	os.WriteFile(afile, []byte("foo"), 0755)

	os.Args = []string{"bin/ociregistry", "prune", "--pattern", "frobozz", "--dry-run"}
	fromCmdline, cfg, err := Parse()
	if err != nil || fromCmdline.Command != "prune" || !fromCmdline.PruneConfig || !cfg.PruneConfig.DryRun ||
		cfg.PruneConfig.Expr != "frobozz" || cfg.PruneConfig.Type != "pattern" {
		t.Fail()
	}

	os.Args = []string{"bin/ociregistry", "prune", "--date", "2025-02-28T12:59:59"}
	fromCmdline, cfg, err = Parse()
	if err != nil || fromCmdline.Command != "prune" || !fromCmdline.PruneConfig || cfg.PruneConfig.DryRun ||
		cfg.PruneConfig.Expr != "2025-02-28T12:59:59" || cfg.PruneConfig.Type != "date" {
		t.Fail()
	}
}

var testCfg = `
---
imagePath: /foo/test
logLevel: test1
logFile: /foo/bar/baz.log
preloadImages: /foo/bar
imageFile: /bar/baz
port: 8888
os: red
arch: yellow
pullTimeout: 123
alwaysPullLatest: true
airGapped: true
health: 9876
helloWorld: true
defaultNs: abc.io
`

var expectConfig = config.Configuration{
	ImagePath:        "/foo/test",
	LogLevel:         "test1",
	LogFile:          "/foo/bar/baz.log",
	PreloadImages:    "/foo/bar",
	ImageFile:        "/bar/baz",
	Port:             8888,
	Os:               "red",
	Arch:             "yellow",
	PullTimeout:      123,
	AlwaysPullLatest: true,
	AirGapped:        true,
	Health:           9876,
	HelloWorld:       true,
	DefaultNs:        "abc.io",
}

// Test that a command line with nothing specified does not overwrite any part of
// existing config.
func TestMergeConfig(t *testing.T) {
	ClearParse()
	if err := config.SetConfigFromStr([]byte(testCfg)); err != nil {
		t.Fail()
	}
	parsedCfg := config.Get()
	if !reflect.DeepEqual(parsedCfg, expectConfig) {
		t.Fail()
	}
	os.Args = []string{"bin/ociregistry", "serve"}
	fromCmdline, cfg, err := Parse()
	if err != nil {
		t.Fail()
	}
	config.Merge(fromCmdline, cfg)
	newCfg := config.Get()
	if !reflect.DeepEqual(newCfg, expectConfig) {
		t.Fail()
	}
}
