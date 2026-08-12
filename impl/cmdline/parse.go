package cmdline

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"slices"
	"strings"

	"github.com/aceeric/ociregistry/impl/config"
	"github.com/aceeric/ociregistry/impl/pullrequest"

	"github.com/urfave/cli/v3"
)

// fromCmdline will be populated with flags indicating which configuration settings were
// specified on the command line.
var fromCmdline config.FromCmdLine

// cfg has the parsed configuration - including defaults (e.g. port) if the user does not override
var cfg = config.Configuration{}

// cmds is for the command line parser urfave/cli
var cmds = &cli.Command{
	Name:  "ociregistry",
	Usage: "a pull-only, pull-through, caching OCI distribution server",
	// define this or the parser terminates the program
	ExitErrHandler: func(_ context.Context, _ *cli.Command, _ error) {},
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:        "log-level",
			Value:       "error",
			Usage:       "Sets the minimum value for logging: debug, warn, info, or error",
			Destination: &cfg.LogLevel,
			Validator: func(lvl string) error {
				validValues := []string{"debug", "warn", "info", "error"}
				if !slices.Contains(validValues, strings.ToLower(lvl)) {
					return fmt.Errorf("must be one of %s", strings.Join(validValues, ", "))
				}
				return nil
			},
			Action: func(ctx context.Context, cmd *cli.Command, _ string) error {
				fromCmdline.LogLevel = true
				return nil
			},
		},
		&cli.StringFlag{
			Name:        "config-file",
			Usage:       "A file to load configuration values from (cmdline overrides file settings)",
			Destination: &cfg.ConfigFile,
			Validator: func(path string) error {
				if fi, err := os.Stat(path); err != nil {
					return fmt.Errorf("file not found")
				} else if fi.IsDir() {
					return fmt.Errorf("not a file")
				}
				return nil
			},
			Action: func(ctx context.Context, cmd *cli.Command, _ string) error {
				fromCmdline.ConfigFile = true
				return nil
			},
		},
		&cli.StringFlag{
			Name:        "image-path",
			Value:       "/var/lib/ociregistry",
			Usage:       "The path for the image cache",
			Destination: &cfg.ImagePath,
			Action: func(ctx context.Context, cmd *cli.Command, _ string) error {
				fromCmdline.ImagePath = true
				return nil
			},
		},
		&cli.StringFlag{
			Name:        "log-file",
			Value:       "",
			Usage:       "log to the specified file rather than the console",
			Destination: &cfg.LogFile,
			Action: func(ctx context.Context, cmd *cli.Command, _ string) error {
				fromCmdline.LogFile = true
				return nil
			},
		},
	},
	Commands: []*cli.Command{
		{
			Name: "serve",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				fromCmdline.Command = "serve"
				return nil
			},
			Description: "Runs the pull-through registry. The --preload-images arg accepts the following: the name of\n" +
				"a plain text file containing an image list, the name of a tarball, a glob expression for either of the\n" +
				"above, or all of the above comma-separated. Glob expressions must be single-quote enclosed to prevent shell\n" +
				"expansion. Complex example: --preload-images 'foo*.tar,myimagelist.txt,myotherlist.txt,frobozz*.tgz'",
			Flags: []cli.Flag{
				// validation at point of use
				&cli.StringFlag{
					Name:        "preload-images",
					Usage:       "Preloads images from file(s) containing a list of image refs, or from tarball(s)",
					Destination: &cfg.PreloadImages,
					Action: func(ctx context.Context, cmd *cli.Command, _ string) error {
						fromCmdline.PreloadImages = true
						return nil
					},
				},
				&cli.IntFlag{
					Name:        "port",
					Value:       8080,
					Usage:       "The port to serve on",
					Destination: &cfg.Port,
					Action: func(ctx context.Context, cmd *cli.Command, _ int) error {
						fromCmdline.Port = true
						return nil
					},
				},
				&cli.StringFlag{
					Name:        "os",
					Value:       runtime.GOOS,
					Usage:       "The operating system to pull images for",
					Destination: &cfg.Os,
					Action: func(ctx context.Context, cmd *cli.Command, _ string) error {
						fromCmdline.Os = true
						return nil
					},
				},
				&cli.StringFlag{
					Name:        "arch",
					Value:       runtime.GOARCH,
					Usage:       "The architecture to pull images for",
					Destination: &cfg.Arch,
					Action: func(ctx context.Context, cmd *cli.Command, _ string) error {
						fromCmdline.Arch = true
						return nil
					},
				},
				&cli.IntFlag{
					Name:        "pull-timeout",
					Value:       60000,
					Usage:       "The max time to pull an image in milliseconds before timing out",
					Destination: &cfg.PullTimeout,
					Action: func(ctx context.Context, cmd *cli.Command, _ int) error {
						fromCmdline.PullTimeout = true
						return nil
					},
				},
				&cli.IntFlag{
					Name:        "health",
					Usage:       "Specify a port number to have the server run a /health endpoint for liveness/readiness",
					Destination: &cfg.Health,
					Action: func(ctx context.Context, cmd *cli.Command, _ int) error {
						fromCmdline.Health = true
						return nil
					},
				},
				&cli.IntFlag{
					Name:        "metrics",
					Usage:       "Enables metrics exposition on the specified port",
					Destination: &cfg.Metrics,
					Action: func(ctx context.Context, cmd *cli.Command, _ int) error {
						fromCmdline.Metrics = true
						return nil
					},
				},
				&cli.BoolFlag{
					Name:        "always-pull-latest",
					Value:       false,
					Usage:       "Always pulls from the upstream if an image tag is 'latest'",
					Destination: &cfg.AlwaysPullLatest,
					Action: func(ctx context.Context, cmd *cli.Command, _ bool) error {
						fromCmdline.AlwaysPullLatest = true
						return nil
					},
				},
				&cli.BoolFlag{
					Name:        "hello-world",
					Value:       false,
					Usage:       "Only serves docker.io/hello-world:latest using built-in files without pulling - for testing",
					Destination: &cfg.HelloWorld,
					Action: func(ctx context.Context, cmd *cli.Command, _ bool) error {
						fromCmdline.HelloWorld = true
						return nil
					},
				},
				&cli.BoolFlag{
					Name:        "air-gapped",
					Value:       false,
					Usage:       "Does not attempt to pull from an upstream if an un-cached image is requested",
					Destination: &cfg.AirGapped,
					Action: func(ctx context.Context, cmd *cli.Command, _ bool) error {
						fromCmdline.AirGapped = true
						return nil
					},
				},
				&cli.StringFlag{
					Name:        "default-ns",
					Value:       "",
					Usage:       "A default namespace if none is provided (otherwise pull without namespace is an error)",
					Destination: &cfg.DefaultNs,
					Action: func(ctx context.Context, cmd *cli.Command, _ string) error {
						fromCmdline.DefaultNs = true
						return nil
					},
				},
				&cli.StringFlag{
					Name:        "host",
					Value:       "0.0.0.0",
					Usage:       "The host to serve on",
					Destination: &cfg.Host,
					Action: func(ctx context.Context, cmd *cli.Command, _ string) error {
						fromCmdline.Host = true
						return nil
					},
				},
			},
		},
		{
			Name: "load",
			//Usage: "Loads the image cache",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				fromCmdline.Command = "load"
				return nil
			},
			Description: "Loads the image cache using the server as a cli. The --image-file arg accepts the following: the\n" +
				"name of a plain text file containing an image list, the name of a tarball, a glob expression for either of the\n" +
				"above, or all of the above comma-separated. Glob expressions must be single-quote enclosed to prevent shell\n" +
				"expansion. Complex example: --image-file 'foo*.tar,myimagelist.txt,myotherlist.txt,frobozz*.tgz'",
			Flags: []cli.Flag{
				// validation at point of use
				&cli.StringFlag{
					Name:        "image-file",
					Usage:       "Loads images from file(s) containing a list of image refs, or from tarball(s)",
					Destination: &cfg.ImageFile,
					Action: func(ctx context.Context, cmd *cli.Command, _ string) error {
						fromCmdline.ImageFile = true
						return nil
					},
				},
				&cli.StringFlag{
					Name:        "resolve-ref",
					Usage:       "Image ref to use when --image-file is a single-image tarball with no usable tag",
					Destination: &cfg.ResolveRef,
					Validator: func(ref string) error {
						if _, err := pullrequest.NewPullRequestFromUrl(ref); err != nil {
							return fmt.Errorf("must be a fully-qualified image ref, e.g. registry.host/repo:tag")
						}
						return nil
					},
					Action: func(ctx context.Context, cmd *cli.Command, _ string) error {
						fromCmdline.ResolveRef = true
						return nil
					},
				},
				&cli.StringFlag{
					Name:        "os",
					Value:       runtime.GOOS,
					Usage:       "The operating system to pull images for",
					Destination: &cfg.Os,
					Action: func(ctx context.Context, cmd *cli.Command, _ string) error {
						fromCmdline.Os = true
						return nil
					},
				},
				&cli.StringFlag{
					Name:        "arch",
					Value:       runtime.GOARCH,
					Usage:       "The architecture to pull images for",
					Destination: &cfg.Arch,
					Action: func(ctx context.Context, cmd *cli.Command, _ string) error {
						fromCmdline.Arch = true
						return nil
					},
				},
				&cli.IntFlag{
					Name:        "pull-timeout",
					Value:       60000,
					Usage:       "The max time to pull an image in milliseconds before timing out",
					Destination: &cfg.PullTimeout,
					Action: func(ctx context.Context, cmd *cli.Command, _ int) error {
						fromCmdline.PullTimeout = true
						return nil
					},
				},
			},
		},
		{
			Name: "list",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				fromCmdline.Command = "list"
				return nil
			},
			Description: "Lists the cache as it is on the file system. Output is compact horizontally and unsorted.\n" +
				"Use linux tools to format and sort. E.g.:\n" +
				"bin/ociregistry list --header --short-digest | awk 'NR==1; NR>1 {print | \"sort -k1,1n\"}' | column -t",
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:        "header",
					Value:       false,
					Usage:       "Displays a header line",
					Destination: &cfg.ListConfig.Header,
					Action: func(ctx context.Context, cmd *cli.Command, _ bool) error {
						fromCmdline.ListConfig = true
						return nil
					},
				},
				&cli.StringFlag{
					Name:        "pattern",
					Usage:       "List images matching the comma-separated pattern(s), e.g. '--pattern cilium,coredns'",
					Destination: &cfg.ListConfig.Expr,
					Action: func(ctx context.Context, cmd *cli.Command, _ string) error {
						fromCmdline.PruneConfig = true
						cfg.PruneConfig.Type = "date"
						return nil
					},
				},
				&cli.BoolFlag{
					Name:        "short-digest",
					Value:       false,
					Usage:       "Displays truncated digests to conserve horizontal space",
					Destination: &cfg.ListConfig.ShortDigest,
					Action: func(ctx context.Context, cmd *cli.Command, _ bool) error {
						fromCmdline.ListConfig = true
						return nil
					},
				},
			},
		},
		{
			Name: "prune",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				fromCmdline.Command = "prune"
				return nil
			},
			Description: "Prunes the cache on the filesystem (server should not be running). Important: --dry-run\n" +
				"defaults to false so typically specify --dry-run until results are as expected, then remove to\n" +
				"actually prune. Important: the date format must be exactly as shown in the --date arg help below.",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:        "pattern",
					Usage:       "Prune images matching the comma-separated pattern(s), e.g. '--pattern cilium,coredns'",
					Destination: &cfg.PruneConfig.Expr,
					Action: func(ctx context.Context, cmd *cli.Command, _ string) error {
						fromCmdline.PruneConfig = true
						cfg.PruneConfig.Type = "pattern"
						return nil
					},
				},
				&cli.StringFlag{
					Name:        "date",
					Usage:       "Prune images created before a timestamp, e.g. '--date 2025-02-28T12:59:59'",
					Destination: &cfg.PruneConfig.Expr,
					Action: func(ctx context.Context, cmd *cli.Command, _ string) error {
						fromCmdline.PruneConfig = true
						cfg.PruneConfig.Type = "date"
						return nil
					},
				},
				&cli.BoolFlag{
					Name:        "dry-run",
					Value:       false,
					Usage:       "Shows what would prune, but does not actually prune",
					Destination: &cfg.PruneConfig.DryRun,
					Action: func(ctx context.Context, cmd *cli.Command, _ bool) error {
						fromCmdline.PruneConfig = true
						return nil
					},
				},
			},
		},
		{
			Name: "version",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				fromCmdline.Command = "version"
				return nil
			},
			Description: "Displays the server version.",
		},
	},
}

// Parse parses the command line. It returns the following:
//
//  1. A FromCmdLine struct which has the command to run ("serve", "list", etc.). If the command
//     is the empty string then no sub-command was specified in which case the parser auto-displays
//     help. This struct also has flags telling you which configuration values were provided by the
//     user on the command line.
//  2. A Configuration struct containing the parsed configuration values. For any configuration flag
//     in the FromCmdLine struct with a false value, the corresponding configuration value in *this*
//     struct will be the default.
//  3. An error, if the parser returned one, else nil.
func Parse() (config.FromCmdLine, config.Configuration, error) {
	if err := cmds.Run(context.Background(), os.Args); err != nil {
		return config.FromCmdLine{}, config.Configuration{}, err
	}
	return fromCmdline, cfg, nil
}

// ClearParse supports unit testing
func ClearParse() {
	fromCmdline = config.FromCmdLine{}
	cfg = config.Configuration{}
}
