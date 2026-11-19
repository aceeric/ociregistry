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
	"strings"

	"github.com/aceeric/imgpull/pkg/imgpull"
	"github.com/aceeric/ociregistry/impl/auth"
	log "github.com/sirupsen/logrus"

	yaml "go.yaml.in/yaml/v4"
)

// redactedValue replaces a secret in the configuration shown by Redacted.
const redactedValue = "*****"

// tokenAuth configures external token provisioning. The Static member is mainly for test.
type tokenAuth struct {
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
	Token           tokenAuth `yaml:"token"`
}

// tlsCfg holds TLS configuration for upstream registry access
type tlsCfg struct {
	Cert               string `yaml:"cert"`
	Key                string `yaml:"key"`
	CA                 string `yaml:"ca"`
	InsecureSkipVerify bool   `yaml:"insecureSkipVerify"`
}

// serverTlsCfg holds TLS configuration for TLS with (downstream) clients,
// i.e. containerd. Valid values for ClientAuth: "none", and "verify"
type serverTlsCfg struct {
	Cert       string `yaml:"cert"`
	Key        string `yaml:"key"`
	CA         string `yaml:"ca"`
	ClientAuth string `yaml:"clientAuth"`
}

// registryConfig combines authCfg and tlsCfg and configures the pull client
// for access to one upstream registry
type registryConfig struct {
	Name        string             `yaml:"name"`
	Description string             `yaml:"description"`
	Auth        authCfg            `yaml:"auth"`
	Tls         tlsCfg             `yaml:"tls"`
	Scheme      string             `yaml:"scheme"`
	Opts        imgpull.PullerOpts `yaml:"opts,omitempty"`
}

// PruneConfig configures the prune behavior. When loaded from the config file, DryRun is true
// unless the file has dryRun: false (see SetConfigFromStr). PruneConfig is used by the
// prune package.
type PruneConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Duration string `yaml:"duration"`
	Type     string `yaml:"type"`
	Freq     string `yaml:"frequency"`
	Count    int    `yaml:"count"`
	Expr     string `yaml:"expr"`
	DryRun   bool   `yaml:"dryRun"`
}

// DefaultPruneCount is the default --count of the prune sub-command: the most manifests it
// prunes, as a safety. It's also used for a prune config with no count.
const DefaultPruneCount = 5

// migrateConfig configures the migrate-cache sub-command. It is only set from the
// command line.
type migrateConfig struct {
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
	Images           string           `yaml:"images"`
	ResolveRef       string           `yaml:"resolveRef"`
	Port             int              `yaml:"port"`
	Os               string           `yaml:"os"`
	Arch             string           `yaml:"arch"`
	PullTimeout      int              `yaml:"pullTimeout"`
	Health           int              `yaml:"health"`
	Metrics          int              `yaml:"metrics"`
	AlwaysPullLatest bool             `yaml:"alwaysPullLatest"`
	NoCache          []string         `yaml:"noCache"`
	AirGapped        bool             `yaml:"airGapped"`
	HelloWorld       bool             `yaml:"helloWorld"`
	DefaultNs        string           `yaml:"defaultNs"`
	Host             string           `yaml:"host"`
	Registries       []registryConfig `yaml:"registries"`
	PruneConfig      PruneConfig      `yaml:"pruneConfig"`
	ListConfig       ListConfig       `yaml:"listConfig"`
	MigrateConfig    migrateConfig    `yaml:"-"`
	ServerTlsCfg     serverTlsCfg     `yaml:"serverTlsConfig"`
}

// FromCmdLine has a flag for every command-line option. The parsing code
// sets the flag to true if the option was explicitly provided on the command
// line by the user. This supports merging command line config into
// file-based config specified via the `--config-file` arg. PruneConfig
// and ListConfig are a little different - the command line accepts multiple
// args each of which configure one element of the struct. So setting any
// arg will set the entire struct-level flag.
type FromCmdLine struct {
	Command          string
	LogLevel         bool
	LogFile          bool
	ConfigFile       bool
	ImagePath        bool
	PreloadImages    bool
	ImageFile        bool
	Images           bool
	ResolveRef       bool
	Port             bool
	Os               bool
	Arch             bool
	PullTimeout      bool
	Health           bool
	Metrics          bool
	AlwaysPullLatest bool
	NoCache          bool
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
	emptyAuth = authCfg{User: "", Password: "", PasswordFromEnv: "", Token: tokenAuth{}}
	emptyTls  = tlsCfg{Cert: "", Key: "", CA: "", InsecureSkipVerify: false}
	emptyOpts = imgpull.PullerOpts{}
)

