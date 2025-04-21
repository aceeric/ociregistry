.PHONY : all
all: docs package index

.PHONY: docs
docs:
	helm-docs

.PHONY: package
package:
	helm package ociregistry

.PHONY: index
index:
	helm repo index --url https://aceeric.github.io/ociregistry-helm .
