#!/bin/bash

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

# This is a collection of useful bash functions and constants, intended
# to be used in test scripts and the like. It doesn't do anything when
# called from command line.

# Default GKE version to be used with build-crd
readonly BUILD_GKE_VERSION=1.9.6-gke.1

# Useful environment variables
[[ -n "${PROW_JOB_ID}" ]] && IS_PROW=1 || IS_PROW=0
readonly IS_PROW
readonly BUILD_ROOT_DIR="$(dirname $(readlink -f ${BASH_SOURCE}))/.."
readonly OUTPUT_GOBIN="${BUILD_ROOT_DIR}/_output/bin"

# Copy of *_OVERRIDE variables
readonly OG_DOCKER_REPO="${DOCKER_REPO_OVERRIDE}"
readonly OG_K8S_CLUSTER="${K8S_CLUSTER_OVERRIDE}"
readonly OG_KO_DOCKER_REPO="${KO_DOCKER_REPO}"

# Returns a UUID
function uuid() {
  # uuidgen is not available in kubekins images
  cat /proc/sys/kernel/random/uuid
}

# Simple header for logging purposes.
function header() {
  echo "================================================="
  echo ${1^^}
  echo "================================================="
}

# Simple subheader for logging purposes.
function subheader() {
  echo "-------------------------------------------------"
  echo $1
  echo "-------------------------------------------------"
}

# Restores the *_OVERRIDE variables to their original value.
function restore_override_vars() {
  export DOCKER_REPO_OVERRIDE="${OG_DOCKER_REPO}"
  export K8S_CLUSTER_OVERRIDE="${OG_K8S_CLUSTER}"
  export KO_DOCKER_REPO="${OG_KO_DOCKER_REPO}"
}

# Remove ALL images in the given GCR repository.
# Parameters: $1 - GCR repository.
function delete_gcr_images() {
  for image in $(gcloud --format='value(name)' container images list --repository=$1); do
    echo "Checking ${image} for removal"
    delete_gcr_images ${image}
    for digest in $(gcloud --format='get(digest)' container images list-tags ${image} --limit=99999); do
      local full_image="${image}@${digest}"
      echo "Removing ${full_image}"
      gcloud container images delete -q --force-delete-tags ${full_image}
    done
  done
}

# Sets the given user as cluster admin.
# Parameters: $1 - user
#             $2 - cluster name
#             $3 - cluster zone
function acquire_cluster_admin_role() {
  # Get the password of the admin and use it, as the service account (or the user)
  # might not have the necessary permission.
  local password=$(gcloud --format="value(masterAuth.password)" \
      container clusters describe $2 --zone=$3)
  kubectl --username=admin --password=$password \
      create clusterrolebinding cluster-admin-binding \
      --clusterrole=cluster-admin \
      --user=$1
}

# Authenticates the current user to GCR in the current project.
function gcr_auth() {
  echo "Authenticating to GCR"
  # kubekins-e2e images lack docker-credential-gcr, install it manually.
  # TODO(adrcunha): Remove this step once docker-credential-gcr is available.
  gcloud components install docker-credential-gcr
  docker-credential-gcr configure-docker
  echo "Successfully authenticated"
}

# Installs ko in $OUTPUT_GOBIN
function install_ko() {
  GOBIN="${OUTPUT_GOBIN}" go install ./vendor/github.com/google/go-containerregistry/cmd/ko
}

# Runs ko; prefers using the one installed by install_ko().
# Parameters: $1..$n - arguments to ko
function ko() {
  if [[ -e "${OUTPUT_GOBIN}/ko" ]]; then
    "${OUTPUT_GOBIN}/ko" $@
  else
    local local_ko="$(which ko)"
    if [[ -z "${local_ko}" ]]; then
      echo "error: ko not installed, either in the system or explicitly"
      return 1
    fi
    $local_ko $@
  fi
}
