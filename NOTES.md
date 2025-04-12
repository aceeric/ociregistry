

TOPLINE:
- do not support hot reload config at this time ?????
- if config provided load it
- if args provided fold them into config
- mod reg lookup to use new struct
- mod prune runner to use new struct



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

--pull-timeout
--always-pull-latest


               port  preload-images  os  arch  pull-timeout  always-pull-latest  concurrent  hello-world  air-gapped
               ----  --------------  --  ----  ------------  ------------------  ----------  -----------  ----------
server serve    X       X            X    X         X        X                                    X           X
server load                          X    X         X                              ?
server list                                
server prune   (see below)                            
server version            

global: --log-level / --config-path / --image-path


or ociregistry prune --duration 1d --type accessed --dry-run --regex  (if type=pattern)


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


