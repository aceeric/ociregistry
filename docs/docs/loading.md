# Loading And Pre-Loading

Loading and Pre-loading supports the air-gapped use case of populating the registry in a connected environment, and then moving it into an air-gapped environment.

You can pre-load the cache two ways:

1. As a startup task before running the service: `bin/ociregistry serve --preload-images ...`. The server will load the image cache and then serve.
2. By using the binary as a CLI: `bin/ociregistry load --image-file ...`. The executable will load the cache and then exit back to the command prompt.

More details are presented in the [Loading Images](configuring-the-server.md#loading-images) section of the  _Configuring The Server_ page. Since the entirety of the image cache consists of files and sub-directories under the image cache directory, you can tar that directory up at any time, copy it somewhere, untar it, and start an _Ociregistry_ server instance there pointing to the copied directory and it will _just work_.

## Image Store

The image store is persisted to the file system. This includes blobs and manifests. Let's say you run the server with `--image-path=/var/lib/ociregistry`, which is the default. Then:

```text
/var/lib/ociregistry
├── blobs
└── manifests
```

1. `blobs` are where the blobs are stored.
2. `manifests` stores the manifests.

Blobs are stored by digest. Manifests are stored by one file named by the SHA-256 digest of the canonical manifest URL. When the server starts it loads everything into an in-memory representation. Each new pull through the server while it is running updates both the in-memory representation of the image store as well as the persistent state on the file system.

!!! Note
    As of version 2.0.0 the filesystem layout changed. When you start the server (via the `serve` subcommand) or when you use the `migrate-cache` sub-command the pre-2.0.0 manifests in `img` and `lts` are migrated to `manifests` and a marker is written. The `img` and `lts` directories remain until you remove them but the server will not "see" those directories after migration to the 2.0.0 storage scheme.

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

Loading is additive, meaning if you run the load command to load 100 images, then run it again to load 100 different images, your image cache will have 200 images. If you load 100 images, and then later load the same 100 images again, the server will detect during the second load that there is nothing to do. And of course, if you first load A, B, and C, and then later load C, D, and E, then the cache will hold A, B, C, D, and E.

## Canonicalization

The server canonicalizes images on intake:

- For every registry the registry host and repository are lowercased, and so is a digest; tags keep their case
- For legacy reasons it is possible to pull `docker.io/hello-world:latest` even though the canonical url is `docker.io/library/hello-world:latest`. Since canonicalization happens in ontake, it is always possible to re-pull `docker.io/hello-world:latest` even though internally it is cached with the `library` segment.
