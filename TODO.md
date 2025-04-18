# TODO

- Instrumentation
- graceful shutdown - after echo stops: wait for prune to finish, wait for GETs to finish (probably with timeout)
- Enable swagger UI (https://github.com/go-swagger/go-swagger)?
- bin/imgpull is inserting "library", on "docker.io" pulls should it? (Would it work otherwise?)
- Resolve all TODO

## Prune API

```shell
curl -X POST "http://localhost:8080/cmd/prune/accessed?dur=1d&dryrun=false" (default true)
curl -X POST "http://localhost:8080/cmd/prune/created?dur=1d&dryrun=false"
curl -X POST "http://localhost:8080/cmd/prune/regex?kubernetesui/dashboard:v2.7.0&dryrun=false"
```

## Admin API

```shell
curl -X GET "http://localhost:8080/cmd/manifest/list?<pattern>"
curl -X GET "http://localhost:8080/cmd/blob/list?<pattern>"
curl -X GET "http://localhost:8080/cmd/image?<pattern> <- get matching manifests and blobs"
curl -X PUT "http://localhost:8080/cmd/manifest/patch/created?2025-04-01T22:08:34"
curl -X PUT "http://localhost:8080/cmd/manifest/patch/accessed?2025-04-01T22:08:34"
```

## Patch older manifest

On the file system:
```shell
sed  -i -e 's/}}$/},"Created":"2025-04-01T22:07:01","Pulled":"2025-04-01T22:08:34"}/' /var/lib/ociregistry/images/img/* /var/lib/ociregistry/images/fat/*
```

## Prune contention

```
blob not in cache cause by:

puller 1 ---> get manifest ----------------------------------> come back for blob (not found)
puller 2 -------------------> lock manifest -> delete blobs -> exit
```
