# Watching For New Images

With a watch frequency configured along with preload, the server keeps checking the preload images (or path) while it runs, and loads any new or changed image tarballs or image list files into the running cache. You add images by copying files into the preload location, with no restart. This is intended for two use cases:

1. Air-gapped environments and upstream outages, where tarballs are the only way to add new images.
2. Using a central file store for image tarballs (EFS, S3, NFS, etc.) and configuring _Ociregistry_ servers in multiple clusters to mount those stores and pick up new images without first requiring a client pull.

The watcher is an extension of [preloading](loading.md): it watches exactly the files and paths that `preloadImages` resolves to, and shares preload's [ledger](loading.md#the-preload-ledger), so neither loads a file the other already loaded.

## Enabling the watcher

Set a watch frequency in the config file, or with the `serve` command's `--watch-frequency` arg, along with the preload images to watch:

```yaml
preloadImages: /mnt/preload
watchFrequency: 30s
```

```shell
ociregistry serve --preload-images /mnt/preload --watch-frequency 30s
```

- The frequency is a Go duration such as `30s`, `5m` or `1h`, of at least `5s`. Anything else is a startup error.
- Without `preloadImages` there's nothing to watch, so no watcher runs (the server logs this at info level).
- `preloadImages` takes the usual [path spec](configuring-the-server.md#loading-images): a file, a directory (its files, one level deep), a glob, or a comma-separated list. A directory is the natural choice for a watched location.

The server never writes to the preload location, so it can be mounted read-only, for example a Kubernetes volume with `readOnly: true`.

## How a file is picked up

Every interval, the watcher:

1. Resolves `preloadImages`, skipping `*.part` files, subdirectories and anything that isn't a regular file.
2. Skips the files the ledger already has, with the same size and modification time.
3. Remembers a file it sees for the first time, or that changed since the last pass, and waits.
4. Loads a file once it's unchanged between two passes, and records the result in the ledger.

So a new file loads one to two intervals after it arrives, and a file still being copied isn't loaded partway. A pass never overlaps the previous one: the next starts one interval after the last finishes.

!!! Note
    Size and modification time alone can't always tell a finished copy from a stalled one, and on NFS (including AWS EFS) file attributes are cached by each client. **Copy files in under a temporary name ending in `.part`, then rename them** (e.g. `cp app.tgz /mnt/preload/app.tgz.part && mv /mnt/preload/app.tgz.part /mnt/preload/app.tgz`). A rename is atomic, and `.part` files are never loaded. On NFS this is required.

## What gets loaded

Archives and image list files are handled the same way by the watcher as by preload:

| Item | `--no-cache` patterns | Already cached |
|-|-|-|
| Image tarballs | ignored | skipped if the same digest, otherwise replaced |
| Image list files | ignored | skipped if the same digest (from the upstream), otherwise replaced |

- **Tarballs** load only the images for the server's OS and architecture.
- **Image list files** pull each image from its upstream, as preload does: the upstream is asked for the image's current digest, and the image is pulled only if the cache doesn't already have that digest. For an image list (multi-platform image), the image for the server's platform is pulled too. Every image in the file is tried: one that fails is logged and doesn't stop the rest, and the file is recorded as failed.
- **Air-gapped:** list files can't be loaded without an upstream, so each one is logged as an error and recorded as failed. Tarballs load as usual.
- **Mutable tags:** `--no-cache` decides how client requests are served afterwards, not what's loaded. Loading `app:latest` from a tarball gives clients a copy to pull during an outage, and while the upstream is reachable the next client request replaces it with the upstream's version.
- **Pull dates:** a loaded image has no pull date until a client pulls it. So [pruning](configuring-the-server.md#prune-configuration) by pull date (`type: accessed`) only removes images that clients have pulled. An image loaded just in case, and never pulled, stays until it's pruned by create date or pattern.

The watcher works alongside client pulls and pruning, using the same rules as an upstream pull: a client requesting an image while it's loading waits for the loaded copy rather than pulling it from the upstream, and a prune can't remove the blobs of an image that's partway loaded.

> I struggled with whether the watcher should process list files - it seemed counter-intuitive in a way. But in the end it allowed load, preload, and watch to function consistently and so I opted for consistency.

## Results

Each file's result is logged and recorded in the ledger:

| Result | Meaning |
|-|-|
| `loaded` | At least one image was added or replaced, with no errors |
| `skipped` | No errors and nothing changed: everything was already cached, or nothing was for the server's platform |
| `failed` | The file couldn't be read, or an image in it failed |

Every result is final until the file changes. A failed file isn't retried, even if the cause was temporary, such as an unreachable upstream. To retry it, replace or `touch` the file, or edit the ledger and restart the server. See [The preload ledger](loading.md#the-preload-ledger).

## Several servers sharing one location

Several servers, for example one per cluster, can watch the same shared volume. Each keeps its own ledger in its own image cache, so each loads every file once, independently, and nothing in the shared location is renamed or removed. Each server should therefore mount read-only. Removing old files from the shared location is up to the person or process that populates those locations. _Ociregistry_ will access them purely read-only.

## Limits

- The resolve ref (`resolveRef`, `--resolve-ref`) is never applied by the watcher: it's for loading one named single-image tarball.
- Subdirectories of a watched directory aren't searched.
- A preload path that doesn't exist (with no glob characters) stops the server from starting. If it disappears while the server runs, for example an unmounted volume, each pass logs an error until it's back.
- A file the watcher can't read, for example for lack of permission, is logged as an error on every pass and isn't recorded, so it's loaded once it's readable.
- The watcher stops when the server shuts down, after finishing the file it's loading. A file interrupted by a forced kill has no ledger entry, so it's processed again on the next start; images already cached are skipped.
