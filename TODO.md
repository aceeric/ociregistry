# TODO

- Unit test mock remotes https://medium.com/zus-health/mocking-outbound-http-requests-in-go-youre-probably-doing-it-wrong-60373a38d2aa
- Support upstream encoded into image url in case its not possible to configure containerd. E.g.:
  - `image: in-cluster-mirror:8181/gcr.io/google-containers/echoserver:1.10`
  - requires doubling the API...
- Support multiple archictures for pre-loading `--arch=choice1,choice2`
- Special handling for `latest` tag?
- Propagate errors better
- Logging cleanup
- For crane download share the cache with the blob cache to improve performance
- OTEL instrumentation
- e2e tests with docker
- Config reloader. Support program args in config to change log level without restart
- Helm chart
- Base URL support (already in echo scaffolding)
- grep "TODO"
- Support `--log-request-headers`
- add CMD api to the Swagger spec
