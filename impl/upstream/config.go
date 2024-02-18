package upstream

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/http"
	"os"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"gopkg.in/yaml.v2"
)

type authCfg struct {
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

type tlsCfg struct {
	Cert     string `yaml:"cert"`
	Key      string `yaml:"key"`
	CA       string `yaml:"ca"`
	Insecure bool   `yaml:"insecure_skip_verify"`
}

type cfgEntry struct {
	Name        string  `yaml:"name"`
	Description string  `yaml:"description"`
	Auth        authCfg `yaml:"auth"`
	Tls         tlsCfg  `yaml:"tls"`
	Opts        []remote.Option
}

var (
	config    map[string]cfgEntry = make(map[string]cfgEntry)
	mu        sync.Mutex
	emptyAuth = authCfg{User: "", Password: ""}
	emptyTls  = tlsCfg{Cert: "", Key: "", CA: "", Insecure: false}
)

// ConfigLoader loads the remote registry configuration from the file
// referenced by the 'configPath' arg. If the arg is the empty string
// then nothing is done and no remote registry configs are defined.
// The result of this will be that every remote registry will be
// accessed anonymously by the image puller.
// TODO start a goroutine to periodically reload the config file
// if it changes on disk. (Maybe hash it?)
func ConfigLoader(configPath string) error {
	log.Debugf("load remote registry configuration from %s", configPath)
	if configPath != "" {
		b, err := os.ReadFile(configPath)
		if err != nil {
			return err
		}
		return parseConfig(b)
	}
	return nil
}

// parseConfig parses the remote registry configuration in the passed
// 'configBytes' arg. which consists on some number of entries, each
// describing the auth and TLS configuration to access a remote registry.
// The results are parsed into the package-level 'config' map keyed by
// the remote name (e.g. 'quay.io').
func parseConfig(configBytes []byte) error {
	var entries []cfgEntry
	err := yaml.Unmarshal(configBytes, &entries)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		_, exists := config[entry.Name]
		if !exists {
			config[entry.Name] = entry
		}
	}
	return nil
}

// configFor looks for a configuration entry keyed by the passed 'registry' arg
// (e.g. 'index.docker.io') and returns an array of options built from the config
// to configure the Crane image puller for that remote registry. If no matching
// config is found, then an empty array is returned with an error. (The error means
// the caller requested a config that didn't exist.) An empty array can be also
// be returned without an error - which means that the registry existed in the
// configuration but it didn't supply any options that would be used to configure
// Crane. This would be the case for a registry with no auth and no TLS. Bottom
// line, the caller can always use the options returned by the function, but may
// wish to log or record the fact that a registry was provided for lookup that
// was not configured.
func configFor(registry string) ([]remote.Option, error) {
	mu.Lock()
	regCfg, exists := config[registry]
	mu.Unlock()
	if !exists {
		return []remote.Option{}, errors.New("no entry in configuration for registry: " + registry)
	}
	if regCfg.Opts != nil {
		// previously calculated
		return regCfg.Opts, nil
	}

	opts := []remote.Option{}

	if regCfg.Auth != emptyAuth {
		basic := &authn.Basic{Username: regCfg.Auth.User, Password: regCfg.Auth.Password}
		opts = append(opts, remote.WithAuth(basic))
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
				log.Errorf("unable to load CA for config entry %s from file: %s", registry, regCfg.Tls.CA)
			}
		}
		if regCfg.Tls.Cert != "" && regCfg.Tls.Key != "" {
			cert, err := tls.LoadX509KeyPair(regCfg.Tls.Cert, regCfg.Tls.Key)
			if err == nil {
				clientCerts = []tls.Certificate{cert}
			} else {
				log.Errorf("unable to load client cert and/or key for config entry %s from files: cert: %s, key: %s", registry, regCfg.Tls.Cert, regCfg.Tls.Key)
			}
		}
		transport := remote.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: regCfg.Tls.Insecure,
			RootCAs:            cp,
			Certificates:       clientCerts,
		}
		opts = append(opts, remote.WithTransport(transport))
	}
	regCfg.Opts = opts
	mu.Lock()
	config[registry] = regCfg
	mu.Unlock()
	return opts, nil
}
