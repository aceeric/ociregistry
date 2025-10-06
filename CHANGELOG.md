# CHANGELOG

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
