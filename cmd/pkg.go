/*
OCIRegistry runs a simple pull-only pull-through OCI Distribution
server that serves images from the filesystem.

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
*/
package main
