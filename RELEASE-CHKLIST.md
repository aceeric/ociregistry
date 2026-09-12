# Release Steps

| Step                                                              | Command                                    |
|-------------------------------------------------------------------|--------------------------------------------|
| Download and install latest go version                            | `sudo make update-go`                      |
| Update golang if new go version is available                      | `make bump-go GO_VERSION=n.nn.n`           |
| Update dependencies                                               | `make update-modules`                      |
| Bump chart version                                                | `make bump-chart CHART_VERSION_NEW=n.nn.n` |
| Update changelog                                                  | manual                                     |
| Commit to main                                                    | manual                                     |
| Tag n.nn.n                                                        | manual                                     |
| Push main branch and tag                                          | manual                                     |
| .github/workflows/chart.yml publishes chart to quay.io            | github workflow                            |
| .github/workflows/server.yml publishes container image to quay.io | github workflow                            |
| Locally, build the server and update the systemd service          | `make server`, `sudo make server-install`  |
