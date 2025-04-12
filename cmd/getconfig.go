package main

import (
	"ociregistry/impl/cmdline"
	"ociregistry/impl/config"
)

// getCfg calls the command line parser to parse the command line. If upon return from
// parsing the command line, one of the args was --config-file then the config loader is
// called to load that config file and then any overrides specified on the command line are
// overwritten into the global configuration. If the --config-file was NOT specified, then
// the config from the cmdline parse is uses in its entirety as the configuration (which
// has all the defaults, like port, etc.)
func getCfg() (string, error) {
	fromCmdline, cfg, err := cmdline.Parse()
	if err != nil {
		return "", err
	}
	if fromCmdline.ConfigFile {
		if err := config.Load(cfg.ConfigFile); err != nil {
			return "", err
		}
		switch {
		case fromCmdline.LogLevel:
			config.NewConfig.LogLevel = cfg.LogLevel
			fallthrough
		case fromCmdline.ConfigFile:
			config.NewConfig.ConfigFile = cfg.ConfigFile
			fallthrough
		case fromCmdline.ImagePath:
			config.NewConfig.ImagePath = cfg.ImagePath
			fallthrough
		case fromCmdline.PreloadImages:
			config.NewConfig.PreloadImages = cfg.PreloadImages
			fallthrough
		case fromCmdline.Port:
			config.NewConfig.Port = cfg.Port
			fallthrough
		case fromCmdline.Os:
			config.NewConfig.Os = cfg.Os
			fallthrough
		case fromCmdline.Arch:
			config.NewConfig.Arch = cfg.Arch
			fallthrough
		case fromCmdline.PullTimeout:
			config.NewConfig.PullTimeout = cfg.PullTimeout
			fallthrough
		case fromCmdline.AlwaysPullLatest:
			config.NewConfig.AlwaysPullLatest = cfg.AlwaysPullLatest
			fallthrough
		case fromCmdline.AirGapped:
			config.NewConfig.AirGapped = cfg.AirGapped
			fallthrough
		case fromCmdline.HelloWorld:
			config.NewConfig.HelloWorld = cfg.HelloWorld
		}
	} else {
		config.NewConfig = cfg
	}
	return fromCmdline.Command, nil
}
