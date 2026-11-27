Sounds good. Please confirm common understanding:

1.  Implementing OCI spec push endpoints is too complex for this simple edge server
2.  New server api command PUT/cmd/tarball - calls new code path similar to pullToCache -> doPull
3.  New watch functionality is mostly for airgap or emergency outage conditions
3a. Honors contract of existing variable `cp concurrentPulls` to interoperate with OCI spec /v2/ APIs
3b. Set `ExcludeRequestBody` so validator accepts the body. `api/ociregistry.yaml` needs a `requestBody` for
    the new path, application/octet-stream, so the generated code and the validator know the body is binary.
4. Current `load` sub-command and `serve --preload` untouched
5. New sub-command `watch`. Needs args:
5a.  `--port <number>` - default 8080
5b.  `--host <hostname>` - default localhost - operator can run the watcher on a different processor than server
5c.  `--watch-frequency <value>` - takes go intervals 30s, 10m, 1h etc. - not re-entrant - an event that fires into a running load is ignored
5d.  `--token <value>` - literal token
5e.  `--token-env <value>` - name of an env var with a token
ADD  `--path <path>` the watched directory
5f. Uses all global options.
5g. `--token` and `--token-env` are added as new `serve` sub-command args used by the new PUT command endpoint if defined. (config.yaml needs `token`, `tokenEnv`, `watchFrequency` as top-level entities). If defined - then required by the server. If not defined then only incoming requests from localhost are accepted. Docs should recommend TLS for remote watcher since token is clear text. Affects redacted config output.
5h. Token passed by watcher and read by server as headers. Consider `Authorization: Bearer <token>`
6. If possible put as much cache-related code as possible into new file `watcher.go` to separate from core cache functions

Please let me know if you agree or see different.

My changes:

- Let's keep `--token <value>` for emergencies, rely on DevOps to know when to use it. Do support `--token-file` (mounted secret use case)
- File handling (1) Agree your time-based approach to detect copy completion
- File handling (2) Add bool flag `--delete-on-load` (default false) if set then delete after successful load to conserve space
- File hanlding (3) Also need .skipped extension right?

I've made minor changes to the main branch. Can you re-pull main, or should I re-upload the project files?



Option   name
A        in-server watcher
B        "
C        current load cli as sidecar
D        2 put commands
E        1 put command, new subcommand, sidecar, token

I want to re-asses the idea of the in-server watcher. This would embed the same logic as the sidecar. It would be like the background pruner. It would not need any auth or new endpoints. It would integrate into the prune, concurrent pull, and manifest cache lock as you've described, using the same logic for new file detection. It would stage each image tarball before commencing lock and add to cache. This approach it may be simpler. Can you estimate the amount of code for the current option you just designed as compared to an in-server watcher as just now described.



main.realMain -> preload.Load (implements the load sub-command to load direct to disk)
subcmd.Serve  -> preload.Load (loads direct to disk before server starts)
              -> cache.Load (loads from disk into the in-mem cache)

preload.Load  -> preload.LoadFromListFile (load to disk)
                 preload.LoadTarball (load to disk)

watcher.Watch -> TDB

Please perform these additional analyses:

1. the new watcher goroutine will have the same initial logic as preload.Load to list files, detect tarballs, extract tarballs, etc. There should be a common unit for both consumers that takes a list of paths, and returns a list of files - accepting an optional filter or flag to exclude non-image archives. The watcher can independently track size change over time for completion detection.

2. Due to complexity of watch config - let's only allow one arg on the cli `--watch /path`. This enables default behavior. See config.yaml:
```
watch:
  path: "" (default blank, if blank, no watcher) (also if == cfg.PreloadImages then start watcher excluding all preloaded - so preload must return list of processed files, watcher remembers only tarballs)
  onLoad: "move", "delete", "rename" (default=rename, extensions are not configurable)
  moveTo: "" (default blank) (requires onLoad = move, must not equal watch.path)
```

| ARGS                                                                 | PRELOAD EFFECT            | WATCH EFFECT |
| ociregistry serve --preload-images /mnt/preload --watch /mnt/preload | preload from /mnt/preload | init watched files from preload, watch new files |
| ociregistry serve --preload-images /mnt/preload --watch /mnt/watch   | preload from /mnt/preload | watch new files in /mnt/watch |
| ociregistry serve --preload-images /mnt/preload                      | preload from /mnt/preload | None |
| ociregistry serve --watch /mnt/watch                                 | None                      | watch new files in /mnt/watch |
| ociregistry serve --watch                                            | None                      | parse error|
| ociregistry serve --preload-images                                   | Parse error               | None |

3. Watcher args
--watch-on-load values "move", "delete", "rename" (default)
--watch-move-to /to/path (log errors) (default blank)


