SERVER_VERSION := 1.5.0
DATETIME       := $(shell date -u +%Y-%m-%dT%T.%2NZ)
REGISTRY       := quay.io
ORG            := appzygy
ROOT           := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

.PHONY : all
all:
	@echo Run 'make help' to see a list of available targets

.PHONY: test
test:
	go test -count=1 ociregistry/cmd ociregistry/impl/extractor ociregistry/impl/helpers ociregistry/impl/memcache\
	  ociregistry/impl/preload ociregistry/impl/pullrequest ociregistry/impl/serialize ociregistry/impl/upstream\
	  ociregistry/impl ociregistry/mock -v --cover

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

test          Runs the unit tests

oapi-codegen  Generates go code in the 'api' directory from the 'ociregistry.yaml'
              open API schema in the project root, and from configuration files in
			  the 'api' directory.

desktop       Builds for desktop testing. After building then: 'bin/server --help'
              to simply run the server on your desktop for testing purposes.

image         Builds the server OCI image and stores it in the local Docker image
              cache.

push          Pushes the image built in the 'image' step to $(REGISTRY).

publish       Invokes oapi-codegen, image, and push.

endef
