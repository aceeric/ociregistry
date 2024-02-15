# Features

1. How to handle "latest". On every pull would have to store the upstream digest, save it with the image. Then on all subsequent pulls, re-get the upstream digest (HEAD) and compare. If different, re-download else serve from cache.
2. Modularize
3. Use structs more to carry state
4. Unit tests
5. Each handler in its own file: handleV2Auth, handleV2Default, handleV2GetOrgImageBlobsDigest, handleV2OrgImageManifestsReference
6. Propagate errors better
7. Improve logging. timestam left, etc. Add file/line:
   - https://stackoverflow.com/questions/58198896/how-to-get-file-and-function-name-for-loggers
   - containerd/vendor/github.com/containerd/log/context.go
8. e2e tests with docker
9. Config reloader. Support program args in config to change log level without restart
10. Helm chart