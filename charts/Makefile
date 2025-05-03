CHART_VERSION ?= 1.8.1

.PHONY : all
all: docs package push

.PHONY: docs
docs:
	helm-docs

.PHONY: package
package:
	helm package ociregistry --app-version $(CHART_VERSION)

.PHONY: push
push:
	helm push ociregistry-$(CHART_VERSION).tgz oci://quay.io/appzygy/helm-charts

.PHONY: artifacthub
artifacthub:
	oras push\
	 quay.io/appzygy/helm-charts/ociregistry:artifacthub.io\
	 --config /dev/null:application/vnd.cncf.artifacthub.config.v1+yaml\
	 artifacthub-repo.yml:application/vnd.cncf.artifacthub.repository-metadata.layer.v1.yaml
