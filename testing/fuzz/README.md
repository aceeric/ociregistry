# ociregistry-fuzz

A small, dependency-free Go command-line tool for testing that the OCI
Distribution Server's REST API is resilient to malicious/malformed
requests: it should never do anything other than return a sane HTTP status
(typically `404`) for paths/parameters it doesn't recognize, and it
should never crash or hang.

No external Go modules are required (stdlib only), so `go build` works
fully offline.

## Modes

Exactly one of the following must be given:

### `--generate-endpoints`

Reads an OpenAPI 3.0.3 spec (`--spec`, defaults to `ociregistry.yaml`) and
writes a large set of test request lines to `--out` (default
`generated_urls.txt`, **overwritten**). For every defined operation it
generates:

1. One "baseline" (well-formed) request.
2. One-at-a-time malicious mutations of every path parameter (empty,
   oversized, path traversal, null bytes, SQL/HTML/shell injection,
   encoding tricks, Unicode tricks, malformed digests, etc. - see
   `fuzzcorpus/corpus.go`).
3. Structural mutations of the path itself: dropping a segment (does the
   router silently match a *different*, shallower registered route?),
   duplicating a segment, and inserting an extra bogus segment. This is
   exactly the "does `/v2/pdq/123/manifests/x` get treated the same as
   some other valid path" class of bug.
4. One-at-a-time fuzzing of every declared query parameter, using
   type-aware values for `integer`-typed parameters.
5. Omission of required query parameters.
6. Extra/unknown query parameters and duplicate query parameters.

`GET /cmd/stop` actually shuts the server down, so it is **always excluded
by default** - not just its baseline request, but *every* line that would
otherwise be generated for it (including things like "hit it with an
unknown query param", since that still resolves to the real route). Use
`--exclude-path` to exclude additional operations the same way (e.g.
`--exclude-path DELETE:/cmd/prune` to protect cached data during a run
against a shared instance), and `--no-default-excludes` if you actually
want `/cmd/stop` generated (e.g. against a disposable/sandboxed instance).
A `# SKIPPED (excluded): ...` comment is written in place of the excluded
operation's requests so it's clear from the file what was left out and
why.

`DELETE /cmd/prune` deletes images by default (the server treats a
missing/malformed `dryRun` as `false`, i.e. "prune for real"), so it gets
dedicated, safety-first handling rather than the generic query-fuzzing
path: **every generated request forces `dryRun=true`**, except the
handful of cases that specifically fuzz `dryRun`'s own value (garbage,
`false`, `0`, etc.) - those instead rely on a second, independent safety
net (`type=pattern` with an `expr` engineered to match no real manifest),
so even a misparsed `dryRun` can't delete anything. `type`, `dur`, and
`expr` are otherwise fuzzed with values aware of their documented
semantics (the three valid `type` enum values plus invalid ones; duration
strings with `d`/`h`/`m` units, bad units, negative/huge/decimal
variants; and regex patterns including classic ReDoS shapes like
`(a+)+$`, since `expr`/`pattern` are compiled as Go regexes server-side).
The same regex-aware and digest-aware fuzzing is applied to `/cmd/image/list`'s
`pattern`/`digest`, `/cmd/blob/list`'s `substr`, and `/cmd/manifest/list`'s
`pattern` - none of those are destructive, so they don't need the
`dryRun`-style protection.

It also samples a set of **undefined** endpoints: a fixed list of common
scanner-style probe paths (`/.git/config`, `/.env`, `/admin`, `/v2/../v2/`,
...) plus randomly generated `/v2/...`-shaped and fully-random paths, using
random HTTP methods.

```
ociregistry-fuzz --generate-endpoints \
  --spec ociregistry.yaml \
  --host localhost:8080 \
  --stress-level 20 \
  --out generated_urls.txt
```

`--scheme` defaults to (and currently only allows) `http` for this mode.

### `--generate-from-urls FILE`

Reads a file of known-good, currently-working URLs (one per line, blank
lines and `#` comments ignored), e.g.:

```
http://localhost:8080/v2/registry.k8s.io/kube-state-metrics/manifests/v2.19.1
https://localhost:8080/v2/library/alpine/blobs/sha256:2408cc74d12b6cd092bb131...
```

and **appends** subtle alterations of each to `--out`: per-segment
character mutations (case flip, truncate, append, inject a special token,
replace with a fuzz-corpus value, drastically lengthen), structural
mutations (drop/duplicate/swap a segment), tag-or-digest-suffix mutation,
query-string mutations, and trailing-slash toggling. Both `http` and
`https` input URLs are accepted, and **every mutated line always keeps
the exact scheme of its source URL** - the server serves only one scheme
at a time, so a scheme-flipped request would just be a guaranteed
connection failure (and could even spuriously trip `--run-endpoints`'
consecutive-failure halt logic). To test the other scheme, run the
server itself with that scheme and point `--generate-from-urls` at a
urls file using it.