// GetLogLevel gets the server log level, e.g. "debug", "warn", "info", or "error".
// Command line arg is `--log-level`.
func GetLogLevel() string {
	return config.LogLevel
}

// GetLogFile gets the log file path, or the empty string if not defined. (By default
// the server logs to stdout). Command line arg is `--log-file`.
func GetLogFile() string {
	return config.LogFile
}

// GetConfigFile gets the configuration file path or the empty string if not defined.
// Command line arg is `--config-file`.
func GetConfigFile() string {
	return config.ConfigFile
}

// GetImagePath gets the path to the file system image cache. Command line arg is
// `--image-path`.
func GetImagePath() string {
	return config.ImagePath
}

// SetImagePath sets the path to the image cache.
func SetImagePath(newVal string) {
	config.ImagePath = newVal
}

// GetPreloadImages gets the preload image arg value for the serve sub-command. Command
// line arg is `--preload-images`.
func GetPreloadImages() string {
	return config.PreloadImages
}

// SetPreloadImages sets the preload image path.
func SetPreloadImages(newVal string) {
	config.PreloadImages = newVal
}

// GetImageFile gets the images to load for the load sub-command: images (the --images arg),
// or if that's empty, imageFile (the deprecated --image-file arg).
func GetImageFile() string {
	if config.Images != "" {
		return config.Images
	}
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

// GetPort gets the port to serve on. Command line arg is `--port`.
func GetPort() int {
	return config.Port
}

// GetOs gets the OS that the server uses to select an image manifest for from a
// multi-arch manifest list. Command line arg is `--os`.
func GetOs() string {
	return config.Os
}

// GetArch gets the architecture that the server uses to select an image manifest for from a
// multi-arch manifest list. Command line arg is `--arch`.
func GetArch() string {
	return config.Arch
}

// GetPullTimeout gets the max time to pull an image in milliseconds before timing
// out. Command line arg is `--pull-timeouet`.
func GetPullTimeout() int {
	return config.PullTimeout
}

// GetHealth gets the port on which the server runs a /health endpoint for liveness/readiness.
// Command line arg is `--health`.
func GetHealth() int {
	return config.Health
}

// GetMetrics gets the port on which the server runs a /metrics endpoint for metrics exposition.
// Command line arg is `--metrics`.
func GetMetrics() int {
	return config.Metrics
}

// GetAlwaysPullLatest is the same as --no-cache ':latest$' (kept for compatibility).
// Command line arg is `--always-pull-latest`.
func GetAlwaysPullLatest() bool {
	return config.AlwaysPullLatest
}

// GetNoCache returns multiple patterns each representing mutable tags. These tags are
// not cached and are always re-pulled from the upstream. Command line arg is `--no-cache`.
func GetNoCache() []string {
	return config.NoCache
}

// latestPattern is the --no-cache pattern that --always-pull-latest stands for.
const latestPattern = ":latest$"

// NoCachePatterns returns the compiled --no-cache patterns. An image whose canonical url
// matches any of them is always pulled from the upstream. If alwaysPullLatest is set, the
// pattern ':latest$' is included. The patterns are validated where they enter the
// configuration (the config file by SetConfigFromStr, the command line by the parser).
// But we trap errors anyway to be safe.
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

// GetAirGapped returns whether the server is air-gapped. Command line arg is `--air-gapped`.
func GetAirGapped() bool {
	return config.AirGapped
}

// SetAirGapped sets the server into (or not into) air-gapped mode based on the
// value of 'newVal'.
func SetAirGapped(newVal bool) {
	config.AirGapped = newVal
}

// GetHelloWorld returns whether or not the server is running in "hello world" test mode.
// Command line arg is `--hello-world`.
func GetHelloWorld() bool {
	return config.HelloWorld
}

// GetDefaultNs gets the default namespace for pulls that don't specify it. Command line arg
// is `--default-ns`.
func GetDefaultNs() string {
	return config.DefaultNs
}

// GetHost gets the host to serve one. Commadn line arg is `--host`.
func GetHost() string {
	return config.Host
}

// GetRegistries gets the upstream registry configuration(s). Supports unit testing.
func GetRegistries() []registryConfig {
	return config.Registries
}

// GetPruneConfig gets the background server prune configuration. There is no command line
// arg - values are specified by operators in the server config specified by `--config-file`.
func GetPruneConfig() PruneConfig {
	return config.PruneConfig
}

// GetListConfig gets the configuration options for the list sub-command. From the command line,
// this is `--header`, `--pattern`, and `--short-digest`.
func GetListConfig() ListConfig {
	return config.ListConfig
}

// GetMigrateConfig gets pre 2.0.0 cache migration options. Currently only `--dry-run` for the
// migrate-cache sub-command.
func GetMigrateConfig() migrateConfig {
	return config.MigrateConfig
}

// GetServerTlsCfg
func GetServerTlsCfg() serverTlsCfg {
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

// Get gets the current configuration which is a merge of the `--config-file` if
// provided and all command-line overrides plus server defaults.
func Get() Configuration {
	return config
}

// Set replaces the configuration with the passed configuration. Used for tests, and
// to set the final server config from `--config-file` settings, plus cmdline overrides,
// and compiled defaults.
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
	}
	config = cfg
	return nil
}

