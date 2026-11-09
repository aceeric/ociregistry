# Release Steps

| Step                                                     | Command                                    |
|----------------------------------------------------------|--------------------------------------------|
| Download and install latest go version                   | `sudo make update-go`                      |
| Update golang if new go version is available             | `make bump-go GO_VERSION=n.nn.n`           |
| Update dependencies                                      | `make update-modules`                      |
| Bump chart version                                       | `make bump-chart CHART_VERSION_NEW=n.nn.n` |
| Update changelog (add a `## n.nn.n` section)             | manual                                     |
| (Optional) Preview the GitHub release notes              | `make release-notes SERVER_VERSION=ref`    |
| Commit to main                                           | manual                                     |
| Tag n.nn.n (use n.nn.n-xxx for a pre-release)            | manual                                     |
| Push main branch and tag                                 | manual                                     |
| `.github/workflows/release.yml` runs automatically       | github workflow                            |
| Locally, build the server and update the systemd service | `make server`, `sudo make server-install`  |
