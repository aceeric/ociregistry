
 1. cli_test should test fat manifests
 2. Resolve all TODO
 3. --hello-world
 4. Replace yaml https://github.com/goccy/go-yaml
 5. Rename `--image-path` `--cache-path` and associated variables
 6. Implement sub-commands (serve, prune, load)
 7. Implement pruning
 8. Rework concurrent load / pre-load
 9. --air-gapped
10. Consider https://github.com/urfave/cli
 

server serve --config-path --port --preload-images <file> --os --arch --pull-timeout --always-pull-latest --concurrent
server load  --config-pat                                 --os --arch --pull-timeout                      --concurrent <file>
server list
server prune --before --pattern --dry-run
server version

global --log-level --image-path  (--cache-path)