#!/usr/bin/env bash

# Copyright 2019 The Tekton Authors
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

# This script calls out to scripts in tektoncd/plumbing to setup a cluster
# and deploy Tekton Pipelines to it for running integration tests.

source $(git rev-parse --show-toplevel)/test/e2e-common.sh


# Setting defaults
PIPELINE_FEATURE_GATE=${PIPELINE_FEATURE_GATE:-stable}
SKIP_INITIALIZE=${SKIP_INITIALIZE:="false"}
RUN_YAML_TESTS=${RUN_YAML_TESTS:="true"}
SKIP_GO_E2E_TESTS=${SKIP_GO_E2E_TESTS:="false"}
E2E_GO_TEST_TIMEOUT=${E2E_GO_TEST_TIMEOUT:="20m"}
RUN_FEATUREFLAG_TESTS=${RUN_FEATUREFLAG_TESTS:="false"}
RESULTS_FROM=${RESULTS_FROM:-termination-message}
KEEP_POD_ON_CANCEL=${KEEP_POD_ON_CANCEL:="false"}
ENABLE_CEL_IN_WHENEXPRESSION=${ENABLE_CEL_IN_WHENEXPRESSION:="false"}
ENABLE_PARAM_ENUM=${ENABLE_PARAM_ENUM:="false"}
ENABLE_ARTIFACTS=${ENABLE_ARTIFACTS:="false"}
ENABLE_CONCISE_RESOLVER_SYNTAX=${ENABLE_CONCISE_RESOLVER_SYNTAX:="false"}
ENABLE_KUBERNETES_SIDECAR=${ENABLE_KUBERNETES_SIDECAR:="false"}

# Script entry point.

if [ "${SKIP_INITIALIZE}" != "true" ]; then
  initialize $@
fi

header "Setting up environment"

set -x
install_pipeline_crd
export SYSTEM_NAMESPACE=tekton-pipelines
set +x

function prepull_common_images() {
  echo ">> Pre-pulling common images on all nodes to reduce flakes from image pulls"
  # Keep the list minimal and focused on frequently used images in examples.
  cat <<'EOF' | kubectl apply -f -
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: tekton-prepull-images
  namespace: kube-system
spec:
  selector:
    matchLabels:
      app: tekton-prepull-images
  template:
    metadata:
      labels:
        app: tekton-prepull-images
    spec:
      tolerations:
      - operator: Exists
      containers:
      - name: pull-alpine
        image: mirror.gcr.io/alpine:latest
        command: ["/bin/sh","-c","sleep 300"]
      - name: pull-ubuntu
        image: mirror.gcr.io/ubuntu:latest
        command: ["/bin/sh","-c","sleep 300"]
      - name: pull-busybox
        image: mirror.gcr.io/busybox:latest
        command: ["/bin/sh","-c","sleep 300"]
      - name: pull-busybox-sha-2f9af5c
        image: busybox@sha256:2f9af5cf39068ec3a9e124feceaa11910c511e23a1670dcfdff0bc16793545fb
        command: ["/bin/sh","-c","sleep 300"]
      - name: pull-busybox-sha-1303dbf
        image: busybox@sha256:1303dbf110c57f3edf68d9f5a16c082ec06c4cf7604831669faf2c712260b5a0
        command: ["/bin/sh","-c","sleep 300"]
      - name: pull-bash
        image: mirror.gcr.io/bash:5.1.4@sha256:c523c636b722339f41b6a431b44588ab2f762c5de5ec3bd7964420ff982fb1d9
        command: ["/bin/sh","-c","sleep 300"]
      - name: pull-bash-latest
        image: mirror.gcr.io/bash:latest
        command: ["/bin/sh","-c","sleep 300"]
      - name: pull-kaniko
        image: gcr.io/kaniko-project/executor:v1.8.1
        command: ["/bin/sh","-c","sleep 300"]
      - name: pull-skopeo-copy-catalog
        image: ghcr.io/tektoncd/catalog/upstream/tasks/skopeo-copy:latest
        command: ["/bin/sh","-c","sleep 300"]
      - name: pull-skopeo-dogfooding
        image: gcr.io/tekton-releases/dogfooding/skopeo:latest
        command: ["/bin/sh","-c","sleep 300"]
      - name: pull-alpine-curl
        image: docker.io/alpine/curl:latest
        command: ["/bin/sh","-c","sleep 300"]
      - name: pull-spire-agent
        image: ghcr.io/spiffe/spire-agent:1.1.1
        command: ["/bin/sh","-c","sleep 300"]
      - name: pull-spiffe-csi
        image: ghcr.io/spiffe/spiffe-csi-driver:nightly
        command: ["/bin/sh","-c","sleep 300"]
      - name: pull-csi-registrar
        image: k8s.gcr.io/sig-storage/csi-node-driver-registrar:v2.0.1
        command: ["/bin/sh","-c","sleep 300"]
      - name: pull-bitnami-memcached
        image: docker.io/bitnami/memcached:1.6.9-debian-10-r114
        command: ["/bin/sh","-c","sleep 300"]
      - name: pull-bitnami-postgres
        image: docker.io/bitnami/postgresql:11.11.0-debian-10-r62
        command: ["/bin/sh","-c","sleep 300"]
      - name: pull-gitea
        image: docker.io/gitea/gitea:1.17.1
        command: ["/bin/sh","-c","sleep 300"]
      hostNetwork: false
      dnsPolicy: ClusterFirst
      restartPolicy: Always
  updateStrategy:
    type: RollingUpdate
EOF

  # Give the DaemonSet some time to schedule and pull.
  kubectl rollout status -n kube-system ds/tekton-prepull-images --timeout=2m || true
}


