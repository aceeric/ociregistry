# TODO

- Unit tests! - mocks for remotes https://medium.com/zus-health/mocking-outbound-http-requests-in-go-youre-probably-doing-it-wrong-60373a38d2aa
- Support upstream encoded into image in case its not possible to configure containerd. E.g.:
  `image: in-cluster-mirror.icm.svc.cluster.local:8181/gcr.io/google-containers/echoserver:1.10`
- Support multiple archictures for pre-loading (--arch=choice1,choice2 ?)
- Special handing for "latest"? Why would anyone pull "latest" from an air-gapped registry?
- Modularize
- Use structs more to carry state
- Each handler in its own file?
- Propagate errors better
- Logging cleanup
- OTEL instrumentation?
- e2e tests with docker
- Config reloader. Support program args in config to change log level without restart
- Helm chart
- Base URL support
- grep "TODO"
- add CMD api to the Swagger spec

# DONE

- Command API for graceful stop
