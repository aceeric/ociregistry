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

	os.Args = []string{"bin/ociregistry", "--image-path", td, "--log-level", "info", "--config-file", afile, "serve", "--port", "22", "--os", "linux", "--arch", "amd64", "--preload-images", afile, "--pull-timeout", "123", "--air-gapped", "--hello-world", "--always-pull-latest", "--health", "9876", "--default-ns", "abc.io", "--host", "1.2.3.4"}
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
	case !fromCmdline.Host:
		t.Fail()
	}
}

// Test that the parser detects when defaults are overridden on the command line for the load command
func TestParseLoad(t *testing.T) {
	ClearParse()
	td, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fail()
	}
	defer os.RemoveAll(td)
	afile := filepath.Join(td, "foo")
	os.WriteFile(afile, []byte("foo"), 0755)

	os.Args = []string{"bin/ociregistry", "load", "--image-file", afile, "--resolve-ref", "registry.host/repo:tag", "--os", "linux", "--arch", "amd64", "--pull-timeout", "123"}
	fromCmdline, cfg, err := Parse()
	if err != nil {
		t.Fail()
	}
	if fromCmdline.Command != "load" {
		t.Fail()
	}
	switch {
	case !fromCmdline.ImageFile:
		t.Fail()
	case !fromCmdline.ResolveRef:
		t.Fail()
	case !fromCmdline.Os:
		t.Fail()
	case !fromCmdline.Arch:
		t.Fail()
	case !fromCmdline.PullTimeout:
		t.Fail()
	case cfg.ImageFile != afile:
		t.Fail()
	case cfg.ResolveRef != "registry.host/repo:tag":
		t.Fail()
	case cfg.Os != "linux":
		t.Fail()
	case cfg.Arch != "amd64":
		t.Fail()
	case cfg.PullTimeout != 123:
		t.Fail()
	}
}

// Test that --resolve-ref rejects a value that doesn't parse as a
// fully-qualified image ref
func TestParseLoadResolveRefValidation(t *testing.T) {
	ClearParse()
	os.Args = []string{"bin/ociregistry", "load", "--resolve-ref", "not-a-valid-ref"}
	if _, _, err := Parse(); err == nil {
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

	os.Args = []string{"bin/ociregistry", "prune", "--pattern", "frobozz", "--dry-run", "true"}
	fromCmdline, cfg, err := Parse()
	if err != nil || fromCmdline.Command != "prune" || !fromCmdline.PruneConfig || !cfg.PruneConfig.DryRun ||
		cfg.PruneConfig.Expr != "frobozz" || cfg.PruneConfig.Type != "pattern" {
		t.Fail()
	}

	// dry run is the default
	ClearParse()
	os.Args = []string{"bin/ociregistry", "prune", "--date", "2025-02-28T12:59:59"}
	fromCmdline, cfg, err = Parse()
	if err != nil || fromCmdline.Command != "prune" || !fromCmdline.PruneConfig || !cfg.PruneConfig.DryRun ||
		cfg.PruneConfig.Expr != "2025-02-28T12:59:59" || cfg.PruneConfig.Type != "date" {
		t.Fail()
	}
}

// Tests every form of the prune --dry-run flag: true unless given as false, with or without
// "=", and an error for anything that isn't true or false, including a missing value.
func TestParsePruneDryRun(t *testing.T) {
	tests := []struct {
		args    []string
		want    bool
		wantErr bool
	}{
		{[]string{"--pattern", "x"}, true, false},
		{[]string{"--pattern", "x", "--dry-run", "true"}, true, false},
		{[]string{"--pattern", "x", "--dry-run=true"}, true, false},
		{[]string{"--pattern", "x", "--dry-run", "false"}, false, false},
		{[]string{"--pattern", "x", "--dry-run=false"}, false, false},
		{[]string{"--dry-run", "false", "--pattern", "x"}, false, false},
		{[]string{"--pattern", "x", "--dry-run", "nope"}, true, true},
		{[]string{"--pattern", "x", "--dry-run"}, true, true},
		{[]string{"--dry-run", "--pattern", "x"}, true, true},
	}
	for _, tst := range tests {
		ClearParse()
		os.Args = append([]string{"bin/ociregistry", "prune"}, tst.args...)
		_, cfg, err := Parse()
		if tst.wantErr {
			if err == nil {
				t.Errorf("%v: expected an error", tst.args)
			}
			continue
		}
		if err != nil || cfg.PruneConfig.DryRun != tst.want {
			t.Errorf("%v: expected DryRun %t, got %t %v", tst.args, tst.want, cfg.PruneConfig.DryRun, err)
		}
	}
}

// Tests that list --pattern sets the list config (and only the list config), so it is merged
// over a config file's list settings.
func TestParseListPattern(t *testing.T) {
	ClearParse()
	os.Args = []string{"bin/ociregistry", "list", "--pattern", "cilium"}
	fromCmdline, cfg, err := Parse()
	if err != nil || fromCmdline.Command != "list" || !fromCmdline.ListConfig || cfg.ListConfig.Expr != "cilium" {
		t.Fatalf("expected the list pattern to be set: %v %+v %+v", err, fromCmdline, cfg.ListConfig)
	}
	if fromCmdline.PruneConfig || cfg.PruneConfig != (config.PruneConfig{}) {
		t.Fatalf("list --pattern must not touch the prune config: %+v", cfg.PruneConfig)
	}
}

// Tests that --no-cache is repeatable and that a comma inside a regex is not split.
func TestParseNoCache(t *testing.T) {
	ClearParse()
	os.Args = []string{"bin/ociregistry", "serve", "--no-cache", ":latest$", "--no-cache", ":pr-[0-9]{1,3}$"}
	fromCmdline, cfg, err := Parse()
	if err != nil || !fromCmdline.NoCache || !reflect.DeepEqual(cfg.NoCache, []string{":latest$", ":pr-[0-9]{1,3}$"}) {
		t.Fatalf("unexpected parse: %v %t %q", err, fromCmdline.NoCache, cfg.NoCache)
	}
	ClearParse()
	os.Args = []string{"bin/ociregistry", "serve", "--no-cache", "[unclosed"}
	if _, _, err := Parse(); err == nil {
		t.Fatal("expected an error for a regex that doesn't compile")
	}
}

var testCfg = `
---
imagePath: /foo/test
logLevel: test1
logFile: /foo/bar/baz.log
preloadImages: /foo/bar
imageFile: /bar/baz
resolveRef: registry.host/repo:tag
port: 8888
os: red
arch: yellow
pullTimeout: 123
alwaysPullLatest: true
airGapped: true
health: 9876
helloWorld: true
defaultNs: abc.io
host: 2.3.4.5
`

var expectConfig = config.Configuration{
	ImagePath:        "/foo/test",
	LogLevel:         "test1",
	LogFile:          "/foo/bar/baz.log",
	PreloadImages:    "/foo/bar",
	ImageFile:        "/bar/baz",
	ResolveRef:       "registry.host/repo:tag",
	Port:             8888,
	Os:               "red",
	Arch:             "yellow",
	PullTimeout:      123,
	AlwaysPullLatest: true,
	AirGapped:        true,
	Health:           9876,
	HelloWorld:       true,
	DefaultNs:        "abc.io",
	Host:             "2.3.4.5",
	// prune dry run defaults to true when the config file doesn't set it
	PruneConfig: config.PruneConfig{DryRun: true},
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
