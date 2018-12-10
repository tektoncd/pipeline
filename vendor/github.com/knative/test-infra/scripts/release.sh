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

# This is a helper script for Knative release scripts.
# See README.md for instructions on how to use it.

source $(dirname ${BASH_SOURCE})/library.sh

# Simple banner for logging purposes.
# Parameters: $1 - message to display.
function banner() {
    make_banner "@" "$1"
}

# Tag images in the yaml file if $TAG is not empty.
# $KO_DOCKER_REPO is the registry containing the images to tag with $TAG.
# Parameters: $1 - yaml file to parse for images.
function tag_images_in_yaml() {
  [[ -z ${TAG} ]] && return 0
  local SRC_DIR="${GOPATH}/src/"
  local DOCKER_BASE="${KO_DOCKER_REPO}/${REPO_ROOT_DIR/$SRC_DIR}"
  echo "Tagging images under '${DOCKER_BASE}' with ${TAG}"
  for image in $(grep -o "${DOCKER_BASE}/[a-z\./-]\+@sha256:[0-9a-f]\+" $1); do
    gcloud -q container images add-tag ${image} ${image%%@*}:${TAG}
  done
}

# Copy the given yaml file to the $RELEASE_GCS_BUCKET bucket's "latest" directory.
# If $TAG is not empty, also copy it to $RELEASE_GCS_BUCKET bucket's "previous" directory.
# Parameters: $1 - yaml file to copy.
function publish_yaml() {
  function verbose_gsutil_cp {
    local DEST=gs://${RELEASE_GCS_BUCKET}/$2/
    echo "Publishing $1 to ${DEST}"
    gsutil cp $1 ${DEST}
  }
  verbose_gsutil_cp $1 latest
  if [[ -n ${TAG} ]]; then
    verbose_gsutil_cp $1 previous/${TAG}
  fi
}

# These are global environment variables.
SKIP_TESTS=0
TAG_RELEASE=0
PUBLISH_RELEASE=0
BRANCH_RELEASE=0
TAG=""
RELEASE_VERSION=""
RELEASE_NOTES=""
RELEASE_BRANCH=""
RELEASE_GCS_BUCKET=""
KO_FLAGS=""
export KO_DOCKER_REPO=""

function abort() {
  echo "error: $@"
  exit 1
}

