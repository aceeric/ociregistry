package config

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime"
	"slices"
	"time"

	"github.com/aceeric/imgpull/pkg/imgpull"
	"github.com/aceeric/ociregistry/impl/auth"
	log "github.com/sirupsen/logrus"

	yaml "go.yaml.in/yaml/v4"
)

// TokenAuth configures external token provisioning. The Static member is mainly for test.
type TokenAuth struct {
	Provider     string `yaml:"provider"`
	ProviderOpts string `yaml:"providerOpts"`
	Expiry       string `yaml:"expiry"`
	Static       string `yaml:"static"`
}

// authCfg holds basic auth user/pass for registry access. The PasswordFromEnv field specifies
// an environment variable name to read the password from which supports running the server as a
// Kubernetes workload. The 'Token' key supports obtaining an auth token for the upstream from
// an external token provisioning service.
type authCfg struct {
	User            string    `yaml:"user"`
	Password        string    `yaml:"password"`
	PasswordFromEnv string    `yaml:"passwordFromEnv"`
	Token           TokenAuth `yaml:"token"`
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

// PruneConfig configures the prune behavior. When loaded from the config file, DryRun is true
// unless the file has dryRun: false (see SetConfigFromStr).
type PruneConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Duration string `yaml:"duration"`
	Type     string `yaml:"type"`
	Freq     string `yaml:"frequency"`
	Count    int    `yaml:"count"`
	Expr     string `yaml:"expr"`
	DryRun   bool   `yaml:"dryRun"`
}

// MigrateConfig configures the migrate-cache sub-command. It is only set from the
// command line.
type MigrateConfig struct {
	DryRun bool `yaml:"-"`
}

// ListConfig configures the list sub-command
type ListConfig struct {
	Header      bool   `yaml:"header"`
	Expr        string `yaml:"expr"`
	ShortDigest bool   `yaml:"shortDigest"`
}

// Configuration represents the totality of configuration knobs and dials for the server.
type Configuration struct {
	LogLevel         string           `yaml:"logLevel"`
	LogFile          string           `yaml:"logFile"`
	ConfigFile       string           `yaml:"configFile"`
	ImagePath        string           `yaml:"imagePath"`
	PreloadImages    string           `yaml:"preloadImages"`
	ImageFile        string           `yaml:"imageFile"`
	ResolveRef       string           `yaml:"resolveRef"`
	Port             int              `yaml:"port"`
	Os               string           `yaml:"os"`
	Arch             string           `yaml:"arch"`
	PullTimeout      int              `yaml:"pullTimeout"`
	Health           int              `yaml:"health"`
	Metrics          int              `yaml:"metrics"`
	AlwaysPullLatest bool             `yaml:"alwaysPullLatest"`
	NoCache          []string         `yaml:"noCache"`
	MetaWriteFreq    string           `yaml:"metaWriteFreq"`
	AirGapped        bool             `yaml:"airGapped"`
	HelloWorld       bool             `yaml:"helloWorld"`
	DefaultNs        string           `yaml:"defaultNs"`
	Host             string           `yaml:"host"`
	Registries       []RegistryConfig `yaml:"registries"`
	PruneConfig      PruneConfig      `yaml:"pruneConfig"`
	ListConfig       ListConfig       `yaml:"listConfig"`
	MigrateConfig    MigrateConfig    `yaml:"-"`
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
	ResolveRef       bool
	Port             bool
	Os               bool
	Arch             bool
	PullTimeout      bool
	Health           bool
	Metrics          bool
	AlwaysPullLatest bool
	NoCache          bool
	MetaWriteFreq    bool
	AirGapped        bool
	HelloWorld       bool
	DefaultNs        bool
	Host             bool
	PruneConfig      bool
	ListConfig       bool
	MigrateConfig    bool
}

