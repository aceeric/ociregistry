package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"runtime"

	"github.com/aceeric/imgpull/pkg/imgpull"

	"gopkg.in/yaml.v3"
)

// authCfg holds basic auth user/pass for registry access
type authCfg struct {
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

// tlsCfg holds TLS configuration for upstream registry access
type tlsCfg struct {
	Cert               string `yaml:"cert"`
	Key                string `yaml:"key"`
	CA                 string `yaml:"ca"`
	InsecureSkipVerify bool   `yaml:"insecureSkipVerify"`
}

// ServerTlsCfg holds TLS configuration for TLS with (downstream) clients,
// i.e. containerd. Valid values for ClientAuth: "none", and "verify"
type ServerTlsCfg struct {
	Cert       string `yaml:"cert"`
	Key        string `yaml:"key"`
	CA         string `yaml:"ca"`
	ClientAuth string `yaml:"clientAuth"`
}

// RegistryConfig combines authCfg and tlsCfg and configures the pull client
// for access to one upstream registry
type RegistryConfig struct {
	Name        string             `yaml:"name"`
	Description string             `yaml:"description"`
	Auth        authCfg            `yaml:"auth"`
	Tls         tlsCfg             `yaml:"tls"`
	Scheme      string             `yaml:"scheme"`
	Opts        imgpull.PullerOpts `yaml:"opts,omitempty"`
}

// PruneConfig configures the prune behavior
type PruneConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Duration string `yaml:"duration"`
	Type     string `yaml:"type"`
	Freq     string `yaml:"frequency"`
	Count    int    `yaml:"count"`
	Expr     string `yaml:"expr"`
	DryRun   bool   `yaml:"dryrun"`
}

// ListConfig configures the list sub-command
type ListConfig struct {
	Header bool   `yaml:"header"`
	Expr   string `yaml:"expr"`
}

// Configuration represents the totality of configuration knobs and dials for the server.
type Configuration struct {
	LogLevel         string           `yaml:"logLevel"`
	LogFile          string           `yaml:"logFile"`
	ConfigFile       string           `yaml:"configFile"`
	ImagePath        string           `yaml:"imagePath"`
	PreloadImages    string           `yaml:"preloadImages"`
	ImageFile        string           `yaml:"imageFile"`
	Port             int64            `yaml:"port"`
	Os               string           `yaml:"os"`
	Arch             string           `yaml:"arch"`
	PullTimeout      int64            `yaml:"pullTimeout"`
	Health           int64            `yaml:"health"`
	AlwaysPullLatest bool             `yaml:"alwaysPullLatest"`
	AirGapped        bool             `yaml:"airGapped"`
	HelloWorld       bool             `yaml:"helloWorld"`
	Registries       []RegistryConfig `yaml:"registries"`
	PruneConfig      PruneConfig      `yaml:"pruneConfig"`
	ListConfig       ListConfig       `yaml:"listConfig"`
	ServerTlsCfg     ServerTlsCfg     `yaml:"serverTlsConfig"`
}

// FromCmdLine has a flag for every command-line option. The parsing code
// sets the flag to true if the option was explicitly provided on the command
// line by the user. This supports merging command line config into
// file-based config.
type FromCmdLine struct {
	Command          string
	LogLevel         bool
	LogFile          bool
	ConfigFile       bool
	ImagePath        bool
	PreloadImages    bool
	ImageFile        bool
	Port             bool
	Os               bool
	Arch             bool
	PullTimeout      bool
	Health           bool
	AlwaysPullLatest bool
	AirGapped        bool
	HelloWorld       bool
	PruneConfig      bool
	ListConfig       bool
}

var (
	// config is the gloal configuration, accessed through getters and setters
	// below
	config    Configuration
	emptyAuth = authCfg{User: "", Password: ""}
	emptyTls  = tlsCfg{Cert: "", Key: "", CA: "", InsecureSkipVerify: false}
	emptyOpts = imgpull.PullerOpts{}
)

// getters and setters for when I re-implement hot reload

func GetLogLevel() string {
	return config.LogLevel
}

func GetLogFile() string {
	return config.LogFile
}

func GetConfigFile() string {
	return config.ConfigFile
}

func GetImagePath() string {
	return config.ImagePath
}

func SetImagePath(newVal string) {
	config.ImagePath = newVal
}

func GetPreloadImages() string {
	return config.PreloadImages
}