Response:
- frequency - please add watch.frequency defaulting to 30s - as before - timer doesn't restart until one pass completes
- failed files with move or delete. I think I made it needlessly complex. New rules:
  - disallow watched path == preload path. Supported use case: --preload /mnt/preload --watch /mnt/preload/watch with move to /mnt/preload on success. This accumulates a growing preload list
  - failed - agree rename to .failed in place
  - skipped - agree rename to .skipped in place
  - success - either rename to .loaded, move, or delete - only move or delete if success (don't move skipped - it may already be in the move target)
- moved files - agree keep same name on move (success) - but move collision should be an error because we can't look inside the tar at that time to know if the content collides
- move across filesystems. Agree - behave like linux `mv` which also hides this complexity

Regarding watch move to preload: imagine pod using aws s3 csi driver so preload and watch are mounts over s3 buckets. Evaluate time to process 100 image tarballs - this adds to pod start time on restart, upgrade or autoscale eviction

Newly realized complexity: 2 clusters running ociregistry each with --preload /mnt/preload --watch /mnt/preload/watch and moveto /mnt/preload - solve by first step renaming to <file>.loading which prevents contention. But then 


east
  dev1-eks-cluster
  stage-eks-cluster
prod
  prod1-eks-cluster
  prod2-eks-cluster
  coop-eks-cluster


--preload /mnt/preload --watch


- Lets simplify further: watch is readonly + keep with ledger - hard-coded and not overridable. As you say, operator can delete the preload/watch ledger on any registry if desired to trigger re-process.
- Agree on *.part and also keep time check 
- Since preload and watch now collaborate and neither one re-loads what it ledgers - let's eliminate `--watch` and replace with `--watch-frequency 30s`. So we re-define watch as an extension to preload. Watch and preload operate on the same read-only path, and `config.yaml` reduces to the following for watch:
```
watchFrequency: 30s
```
if no watch frequency then no watcher.
- Let's create new directory peer to `manifests` called `metadata` which holds journal `loaded.yaml` - prefer yaml for this as it is more concise:
```
files:
  - name: /mnt/preload/abc.tgz
    size: 10646
    timestamp: "2026-11-11T01:22:44Z" (file timestamp)
    processed: "2026-11-18T23:12:31Z" (loaded timestamp)
    result: loaded / skipped / failed
  - name: /mnt/preload/pdq.tar
    size: 452323
    timestamp: "2026-01-23T12:11:18Z"
    processed: "2026-11-18T23:14:55Z"
    result: loaded / skipped / failed
  - etc...
```
- New question - can/should 1) load subcmd, 2) serve + preload, 3) serve + watch: honor mutable tags? 1 and 2 do not today. So if watch does, that will be inconsistent.




yq -r '
  (.files[] | [.name, .size, .timestamp, .processed, .result])
  | @tsv
' - <<'EOF' | column -N NAME,SIZE,TIMESTAMP,PROCESSED,RESULT -t -s $'\t'
files:
  - name: /mnt/preload/abc.tgz
    size: 10646
    timestamp: "2026-11-11T01:22:44Z"
    processed: "2026-11-18T23:12:31Z"
    result: loaded
  - name: /mnt/preload/pdq.tar
    size: 452323
    timestamp: "2026-01-23T12:11:18Z"
    processed: "2026-11-18T23:14:55Z"
    result: failed
EOF

Excellent!

Response to Details you added
- agree `version: 1` in `loaded.yaml`
- small change to corrupted ledger - log as error otherwise if `--log-level=error` (common to reduce log output), then log as `warn` could be missed
- agree ledger write timing
- agree remove entries for non-existing files
- slight change no matches at startup - preload `info` log if not files because it may be normal since now watch amends and requires preload
- agree CreateDirs creates metadata
Response to questions
1. prune vs. the ledger - let's solve this use case:
   - operator places foo1.tar with myco.org/foo/foo:v1.0.0
   - loaded by preload (or watch)
   - operator places foo2.tar with myco.org/foo/foo:v2.0.0
   - Workloads upgrade to pull myco.org/foo/foo:v2.0.0
   - myco.org/foo/foo:v1.0.0 ages out via prune
   This will be the common case so - preload should never re-load foo1.tar and undo prune because it will re-prune next pruner run - operator can edit or remove the journal to achieve non-standard behaviors
2. list files handled by watcher - agree process all file types with slight change: in airgap mode list files can never work but tarballs can always work so each list file is a logged error but processing continues
3. Load sub-command and ledger - `ociregistry load` and `ociregistry serve --preload` are orthogonal use cases. The load sub-command should be kept pure and simple - no awareness of journaling, etc. For example operator can `ociregistry load --image-file /mnt/preload/*` and completely reload everything, undo prune, etc. Additionally: `ociregistry load --image-file` arg is mis-named. Please add `--images` to load sub-command that is identical to `--image-file` and update `--image-file` description to "DEPRECATED (will be removed in future) use --images instead"
4. Mutable tags - consistent per the table per our agreement just now:
|    m           | preload  |                                  | watcher  |                                  |
| item           | no-cache | preload cached                   | no-cache | watcher cached                   |
|----------------|----------|----------------------------------|----------|----------------------------------|
| image archives | ignored  | skip if same digest else replace | ignored  | skip if same digest else replace |
| list files     | ignored  | skip if same digest else replace | ignored  | skip if same digest else replace |

Can you now generate a full spec for the change, listing the requirements by package, and the implementation strategy?