var (
	// config is the global configuration, accessed through getters and setters
	// below
	config    Configuration
	emptyAuth = authCfg{User: "", Password: "", PasswordFromEnv: "", Token: TokenAuth{}}
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

// GetResolveRef returns the ref to use (via preload.RefResolver) when
// loading a single-image tarball that the operator knows was exported
// without a usable tag - e.g. `crane pull --format tarball <ref>@sha256:...`
// writes a placeholder tag like "...:i-was-a-digest" rather than a real one.
// Empty string means no override was configured.
func GetResolveRef() string {
	return config.ResolveRef
}

func GetPort() int {
	return config.Port
}

func GetOs() string {
	return config.Os
}

func GetArch() string {
	return config.Arch
}

func GetPullTimeout() int {
	return config.PullTimeout
}

func GetHealth() int {
	return config.Health
}

func GetMetrics() int {
	return config.Metrics
}

func GetAlwaysPullLatest() bool {
	return config.AlwaysPullLatest
}

func GetNoCache() []string {
	return config.NoCache
}

// DefaultMetaWriteFreq is how often the server writes cache metadata (pull dates) to the file
// system if metaWriteFreq / --meta-write-freq isn't set.
const DefaultMetaWriteFreq = "5m"

// GetMetaWriteFreq returns how often the server writes cache metadata to the file system. The
// value is validated where it enters the configuration (the config file by SetConfigFromStr,
// the command line by the parser), so one that doesn't parse here is a bug: it's logged and the
// default is used.
func GetMetaWriteFreq() time.Duration {
	freq := config.MetaWriteFreq
	if freq == "" {
		freq = DefaultMetaWriteFreq
	}
	d, err := ParseMetaWriteFreq(freq)
	if err != nil {
		log.Errorf("%s (should never happen) - using %s", err, DefaultMetaWriteFreq)
		d, _ = ParseMetaWriteFreq(DefaultMetaWriteFreq)
	}
	return d
}

// ParseMetaWriteFreq parses a metaWriteFreq / --meta-write-freq value: a positive Go duration
// such as 30s, 5m or 1h.
func ParseMetaWriteFreq(freq string) (time.Duration, error) {
	d, err := time.ParseDuration(freq)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("metaWriteFreq %q must be a positive duration such as 30s, 5m or 1h", freq)
	}
	return d, nil
}

// latestPattern is the --no-cache pattern that --always-pull-latest stands for.
const latestPattern = ":latest$"

