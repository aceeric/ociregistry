# IN PROGRESS

- use issues
- Implement HEAD /v2/
  WARN[0054] echo server HEAD:/v2/ status=405 latency=3.434µs host=localhost:8088 ip=127.0.0.1 
- preload has confusing message "loaded 106 images to the file system cache " doesn't actually
  match the image count
- handler pullAndCache has a problem. The mem cache is updated outside of a "transaction"
  that pulls the image so - multiple pulls could overlap. upstream.Get should take an optional
  func pointer that updates the in-mem cache BEFORE the puller goroutine signals waiters.
- doneGet - refactor like imgpull

# TODO

- Support upstream encoded into image url in case its not possible to configure containerd. E.g.:
  - `image: in-cluster-mirror:8181/gcr.io/google-containers/echoserver:1.10`
  - requires doubling the API...
- Support multiple architectures for pre-loading `--arch=choice1,choice2`
- Special handling for `latest` tag?
- OTEL instrumentation
  - number of images
  - file system storage size
  - memory size
  - pulls over time
  - remote IPs
- Base URL support (already in echo scaffolding)
- Support `--log-request-headers`
- add CMD api to the openapi spec
- enable swagger UI