func SetPreloadImages(newVal string) {
	config.PreloadImages = newVal
}

func GetImageFile() string {
	return config.ImageFile
}

func GetPort() int64 {
	return config.Port
}

func GetOs() string {
	return config.Os
}

func GetArch() string {
	return config.Arch
}

func GetPullTimeout() int64 {
	return config.PullTimeout
}

func GetHealth() int64 {
	return config.Health
}

func GetAlwaysPullLatest() bool {
	return config.AlwaysPullLatest
}

func GetAirGapped() bool {
	return config.AirGapped
}

func SetAirGapped(newVal bool) {
	config.AirGapped = newVal
}

func GetHelloWorld() bool {
	return config.HelloWorld
}

func GetRegistries() []RegistryConfig {
	return config.Registries
}

func GetPruneConfig() PruneConfig {
	return config.PruneConfig
}

func GetListConfig() ListConfig {
	return config.ListConfig
}

func GetServerTlsCfg() ServerTlsCfg {
	return config.ServerTlsCfg
}

// Load loads the passed configuration file into the global configuration struct
func Load(configFile string) error {
	if _, err := os.Stat(configFile); err != nil {
		return fmt.Errorf("unable to stat configuration file: %s", configFile)
	}
	if contents, err := os.ReadFile(configFile); err != nil {
		return fmt.Errorf("error reading configuration file: %s", configFile)
	} else if err := SetConfigFromStr(contents); err != nil {
		return fmt.Errorf("error parsing configuration file: %s, the error was: %s", configFile, err)
	}
	return nil
}

// Get gets the current configuration
func Get() Configuration {
	return config
}

// Set replaces the configuration with the passed configuration
func Set(cfg Configuration) {
	config = cfg
}

// SetConfigFromStr parses the yaml input and sets the configuration from it
func SetConfigFromStr(configBytes []byte) error {
	var cfg Configuration
	if err := yaml.Unmarshal(configBytes, &cfg); err != nil {
		return err
	} else {
		config = cfg
	}
	return nil
}

// ConfigFor looks for an upstream registry configuration entry keyed by the passed 'registry'
// arg (e.g. 'index.docker.io') and returns configuration options for that registry from the
// config. If no matching config is found, then a default configuration is returned specifying
// insecure https, and the runtime OS and architecture.
//
// Since the config might involve loading and calculating a tls.Config with certs, once the
// parsing is complete, the final config struct saved for reuse so it doesn't need to be
// re-parsed in the future.
func ConfigFor(registry string) (imgpull.PullerOpts, error) {
	// default options if no configuration
	opts := imgpull.PullerOpts{
		Scheme:   "https",
		OStype:   runtime.GOOS,
		ArchType: runtime.GOARCH,
	}

	found := RegistryConfig{}
	for _, reg := range config.Registries {
		if reg.Name == registry {
			found = reg
			break
		}
	}

	if found == (RegistryConfig{}) {
		return opts, nil
	}

	if found.Opts != emptyOpts {
		// already parsed
		return found.Opts, nil
	}

	if found.Scheme != "" {
		opts.Scheme = found.Scheme
	}

	if found.Auth != emptyAuth {
		opts.Username = found.Auth.User
		opts.Password = found.Auth.Password
	}

	if found.Tls != emptyTls {
		var cp *x509.CertPool
		var clientCerts []tls.Certificate = []tls.Certificate{}
		if found.Tls.CA != "" {
			cp = x509.NewCertPool()
			caCert, err := os.ReadFile(found.Tls.CA)
			if err == nil {
				cp.AppendCertsFromPEM(caCert)
			} else {
				return opts, fmt.Errorf("unable to load CA for config entry %s from file: %s", registry, found.Tls.CA)
			}
		}
		if found.Tls.Cert != "" && found.Tls.Key != "" {
			cert, err := tls.LoadX509KeyPair(found.Tls.Cert, found.Tls.Key)
			if err == nil {
				clientCerts = []tls.Certificate{cert}
			} else {
				return opts, fmt.Errorf("unable to load client cert and/or key for config entry %s from files: cert: %s, key: %s", registry, found.Tls.Cert, found.Tls.Key)
			}
		}
		opts.TlsCfg = &tls.Config{
			InsecureSkipVerify: found.Tls.InsecureSkipVerify,
			RootCAs:            cp,
			Certificates:       clientCerts,
		}
	}
	found.Opts = opts
	return opts, nil
}
