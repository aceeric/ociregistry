package config

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
	if fromCmdline.Health || config.Health == 0 {
		config.Health = cfg.Health
	}
	if fromCmdline.Metrics || config.Metrics == 0 {
		config.Metrics = cfg.Metrics
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
