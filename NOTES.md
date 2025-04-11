
 2. Resolve all TODO
 3. --hello-world
 4. Replace yaml https://github.com/goccy/go-yaml if possible (may be sub-dependency)
 5. Implement sub-commands (serve, prune, load)
 6. Implement pruning vi cmd
 7. Rework concurrent load / pre-load
 8. --air-gapped
 9. --config  path-to-config-file (what do to about current config...)
10. Consider https://github.com/urfave/cli


curl http://localhost/cmd/prune/accessed&dur=1d
curl http://localhost/cmd/prune/created&dur=1d
curl http://localhost/cmd/prune/pattern?kubernetesui/dashboard:v2.7.0&dryRun=true

server serve --config-path --port --preload-images <file> --os --arch --pull-timeout --always-pull-latest --concurrent
server load  --config-pat                                 --os --arch --pull-timeout                      --concurrent <file>
server list
server prune --before --pattern --dry-run
server version

--config-path > --registry-config

global --log-level --image-path  (--cache-path)

config:
  created: 1d
  accessed: 1d
  frequency: 1h
  count: 10


--prune: '{"created": "1d", "accessed": "1d", "freq": "1h", "count": "10"}'

```yaml
# configuration of ociregistry
---
imagePath: /var/lib/ociregistry
port: 8080
os: linux
arch: amd64
pullTimeout: 
alwaysPullLatest: false
airGapped: false
helloWorld: false
concurrent: TODO
registries:
  - name: localhost:8080
    description: The ociregistry server running on my desktop
    scheme: http
pruneConfig:
  enabled: false
  duration: 30d
  type: accessed
  frequency: 1d
  count: -1
  dryRun: false
```