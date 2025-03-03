SERVER_VERSION := 1.7.2
DATETIME       := $(shell date -u +%Y-%m-%dT%T.%2NZ)
REGISTRY       := quay.io
ORG            := appzygy
ROOT           := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

.PHONY : all
all:
	@echo Run 'make help' to see a list of available targets

.PHONY: test
test:
	go test -count=1 $(ROOT)/cmd $(ROOT)/impl/... $(ROOT)/mock -v --cover

.PHONY: coverprof
coverprof: test
	go tool cover -html=$(ROOT)/prof.out

.PHONY: oapi-codegen
oapi-codegen:
	oapi-codegen -config $(ROOT)/api/server.cfg.yaml $(ROOT)/ociregistry.yaml
	oapi-codegen -config $(ROOT)/api/models.cfg.yaml $(ROOT)/ociregistry.yaml

.PHONY: desktop
desktop:
	CGO_ENABLED=0 go build -ldflags "-X 'main.buildVer=$(SERVER_VERSION)' -X 'main.buildDtm=$(DATETIME)'"\
	 -a -o $(ROOT)/bin/server $(ROOT)/cmd/*.go

.PHONY: image
image:
	docker buildx build --tag $(REGISTRY)/$(ORG)/ociregistry:$(SERVER_VERSION)\
	 --build-arg SERVER_VERSION=$(SERVER_VERSION)\
	 --build-arg DATETIME=$(DATETIME)\
	 $(ROOT)

.PHONY: push
push:
	docker push $(REGISTRY)/$(ORG)/ociregistry:$(SERVER_VERSION)

.PHONY: publish
publish: oapi-codegen image push

.PHONY : help
help:
	@echo "$$HELPTEXT"

export HELPTEXT
define HELPTEXT
This make file provides the following targets:

test          Runs the unit tests.

coverprof     Runs the unit tests, then runs 'go tool cover' to show coverage in
              a browser window.

oapi-codegen  Generates go code in the 'api' directory from the 'ociregistry.yaml'
              open API schema in the project root, and from configuration files in
              the 'api' directory.

desktop       Builds the server binary on your desktop. After building then:
              'bin/server --help' to simply run the server on your desktop for
              testing purposes. You can also use the server binary as a systemd
              service. See the 'systemd-service' directory for more details.

image         Builds the server OCI image and stores it in the local Docker image
              cache.

push          Pushes the image built in the 'image' step to the '$(REGISTRY)' OCI
              distribution server, in the '$(ORG)' user/org. Requires the
              appropriate push permissions, of course.

publish       Invokes, in order, the 1) oapi-codegen, 2) image, and 3) push
              targets.

endef