function add_spire() {
  local gate="$1"
  if [ "$gate" == "alpha" ] ; then
    DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
    printf "Setting up environment for alpha features"
    install_spire
    patch_pipeline_spire
    kubectl apply -n tekton-pipelines -f "$DIR"/testdata/spire/config-spire.yaml
  fi
}

function set_feature_gate() {
  local gate="$1"
  if [ "$gate" != "alpha" ] && [ "$gate" != "stable" ] && [ "$gate" != "beta" ] ; then
    printf "Invalid gate %s\n" ${gate}
    exit 255
  fi
  printf "Setting feature gate to %s\n", ${gate}
  jsonpatch=$(printf "{\"data\": {\"enable-api-fields\": \"%s\"}}" $1)
  echo "feature-flags ConfigMap patch: ${jsonpatch}"
  with_retries kubectl patch configmap feature-flags -n tekton-pipelines -p "$jsonpatch"
}

function set_result_extraction_method() {
  local method="$1"
  if [ "$method" != "termination-message" ] && [ "$method" != "sidecar-logs" ]; then
    printf "Invalid value for results-from %s\n" ${method}
    exit 255
  fi
  printf "Setting results-from to %s\n", ${method}
  jsonpatch=$(printf "{\"data\": {\"results-from\": \"%s\"}}" $1)
  echo "feature-flags ConfigMap patch: ${jsonpatch}"
  with_retries kubectl patch configmap feature-flags -n tekton-pipelines -p "$jsonpatch"
}

function set_keep_pod_on_cancel() {	
    local method="$1"	
    if [ "$method" != "false" ] && [ "$method" != "true" ]; then	
      printf "Invalid value for keep-pod-on-cancel %s\n" ${method}	
      exit 255	
    fi	
    printf "Setting keep-pod-on-cancel to %s\n", ${method}	
    jsonpatch=$(printf "{\"data\": {\"keep-pod-on-cancel\": \"%s\"}}" $1)	
    echo "feature-flags ConfigMap patch: ${jsonpatch}"	
    with_retries kubectl patch configmap feature-flags -n tekton-pipelines -p "$jsonpatch"
}

function set_cel_in_whenexpression() {
  local method="$1"
  if [ "$method" != "false" ] && [ "$method" != "true" ]; then
    printf "Invalid value for enable-cel-in-whenexpression %s\n" ${method}
    exit 255
  fi
  jsonpatch=$(printf "{\"data\": {\"enable-cel-in-whenexpression\": \"%s\"}}" $1)
  echo "feature-flags ConfigMap patch: ${jsonpatch}"
  with_retries kubectl patch configmap feature-flags -n tekton-pipelines -p "$jsonpatch"
}

function set_enable_param_enum() {
  local method="$1"
  if [ "$method" != "false" ] && [ "$method" != "true" ]; then
    printf "Invalid value for enable-param-enum %s\n" ${method}
    exit 255
  fi
  printf "Setting enable-param-enum to %s\n", ${method}
  jsonpatch=$(printf "{\"data\": {\"enable-param-enum\": \"%s\"}}" $1)
  echo "feature-flags ConfigMap patch: ${jsonpatch}"
  with_retries kubectl patch configmap feature-flags -n tekton-pipelines -p "$jsonpatch"
}

