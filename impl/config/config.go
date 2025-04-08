package config

import (
	"crypto/md5"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/aceeric/imgpull/pkg/imgpull"
	log "github.com/sirupsen/logrus"

	"gopkg.in/yaml.v3"
)

// authCfg holds basic auth user/pass
type authCfg struct {
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

// tlsCfg holds TLS configuration
type tlsCfg struct {
	Cert     string `yaml:"cert"`
	Key      string `yaml:"key"`
	CA       string `yaml:"ca"`
	Insecure bool   `yaml:"insecure_skip_verify"`
}

// cfgEntry combines authCfg and tlsCfg
type cfgEntry struct {
	Name        string
	Description string
	Auth        authCfg
	Tls         tlsCfg
	Scheme      string
	Opts        imgpull.PullerOpts
}

var (
	config    map[string]cfgEntry = make(map[string]cfgEntry)
	mu        sync.Mutex
	emptyAuth = authCfg{User: "", Password: ""}
	emptyTls  = tlsCfg{Cert: "", Key: "", CA: "", Insecure: false}
	emptyOpts = imgpull.PullerOpts{}
)

// ConfigLoader loads remote registry configurations from the file referenced by the
// 'configPath' arg. If the arg is the empty string then nothing is done and no remote
// registry configs are defined. In that case, every remote registry will be accessed
// anonymously.
//
// The function loops forever checking the config file for changes every 'chkSeconds' seconds
// (unless chkSeconds is zero in which case it only runs once.) The config file can contain
// multiple entries (it is a yaml list.) A fully-populated single configuration entry looks like:
//
//	---
//	- name: localhost:5001
//	  description: An optional mnemonic that you deem useful
//	  scheme: https (the default, or, http)
//	  auth:
//	    user: foo
//	    password: bar
//	  tls:
//	    ca: /etc/certs/ca.crt
//	    cert: /etc/certs/server.crt
//	    key: /etc/certs/server.key
//	    insecure_skip_verify: true
//
// The only mandatory key is 'name'. Everything else is optional. So if 'auth' is omitted then
// there's no basic auth. If 'tls' is omitted then insecure is the default (because zero is
// false.) If scheme is omitted the default is 'https'.
func ConfigLoader(configPath string, chkSeconds int) {
	if configPath != "" {
		var lastHash [md5.Size]byte
		for {
			func() {
				_, err := os.Stat(configPath)
				if err != nil {
					if errors.Is(err, os.ErrNotExist) {
						log.Warnf("config file does not exist, ignoring: %s", configPath)
					} else {
						log.Errorf("unable to stat configuration file: %s", configPath)
					}
					return
				}
				contents, err := os.ReadFile(configPath)
				if err != nil {
					log.Errorf("error reading configuration from %s", configPath)
					return
				}
				hash := md5.Sum(contents)
				if hash != lastHash {
					start := time.Now()
					lastHash = hash
					log.Info("load remote registry configuration")
					newConfig, err := parseConfig(contents)
					if err != nil {
						log.Error("error parsing configuration")
						return
					}
					mu.Lock()
					config = newConfig
					mu.Unlock()
					log.Infof("loaded %d registry configurations from %s in %s", len(config), configPath, time.Since(start))
				}
			}()
			if chkSeconds <= 0 {
				break
			}
			time.Sleep(time.Second * time.Duration(chkSeconds))
		}
	}
}

// AddConfig supports unit testing by creating an upstream config from the passed bytes as if they
// had been read from a config file. It doesn't have any concurrency because it expects to be called
// by a unit test running in isolation.
func AddConfig(configBytes []byte) error {
	if newConfig, err := parseConfig(configBytes); err != nil {
		return err
	} else {
		config = newConfig
	}
	return nil
}

// parseConfig parses the remote registry yaml config in the passed 'configBytes' arg. which
// consists of some number of entries, each describing the auth and TLS configuration to access
// one remote registry. The results are parsed into a map of 'cfgEntry' structs and returned to
// the caller. The map key is the 'name' element of each configuration which must exactly match
// a remote registry URL with no scheme, e.g.: quay.io, or: our.private.registry.gov:6443, or
// 129.168.1.1:8080, or index.docker.io, etc.
func parseConfig(configBytes []byte) (map[string]cfgEntry, error) {
	var entries []cfgEntry
	err := yaml.Unmarshal(configBytes, &entries)
	if err != nil {
		return map[string]cfgEntry{}, err
	}
	config := make(map[string]cfgEntry)

	for _, entry := range entries {
		_, exists := config[entry.Name]
		if !exists {
			config[entry.Name] = entry
		}
	}
	return config, nil
}

// ConfigFor looks for a configuration entry keyed by the passed 'registry' arg (e.g.
// 'index.docker.io') and returns configuration options for that registry from the config.
// If no matching config is found, then a default configuration is returned specifying insecure
// https, and the runtime OS and architecture.
//
// Since the config might involve loading and calculating a tls.Config with certs, once the
// parsing is complete, the final config struis saved for reuse so it doesn't need to be
// re-parsed in the future. (Unless the config file changes, which is handled by the
// ConfigLoader func.)
func ConfigFor(registry string) (imgpull.PullerOpts, error) {
	// default options if no configuration
	opts := imgpull.PullerOpts{
		Scheme:   "https",
		OStype:   runtime.GOOS,
		ArchType: runtime.GOARCH,
	}

	mu.Lock()
	regCfg, exists := config[registry]
	mu.Unlock()

	if !exists {
		return opts, nil
	}

	if regCfg.Opts != emptyOpts {
		// already parsed
		return regCfg.Opts, nil
	}

	if regCfg.Scheme != "" {
		opts.Scheme = regCfg.Scheme
	}

	if regCfg.Auth != emptyAuth {
		opts.Username = regCfg.Auth.User
		opts.Password = regCfg.Auth.Password
	}

	if regCfg.Tls != emptyTls {
		var cp *x509.CertPool
		var clientCerts []tls.Certificate = []tls.Certificate{}
		if regCfg.Tls.CA != "" {
			cp = x509.NewCertPool()
			caCert, err := os.ReadFile(regCfg.Tls.CA)
			if err == nil {
				cp.AppendCertsFromPEM(caCert)
			} else {
				return opts, fmt.Errorf("unable to load CA for config entry %s from file: %s", registry, regCfg.Tls.CA)
			}
		}
		if regCfg.Tls.Cert != "" && regCfg.Tls.Key != "" {
			cert, err := tls.LoadX509KeyPair(regCfg.Tls.Cert, regCfg.Tls.Key)
			if err == nil {
				clientCerts = []tls.Certificate{cert}
			} else {
				return opts, fmt.Errorf("unable to load client cert and/or key for config entry %s from files: cert: %s, key: %s", registry, regCfg.Tls.Cert, regCfg.Tls.Key)
			}
		}
		opts.TlsCfg = &tls.Config{
			InsecureSkipVerify: regCfg.Tls.Insecure,
			RootCAs:            cp,
			Certificates:       clientCerts,
		}
	}
	regCfg.Opts = opts
	mu.Lock()
	config[registry] = regCfg
	mu.Unlock()
	return opts, nil
}
