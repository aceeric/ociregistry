# TODO

1. README
3. Help should not validate the cache directory
4. Readthedocs
   - concurrency design
   - volumetrics
5. Instrumentation
6. Enable swagger UI (https://github.com/go-swagger/go-swagger)?
7. bin/imgpull is inserting "library", on "docker.io" pulls should it? (Would it work otherwise?)

## Patch older manifest

On the file system:
```shell
sed  -i -e 's/}}$/},"Created":"2025-04-01T22:07:01","Pulled":"2025-04-01T22:08:34"}/' /var/lib/ociregistry/images/img/* /var/lib/ociregistry/images/fat/*
```
