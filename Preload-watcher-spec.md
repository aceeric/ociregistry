# Preload watcher spec

Nov 18, 2026 · @Eric Ace

The watcher extends preload: with `watchFrequency` set, the server re-resolves `preloadImages` every interval and loads new or changed archives and image-list files into the running cache, recording each file in a per-server ledger so neither preload nor the watcher loads a file twice. Built against main 3e76215.

## Behavior

The watcher is on when `watchFrequency` is set and `preloadImages` is set; it watches exactly the files `preloadImages` resolves to, which are never written to and may be mounted read-only.

**Config and flags**

| Setting | Config key | `serve` flag | Default |
| --- | --- | --- | --- |
| Watch interval | `watchFrequency` (top level, Go duration) | `--watch-frequency 30s` | empty: no watcher |
| Files watched | `preloadImages` (unchanged) | `--preload-images` (unchanged) | empty: no preload, no watcher |

- `watchFrequency` must be a positive duration; an invalid value fails config loading.
- `watchFrequency` without `preloadImages` is logged at info and starts no watcher.
- Hello-world mode starts no watcher.

**At startup** (`serve`)

1. Preload resolves `preloadImages` and processes each file the ledger doesn't already have, as today, writing to disk only.
2. A path or glob that matches nothing is logged at info, not an error, because files may arrive later.
3. `cache.Load` loads the cache.
4. If `watchFrequency` is set, the watcher starts.

**Each watcher pass**

1. Resolve `preloadImages` again, skipping `*.part`, directories, symlinks and other non-regular files.
2. Skip files in the ledger with the same size and timestamp.
3. A file seen for the first time, or changed since the last pass, has its size and timestamp remembered in memory and waits for the next pass.
4. A file unchanged since the last pass is processed: archives through the cache's tarball path, list files through the cache's pull path.
5. Its outcome is written to the ledger.

Passes never overlap: the next pass starts one interval after the previous one finishes.

**Already cached, and `--no-cache`** — identical for preload and the watcher:

| Item           | `--no-cache` patterns | Already cached |
| ---            | ---                   | --- |
| Image archives | ignored               | skip if same digest, else replace |
| List files     | ignored               | skip if same digest (from an upstream `HEAD`), else replace |

`--no-cache` only decides how client requests are served afterwards: a forced pull while the upstream is reachable, the cached copy if that pull fails, and never a pull when air-gapped.

**Air-gapped:** list files can't work, so each one is logged at error and recorded `failed`; archives still load.

**The `load` command** is unchanged: it never reads or writes the ledger, so `ociregistry load --images '/mnt/preload/*'` reloads everything, including images a prune removed. `--images` is added, identical to `--image-file`, whose usage becomes "DEPRECATED (will be removed in future) use --images instead".

## Ledger

Each server keeps `<imagePath>/metadata/loaded.yaml`, beside `manifests` and `blobs`, listing every file preload or the watcher processed.

```yaml
version: 1
files:
  - name: /mnt/preload/abc.tgz
    size: 10646
    timestamp: "2026-11-11T01:22:44Z"   # the file's modification time
    processed: "2026-11-18T23:12:31Z"   # when it was processed
    result: loaded                      # loaded | skipped | failed
```

Rules:

- **Match:** a file is skipped when an entry has the same `name` (absolute, cleaned path), `size` and `timestamp` (UTC, one-second precision). Anything else is processed, and its entry replaced.
- **Results:**
  - `loaded`: no errors, at least one image added or replaced;
  - `skipped`: no errors, nothing changed (all cached with the same digest, or no image for the server's platform);
  - `failed`: the file couldn't be read, or any image or URL in it failed.
- **Every result is final** until the file changes or the ledger is edited, including `failed`. Images a prune removes are never reloaded by preload or the watcher: in the common case, `foo:v1.0.0` is pruned after workloads move to `foo:v2.0.0`, and reloading it would only be pruned again.
- **Writes:** atomic (temp file then rename), after each file. A mutex guards the in-memory copy; only preload (before serving) and the single watcher goroutine write it.
- **Cleanup:** entries for files that no longer exist are removed on each pass.
- **Damaged ledger:** a file that can't be read or parsed, or has a version above 1, is logged at **error**, so it shows with `--log-level error`, and treated as empty: everything is processed again. Decoding is strict (unknown keys are an error).
- **Operator control:** the ledger is read once, at startup. To reprocess files, edit or delete `loaded.yaml` and restart the server. A new or rebuilt cache has no ledger, so it processes everything.
- `serialize.CreateDirs` creates `metadata`. Nothing that walks the cache looks outside `manifests` and `blobs`.

## Requirements by package

No API changes: the `PUT /cmd/tarball` endpoint, token settings and validator change from the earlier step 1 patch are dropped.

| Package                      | Requirement |
| ---                          | --- |
| `impl/globals`               | `MetadataPath = "metadata"`. |
| `impl/serialize`             | `CreateDirs` also creates `metadata`. |
| `impl/config`                | `WatchFrequency string` (`watchFrequency`) and `FromCmdLine.WatchFrequency`; validated in `SetConfigFromStr` as a positive duration; `GetWatchFrequency() time.Duration`, 0 meaning off; merged like other `serve` settings. |
| `impl/cmdline`               | `serve --watch-frequency` (string, validated). `load --images`, same destination and config field as `--image-file`; `--image-file` usage becomes "DEPRECATED (will be removed in future) use --images instead"; giving both is an error. Usage strings 85 characters or less. |
| `impl/preload`               | `ResolveFiles(paths []string, opts ResolveOpts) ([]ResolvedFile, error)`: comma list, globs and (new) directories, one level; regular files only; `*.part` skipped; detects tarballs by content; `TarballsOnly`, `AllowNoMatch`. `Load` (the `load` command) uses it, otherwise unchanged and ledger-free.       |
| `impl/preload` (`ledger.go`) | Read, match, record, prune missing entries, write `loaded.yaml` atomically; damaged ledger logged at error and treated as empty. |
| `impl/preload` (`preload`)   | New `Preload(spec, resolveRef)` for `serve`: `ResolveFiles` with `AllowNoMatch`, info log for no matches, skip ledgered files, record each outcome. Disk-only paths (`LoadTarball`, `LoadFromListFile`), as today. |
| `impl/preload` (`watch.go`)  | `RunWatcher(spec, interval, stopCh, stoppedCh)`, like `RunPruner`: one goroutine, the pass described in Behavior, stop checked between files. Archives via `cache.LoadTarball`; list files via the list-file path below. Never calls the disk-only preload functions. |
| `impl/cache` (`tarball.go`)  | From step 1, unchanged: `LoadTarball`, `loadFromTarball`, `storeFromTarball`, `TarballResult`. |
| `impl/cache`                 | `CachedDigest(url string) (string, bool)`, under `mc`, for the list-file comparison. |
| watcher list-file path       | Per URL: upstream `HEAD`; same digest as `CachedDigest` → skip; else `cache.GetManifest(pr, imagePath, pullTimeout, true)`; for an image list, the same for the server's platform image by digest URL. |
| `cmd/subcmd/serve.go`        | `Preload` instead of `preload.Load`; start the watcher after `cache.Load`; at shutdown stop it before `WaitPulls`. |
| `cmd/ociregistry.go`         | `load` uses `--images`/`--image-file` as before. |
| `mock`                       | `DockerSaveTarball` from step 1, for tests. |

## How a load joins the running cache

The watcher uses the same pieces as a client request (`enqueuePull`, `signalWaiters`, `pullGate`, `replaceInCache`), so loads, pulls and prunes stay consistent. Preload runs before the server accepts requests and needs none of this.

For each manifest in an archive, in the order the reader yields them (an image list before its images), `loadFromTarball`:

1. Calls `enqueuePull(url)`. If another goroutine is pulling the URL (`mustWait`), waits up to `pullTimeout` and goes back to step 1, so the archive's manifest is the one left in the cache. A timeout records that image as an error.
2. As the puller (`mustPull`), defers `signalWaiters(url)`: client requests for the URL wait for the archive's manifest instead of pulling from the upstream.
3. Takes `pullGate` shared; no sweep can run until it's released.
4. Under `mc`, compares with the cached manifest: same digest → `cached`, nothing written.
5. Sets `Created`, saves the blobs (`SaveBlobs`, image manifests only), then writes the manifest atomically.
6. `replaceInCache`: checks the blobs, adds the new references, removes the old entry's, updates the entry and aliases.
7. Releases the gate; the deferred signal wakes the waiters, who read the archive's manifest.

After the archive, if any manifest replaced one with a different digest, `trySweepBlobs` removes blobs left without references, or leaves them for the next prune if a pull or prune is in progress.

Other points:

- **Prune:** the gate is held per image, so a prune can run between images. During a very large image it waits up to `pruneLockWait`, gives up with `ErrPruneBusy` and retries next interval, as during a long upstream pull.
- **One URL at a time** is enqueued, so there's no deadlock against a request waiting on another URL.
- **List files** go through `cache.GetManifest` with a forced pull, which applies the same protocol.
- **Shutdown:** the watcher stops between files, before `WaitPulls`. An archive in progress finishes. If the pod is killed partway, the file has no ledger entry, so the next start processes it again and skips images already cached.
- **A file replaced while being read:** on NFS or EFS a read from another client can fail with a stale handle. The file is recorded `failed` with its old size and timestamp; the replacement differs, so it's processed next pass. Producers should write `*.part` and rename.

## Implementation strategy

Six patches, each against the then-current main, each with its tests and passing `go test -race ./...`, so each can be reviewed and merged on its own:

1. **`load --images`:** the new flag and the deprecated `--image-file` text. Independent of the rest.
2. **`ResolveFiles`:** factor `expandPaths` and `looksLikeTar` out of `preload.Load` into the shared function, adding directories, `*.part` and the two options. `load` behaves as today.
3. **Ledger and `Preload`:** `MetadataPath`, `CreateDirs`, `ledger.go`, and `Preload` in `serve` with the info log for no matches. After this, restarts skip ledgered files.
4. **Cache tarball path:** `impl/cache/tarball.go` and `mock/tarball.go` from step 1, without the endpoint, plus `CachedDigest`.
5. **Watcher:** `watch.go`, `watchFrequency` and `--watch-frequency`, and the `serve.go` start and stop.
6. **Docs:** preload, watch, `load --images`, the config reference and release notes.

Estimated size: about 450 lines of production code and 550 of tests, of which about 160 and 230 already exist in step 1.

Build and test: I test with Go 1.25 in a scratch copy (Go 1.27 can't be downloaded here), with the one Go 1.26+ call in `listcache.go` replaced there only. Before merging, run `go test ./...` with Go 1.27.4 and `make staticcheck`.

## Tests

Each patch's key tests are also checked by removing the safeguard they cover and confirming they fail.

| Patch | Tests |
| --- | --- |
| 1 `load --images` | both flags set the same field; both given is an error; usage text |
| 2 `ResolveFiles` | comma lists, globs, directories (one level), `*.part`, non-regular files, content detection, `TarballsOnly`, `AllowNoMatch`, no-match error otherwise; existing `load` tests unchanged |
| 3 ledger, `Preload` | round trip; match on name, size and timestamp; a changed file processed again; missing files removed; damaged, unknown-version and unknown-key ledgers logged at error and treated as empty; atomic write; `Preload` skips ledgered files and records loaded, skipped and failed; no matches logged at info |
| 4 cache tarball path | the step 1 tests: load and reload, gzip, platform filter, shared-layer references, moved tag and sweep, `resolveRef`, malformed archive, waiting for a pull in progress and its timeout, a client request getting the archive's manifest; `CachedDigest` |
| 5 watcher | a new file waits one pass then loads; a file still changing waits; ledgered files skipped; a replaced file reloaded; a list file with an unchanged digest skipped and a moved tag replaced (mock upstream); air-gapped list file `failed` with processing continuing; stop between files; ledger written after each file |

## Docs and release notes

Docs to update:

- **Preload:** directories accepted; the ledger, what it skips, and how to reprocess (edit or delete `loaded.yaml`, restart); prune isn't undone; no matches is no longer an error.
- **Watch (new page):** `watchFrequency`; read-only mounts; the `*.part`-then-rename convention, required on NFS and EFS; several clusters sharing one preload volume, each with its own ledger; air-gapped behavior; `--no-cache` and already-cached behavior (the table above).
- **`load`:** `--images`; `--image-file` deprecated; `load` ignores the ledger.
- **Config reference:** `watchFrequency`.

Release notes:

- New: the preload watcher (`watchFrequency`, `--watch-frequency`).
- New: the preload ledger, `<imagePath>/metadata/loaded.yaml`. On upgrade, the first start has no ledger, so preload processes every file once, as before, and records it.
- Changed: a preload path or glob matching no files is logged instead of stopping `serve`.
- Changed: archives a server has already processed aren't read again at restart, so images pruned since stay pruned.
- New: `load --images`; `--image-file` deprecated.

## Open decisions

- [ ] **Preload when a file fails.** Today one bad file stops `serve`. With a ledger, the failure is recorded, so the next start skips it and starts anyway, which makes stopping only on the first start inconsistent. Recommendation: log at error, record `failed` and continue, in `Preload` as in the watcher.
- [ ] **`resolveRef` and the watcher.** `resolveRef` renames the one image in a single-image archive. Applied by the watcher, every new archive's image would get the same name. Recommendation: the watcher never applies it, with an info log at startup when both are set; `Preload` keeps applying it as today.
- [ ] **Minimum `watchFrequency`.** Recommendation: reject below 5s, so a typo such as `30ms` doesn't make the server scan the shared volume continuously.
- [ ] **Transient failures are final.** An upstream outage during a list file, or a pull-timeout while waiting on a pull in progress, records `failed` until the file changes or the ledger is edited. Recommendation: accept, and document it under operator control.