# Parses flags and sets environment variables accordingly.
function parse_flags() {
  TAG=""
  RELEASE_VERSION=""
  RELEASE_NOTES=""
  RELEASE_BRANCH=""
  KO_FLAGS="-P"
  KO_DOCKER_REPO="gcr.io/knative-nightly"
  RELEASE_GCS_BUCKET="knative-nightly/$(basename ${REPO_ROOT_DIR})"
  local has_gcr_flag=0
  local has_gcs_flag=0

  cd ${REPO_ROOT_DIR}
  while [[ $# -ne 0 ]]; do
    local parameter=$1
    case $parameter in
      --skip-tests) SKIP_TESTS=1 ;;
      --tag-release) TAG_RELEASE=1 ;;
      --notag-release) TAG_RELEASE=0 ;;
      --publish) PUBLISH_RELEASE=1 ;;
      --nopublish) PUBLISH_RELEASE=0 ;;
      --release-gcr)
        shift
        [[ $# -ge 1 ]] || abort "missing GCR after --release-gcr"
        KO_DOCKER_REPO=$1
        has_gcr_flag=1
        ;;
      --release-gcs)
        shift
        [[ $# -ge 1 ]] || abort "missing GCS bucket after --release-gcs"
        RELEASE_GCS_BUCKET=$1
        has_gcs_flag=1
        ;;
      --version)
        shift
        [[ $# -ge 1 ]] || abort "missing version after --version"
        [[ $1 =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || abort "version format must be '[0-9].[0-9].[0-9]'"
        RELEASE_VERSION=$1
        ;;
      --branch)
        shift
        [[ $# -ge 1 ]] || abort "missing branch after --commit"
        [[ $1 =~ ^release-[0-9]+\.[0-9]+$ ]] || abort "branch name must be 'release-[0-9].[0-9]'"
        RELEASE_BRANCH=$1
        ;;
      --release-notes)
        shift
        [[ $# -ge 1 ]] || abort "missing release notes file after --release-notes"
        [[ ! -f "$1" ]] && abort "file $1 doesn't exist"
        RELEASE_NOTES=$1
        ;;
      *) abort "unknown option ${parameter}" ;;
    esac
    shift
  done

  # Update KO_DOCKER_REPO and KO_FLAGS if we're not publishing.
  if (( ! PUBLISH_RELEASE )); then
    (( has_gcr_flag )) && echo "Not publishing the release, GCR flag is ignored"
    (( has_gcs_flag )) && echo "Not publishing the release, GCS flag is ignored"
    KO_DOCKER_REPO="ko.local"
    KO_FLAGS="-L ${KO_FLAGS}"
    RELEASE_GCS_BUCKET=""
  fi

  if (( TAG_RELEASE )); then
    # Get the commit, excluding any tags but keeping the "dirty" flag
    local commit="$(git describe --always --dirty --match '^$')"
    [[ -n "${commit}" ]] || abort "Error getting the current commit"
    # Like kubernetes, image tag is vYYYYMMDD-commit
    TAG="v$(date +%Y%m%d)-${commit}"
  fi

  if [[ -n "${RELEASE_VERSION}" ]]; then
    TAG="v${RELEASE_VERSION}"
  fi

  [[ -n "${RELEASE_VERSION}" ]] && (( PUBLISH_RELEASE )) && BRANCH_RELEASE=1

  readonly SKIP_TESTS
  readonly TAG_RELEASE
  readonly PUBLISH_RELEASE
  readonly BRANCH_RELEASE
  readonly TAG
  readonly RELEASE_VERSION
  readonly RELEASE_NOTES
  readonly RELEASE_BRANCH
  readonly RELEASE_GCS_BUCKET
  readonly KO_DOCKER_REPO
}

# Run tests (unless --skip-tests was passed). Conveniently displays a banner indicating so.
# Parameters: $1 - executable that runs the tests.
function run_validation_tests() {
  if (( ! SKIP_TESTS )); then
    banner "Running release validation tests"
    # Run tests.
    if ! $1; then
      banner "Release validation tests failed, aborting"
      exit 1
    fi
  fi
}

# Initialize everything (flags, workspace, etc) for a release.
function initialize() {
  parse_flags $@

  # Log what will be done and where.
  banner "Release configuration"
  echo "- Destination GCR: ${KO_DOCKER_REPO}"
  (( SKIP_TESTS )) && echo "- Tests will NOT be run" || echo "- Tests will be run"
  if (( TAG_RELEASE )); then
    echo "- Artifacts will tagged '${TAG}'"
  else
    echo "- Artifacts WILL NOT be tagged"
  fi
  if (( PUBLISH_RELEASE )); then
    echo "- Release WILL BE published to '${RELEASE_GCS_BUCKET}'"
  else
    echo "- Release will not be published"
  fi
  if (( BRANCH_RELEASE )); then
    echo "- Release WILL BE branched from '${RELEASE_BRANCH}'"
  fi
  [[ -n "${RELEASE_NOTES}" ]] && echo "- Release notes are generated from '${RELEASE_NOTES}'"

  # Checkout specific branch, if necessary
  if (( BRANCH_RELEASE )); then
    git checkout upstream/${RELEASE_BRANCH} || abort "cannot checkout branch ${RELEASE_BRANCH}"
  fi
}

# Create a new release on GitHub, also git tagging it (unless this is not a versioned release).
# Parameters: $1 - Module name (e.g., "Knative Serving").
#             $2 - YAML files to add to the release, space separated.
function branch_release() {
  (( BRANCH_RELEASE )) || return 0
  local title="$1 release ${TAG}"
  local attachments=()
  local description="$(mktemp)"
  local attachments_dir="$(mktemp -d)"
  # Copy each YAML to a separate dir
  for yaml in $2; do
    cp ${yaml} ${attachments_dir}/
    attachments+=("--attach=${yaml}#$(basename ${yaml})")
  done
  echo -e "${title}\n" > ${description}
  if [[ -n "${RELEASE_NOTES}" ]]; then
    cat ${RELEASE_NOTES} >> ${description}
  fi
  git tag -a ${TAG} -m "${title}"
  git push $(git remote get-url upstream) tag ${TAG}
  run_go_tool github.com/github/hub hub release create \
      --prerelease \
      ${attachments[@]} \
      --file=${description} \
      --commitish=${RELEASE_BRANCH} \
      ${TAG}
}
