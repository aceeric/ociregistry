# IN PROGRESS

# TODO

- TLS in mock server
- Support upstream encoded into image url in case its not possible to configure containerd. E.g.:
  - `image: in-cluster-mirror:8181/gcr.io/google-containers/echoserver:1.10`
  - requires doubling the API...
- Support multiple architectures for pre-loading `--arch=choice1,choice2`
- Special handling for `latest` tag?
- For crane download share the cache with the blob cache to improve performance
- OTEL instrumentation
- e2e tests
- Base URL support (already in echo scaffolding)
- grep "TODO"
- Support `--log-request-headers`
- add CMD api to the Swagger spec
