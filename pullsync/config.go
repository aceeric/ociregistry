package pullsync

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"ociregistry/globals"
	"os"
	"sync"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
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
}

var (
	config    map[string]cfgEntry = make(map[string]cfgEntry)
	mu        sync.Mutex
	emptyAuth = authCfg{User: "", Password: ""}
	emptyTls  = tlsCfg{Cert: "", Key: "", CA: "", Insecure: false}
)

// TODO start a goroutine to periodically reload
func ConfigLoader(configPath string) error {
	b, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	return parseConfig(b)
}

// parseConfig parses the configuration in the passed 'configBytes' arg
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
// (e.g. 'index.docker.io') and returns an array of options to configure the Crane
// image puller that are built from the config. If no matching config is found,
// then an empty array is returned with an error. An empty array can be returned
// without an error - which means that the registry existed in the configuration
// but it didn't supply any options that would be used to configure Crane. Bottom line,
// the caller can always use the options returned by the function, but may wish to
// log or record the fact that a registry was provided for lookup that was not
// configured.
func configFor(registry string) ([]crane.Option, error) {
	opts := []crane.Option{}
	mu.Lock()
	regCfg, exists := config[registry]
	mu.Unlock()
	if !exists {
		return opts, errors.New("no entry in configuration for registry: " + registry)
	}
	if regCfg.Auth != emptyAuth {
		basic := &authn.Basic{Username: regCfg.Auth.User, Password: regCfg.Auth.Password}
		ba := func(o *crane.Options) {
			o.Remote = append(o.Remote, remote.WithAuth(basic))
		}
		opts = append(opts, ba)
	}
	if regCfg.Tls != emptyTls {
		tls := func(o *crane.Options) {
			var cp *x509.CertPool
			if regCfg.Tls.CA != "" {
				cp = x509.NewCertPool()
				caCert, err := os.ReadFile(regCfg.Tls.CA)
				if err == nil {
					cp.AppendCertsFromPEM(caCert)
				} else {
					globals.Logger().Error(fmt.Sprintf("unable to load CA cert for config entry %s from file: %s", registry, regCfg.Tls.CA))
				}
			}
			transport := remote.DefaultTransport.(*http.Transport).Clone()
			transport.TLSClientConfig = &tls.Config{
				// TODO cert, key, ca
				InsecureSkipVerify: regCfg.Tls.Insecure,
				RootCAs:            cp,
			}
			o.Transport = transport
			// since o.Remote is what is passed to crane 'remote.Get(ref, o.Remote...)' it
			// has to be appended here...
			o.Remote = append(o.Remote, remote.WithTransport(transport))
		}
		opts = append(opts, tls)
	}
	return opts, nil
}