If a mutated line's path still resolves exactly to `/cmd/prune` (this
mode doesn't do per-parameter-aware mutation the way `--generate-endpoints`
does, so this check is path-based, not parameter-based), `dryRun=true` is
forced into its query string unconditionally - even overriding an
explicit `dryRun=false` in the source URL - since that endpoint deletes
images by default. Lines whose path mutation changed the route (e.g.
`/cmd/prun`, `/prune`, `/cmd/prune/prune`) are left alone, since they
won't reach the real handler.

To generate a test file from the images in the server that are digest URLs:

```
curl http://localhost:8080/cmd/manifest/list?count=-1 | grep @sha |\
  sed -E 's/^([^ ]+)@sha256:[0-9a-f]+[[:space:]]+([0-9a-f]+).*/http:\/\/localhost:8080\/\1@sha256:\2/' >|\
  known_good_urls.txt
```

```
ociregistry-fuzz --generate-from-urls known_good_urls.txt \
  --stress-level 20 \
  --out generated_urls.txt
```

### `--run-endpoints`

Reads `--in` (default `generated_urls.txt`), issues each request, and
records **status code and response body size only** (bodies, including
hundred-megabyte blobs, are streamed and discarded, never buffered or
inspected). Comment (`#`) and blank lines are skipped.

Output is CSV (`--results FILE`, default stdout):

```
line_number,method,status,body_size_bytes,elapsed_ms,url
```

`status` is either a numeric HTTP status, `TIMEOUT`, or `ERROR:<message>`.
A request that times out, or fails to connect (connection
refused/reset, DNS failure, etc. - anything that could mean the server
died), counts toward a halt counter; **3 such failures in a row** stop the
run immediately, since the server may have crashed. A subsequent
successful response resets the counter.

```
ociregistry-fuzz --run-endpoints \
  --in generated_urls.txt \
  --timeout 10s \
  --results results.csv
```

## Common flags

| Flag | Default | Applies to | Meaning |
|---|---|---|---|
| `--spec` | `ociregistry.yaml` | generate-endpoints | OpenAPI spec path |
| `--host` | `localhost:8080` | generate-endpoints | target `host:port` |
| `--scheme` | `http` | generate-endpoints | only `http` supported currently |
| `--out` | `generated_urls.txt` | both generate modes | overwritten by generate-endpoints, appended by generate-from-urls |
| `--in` | `generated_urls.txt` | run-endpoints | file to replay |
| `--results` | stdout | run-endpoints | CSV output path |
| `--stress-level` | `1` | both generate modes | 1-100; higher = more/larger/more extreme cases |
| `--timeout` | `10s` | run-endpoints | per-request timeout |
| `--seed` | current time | both generate modes | RNG seed, for reproducible generation |
| `--max-body-bytes` | `0` (unlimited) | run-endpoints | cap on bytes *counted* per response (the rest is still drained) |
| `--exclude-path` | `/cmd/stop` | generate-endpoints | operation(s) to skip entirely; repeatable or comma-separated; `/path` or `METHOD:/path` |
| `--no-default-excludes` | off | generate-endpoints | also generate `/cmd/stop` requests (not recommended against a shared server) |

## Stress level guidance

Roughly, at the included spec's size (20 path templates):

| `--stress-level` | approx. generated file size |
|---|---|
| 1 | ~2 MB |
| 20 | ~5-8 MB |
| 50 | ~12 MB |
| 100 | ~80 MB (includes payloads up to 500 KB) |

Higher levels are appropriate for periodic/thorough runs; keep it low
(1-10) for quick sanity checks or CI smoke tests.

## Notes / design choices

- The spec parser (`spec/spec.go`) is a small, purpose-built parser for
  this project's regular, 2-space-indented, generator-produced OpenAPI
  YAML style - not a general YAML/OpenAPI parser. If the spec's
  formatting changes substantially, the parser may need adjusting. This
  was a deliberate choice to keep the tool dependency-free.
- No request bodies are sent (even for PUT/POST/PATCH): the tool's scope
  is path/query-parameter and routing robustness, not upload-payload
  validation.
- `http.Client.Timeout` covers the entire round trip including reading
  the response body, so an extremely slow (but legitimate) multi-hundred
  megabyte blob transfer could register as a `TIMEOUT` at a short
  `--timeout`; raise `--timeout` if you expect to exercise real large
  blobs via `--generate-from-urls`.

## Building

```
go build -o ociregistry-fuzz .
```
