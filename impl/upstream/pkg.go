// Package upstream talks to the upstream registries, gets manifests and blobs,
// and so on. Presently, I embed the Google Crane code to actually interact
// with the upstreams:
//
//	https://github.com/google/go-containerregistry/blob/main/cmd/crane/doc/crane.md
//
// The package handles concurrent requests for the same images - which is likely to
// be common when the k8s cluster first starts up and DaemonSets on different nodes
// all pull the same images.
package upstream
