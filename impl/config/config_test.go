package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/aceeric/imgpull/pkg/imgpull"
	log "github.com/sirupsen/logrus"
)

var cfg = `
---
- name: %s
  description: %s
  scheme: %s
  auth:
    user: %s
    password: %s
  tls:
    ca: %s
    cert: %s
    key: %s
    insecure_skip_verify: %s`

func init() {
	log.SetOutput(io.Discard)
}

func TestCfg(t *testing.T) {
	names := []string{"t1", "t2"}
	descriptions := []string{"t3", "t4"}
	users := []string{"t5", "t6"}
	passs := []string{"t7", "t8"}
	cas := []string{"t9", "t10"}
	certs := []string{"t11", "t12"}
	keys := []string{"t13", "t14"}
	schemes := []string{"t15", "t16"}
	insecures := []string{"true", "false"}

	f, err := os.CreateTemp("", "")
	if err != nil {
		t.Fail()
	}
	f.Close()
	defer os.Remove(f.Name())

	// reload configuration every second
	go ConfigLoader(f.Name(), 1)

	for i := 0; i <= 1; i++ {
		name := names[i]
		description := descriptions[i]
		scheme := schemes[i]
		user := users[i]
		pass := passs[i]
		ca := cas[i]
		cert := certs[i]
		key := keys[i]
		insecure := insecures[i]
		manifest := fmt.Sprintf(cfg, name, description, scheme, user, pass, ca, cert, key, insecure)

		// write the file and sleep 2 secs which is enough time for the reloader to
		// to reload since its on a 1-second cycle
		os.WriteFile(f.Name(), []byte(manifest), 0700)
		time.Sleep(time.Second * time.Duration(2))

		entry, err := configEntryFor(name)
		if err != nil {
			t.Fail()
		}
		insecureVal, _ := strconv.ParseBool(insecures[i])
		if entry.Description != descriptions[i] ||
			entry.Scheme != schemes[i] ||
			entry.Auth.User != users[i] ||
			entry.Auth.Password != passs[i] ||
			entry.Tls.CA != cas[i] ||
			entry.Tls.Cert != certs[i] ||
			entry.Tls.Key != keys[i] ||
			entry.Tls.Insecure != insecureVal {
			t.Fail()
		}
	}
}

// configEntryFor returns a configuration entry from the config map that
// matches the passed 'registry', or and empty config if no matching entry
// exists.
func configEntryFor(registry string) (RegistryConfig, error) {
	mu.Lock()
	regCfg, exists := config[registry]
	mu.Unlock()
	if !exists {
		return RegistryConfig{}, errors.New("no entry in configuration for registry: " + registry)
	}
	return regCfg, nil
}

var testCfg = `
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
	PreloadImages:    "/foo/bar",
	Port:             8080,
	Os:               "linux",
	Arch:             "amd64",
	PullTimeout:      60000,
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

func TestLoadConfigFile(t *testing.T) {
	td, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fail()
	}
	defer os.Remove(td)
	cfgFile := filepath.Join(td, "testcfg.yaml")
	os.WriteFile(cfgFile, []byte(testCfg), 0700)
	if Load(cfgFile) != nil {
		t.Fail()
	}
	if !reflect.DeepEqual(NewConfig, expectConfig) {
		t.Fail()
	}
}
