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

# Github upstream.
readonly KNATIVE_UPSTREAM="github.com/knative/${REPO_NAME}"

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

    # Georeplicate to {us,eu,asia}.gcr.io
    gcloud -q container images add-tag ${image} us.${image%%@*}:${TAG}
    gcloud -q container images add-tag ${image} eu.${image%%@*}:${TAG}
    gcloud -q container images add-tag ${image} asia.${image%%@*}:${TAG}
  done
}

# Copy the given yaml file to the $RELEASE_GCS_BUCKET bucket's "latest" directory.
# If $TAG is not empty, also copy it to $RELEASE_GCS_BUCKET bucket's "previous" directory.
# Parameters: $1 - yaml file to copy.
function publish_yaml() {
  function verbose_gsutil_cp {
    local DEST="gs://${RELEASE_GCS_BUCKET}/$2/"
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
export GITHUB_TOKEN=""

# Convenience function to run the hub tool.
# Parameters: $1..$n - arguments to hub.
function hub_tool() {
  run_go_tool github.com/github/hub hub $@
}

# Return the master version of a release.
# For example, "v0.2.1" returns "0.2"
# Parameters: $1 - release version label.
function master_version() {
  local release="${1//v/}"
  local tokens=(${release//\./ })
  echo "${tokens[0]}.${tokens[1]}"
}

# Return the release build number of a release.
# For example, "v0.2.1" returns "1".
# Parameters: $1 - release version label.
function release_build_number() {
  local tokens=(${1//\./ })
  echo "${tokens[2]}"
}

# Setup the repository upstream, if not set.
function setup_upstream() {
  # hub and checkout need the upstream URL to be set
  # TODO(adrcunha): Use "git remote get-url" once available on Prow.
  local upstream="$(git config --get remote.upstream.url)"
  echo "Remote upstream URL is '${upstream}'"
  if [[ -z "${upstream}" ]]; then
    upstream="https://${KNATIVE_UPSTREAM}"
    echo "Setting remote upstream URL to '${upstream}'"
    git remote add upstream ${upstream}
  fi
  git fetch --all
}

# Setup version, branch and release notes for a "dot" release.
function prepare_dot_release() {
  echo "Dot release requested"
  TAG_RELEASE=1
  PUBLISH_RELEASE=1
  # List latest release
  local releases # don't combine with the line below, or $? will be 0
  releases="$(hub_tool release)"
  [[ $? -eq 0 ]] || abort "cannot list releases"
  # If --release-branch passed, restrict to that release
  if [[ -n "${RELEASE_BRANCH}" ]]; then
    local version_filter="v${RELEASE_BRANCH##release-}"
    echo "Dot release will be generated for ${version_filter}"
    releases="$(echo "${releases}" | grep ^${version_filter})"
  fi
  local last_version="$(echo "${releases}" | grep '^v[0-9]\+\.[0-9]\+\.[0-9]\+$' | sort -r | head -1)"
  [[ -n "${last_version}" ]] || abort "no previous release exist"
  if [[ -z "${RELEASE_BRANCH}" ]]; then
    echo "Last release is ${last_version}"
    # Determine branch
    local major_minor_version="$(master_version ${last_version})"
    RELEASE_BRANCH="release-${major_minor_version}"
    echo "Last release branch is ${RELEASE_BRANCH}"
  fi
  # Ensure there are new commits in the branch, otherwise we don't create a new release
  local last_release_commit="$(git rev-list -n 1 ${last_version})"
  local release_branch_commit="$(git rev-list -n 1 upstream/${RELEASE_BRANCH})"
  [[ -n "${last_release_commit}" ]] || abort "cannot get last release commit"
  [[ -n "${release_branch_commit}" ]] || abort "cannot get release branch last commit"
  if [[ "${last_release_commit}" == "${release_branch_commit}" ]]; then
    echo "*** Branch ${RELEASE_BRANCH} is at commit ${release_branch_commit}"
    echo "*** Branch ${RELEASE_BRANCH} has no new cherry-picks since release ${last_version}"
    echo "*** No dot release will be generated, as no changes exist"
    exit 0
  fi
  # Create new release version number
  local last_build="$(release_build_number ${last_version})"
  RELEASE_VERSION="${major_minor_version}.$(( last_build + 1 ))"
  echo "Will create release ${RELEASE_VERSION} at commit ${release_branch_commit}"
  # If --release-notes not used, copy from the latest release
  if [[ -z "${RELEASE_NOTES}" ]]; then
    RELEASE_NOTES="$(mktemp)"
    hub_tool release show -f "%b" ${last_version} > ${RELEASE_NOTES}
    echo "Release notes from ${last_version} copied to ${RELEASE_NOTES}"
  fi
}

# Parses flags and sets environment variables accordingly.
function parse_flags() {
  TAG=""
  RELEASE_VERSION=""
  RELEASE_NOTES=""
  RELEASE_BRANCH=""
  KO_FLAGS="-P"
  KO_DOCKER_REPO="gcr.io/knative-nightly"
  RELEASE_GCS_BUCKET="knative-nightly/${REPO_NAME}"
  GITHUB_TOKEN=""
  local has_gcr_flag=0
  local has_gcs_flag=0
  local is_dot_release=0

  cd ${REPO_ROOT_DIR}
  while [[ $# -ne 0 ]]; do
    local parameter=$1
    case ${parameter} in
      --skip-tests) SKIP_TESTS=1 ;;
      --tag-release) TAG_RELEASE=1 ;;
      --notag-release) TAG_RELEASE=0 ;;
      --publish) PUBLISH_RELEASE=1 ;;
      --nopublish) PUBLISH_RELEASE=0 ;;
      --dot-release) is_dot_release=1 ;;
      *)
        [[ $# -ge 2 ]] || abort "missing parameter after $1"
        shift
        case ${parameter} in
          --github-token)
            [[ ! -f "$1" ]] && abort "file $1 doesn't exist"
            GITHUB_TOKEN="$(cat $1)"
            [[ -n "${GITHUB_TOKEN}" ]] || abort "file $1 is empty"
            ;;
          --release-gcr)
            KO_DOCKER_REPO=$1
            has_gcr_flag=1
            ;;
          --release-gcs)
            RELEASE_GCS_BUCKET=$1
            has_gcs_flag=1
            ;;
          --version)
            [[ $1 =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || abort "version format must be '[0-9].[0-9].[0-9]'"
            RELEASE_VERSION=$1
            ;;
          --branch)
            [[ $1 =~ ^release-[0-9]+\.[0-9]+$ ]] || abort "branch name must be 'release-[0-9].[0-9]'"
            RELEASE_BRANCH=$1
            ;;
          --release-notes)
            [[ ! -f "$1" ]] && abort "file $1 doesn't exist"
            RELEASE_NOTES=$1
            ;;
          *) abort "unknown option ${parameter}" ;;
        esac
    esac
    shift
  done

  # Setup dot releases
  if (( is_dot_release )); then
    setup_upstream
    prepare_dot_release
  fi

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
    setup_upstream
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
  local repo_url="${KNATIVE_UPSTREAM}"
  [[ -n "${GITHUB_TOKEN}}" ]] && repo_url="${GITHUB_TOKEN}@${repo_url}"
  hub_tool push https://${repo_url} tag ${TAG}

  hub_tool release create \
      --prerelease \
      ${attachments[@]} \
      --file=${description} \
      --commitish=${RELEASE_BRANCH} \
      ${TAG}
}
