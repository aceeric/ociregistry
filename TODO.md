# TODO

- Prune changes (below)
- Test continuous pruning
- Command API: add to oapi spec
- impl/cache/cache.go - on force pull don't delete blobs BUT - then need a background orphaned blob cleaner
- Instrumentation
- Base URL support ?
- Enable swagger UI
- Why: bin/imgpull localhost:8888/docker.io/hello-world:latest deleteme.tar --scheme http
       but cache has: docker.io/library/hello-world:latest
       Should registry try "library" if omitted for docker?
- Resolve all TODO

## Prune changes

1. When pruning, guarantee two removed if two added. If a manifest is keyed by digest but its
   url is by tag then its the second one so in that case the tagged one should also be removed.
   So:
   - If by tag then get the digest and remove
   - If by digest then get the tag and remove
2. getManifestsToPrune should return map keys And ensure BOTH are returned. Process should be driven
   by the map keys

## Prune API

```shell
curl -X POST http://localhost:8080/cmd/prune/accessed?dur=1d&dryRun
curl -X POST http://localhost:8080/cmd/prune/created?dur=1d&dryRun
curl -X POST http://localhost:8080/cmd/prune/regex?kubernetesui/dashboard:v2.7.0&dryRun
```

## Support API

```shell
curl -X GET http://localhost:8080/cmd/manifest/list
curl -X GET http://localhost:8080/cmd/blob/list
```

## Patch older manifest

```shell
sed  -i -e 's/}}$/},"Created":"2025-04-01T22:07:01","Pulled":"2025-04-01T22:08:34"}/' /tmp/images/img/* /tmp/images/fat/*
```
