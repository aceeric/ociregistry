# CHANGELOG

## 1.11.1

21-Dec-2025

1. Fix load/preload by digest (#27)

## 1.11.0

20-Dec-2025

1. Implement Amazon ECR token-based auth (#26).

## 1.10.0

18-Dec-2025

1. Handle up to three levels of repository segments including in-path namespace (total 4 path components). Example: `docker pull ociregistry:8080/registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:x86_64-v18.5.0` (#24).

## 1.9.9

06-Dec-2025

1. Go from 1.25.4 to 1.25.5

## 1.9.8

05-Dec-2025

1. Support a default namespace, e.g. `pull localhost:8080/hello-world:latest` from Dockerhub.
2. Clone the default transport to avoid leaking goroutines (#22).
3. Implement observability using Prometheus/Grafana.
4. Implement and document a load test (#23).

## 1.9.7

11-Nov-2025

1. Do not set blob content length header if the client sends the `Range` header in the http request. Fixes problem when containerd is configured for chunking. Delegates chunking to the go `http` package
1. Go from 1.25.1 to 1.25.4
1. Remove un-needed `coverage` target in workflow
1. Support registry password as env var (#21)
1. When logging is set to `--log-level=debug`, log HTTP request headers
1. Support a default namespace (e.g. `--default-ns=docker.io`) allowing `docker pull ociregistry:8080/hello-world` to pull from `docker.io` (for example)
1. Misc. documentation improvements

## 1.9.6

06-Oct-2025

1. Fix bug in bearer auth.

## 1.9.5

27-Sep-2025

1. Module Updates: `aceeric/imgpull=v1.12.2`, `getkin/kin-openapi=v0.133.0`, `labstack/echo/v4=v4.13.4`, `oapi-codegen/runtime=v1.1.2`, `urfave/cli/v3=v3.4.1`
1. Switch from `gopkg.in/yaml.v3` to `go.yaml.in/yaml/v4`
1. Don't check for writable cache dir when running `ociregistry list` command
1. Go from `1.25.0` to `1.25.1`
1. Improve help
1. Support -1 count for "all" on `cmd` REST methods (e.g. `curl "http://localhost:8080/cmd/manifest/list?count=-1"`)
1. Add documentation using Mkdocs hosted on GitHub pages - https://aceeric.github.io/ociregistry
1. Add this change log

## 1.9.4

08-Sep-2025

1. Go formatting
1. Fix typo in source code docs
1. Helm chart support fot priorityClassName (#16)
