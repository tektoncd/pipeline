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

# Tag images in the yaml file with a tag. If not tag is passed, does nothing.
# Parameters: $1 - yaml file to parse for images.
#             $2 - registry where the images are stored.
#             $3 - tag to apply (optional).
function tag_images_in_yaml() {
  [[ -z $3 ]] && return 0
  local src_dir="${GOPATH}/src/"
  local BASE_PATH="${REPO_ROOT_DIR/$src_dir}"
  echo "Tagging images under '${BASE_PATH}' with $3"
  for image in $(grep -o "$2/${BASE_PATH}/[a-z\./-]\+@sha256:[0-9a-f]\+" $1); do
    gcloud -q container images add-tag ${image} ${image%%@*}:$3
  done
}

# Copy the given yaml file to a GCS bucket. Image is tagged :latest, and optionally :$2.
# Parameters: $1 - yaml file to copy.
#             $2 - destination bucket name.
#             $3 - tag to apply (optional).
function publish_yaml() {
  gsutil cp $1 gs://$2/latest/
  [[ -n $3 ]] && gsutil cp $1 gs://$2/previous/$3/ || true
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
KO_FLAGS=""

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
  cd ${REPO_ROOT_DIR}
  while [[ $# -ne 0 ]]; do
    local parameter=$1
    case $parameter in
      --skip-tests) SKIP_TESTS=1 ;;
      --tag-release) TAG_RELEASE=1 ;;
      --notag-release) TAG_RELEASE=0 ;;
      --publish) PUBLISH_RELEASE=1 ;;
      --nopublish) PUBLISH_RELEASE=0 ;;
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
    KO_DOCKER_REPO="ko.local"
    KO_FLAGS="-L ${KO_FLAGS}"
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
