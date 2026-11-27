# Loading And Pre-Loading

Loading and Pre-loading supports the air-gapped use case of populating the registry in a connected environment, and then moving it into an air-gapped environment.

You can pre-load the cache three ways:

1. As a startup task before running the service: `bin/ociregistry serve --preload-images ...`. The server will load the image cache and then serve. Each file is recorded in the [preload ledger](#the-preload-ledger), so a restart doesn't load it again.
2. While the server runs, by also giving a watch frequency: `bin/ociregistry serve --preload-images ... --watch-frequency 30s`. The server keeps checking the preload images and loads new or changed files into the running cache. See [Watching For New Images](watching.md).
3. By using the binary as a CLI: `bin/ociregistry load --images ...`. The executable will load the cache and then exit back to the command prompt. It tries every file, and its exit code is `0` if everything loaded (or was already cached), `1` if the path couldn't be resolved (nothing is loaded), and `3` if some files failed: they're logged, and the rest are loaded.

> Preload and watch use a persistent ledger file and don't re-process the same files. The CLI version does not use the ledger. (It _does_ detect when an image is already cached and won't re-load it.)

More details are presented in the [Loading Images](configuring-the-server.md#loading-images) section of the  _Configuring The Server_ page. Since the entirety of the image cache consists of files and sub-directories under the image cache directory, you can tar that directory up at any time, copy it somewhere, untar it, and start an _Ociregistry_ server instance there pointing to the copied directory and it will _just work_.

## Image Store

The image store is persisted to the file system. This includes blobs and manifests. Let's say you run the server with `--image-path=/var/lib/ociregistry`, which is the default. Then:

```text
/var/lib/ociregistry
├── blobs
├── manifests
└── metadata
```

1. `blobs` are where the blobs are stored.
2. `manifests` stores the manifests.
3. `metadata` stores information about the cache: at this time, just the [preload ledger](#the-preload-ledger).

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

## The preload ledger

When the server preloads (`serve --preload-images`) or watches for new files (`--watch-frequency`), it records each file it processed in the preload ledger, `metadata/loaded.yaml` in the image cache:

```yaml
version: 1
files:
  - name: /mnt/preload/abc.tgz
    size: 10646
    timestamp: "2026-11-11T01:22:44Z"
    processed: "2026-11-18T23:12:31Z"
    result: loaded
```

- `name`, `size` and `timestamp` (the file's modification time, in UTC) identify a version of a file. A file whose entry matches all three is skipped. Any other file is processed, and its entry replaced.
- `processed` is when it was processed, and `result` is `loaded`, `skipped` or `failed` (see [Results](watching.md#results)).
- **Every result is final** until the file changes, `failed` included. In particular, images a prune removed are not loaded again at the next start: an image loaded from `foo1.tar` that later ages out of the cache stays out.
- A file that fails to preload at startup is logged as an error and recorded as failed. The server still starts, and the other files are still loaded.
- Entries for files in the preload directory that no longer exist are removed from the ledger. (If you re-add the file, it will be re-processed.)
- If the ledger can't be read, or isn't a version this server understands, the server logs an error and treats it as empty, processing every file again.
- A new image cache has no ledger, so everything is processed.

The ledger is read when the server starts and updated on every watch cycle. To process files again, stop the server, edit or delete `loaded.yaml`, and re-start. Each server has its own ledger in its own image cache, and preload/watch files are accessed read-only, so multiple _ociregistry_ servers can safely preload and watch from the same shared location.

## Canonicalization

The server canonicalizes images on intake:

- For every registry the registry host and repository are lowercased, and so is a digest; tags keep their case
- For legacy reasons it is possible to pull `docker.io/hello-world:latest` even though the canonical url is `docker.io/library/hello-world:latest`. Since canonicalization happens in ontake, it is always possible to re-pull `docker.io/hello-world:latest` even though internally it is cached with the `library` segment.