function set_enable_artifacts() {
  local method="$1"
  if [ "$method" != "false" ] && [ "$method" != "true" ]; then
    printf "Invalid value for enable-artifacts %s\n" ${method}
    exit 255
  fi
  printf "Setting enable-artifacts to %s\n", ${method}
  jsonpatch=$(printf "{\"data\": {\"enable-artifacts\": \"%s\"}}" $1)
  echo "feature-flags ConfigMap patch: ${jsonpatch}"
  with_retries kubectl patch configmap feature-flags -n tekton-pipelines -p "$jsonpatch"
}

function set_enable_concise_resolver_syntax() {
  local method="$1"
  if [ "$method" != "false" ] && [ "$method" != "true" ]; then
    printf "Invalid value for enable-concise-resolver-syntax %s\n" ${method}
    exit 255
  fi
  printf "Setting enable-concise-resolver-syntax to %s\n", ${method}
  jsonpatch=$(printf "{\"data\": {\"enable-concise-resolver-syntax\": \"%s\"}}" $1)
  echo "feature-flags ConfigMap patch: ${jsonpatch}"
  with_retries kubectl patch configmap feature-flags -n tekton-pipelines -p "$jsonpatch"
}

function set_enable_kubernetes_sidecar() {
  local method="$1"
  if [ "$method" != "false" ] && [ "$method" != "true" ]; then
    printf "Invalid value for enable-kubernetes-sidecar %s\n" ${method}
    exit 255
  fi
  printf "Setting enable-kubernetes-sidecar to %s\n", ${method}
  jsonpatch=$(printf "{\"data\": {\"enable-kubernetes-sidecar\": \"%s\"}}" $1)
  echo "feature-flags ConfigMap patch: ${jsonpatch}"
  with_retries kubectl patch configmap feature-flags -n tekton-pipelines -p "$jsonpatch"
}

function set_default_sidecar_log_polling_interval() {
  # Sets the default-sidecar-log-polling-interval in the config-defaults ConfigMap to 0ms for e2e tests
  echo "Patching config-defaults ConfigMap: setting default-sidecar-log-polling-interval to 0ms"
  jsonpatch='{"data": {"default-sidecar-log-polling-interval": "0ms"}}'
  with_retries kubectl patch configmap config-defaults -n tekton-pipelines -p "$jsonpatch"
}

function run_e2e() {
  # Run the integration tests
  header "Running Go e2e tests"
  # Skip ./test/*.go tests if SKIP_GO_E2E_TESTS == true
  if [ "${SKIP_GO_E2E_TESTS}" != "true" ]; then
    go_test_e2e -timeout=${E2E_GO_TEST_TIMEOUT} ./test/...
  fi

  # Run these _after_ the integration tests b/c they don't quite work all the way
  # and they cause a lot of noise in the logs, making it harder to debug integration
  # test failures.
  if [ "${RUN_YAML_TESTS}" == "true" ]; then
    go_test_e2e -mod=readonly -tags=examples -timeout=${E2E_GO_TEST_TIMEOUT} ./test/
  fi

  if [ "${RUN_FEATUREFLAG_TESTS}" == "true" ]; then
    go_test_e2e -mod=readonly -tags=featureflags -timeout=${E2E_GO_TEST_TIMEOUT} ./test/
  fi
}

set -eo pipefail
trap '[[ "$?" == "0" ]] || fail_test' EXIT

add_spire "$PIPELINE_FEATURE_GATE"
set_feature_gate "$PIPELINE_FEATURE_GATE"
set_result_extraction_method "$RESULTS_FROM"
set_keep_pod_on_cancel "$KEEP_POD_ON_CANCEL"
set_cel_in_whenexpression "$ENABLE_CEL_IN_WHENEXPRESSION"
set_enable_param_enum "$ENABLE_PARAM_ENUM"
set_enable_artifacts "$ENABLE_ARTIFACTS"
set_enable_concise_resolver_syntax "$ENABLE_CONCISE_RESOLVER_SYNTAX"
set_enable_kubernetes_sidecar "$ENABLE_KUBERNETES_SIDECAR"
set_default_sidecar_log_polling_interval
prepull_common_images
run_e2e

success
