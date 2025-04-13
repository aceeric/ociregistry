package main

import (
	"ociregistry/impl/cmdline"
	"ociregistry/impl/config"
)

// getCfg calls the command line parser to parse the command line. If upon return from
// parsing the command line, one of the args was --config-file then the config loader is
// called to load that config file and then any overrides specified on the command line are
// overwritten into the global configuration. If the --config-file was NOT specified, then
// the config from the cmdline parse is used in its entirety as the configuration (which
// has all the defaults, like port, etc.)
//
// The sub-command specified on the command line (serve, load, etc.) is returned in return
// value one.
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
