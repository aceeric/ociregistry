package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// Config holds the parsed command line arguments except for the images member which isn't
// parsed on the command line but is populated by the main function.
type Config struct {
	prune            bool
	dryRun           bool
	shuffle          bool
	iterationSeconds int
	tallySeconds     int
	patterns         []string
	metricsFile      string
	logFile          string
	registryURL      string
	pullthroughURL   string
	filter           string
	images           []imageInfo
}

// ParseArgs parses command line arguments supporting both --arg=value and --arg value formats
func ParseArgs(args []string) (Config, error) {
	config := Config{
		iterationSeconds: 60,
		tallySeconds:     15,
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]

		// Handle --arg=value format
		if strings.Contains(arg, "=") {
			parts := strings.SplitN(arg, "=", 2)
			key := parts[0]
			value := parts[1]

			if err := setConfigValue(&config, key, value); err != nil {
				return Config{}, err
			}
			continue
		}

		// Handle --arg value format (or boolean flags)
		switch arg {
		case "--prune":
			config.prune = true

		case "--dry-run":
			config.dryRun = true

		case "--shuffle":
			config.shuffle = true

		case "--iteration-seconds":
			if i+1 >= len(args) {
				return Config{}, fmt.Errorf("--iteration-seconds requires a value")
			}
			i++
			seconds, err := strconv.Atoi(args[i])
			if err != nil {
				return Config{}, fmt.Errorf("--iteration-seconds must be a number: %w", err)
			}
			config.iterationSeconds = seconds

		case "--tally-seconds":
			if i+1 >= len(args) {
				return Config{}, fmt.Errorf("--tally-seconds requires a value")
			}
			i++
			seconds, err := strconv.Atoi(args[i])
			if err != nil {
				return Config{}, fmt.Errorf("--tally-seconds must be a number: %w", err)
			}
			config.tallySeconds = seconds

		case "--patterns":
			if i+1 >= len(args) {
				return Config{}, fmt.Errorf("--patterns requires a value")
			}
			i++
			// Support comma-separated patterns
			patterns := strings.Split(args[i], ",")
			for _, p := range patterns {
				if trimmed := strings.TrimSpace(p); trimmed != "" {
					config.patterns = append(config.patterns, trimmed)
				}
			}

		case "--metrics-file":
			if i+1 >= len(args) {
				return Config{}, fmt.Errorf("--metrics-file requires a value")
			}
			i++
			config.metricsFile = args[i]

		case "--log-file":
			if i+1 >= len(args) {
				return Config{}, fmt.Errorf("--log-file requires a value")
			}
			i++
			config.logFile = args[i]

		case "--registry-url":
			if i+1 >= len(args) {
				return Config{}, fmt.Errorf("--registry-url requires a value")
			}
			i++
			config.registryURL = args[i]

		case "--pullthrough-url":
			if i+1 >= len(args) {
				return Config{}, fmt.Errorf("--pullthrough-url requires a value")
			}
			i++
			config.pullthroughURL = args[i]

		case "--filter":
			if i+1 >= len(args) {
				return Config{}, fmt.Errorf("--filter requires a value")
			}
			i++
			config.filter = args[i]

		default:
			return Config{}, fmt.Errorf("unknown argument: %s", arg)
		}
	}

	return config, nil
}

// setConfigValue sets a config value from --arg=value format
func setConfigValue(config *Config, key, value string) error {
	switch key {
	case "--prune":
		config.prune = true

	case "--dry-run":
		config.dryRun = true

	case "--shuffle":
		config.shuffle = true

	case "--iteration-seconds":
		seconds, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("--iteration-seconds must be a number: %w", err)
		}
		config.iterationSeconds = seconds

	case "--tally-seconds":
		seconds, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("--tally-seconds must be a number: %w", err)
		}
		config.tallySeconds = seconds

	case "--patterns":
		// Support comma-separated patterns
		patterns := strings.Split(value, ",")
		for _, p := range patterns {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				config.patterns = append(config.patterns, trimmed)
			}
		}

	case "--metrics-file":
		config.metricsFile = value

	case "--log-file":
		config.logFile = value

	case "--registry-url":
		config.registryURL = value

	case "--pullthrough-url":
		config.pullthroughURL = value

	case "--filter":
		config.filter = value

	default:
		return fmt.Errorf("unknown argument: %s", key)
	}

	return nil
}

// Validate checks if required arguments are provided
func (c *Config) Validate() error {
	if c.registryURL == "" {
		return fmt.Errorf("--registry-url is required")
	}
	if c.pullthroughURL == "" {
		return fmt.Errorf("--pullthrough-url is required")
	}
	if c.iterationSeconds == 0 {
		return fmt.Errorf("--iteration-seconds must be greater than 0")
	}
	if c.tallySeconds == 0 {
		return fmt.Errorf("--tally-seconds must be greater than 0")
	}
	if c.patterns == nil {
		return fmt.Errorf("--patterns is required")
	}
	for _, pat := range c.patterns {
		if _, err := regexp.Compile(pat); err != nil {
			return fmt.Errorf("pattern %s not a valid regex. error: %s", pat, err)

		}
	}
	return nil
}

// PrintUsage prints usage information
func PrintUsage() {
	usage := `Usage: program [OPTIONS]

Options:
  --prune                    Enable pruning (boolean - default false)
  --iteration-seconds VALUE  Seconds between iterations (default: 60)
  --tally-seconds VALUE      Interval for tallying pull rate (default: 15)
  --patterns VALUE           Comma-separated batching patterns, can specify multiple (at least one required)
  --metrics-file VALUE       Path to metrics output file (stdout if omitted)
  --log-file VALUE           Path to log file (stdout if omitted)
  --registry-url VALUE       Upstream registry URL (required)
  --pullthrough-url VALUE    Pull through (ociregistry server) URL (required)
  --filter VALUE             Repo filter (optional to create a smaller test set)
  --dry-run                  Does everything except actually pull from the registry (boolean - default false)
  --shuffle                  If specified, then shuffles the image list between pull passes (boolean - default false)

Format:
  Arguments can be specified in two ways:
    --arg=value
    --arg value

Examples:
  program --registry-url=https://registry.example.com --pullthrough-url=http://localhost:8080 --prune
  program --registry-url https://registry.example.com --pullthrough-url http://localhost:8080 --iteration-seconds 30
  program --registry-url=https://registry.example.com --pullthrough-url=http://localhost:8080 --patterns ".*app.*,.*service.*"
`
	fmt.Fprint(os.Stderr, usage)
}
