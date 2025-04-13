package main

import (
	"ociregistry/impl/cmdline"
	"ociregistry/impl/config"
)

// getCfg calls the command line parser to parse the command line. If one of the command line
// args was '--config-file' then the the function calls the config loader to load that config file
// into the global configuration. Then any overrides from the command line are overwritten into
// the global configuration. If '--config-file' was NOT provided on the command line, then
// the config from the parsed cmdline is used in its entirety as the configuration (which has all
// the defaults, like port, etc.)
//
// In summary, the function supports getting config from both a config file and the cmdline with
// individual cmdline values taking precedence over those in the file. Note: some configs can ONLY
// be provided via the config file, e.g.: prune configuration, and upstream registry config.
//
// The sub-command specified on the command line (serve, load, etc.) is returned in the first
// return value.
func getCfg() (string, error) {
	fromCmdline, cfg, err := cmdline.Parse()
	if err != nil {
		return "", err
	}
	if fromCmdline.ConfigFile {
		if err := config.Load(cfg.ConfigFile); err != nil {
			return "", err
		}
		config.Merge(fromCmdline, cfg)
	} else {
		config.Set(cfg)
	}
	return fromCmdline.Command, nil
}
