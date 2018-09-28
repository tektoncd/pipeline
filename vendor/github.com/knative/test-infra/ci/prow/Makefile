# Copyright 2018 The Knative Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

CLUSTER       ?= prow
PROJECT       ?= knative-tests
ZONE          ?= us-central1-f
JOB_NAMESPACE ?= test-pods

PROW_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

get-cluster-credentials:
	gcloud container clusters get-credentials "$(CLUSTER)" --project="$(PROJECT)" --zone="$(ZONE)"

update-config: get-cluster-credentials
	kubectl create configmap config --from-file=config.yaml=config.yaml --dry-run -o yaml | kubectl replace configmap config -f -

update-plugins: get-cluster-credentials
	kubectl create configmap plugins --from-file=plugins.yaml=plugins.yaml --dry-run -o yaml | kubectl replace configmap plugins -f -

update-boskos: get-cluster-credentials
	kubectl apply -f boskos/config.yaml

update-boskos-config: get-cluster-credentials
	kubectl create configmap resources --from-file=config=boskos/resources.yaml --dry-run -o yaml | kubectl --namespace="$(JOB_NAMESPACE)" replace configmap resources -f -

update-cluster: get-cluster-credentials
	kubectl apply -f cluster.yaml

test:
	bazel run @k8s//prow/cmd/config -- --plugin-config=$(PROW_DIR)/plugins.yaml
	bazel run @k8s//prow/cmd/config -- --config-path=$(PROW_DIR)/config.yaml
