SHELL := /bin/bash

BIN := $(CURDIR)/.bin
PATH := $(abspath $(BIN)):$(PATH)

UNAME_OS := $(shell uname -s)

$(BIN):
	@mkdir -p $(BIN)

KIND := $(BIN)/kind
KIND_VERSION := v0.11.0
$(KIND): | $(BIN)
	@curl -sSLo $(KIND) "https://kind.sigs.k8s.io/dl/$(KIND_VERSION)/kind-$(UNAME_OS)-amd64"
	@chmod +x $(KIND)

CLUSTER_NAME ?= kpoward-cluster
KUBECONFIG ?= $(CURDIR)/.kube/config
export KUBECONFIG

cluster/create: $(KIND)
	$(KIND) create cluster --name $(CLUSTER_NAME) --config testdata/config/cluster.yaml

cluster/delete: $(KIND)
	$(KIND) delete cluster --name $(CLUSTER_NAME)

deploy:
	kubectl apply -f testdata/config/manifest.yaml

wait:
	{ \
	set -e ;\
	while true; do \
		POD_NAME=$$(KUBECONFIG=$(KUBECONFIG) kubectl get pod | grep Running | grep echo | awk '{print $$1}'); \
		if [ "$$POD_NAME" != "" ]; then \
			exit 0; \
		fi; \
		sleep 1; \
	done; \
	}

test: wait
	go test -v -coverprofile=coverage.out ./ -count=1
