SERVER_VERSION := 1.0.0
DATETIME       := $(shell date -u +%Y-%m-%dT%T.%2NZ)
REGISTRY       := quay.io
ORG            := appzygy

.PHONY : all
all:
	@echo Run 'make help' to see a list of available targets

.PHONY: oapi-codegen
oapi-codegen:
	oapi-codegen -config api/server.cfg.yaml ociregistry.yaml
	oapi-codegen -config api/models.cfg.yaml ociregistry.yaml

.PHONY: desktop
desktop: oapi-codegen
	CGO_ENABLED=0 go build -ldflags "-X 'main.buildVer=$(SERVER_VERSION)' -X 'main.buildDtm=$(DATETIME)'" -a -o bin/server cmd/*.go

.PHONY: image
image: oapi-codegen
	docker buildx build --tag $(REGISTRY)/$(ORG)/ociregistry:$(SERVER_VERSION)\
	 --build-arg SERVER_VERSION=$(SERVER_VERSION)\
	 --build-arg DATETIME=$(DATETIME)\
	 .

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
