#!/usr/bin/env bash

# Copyright 2018 The Knative Authors.
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

# smoke_test.sh will attempt to deploy all of the example CRDs to whatever
# is the current kubectl cluster context.
# It will not wait for any Runs to complete and instead will destroy the CRDs
# immediately after creation, so at the moment this pretty much just makes
# sure the structure is valid (and this might even have weird side effects..).

set -o xtrace
set -o errexit
set -o pipefail

kubectl apply -f examples/
kubectl apply -f examples/run

sleep 5

kubectl delete -f examples/run
kubectl delete -f examples/
