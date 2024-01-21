/*
OCIRegistry runs a simple pull-only OCI Distribution server that serves
images from the filesystem.

Usage:

	server-http [flags] [path ...]

The flags are:

	--image-path
		The filesystem from which to server images. Defaults to
		"../images"
	--log-level
		DEBUG, INFO, WARN, OFF, ERROR. Defaults to ERROR.
	--port
		The port to server on. Defaults to 8080.
*/
package main
