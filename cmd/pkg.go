/*
OCIRegistry runs a simple pull-only pull-through OCI Distribution
server that serves images from the filesystem after the image has
been pulled from an upstream and cached on the filesystem.

Usage:

	server [flags]

Flags:

	To run as a server:

	    --preload-images
			Loads images enumerated in the specified file into cache at startup and then
			continues to serve.
	    --port string
			Port for server. Defaults to 8080.
		--always-pull-latest
			When the 'latest' tag is pulled, don't cache anything. In other words, the server
			acts like a simple proxy for the 'latest' tag.

	To run as a CLI:

		--load-images
			Loads images enumerated in the specified file into cache and then exits.
	    --list-cache
			Lists the cached images and exits.
	    --version
			Displays the version and exits.

	Common:

	    --concurrent int
			For --load-images and --preload-images, the number of concurrent goroutines
			to perform the load with. Default is one.
		--image-path string
			Path for the image store. Defaults to '/var/lib/ociregistry'.
		--log-level string
			Log level. Defaults to 'error'.
		--config-path string
			Remote registry configuration file. Defaults to empty string (all remotes anonymous).
	    --pull-timeout
			Max time in millis to pull an image from an upstream. Defaults to one minute.
		--arch
			Architecture for the --load-images and --preload-images arg. Defaults to
			host architecture.
		--os
			Operating system for the --load-images and --preload-images arg.  Defaults to
			host OS.
*/
package main
