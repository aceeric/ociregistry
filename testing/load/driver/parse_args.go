package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds the parsed command line arguments
type Config struct {
	Prune            bool
	IterationSeconds int
	Patterns         []string
	MetricsFile      string
	LogFile          string
	RegistryURL      string
	Filter           string
}

// ParseArgs parses command line arguments supporting both --arg=value and --arg value formats
func ParseArgs(args []string) (*Config, error) {
	config := &Config{
		IterationSeconds: 60, // default value
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]

		// Handle --arg=value format
		if strings.Contains(arg, "=") {
			parts := strings.SplitN(arg, "=", 2)
			key := parts[0]
			value := parts[1]

			if err := setConfigValue(config, key, value); err != nil {
				return nil, err
			}
			continue
		}

		// Handle --arg value format (or boolean flags)
		switch arg {
		case "--prune":
			config.Prune = true

		case "--iteration-seconds":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--iteration-seconds requires a value")
			}
			i++
			seconds, err := strconv.Atoi(args[i])
			if err != nil {
				return nil, fmt.Errorf("--iteration-seconds must be a number: %w", err)
			}
			config.IterationSeconds = seconds

		case "--patterns":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--patterns requires a value")
			}
			i++
			// Support comma-separated patterns
			patterns := strings.Split(args[i], ",")
			for _, p := range patterns {
				if trimmed := strings.TrimSpace(p); trimmed != "" {
					config.Patterns = append(config.Patterns, trimmed)
				}
			}

		case "--metrics-file":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--metrics-file requires a value")
			}
			i++
			config.MetricsFile = args[i]

		case "--log-file":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--log-file requires a value")
			}
			i++
			config.LogFile = args[i]

		case "--registry-url":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--registry-url requires a value")
			}
			i++
			config.RegistryURL = args[i]

		case "--filter":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--filter requires a value")
			}
			i++
			config.Filter = args[i]

		default:
			return nil, fmt.Errorf("unknown argument: %s", arg)
		}
	}

	return config, nil
}

// setConfigValue sets a config value from --arg=value format
func setConfigValue(config *Config, key, value string) error {
	switch key {
	case "--prune":
		config.Prune = true

	case "--iteration-seconds":
		seconds, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("--iteration-seconds must be a number: %w", err)
		}
		config.IterationSeconds = seconds

	case "--patterns":
		// Support comma-separated patterns
		patterns := strings.Split(value, ",")
		for _, p := range patterns {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				config.Patterns = append(config.Patterns, trimmed)
			}
		}

	case "--metrics-file":
		config.MetricsFile = value

	case "--log-file":
		config.LogFile = value

	case "--registry-url":
		config.RegistryURL = value

	case "--filter":
		config.Filter = value

	default:
		return fmt.Errorf("unknown argument: %s", key)
	}

	return nil
}

// Validate checks if required arguments are provided
func (c *Config) Validate() error {
	if c.RegistryURL == "" {
		return fmt.Errorf("--registry-url is required")
	}
	if c.IterationSeconds == 0 {
		return fmt.Errorf("--iteration-seconds must be greater than 0")
	}
	if c.Patterns == nil {
		return fmt.Errorf("--patterns is required")
	}
	if c.MetricsFile == "" {
		return fmt.Errorf("--metrics-file is required")
	}
	if c.LogFile == "" {
		return fmt.Errorf("--log-file is required")
	}
	return nil
}

// PrintUsage prints usage information
func PrintUsage() {
	usage := `Usage: program [OPTIONS]

Options:
  --prune                    Enable pruning mode (boolean flag - default false)
  --iteration-seconds VALUE  Seconds between iterations (default: 60)
  --patterns VALUE           Comma-separated batching patterns, can specify multiple (at least one required)
  --metrics-file VALUE       Path to metrics output file (required)
  --log-file VALUE           Path to log file (required)
  --registry-url VALUE       Docker registry URL (required)
  --filter VALUE             Repo filter (optional)

Format:
  Arguments can be specified in two ways:
    --arg=value
    --arg value

Examples:
  program --registry-url=https://registry.example.com --prune
  program --registry-url https://registry.example.com --iteration-seconds 30
  program --registry-url=https://registry.example.com --patterns ".*app.*,.*service.*"
`
	fmt.Fprint(os.Stderr, usage)
}
