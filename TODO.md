# TODO

1. Command API:  /v2 and add to oapi spec
1. Re-implement pruning vi cmd - just call API
1. Implement --hello-world
1. impl/cache/cache.go - on force pull dont delete blobs BUT - need a background goroutine to clean orphaned blobs
1. Instrumentation
1. Base URL support
1. Enable swagger UI
1. Resolve all TODO

## Prune 

```
POST
curl http://localhost/cmd/prune/accessed?dur=1d&dryRun
curl http://localhost/cmd/prune/created?dur=1d&dryRun
curl http://localhost/cmd/prune/regex?kubernetesui/dashboard:v2.7.0&dryRun

CLI

type is accessed or created --dry-run is always supported

ociregistry prune --type ? --duration ? --dry-run
ociregistry prune --type ? --duration ? --dry-run
ociregistry prune --regex foo --dry-run
```