// ParseTls parses the TLS configuration for the server to use to establish TLS with
// downsteam clients like containerd. Supports:
//   - 1-way: we provide our certs to the client and do not request client certs
//   - mTls: we provide our certs to the client, and require and verify client certs
//
// Client cert verification can be via the OS trust store (no CA specified in config), or
// via the provided CA. If there is no TLS configuration, then a nil tls.Config is returned
// to the caller. This means the server should serve on HTTP. Otherwise the specifics
// of the TLS handshake requirements will be encapsulated in the returned struct.
func ParseTls() (*tls.Config, error) {
	tlsCfg := config.ServerTlsCfg
	cfg := &tls.Config{}
	hasCfg := false
	if tlsCfg.Cert != "" && tlsCfg.Key != "" {
		if cert, err := tls.LoadX509KeyPair(tlsCfg.Cert, tlsCfg.Key); err != nil {
			return nil, err
		} else {
			cfg.Certificates = []tls.Certificate{cert}
			hasCfg = true
		}
	}
	cliAuth := strings.ToLower(tlsCfg.ClientAuth)
	if !slices.Contains([]string{"", "none", "verify"}, cliAuth) {
		return nil, fmt.Errorf("unsupported client auth value: %s", tlsCfg.ClientAuth)
	}
	if cliAuth == "verify" {
		hasCfg = true
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
		if tlsCfg.CA != "" {
			if caCert, err := os.ReadFile(tlsCfg.CA); err != nil {
				return nil, err
			} else {
				cp := x509.NewCertPool()
				if !cp.AppendCertsFromPEM(caCert) {
					return nil, err
				}
				cfg.ClientCAs = cp
			}
		}
	}
	if hasCfg {
		return cfg, nil
	}
	return nil, nil
}

// ConfigFor looks for an upstream registry configuration entry keyed by the passed 'registry'
// arg (e.g. 'index.docker.io') and returns configuration options for that registry from the
// config. If no matching config is found, then a default configuration is returned specifying
// insecure https, and the runtime OS and architecture.
//
// Since the config might involve loading and calculating a tls.Config with certs, once the
// parsing is complete, the final config struct saved for reuse so it doesn't need to be
// re-parsed in the future. Currently these aren't hot-loaded so a config file change requires
// a server restart.
func ConfigFor(registry string) (imgpull.PullerOpts, error) {
	// default options if no configuration
	opts := imgpull.PullerOpts{
		Scheme:   "https",
		OStype:   runtime.GOOS,
		ArchType: runtime.GOARCH,
	}

	found := registryConfig{}
	for _, reg := range config.Registries {
		if reg.Name == registry {
			found = reg
			break
		}
	}

	if found == (registryConfig{}) {
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
func UpstreamAuthProviders(yield func(tokenAuth) bool) {
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
