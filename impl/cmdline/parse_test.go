package cmdline

import (
	"os"
	"path/filepath"
	"testing"
)

// Test that the parser detects when defaults are overridden on the command line for the serve command
func TestParseServe(t *testing.T) {
	td, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fail()
	}
	defer os.Remove(td)
	afile := filepath.Join(td, "foo")
	os.WriteFile(afile, []byte("foo"), 0755)

	os.Args = []string{"bin/ociregistry", "--image-path", td, "--log-level", "info", "--config-file", afile, "serve", "--port", "22", "--os", "linux", "--arch", "amd64", "--preload-images", afile, "--pull-timeout", "123", "--air-gapped", "--hello-world", "--always-pull-latest"}
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
	case !fromCmdline.AlwaysPullLatest:
		t.Fail()
	case !fromCmdline.AirGapped:
		t.Fail()
	case !fromCmdline.HelloWorld:
		t.Fail()
	}
}

// Test that the parser detects when defaults are overridden on the command line for the prune command
func TestParsePrune(t *testing.T) {
	td, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fail()
	}
	defer os.Remove(td)
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
