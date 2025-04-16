# TODO

- Test continuous pruning ("Prune test" below)
- Command API: add to oapi spec?
- Instrumentation
- Update Go version to latest
- Enable swagger UI (https://github.com/go-swagger/go-swagger)?
- bin/imgpull is inserting "library", on "docker.io" pulls should it? (Would it work otherwise?)
- Resolve all TODO
- low: impl/cache/cache.go - on force pull don't delete blobs BUT - then need a background orphaned blob cleaner
  - if you set always pull latest, maybe its just inefficient
- low: base URL support? (Echo supports it...)

## Prune Test

- Start docker daemon
- run registry
- load several images
  - docker pull / docker tag / docker push
- configure ociregistry for local registry (https/insecure)
- configure ociregistry continuous prune
- start ociregistry tee logs
- start multiple consoles running imgpull cycling through all 10 images randomly
- let run for some period
- stop imgpull consoles
- stop ociregistry
- examine logs

## Prune API

```shell
curl -X POST "http://localhost:8080/cmd/prune/accessed?dur=1d&dryrun"
curl -X POST "http://localhost:8080/cmd/prune/created?dur=1d&dryrun"
curl -X POST "http://localhost:8080/cmd/prune/regex?kubernetesui/dashboard:v2.7.0&dryrun"
```

## Admin API

```shell
curl -X GET "http://localhost:8080/cmd/manifest/list?<pattern>"
curl -X GET "http://localhost:8080/cmd/blob/list?<pattern>"
curl -X GET "http://localhost:8080/cmd/image?<pattern> <- get matching manifests and blobs"
curl -X PUT "http://localhost:8080/cmd/manifest/patch?created=2025-04-01T22:08:34"
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
