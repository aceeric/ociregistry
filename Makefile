.PHONY: build
build:
	oapi-codegen -config api/server.cfg.yaml ociregistry.yaml
	oapi-codegen -config api/models.cfg.yaml ociregistry.yaml
	go build -o bin/server-http cmd/*.go
