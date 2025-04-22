# TODO

1. Readthedocs
   - concurrency design
   - volumetrics
2. Instrumentation
3. Enable swagger UI (https://github.com/go-swagger/go-swagger)?
4. bin/imgpull is inserting "library", on "docker.io" pulls should it? (Would it work otherwise?)

## Patch older manifest

On the file system:
```shell
sed  -i -e 's/}}$/},"Created":"2025-04-01T22:07:01","Pulled":"2025-04-01T22:08:34"}/' /var/lib/ociregistry/images/img/* /var/lib/ociregistry/images/fat/*
```
