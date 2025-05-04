package main

import (
	"os"
	"testing"

	"github.com/aceeric/ociregistry/impl/config"
)

// Test that lower-level command line parse failures are returned from the config function
func TestHelloWorld(t *testing.T) {
	setup()
	dir, err := helloWorldMode()
	if err != nil {
		t.Fail()
	}
	defer os.RemoveAll(dir)
	if !config.GetAirGapped() {
		t.Fail()
	} else if config.GetPreloadImages() != "" {
		t.Fail()
	} else if config.GetImagePath() != dir {
		t.Fail()
	}
}
