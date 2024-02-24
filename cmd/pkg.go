/*
OCIRegistry runs a simple pull-only pull-through OCI Distribution
server that serves images from the filesystem after the image has
been pulled from an upstream and cached on filesystem.

Usage:

	server [flags]

The flags are:

	--config-path string
	    Remote registry configuration file. Defaults to empty string (all remotes anonymous)
	--image-path string
	    Path for the image store. Defaults to '/var/lib/ociregistry'
	--log-level string
	    Log level. Defaults to 'error'
	--port string
	    Port for server. Defaults to 8080
	--load-images
	    Loads the registry with images listed in the specified file by pulling and saving
	--arch
	    Architecture for the --load-images arg
	--os
		Operating system for the --load-images arg
*/
package main
