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

// tlsCfg holds TLS configuration for registry access
type tlsCfg struct {
	Cert               string `yaml:"cert"`
	Key                string `yaml:"key"`
	CA                 string `yaml:"ca"`
	InsecureSkipVerify bool   `yaml:"insecureSkipVerify"`
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
	AlwaysPullLatest bool             `yaml:"alwaysPullLatest"`
	AirGapped        bool             `yaml:"airGapped"`
	HelloWorld       bool             `yaml:"helloWorld"`
	Registries       []RegistryConfig `yaml:"registries"`
	PruneConfig      PruneConfig      `yaml:"pruneConfig"`
	ListConfig       ListConfig       `yaml:"listConfig"`
}

// FromCmdLine has a flag for every command-line option. The parsing code
// sets the flag to true if the option was explicitly provided on the command
// line by the user.
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
	AlwaysPullLatest bool
	AirGapped        bool
	HelloWorld       bool
	PruneConfig      bool
	ListConfig       bool
}

var (
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

// Load loads the passed configuration file into the configuration struct
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

// Merge takes a struct indicating which configuration options have been provided on the command
// line, as well as a configuration struct parsed from the command line which ALSO includes defaults
// that the user didn't specify. For example the default port is 8080 and if you don't specify
// that on the command line - it gets defaulted into the parsed configuration struct. So:
//
//  1. User provided a value: overwrite current config using the user's value
//  2. User did not provide a value, current config is unspecified: use the default in the parsed config
//  3. User did not provide a value, current config is specified: leave the current config untouched
func Merge(fromCmdline FromCmdLine, cfg Configuration) {
	if fromCmdline.LogLevel || config.LogLevel == "" {
		config.LogLevel = cfg.LogLevel
	}
	if fromCmdline.LogFile || config.LogFile == "" {
		config.LogFile = cfg.LogFile
	}
	if fromCmdline.ConfigFile || config.ConfigFile == "" {
		config.ConfigFile = cfg.ConfigFile
	}
	if fromCmdline.ImagePath || config.ImagePath == "" {
		config.ImagePath = cfg.ImagePath
	}
	if fromCmdline.PreloadImages || config.PreloadImages == "" {
		config.PreloadImages = cfg.PreloadImages
	}
	if fromCmdline.ImageFile || config.ImageFile == "" {
		config.ImageFile = cfg.ImageFile
	}
	if fromCmdline.Port || (config.Port == 0) {
		config.Port = cfg.Port
	}
	if fromCmdline.Os || config.Os == "" {
		config.Os = cfg.Os
	}
	if fromCmdline.Arch || config.Arch == "" {
		config.Arch = cfg.Arch
	}
	if fromCmdline.PullTimeout || config.PullTimeout == 0 {
		config.PullTimeout = cfg.PullTimeout
	}
	if fromCmdline.AlwaysPullLatest || !config.AlwaysPullLatest {
		config.AlwaysPullLatest = cfg.AlwaysPullLatest
	}
	if fromCmdline.AirGapped || !config.AirGapped {
		config.AirGapped = cfg.AirGapped
	}
	if fromCmdline.HelloWorld || !config.HelloWorld {
		config.HelloWorld = cfg.HelloWorld
	}
	if fromCmdline.PruneConfig || config.PruneConfig == (PruneConfig{}) {
		config.PruneConfig = cfg.PruneConfig
	}
	if fromCmdline.ListConfig || config.ListConfig == (ListConfig{}) {
		config.ListConfig = cfg.ListConfig
	}
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

// ConfigFor looks for a configuration entry keyed by the passed 'registry' arg (e.g.
// 'index.docker.io') and returns configuration options for that registry from the config.
// If no matching config is found, then a default configuration is returned specifying insecure
// https, and the runtime OS and architecture.
//
// Since the config might involve loading and calculating a tls.Config with certs, once the
// parsing is complete, the final config struct saved for reuse so it doesn't need to be
// re-parsed in the future. (Unless the config file changes, which is handled by the
// ConfigLoader func.)
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
