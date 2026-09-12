ROOT           := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
SERVER_VERSION ?= $(shell git -C $(ROOT) describe --tags --always --dirty 2>/dev/null || echo dev)
GO_VERSION     := $(shell awk '/^go /{print $$2}' ${ROOT}/go.mod)
DATETIME       := $(shell date -u +%Y-%m-%dT%T.%2NZ)
REGISTRY       := quay.io
ORG            := appzygy
CHART_VERSION  := $(shell grep '^version:' ${ROOT}/charts/ociregistry/Chart.yaml | awk '{print $$2}')

.PHONY : all
all:
	@echo Run 'make help' to see a list of available targets

.PHONY: vartest
vartest:
	@echo SERVER_VERSION=$(SERVER_VERSION)
	@echo GO_VERSION=$(GO_VERSION)
	@echo CHART_VERSION=$(CHART_VERSION)

.PHONY: go-install
go-install:
	go install github.com/norwoodj/helm-docs/cmd/helm-docs@latest
	go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
	go install github.com/vladopajic/go-test-coverage/v2@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest

.PHONY: oapi-codegen # requires go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
oapi-codegen:
	oapi-codegen -config $(ROOT)/api/server.cfg.yaml $(ROOT)/api/ociregistry.yaml
	oapi-codegen -config $(ROOT)/api/models.cfg.yaml $(ROOT)/api/ociregistry.yaml

.PHONY: test
test:
	go test $(ROOT)/cmd/... $(ROOT)/impl/... $(ROOT)/mock/...\
	  -coverpkg ./... -covermode=atomic\
	  -v -cover -coverprofile=$(ROOT)/cover.out

.PHONY: coverage
coverage:
	go tool cover -html=$(ROOT)/cover.out

.PHONY: update-modules
update-modules:
	go get -u ./...
	rm go.sum
	sed -i '/\/\/ indirect/d' go.mod
	go mod tidy
	go build ./...
	$(MAKE) test

.PHONY: coverage-rpt
coverage-rpt: # requires go install github.com/vladopajic/go-test-coverage/v2@latest
	go-test-coverage --config=$(ROOT)/.testcoverage.yml

.PHONY: vet
vet:
	go vet $(ROOT)/cmd $(ROOT)/impl/... $(ROOT)/mock

.PHONY: vulncheck # requires go install golang.org/x/vuln/cmd/govulncheck@latest
vulncheck:
	govulncheck -show verbose $(ROOT)/cmd/... $(ROOT)/impl/...

.PHONY: gocyclo
gocyclo:
	gocyclo -over 15 -ignore "merge.go|_test|ociregistry.go" $(ROOT)/cmd $(ROOT)/impl/

.PHONY: server
server:
	CGO_ENABLED=0 go build -ldflags "-X 'main.buildVer=$(SERVER_VERSION)' -X 'main.buildDtm=$(DATETIME)'"\
	 -a -o $(ROOT)/bin/ociregistry $(ROOT)/cmd/*.go

.PHONY: server-install
server-install:
	$(ROOT)/systemd-service/manual-install

.PHONY: image
image:
	docker buildx inspect ociregistry > /dev/null 2>&1 && docker buildx rm ociregistry || :
	docker buildx create --name ociregistry --driver docker-container
	docker buildx build --platform linux/arm64,linux/amd64\
	 --builder=ociregistry\
	 --push --provenance=false --sbom=false\
	 --tag $(REGISTRY)/$(ORG)/ociregistry:$(SERVER_VERSION)\
	 --build-arg SERVER_VERSION=$(SERVER_VERSION)\
	 --build-arg DATETIME=$(DATETIME)\
	 --build-arg GO_VERSION=$(GO_VERSION)\
	 $(ROOT)

.PHONY: old-image # save this for desktop testing
old-image:
	docker buildx build --tag $(REGISTRY)/$(ORG)/ociregistry:$(SERVER_VERSION)\
	 --build-arg SERVER_VERSION=$(SERVER_VERSION)\
	 --build-arg DATETIME=$(DATETIME)\
	 --build-arg GO_VERSION=$(GO_VERSION)\
	 $(ROOT)

.PHONY: helm-docs # requires https://github.com/norwoodj/helm-docs
helm-docs:
	helm-docs --chart-search-root $(ROOT)/charts

.PHONY: helm-package
helm-package:
	helm package $(ROOT)/charts/ociregistry

.PHONY: helm-push
helm-push:
	helm push $(ROOT)/ociregistry-$(CHART_VERSION).tgz oci://quay.io/appzygy/helm-charts

.PHONY: helm-artifacthub # requires https://oras.land/docs/installation/#linux
helm-artifacthub:
	oras push\
	 quay.io/appzygy/helm-charts/ociregistry:artifacthub.io\
	 --config /dev/null:application/vnd.cncf.artifacthub.config.v1+yaml\
	 $(ROOT)/charts/artifacthub-repo.yml:application/vnd.cncf.artifacthub.repository-metadata.layer.v1.yaml

.PHONY: update-go
update-go:
	@$(ROOT)/scripts/update-golang

.PHONY: bump-go
bump-go:
	@test -n "$(GO_VERSION)" || (echo "GO_VERSION required, e.g. make bump-go GO_VERSION=1.27.2"; exit 1)
	sed -i "s/^go .*/go $(GO_VERSION)/" $(ROOT)/go.mod
	go build ./...
	$(MAKE) test

