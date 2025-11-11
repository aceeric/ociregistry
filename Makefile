SERVER_VERSION ?= 1.9.7
GO_VERSION     ?= 1.25.4
DATETIME       := $(shell date -u +%Y-%m-%dT%T.%2NZ)
REGISTRY       := quay.io
ORG            := appzygy
ROOT           := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
CHART_VERSION  := $(shell grep '^version:' ${ROOT}/charts/ociregistry/Chart.yaml | awk '{print $$2}')

.PHONY : all
all:
	@echo Run 'make help' to see a list of available targets

.PHONY : vartest
vartest:
	@echo SERVER_VERSION=$(SERVER_VERSION)
	@echo GO_VERSION=$(GO_VERSION)
	@echo CHART_VERSION=$(CHART_VERSION)

.PHONY: oapi-codegen # requires go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
oapi-codegen:
	oapi-codegen -config $(ROOT)/api/server.cfg.yaml $(ROOT)/api/ociregistry.yaml
	oapi-codegen -config $(ROOT)/api/models.cfg.yaml $(ROOT)/api/ociregistry.yaml

.PHONY: test
test:
	go test $(ROOT)/cmd/... $(ROOT)/impl/... $(ROOT)/mock/...\
	  -coverpkg ./...\
	  -v -cover -coverprofile=$(ROOT)/cover.out

.PHONY: coverage
coverage:
	go tool cover -html=$(ROOT)/cover.out

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

.PHONY : help
help:
	@echo "$$HELPTEXT"

export HELPTEXT
define HELPTEXT
This make file provides the following targets:

test              Runs the unit tests.

vet               Runs go vet.

vulncheck         Runs govulncheck.
                  Requires 'go install golang.org/x/vuln/cmd/govulncheck@latest'.

gocyclo           Runs gocyclo.

coverage          Runs 'go tool cover' to show coverage of the most recent test run in a browser
                  window. (Does not run the unit tests.)

coverage-rpt      Creates a coverage report of the most recent test run. (Does not run the unit tests.)
                  Requires 'go install github.com/vladopajic/go-test-coverage/v2@latest'

oapi-codegen      Generates go code in the 'api' directory from the 'ociregistry.yaml' open API
                  schema and configuration files in that directory.
                  Requires 'go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest'.

server            Builds the server binary on your desktop. After building then run the server
                  on your desktop: 'bin/ociregistry --help' for testing. You can also run the server
                  binary as a systemd service. See the 'systemd-service' directory for more details.

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
