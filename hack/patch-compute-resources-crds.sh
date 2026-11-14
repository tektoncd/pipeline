#!/usr/bin/env bash

# Copyright 2025 The Tekton Authors
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

# Add x-kubernetes-preserve-unknown-fields: true to computeResources fields
# in CRDs. This is needed because ComputeResourceRequirements uses custom
# JSON marshaling to support variable references like $(params.MEM), and
# without this annotation the apiserver would prune those values.
#
# The CRD schema generator doesn't produce this annotation automatically,
# so we patch it in after generation.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CRD_DIR="${SCRIPT_DIR}/../config/300-crds"

for crd in "${CRD_DIR}/300-task.yaml" "${CRD_DIR}/300-taskrun.yaml" "${CRD_DIR}/300-pipelinerun.yaml"; do
  if [ ! -f "$crd" ]; then
    continue
  fi
  python3 -c "
import re, sys
with open('${crd}') as f:
    lines = f.readlines()
result = []
for i, line in enumerate(lines):
    result.append(line)
    # After 'type: object' that follows a 'computeResources:' key, add preserve-unknown-fields
    if re.match(r'^(\s+)type: object\s*$', line):
        indent = re.match(r'^(\s+)', line).group(1)
        expected = indent[:-2] if len(indent) >= 2 else ''
        for j in range(len(result)-2, max(len(result)-8, -1), -1):
            if re.match(rf'^{re.escape(expected)}computeResources:\s*$', result[j]):
                annotation = indent + 'x-kubernetes-preserve-unknown-fields: true\n'
                # Don't add if already present
                if i + 1 < len(lines) and lines[i+1].strip() == 'x-kubernetes-preserve-unknown-fields: true':
                    break
                result.append(annotation)
                break
            if re.match(r'^\s+\w+:', result[j]) and 'description' not in result[j].lower() and 'type' not in result[j].lower():
                break
with open('${crd}', 'w') as f:
    f.writelines(result)
"
  echo "Patched computeResources in $(basename "$crd")"
done