// NoCachePatterns returns the compiled --no-cache patterns. An image whose canonical url
// matches any of them is always pulled from the upstream. If alwaysPullLatest is set, the
// pattern ':latest$' is included. The patterns are validated where they enter the
// configuration (the config file by SetConfigFromStr, the command line by the parser).
func NoCachePatterns() []*regexp.Regexp {
	patterns := slices.Clone(config.NoCache)
	if config.AlwaysPullLatest && !slices.Contains(patterns, latestPattern) {
		patterns = append(patterns, latestPattern)
	}
	compiled := []*regexp.Regexp{}
	for _, p := range patterns {
		if re, err := regexp.Compile(p); err != nil {
			log.Errorf("no-cache pattern %q did not compile (should never happen): %s", p, err)
		} else {
			compiled = append(compiled, re)
		}
	}
	return compiled
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

func GetDefaultNs() string {
	return config.DefaultNs
}

func GetHost() string {
	return config.Host
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

func GetMigrateConfig() MigrateConfig {
	return config.MigrateConfig
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

// SetConfigFromStr parses and validates the yaml input and sets the configuration from it.
// Parsing is strict: a key that isn't a configuration field is an error, so a misspelled key
// (e.g. "dryrun" for "dryRun") stops the server rather than being silently ignored. The
// configuration is unchanged if there is an error.
//
// Prune dry run defaults to true: the decoder only sets fields whose keys are present with a
// value, so pruning for real requires dryRun: false in the file. An absent key, an absent
// pruneConfig, or a null value (dryRun: with nothing after it) all leave it true.
func SetConfigFromStr(configBytes []byte) error {
	cfg := Configuration{PruneConfig: PruneConfig{DryRun: true}}
	dec := yaml.NewDecoder(bytes.NewReader(configBytes))
	dec.KnownFields(true)
	// io.EOF means the input is empty, or only comments: an empty configuration
	if err := dec.Decode(&cfg); err != nil && !errors.Is(err, io.EOF) {
		return err
	} else if err := validateNoCache(cfg.NoCache); err != nil {
		return err
	} else if cfg.MetaWriteFreq != "" {
		if _, err := ParseMetaWriteFreq(cfg.MetaWriteFreq); err != nil {
			return err
		}
	}
	config = cfg
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
		// already parsed, but if the token comes from an auth provider it has to be gotten every time in
		// case it has expired
		if found.Auth.Token.Provider != "" && found.Auth.Token.Static == "" {
			if token, err := auth.GetToken(found.Auth.Token.Provider); err != nil {
				return opts, fmt.Errorf("error getting token for config entry %s, provider: %s", registry, found.Auth.Token.Provider)
			} else {
				found.Opts.Token = token
			}
		}
		return found.Opts, nil
	}

	if found.Scheme != "" {
		opts.Scheme = found.Scheme
	}

	if found.Auth != emptyAuth {
		opts.Username = found.Auth.User
		// If passwordFromEnv is specified, read password from environment variable
		if found.Auth.PasswordFromEnv != "" {
			opts.Password = os.Getenv(found.Auth.PasswordFromEnv)
		} else {
			opts.Password = found.Auth.Password
		}
		if found.Auth.Token.Static != "" {
			opts.Token = found.Auth.Token.Static
		} else if found.Auth.Token.Provider != "" {
			if token, err := auth.GetToken(found.Auth.Token.Provider); err != nil {
				return opts, fmt.Errorf("error getting token for config entry %s, provider: %s", registry, found.Auth.Token.Provider)
			} else {
				opts.Token = token
			}
		}
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

// UpstreamAuthProviders is an iterator over the registries configuration that returns
// the TokenAuth struct for all the registries that have an auth token provider configured.
func UpstreamAuthProviders(yield func(TokenAuth) bool) {
	for _, reg := range config.Registries {
		if reg.Auth.Token.Provider != "" {
			if !yield(reg.Auth.Token) {
				return
			}
		}
	}
}

// Redacted returns a copy of the configuration that is safe to show: secrets that are set are
// replaced by redactedValue, so it's still visible that they are set. The secrets are each
// registry's password, static token and token provider options, and the password and token in
// its parsed puller options, whose TLS configuration (which can hold private keys) is removed.
// Names of environment variables and file paths are not secrets and are kept.
func Redacted() Configuration {
	cfg := config
	cfg.Registries = slices.Clone(config.Registries)
	for i := range cfg.Registries {
		reg := &cfg.Registries[i]
		reg.Auth.Password = redact(reg.Auth.Password)
		reg.Auth.Token.Static = redact(reg.Auth.Token.Static)
		reg.Auth.Token.ProviderOpts = redact(reg.Auth.Token.ProviderOpts)
		reg.Opts.Password = redact(reg.Opts.Password)
		reg.Opts.Token = redact(reg.Opts.Token)
		reg.Opts.TlsCfg = nil
	}
	return cfg
}

// Marshal returns the redacted configuration (see Redacted) as "yaml", with the keys of the
// config file, or "json", with the same keys.
func Marshal(format string) ([]byte, error) {
	b, err := yaml.Marshal(Redacted())
	if err != nil || format == "yaml" {
		return b, err
	} else if format != "json" {
		return nil, fmt.Errorf("unsupported format %q: must be yaml or json", format)
	}
	// via the yaml, so json has the same keys as the config file without a json tag per field
	var v any
	if err := yaml.Unmarshal(b, &v); err != nil {
		return nil, err
	}
	b, err = json.MarshalIndent(v, "", "  ")
	return append(b, '\n'), err
}

// redactedValue replaces a secret in the configuration shown by Redacted.
const redactedValue = "*****"

// redact returns redactedValue for a secret that is set, and an empty one unchanged.
func redact(secret string) string {
	if secret == "" {
		return ""
	}
	return redactedValue
}

// validateNoCache returns an error naming the first noCache pattern that doesn't compile.
func validateNoCache(patterns []string) error {
	for _, p := range patterns {
		if _, err := regexp.Compile(p); err != nil {
			return fmt.Errorf("noCache pattern %q does not compile: %s", p, err)
		}
	}
	return nil
}
