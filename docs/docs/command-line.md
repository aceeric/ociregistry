# The Command Line

The command line parser uses the [urfave/cli](https://github.com/urfave/cli) parser. Running the server with no arguments shows the following sub-commands:

```text
NAME:
   ociregistry - a pull-only, pull-through, caching OCI distribution server

USAGE:
   ociregistry [global options] [command [command options]]

COMMANDS:
   serve          Runs the pull-through registry
   load           Loads the image cache using the server as a cli
   list           Lists the cache as it is on the file system
   prune          Prunes the cache on the filesystem
   migrate-cache  Migrates the cache from the legacy (pre v2.n.n) layout
   version        Displays the server version.
   help, h        Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --log-level string    Sets the minimum value for logging: debug, warn, info, or error (default: "error")
   --config-file string  A file to load configuration values from (cmdline overrides file settings)
   --image-path string   The path for the image cache (default: "/var/lib/ociregistry")
   --log-file string     log to the specified file rather than the console
   --help, -h            show help
```

The simplest way to run the  server with all defaults is:

```shell
ociregistry serve
```

Each sub-command also supports help, as expected. E.g.: `ociregistry serve --help`
