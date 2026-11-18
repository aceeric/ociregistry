
Sounds good. Please confirm common understanding:

1.  Implementing OCI spec push endpoints is too complex for this simple edge server
2.  New server api command PUT/cmd/tarball - calls new code path similar to cache.pullToCache -> cache.pull
3.  New watch functionality is mostly for airgap or emergency outage conditions
3a. Honors contract of existing variable `cp concurrentPulls` to interoperate with OCI spec /v2/ APIs
3b. Set `ExcludeRequestBody` so validator accepts the body
4. Current `load` sub-command and `serve --preload` untouched
5. New sub-command `watch`. Needs args:
5a.  `--port <number>` - default 8080
5b.  `--host <hostname>` - default localhost - operator can run the watcher on a different processor than server
5c.  `--watch-frequency <value>` - takes go intervals 30s, 10m, 1h etc. - not re-entrant - an event that fires into a running load is ignored
5d.  `--token <value>` - literal token
5e.  `--token-env <value>` - name of an env var with a token
5f. Uses all global options.
5g. When watcher and server are not in the same processor (e.g. pod/host), operator SHOULD use the token for security. `--token` and `--token-env` are added as new `serve` sub-command args used by the new PUT command endpoint if defined. (config.yaml needs `token`, `tokenEnv`, `watchFrequency` as top-level entities)
5h. Token passed by watcher and read by server as headers. Consider `Authorization: Bearer <token>`
6. If possible put as much cache-related code as possible into new file `watcher.go` to separate from core cache functions

Please let me know if you agree or see different.

I've made minor changes to the main branch. Can you re-pull main, or should I re-upload the project files?

