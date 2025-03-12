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

set -o errexit
set -o nounset
set -o pipefail

OLDGOFLAGS="${GOFLAGS:-}"
GOFLAGS=""

CRD_PATH=$(dirname "${0}")/../config/300-crds
API_PATH=$(dirname "${0}")/../pkg/apis

TEMP_DIR_LOGS=$(mktemp -d)

for FILENAME in `find $CRD_PATH -type f`;
do
  echo "Gererating CRD schema for $FILENAME"

  # NOTE: APIs for the group tekton.dev are implemented under ./pkg/apis/pipeline,
  # while the ResolutionRequest from group resolution.tekton.dev is implemented under ./pkg/apis/resolution

  GROUP=$(grep -E '^  group:' $FILENAME)
  GROUP=${GROUP#"  group: "}
  if [ "$GROUP" = "tekton.dev" ]; then
    API_SUBDIR='pipeline'
  else
    API_SUBDIR=${GROUP%".tekton.dev"}
  fi

  TEMP_DIR=$(mktemp -d)
  cp -p $FILENAME $TEMP_DIR/.
  LOG_FILE=$TEMP_DIR_LOGS/log-schema-generation-$(basename $FILENAME)
    
  counter=0 limit=10
  while [ "$counter" -lt "$limit" ]; do
    # FIXME:(burigolucas): add schema for fields with generic type once supported by controller-tools
    # FIXME:(burigolucas): add schema for recursive fields once supported by controller-tools
    # FIXME:(burigolucas): add reference for dependent external/internal schemas once supported in CRD specification
    # FIXME:(burigolucas): controller-gen return status 1 with message "Error: not all generators ran successfully"
    set +e
    go run sigs.k8s.io/controller-tools/cmd/controller-gen@v0.17.1 \
      schemapatch:manifests=$TEMP_DIR,generateEmbeddedObjectMeta=false \
      output:dir=$CRD_PATH \
      paths=$API_PATH/$API_SUBDIR/... > $LOG_FILE 2>&1
    rc=$?
    set -e
    if [ $rc -eq 0 ]; then
      break
    fi
    if grep -q 'exit status 1' $LOG_FILE; then
      echo "[WARNING] Ignoring errors/warnings from CRD schema generation, check $LOG_FILE for details"
      break
    fi
    counter="$(( $counter + 1 ))"
    if [ $counter -eq $limit ]; then
      echo "[ERROR] Failed to generate CRD schema"
      exit 1
    fi
  done

  rm -rf $TEMP_DIR
done

GOFLAGS="${OLDGOFLAGS}"
