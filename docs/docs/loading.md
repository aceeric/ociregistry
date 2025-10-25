# Loading And Pre-Loading

Loading and Pre-loading supports the air-gapped use case of populating the registry in a connected environment, and then moving it into an air-gapped environment.

You can pre-load the cache two ways:

1. As a startup task before running the service: `bin/ociregistry serve --preload-images <file>`. The server will load the image cache and then serve.
2. By using the binary as a CLI: `bin/ociregistry load --image-file <file>`. The executable will load the cache and then exit back to the command prompt.

In both cases, you create a file with a list of image references. Example:

```shell
cat <<EOF >| imagelist
quay.io/jetstack/cert-manager-cainjector:v1.11.2
quay.io/jetstack/cert-manager-controller:v1.11.2
quay.io/jetstack/cert-manager-webhook:v1.11.2
registry.k8s.io/metrics-server/metrics-server:v0.6.2
registry.k8s.io/ingress-nginx/controller:v1.8.1
registry.k8s.io/pause:3.8
docker.io/kubernetesui/dashboard-api:v1.0.0
docker.io/kubernetesui/metrics-scraper:v1.0.9
docker.io/kubernetesui/dashboard-web:v1.0.0
EOF
```

Since the entirety of the image cache consists of files and sub-directories under the image cache directory, you can tar that directory up at any time, copy it somewhere, untar it, and start an _Ociregistry_ server instance there pointing to the copied directory and it will _just work_.

## Image Store

The image store is persisted to the file system. This includes blobs and manifests. Let's say you run the server with `--image-path=/var/lib/ociregistry`, which is the default. Then:

```text
/var/lib/ociregistry
├── blobs
├── img
└── lts
```

1. `blobs` are where the blobs are stored.
2. `img` stores the non-`latest`-tagged image manifests.
3. `lts` stores the `latest`-tagged image manifests. (See _About "Latest"_ below.)

Everything is stored by digest. When the server starts it loads everything into an in-memory representation. Each new pull through the server while it is running updates both the in-memory representation of the image store as well as the persistent state on the file system.

The software uses a data structure called a [ManifestHolder](https://github.com/aceeric/imgpull/blob/e545697c45354370cf31d7bdf745e8ea55db1edb/pkg/imgpull/manifest_holder.go#L74) to hold all the image metadata and the actual manifest bytes from the upstream registry. These are simply serialized to the file system as JSON. (So you can find and inspect them if needed for troubleshooting with `grep`, `cat`, and `jq`.)

A `ManifestHolder` looks like this:
```go
type ManifestHolder struct {
	Type                 ManifestType
	Digest               string
	ImageUrl             string
	Bytes                []byte
	V1ociIndex           v1oci.Index
	V1ociManifest        v1oci.Manifest
	V2dockerManifestList v2docker.ManifestList
	V2dockerManifest     v2docker.Manifest
	Created              string
	Pulled               string
}
```

The `Bytes` field has the actual manifest bytes from the upstream. You can see the supported manifest types: `V1ociIndex`, `V1ociManifest`, `V2dockerManifestList`, and `V2dockerManifest`.

## Loading behavior

Loading is additive, meaning if you run the load command to load 100 images, then run it again to load 100 different images, your image cache will have 200 images. If you load 100 images, and then later load the same 100 images again, the server will detect during the second load that there is nothing to do. And of course, if you first load A, B, and C, and then later load C, D, and E, then the cache will hold A, B, C, and D.

## About "Latest"

Internally, `latest`-tagged images are stored side-by-side with non-latest images and treated as separate manifests. This enables the server to support cases that occur in development environments where `latest` images are in a constant state of flux. Storing `latest` images this way works in tandem with the `--always-pull-latest` flag as follows:

| Action | `--always-pull-latest` {: .nowrap-column } | Result |
|-|-|-|
| Pull `foo:latest` | `false` (the default) | The image is pulled exactly once. All subsequent pulls return the same image regardless of what happens in the upstream. |
| Pull `foo:latest` {: .nowrap-column } | `true` | The image is pulled from the upstream on each pull from the pull-through server **for each client**. Each pull completely replaces the prior pull. In other words - for latest images the server is a stateless proxy. (This could consume a fair bit of network bandwidth.) |
