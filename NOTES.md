
 1. Resolve all TODO
 2. --hello-world
 3. Re-implement pruning vi cmd
 4. Load/Preload is broken - SIMPLIFY
 6. impl/cache/cache.go - on forcepull dont delete blobs BUT - need a background goroutine to clean orphaned blobs


POST
curl http://localhost/cmd/prune/accessed?dur=1d&dryRun
curl http://localhost/cmd/prune/created?dur=1d&dryRun
curl http://localhost/cmd/prune/regex?kubernetesui/dashboard:v2.7.0&dryRun

// type is accessed or created --dry-run is always supported

ociregistry prune --type ? --duration ? --dry-run
ociregistry prune --type ? --duration ? --dry-run
ociregistry prune --regex foo --dry-run

```yaml
# configuration of ociregistry
---
imagePath: /var/lib/ociregistry
port: 8080
os: linux
arch: amd64
pullTimeout: 60000
alwaysPullLatest: false
airGapped: false
helloWorld: false
concurrent: 12
registries:
  - name: localhost:8080
    description: server running on the desktop
    scheme: http
pruneConfig:
  enabled: false
  duration: 30d
  type: accessed
  frequency: 1d
  count: -1
  dryRun: false
```


