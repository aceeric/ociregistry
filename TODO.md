# TODO

- Finish API
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

ALL ACCEPT COUNT WITH DEFAULT 10

## Patch older manifest

On the file system:
```shell
sed  -i -e 's/}}$/},"Created":"2025-04-01T22:07:01","Pulled":"2025-04-01T22:08:34"}/' /var/lib/ociregistry/images/img/* /var/lib/ociregistry/images/fat/*
```
