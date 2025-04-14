// Package cache in the in-memory representation of the image cache. It has all the
// manifests in a map keyed by manifest URL (like quay.io/curl/curl:8.10.1). It also
// has a map of blobs with refcount.
//
// The top-level function is GetManifest, which gets manifests from cache or from
// upstreams. When an image manifest is requested, the blobs for that image manifest
// are pulled at the same time.
package cache