.PHONY: bump-chart
bump-chart:
	@test -n "$(CHART_VERSION_NEW)" || (echo "CHART_VERSION_NEW required, e.g. make bump-chart CHART_VERSION_NEW=1.20.0"; exit 1)
	sed -i "s/^version:.*/version: $(CHART_VERSION_NEW)/" $(ROOT)/charts/ociregistry/Chart.yaml
	sed -i "s/^appVersion:.*/appVersion: $(CHART_VERSION_NEW)/" $(ROOT)/charts/ociregistry/Chart.yaml

.PHONY : help
help:
	@echo "$$HELPTEXT"

export HELPTEXT
define HELPTEXT
This make file provides the following targets:

update-go         Pulls down and installs the latest golang. Must run as sudo. E.g.
                  'sudo make update-go'.

bump-go           Updates the go version in go.mod, builds (with no binary output - just a test
                  build, and runs the unit tests.) E.g.: 'make bump-go GO_VERSION=n.nn.n'

bump-chart        Updates the chart version in charts/ociregistry/Chart.yaml. E.g.:
                  'make bump-chart CHART_VERSION_NEW=n.nn.n'

test              Runs the unit tests.

vet               Runs go vet.

go-install        Runs go install for components needed locally.

vulncheck         Runs govulncheck.
                  Requires 'go install golang.org/x/vuln/cmd/govulncheck@latest'.

gocyclo           Runs gocyclo.

coverage          Runs 'go tool cover' to show coverage of the most recent test run in a browser
                  window. (Does not run the unit tests.)

update-modules    Runs 'go get -u' and 'go mod tidy' and does a validation build/test to ensure
                  nothing about the module updates broke the server.

coverage-rpt      Creates a coverage report of the most recent test run. (Does not run the unit tests.)
                  Requires 'go install github.com/vladopajic/go-test-coverage/v2@latest'

oapi-codegen      Generates go code in the 'api' directory from the 'ociregistry.yaml' open API
                  schema and configuration files in that directory.
                  Requires 'go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest'.

server            Builds the server binary on your desktop. After building then run the server
                  on your desktop: 'bin/ociregistry --help' for testing. You can also run the server
                  binary as a systemd service. See the 'systemd-service' directory for more details.

server-install    Runs systemd-service/manual-install to install the server as a systemd service.
                  Requires sudo. E.g.: 'sudo make server-install'

image             Builds the server '$(SERVER_VERSION)' OCI image and and pushes it to the
                  '$(REGISTRY)' OCI distribution server, in the '$(ORG)' user/org.
                  Requires the appropriate push permissions, of course.

helm-docs         Builds the Helm chart README from values and the README template.
                  Requires 'go install github.com/norwoodj/helm-docs/cmd/helm-docs@latest'.

helm-package      Builds the Helm chart tarball.

helm-push         Publishes the Helm chart to Quay.

helm-artifacthub  Pushes Artifact hub verified publisher file to Quay.
                  Requires https://oras.land/docs/installation/#linux.

endef
