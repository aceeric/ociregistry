CHART_VERSION := 1.8.0

.PHONY : all
all: docs package index

.PHONY: docs
docs:
	helm-docs

.PHONY: package
package:
	helm package ociregistry --app-version $(CHART_VERSION)

.PHONY: push
push:
	helm push ociregistry-$(CHART_VERSION).tgz oci://quay.io/appzygy/helm-charts
