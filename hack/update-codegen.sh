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

set -o errexit
set -o nounset
set -o pipefail

source $(git rev-parse --show-toplevel)/vendor/github.com/tektoncd/plumbing/scripts/library.sh

# generate the code with:
# --output-base    because this script should also be able to run inside the vendor dir of
#                  k8s.io/kubernetes. The output-base is needed for the generators to output into the vendor dir
#                  instead of the $GOPATH directly. For normal projects this can be dropped.

bash ${REPO_ROOT_DIR}/hack/generate-groups.sh "deepcopy,client,informer,lister" \
  github.com/tektoncd/pipeline/pkg/client github.com/tektoncd/pipeline/pkg/apis \
  "pipeline:v1alpha1,v1alpha2" \
  --go-header-file ${REPO_ROOT_DIR}/hack/boilerplate/boilerplate.go.txt

# Depends on generate-groups.sh to install bin/deepcopy-gen
${GOPATH}/bin/deepcopy-gen \
  -O zz_generated.deepcopy \
  --go-header-file ${REPO_ROOT_DIR}/hack/boilerplate/boilerplate.go.txt \
  -i github.com/tektoncd/pipeline/pkg/apis/config

# Knative Injection
bash ${REPO_ROOT_DIR}/hack/generate-knative.sh "injection" \
  github.com/tektoncd/pipeline/pkg/client github.com/tektoncd/pipeline/pkg/apis \
  "pipeline:v1alpha1,v1alpha2" \
  --go-header-file ${REPO_ROOT_DIR}/hack/boilerplate/boilerplate.go.txt

# Make sure our dependencies are up-to-date
${REPO_ROOT_DIR}/hack/update-deps.sh
