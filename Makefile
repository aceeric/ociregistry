SERVER_VERSION := 1.0.0
DATETIME       := $(shell date +'%Y-%m-%dT%T')

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

.PHONY: container
container: #oapi-codegen
	docker buildx build --tag ociregistry:$(SERVER_VERSION)\
	 --build-arg SERVER_VERSION=$(SERVER_VERSION)\
	 --build-arg DATETIME=$(DATETIME)\
	 .

.PHONY: push
push:
	@echo TODO

.PHONY : help
help:
	@echo "$$HELPTEXT"

export HELPTEXT
define HELPTEXT
This make file provides the following targets

oapi-codegen  Generates go code in the 'api' directory from the 'ociregistry.yaml'
              Open API schema, and from configuration files in the 'api' directory.

desktop       Builds for desktop testing. After building then: 'bin/server --version'
              to simply run the server on your desktop for testing purposes

container     Builds the server and creates an OCI image in the local Docker
              registry tagged as: ociregistry:v$(SERVER_VERSION)

push          TODO
endef
