#!/usr/bin/env bash

# Copyright 2020 The Knative Authors
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

set -e

source "$(dirname "${BASH_SOURCE[0]}")/library.sh"

if (( ! IS_PROW )); then
  echo "Local run of shellcheck-presubmit detected"
  echo "This notably DOES NOT ACT LIKE THE GITHUB PRESUBMIT"
  echo "The Github presubmit job only runs shellcheck on files you touch"
  echo "There's no way to locally determine which files you touched:"
  echo " as git is a distributed VCS, there is no notion of parent until merge"
  echo " is attempted."
  echo "So it checks the current content of all files changed in the previous commit"
  echo " and/or currently staged."
fi

if [[ "$1" == "--run" ]]; then
  shellcheck_new_files
fi
