#!/usr/bin/env bash

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

source "$(dirname "${BASH_SOURCE[0]}")/library.sh"

# Organization name in GitHub; defaults to Knative.
readonly ORG_NAME="${ORG_NAME:-knative}"

# GitHub upstream.
readonly REPO_UPSTREAM="https://github.com/${ORG_NAME}/${REPO_NAME}"

# GCRs for Knative releases.
readonly NIGHTLY_GCR="gcr.io/knative-nightly/github.com/${ORG_NAME}/${REPO_NAME}"
readonly RELEASE_GCR="gcr.io/knative-releases/github.com/${ORG_NAME}/${REPO_NAME}"

# Signing identities for knative releases.
readonly NIGHTLY_SIGNING_IDENTITY="signer@knative-nightly.iam.gserviceaccount.com"
readonly RELEASE_SIGNING_IDENTITY="signer@knative-releases.iam.gserviceaccount.com"

# Simple banner for logging purposes.
# Parameters: $* - message to display.
function banner() {
  subheader "$*"
}

# Copy the given files to the $RELEASE_GCS_BUCKET bucket's "latest" directory.
# If $TAG is not empty, also copy them to $RELEASE_GCS_BUCKET bucket's "previous" directory.
# Parameters: $1..$n - files to copy.
function publish_to_gcs() {
  function verbose_gsutil_cp {
    local DEST="gs://${RELEASE_GCS_BUCKET}/$1/"
    shift
    echo "Publishing [$@] to ${DEST}"
    gsutil -m cp $@ "${DEST}"
  }
  # Before publishing the files, cleanup the `latest` dir if it exists.
  local latest_dir="gs://${RELEASE_GCS_BUCKET}/latest"
  if [[ -n "$(gsutil ls "${latest_dir}" 2> /dev/null)" ]]; then
    echo "Cleaning up '${latest_dir}' first"
    gsutil -m rm "${latest_dir}"/**
  fi
  verbose_gsutil_cp latest $@
  [[ -n ${TAG} ]] && verbose_gsutil_cp previous/"${TAG}" $@
}

# These are global environment variables.
SKIP_TESTS=0
PRESUBMIT_TEST_FAIL_FAST=1
export TAG_RELEASE=0
PUBLISH_RELEASE=0
PUBLISH_TO_GITHUB=0
export TAG=""
BUILD_COMMIT_HASH=""
BUILD_YYYYMMDD=""
BUILD_TIMESTAMP=""
BUILD_TAG=""
RELEASE_VERSION=""
RELEASE_NOTES=""
RELEASE_BRANCH=""
RELEASE_GCS_BUCKET="knative-nightly/${REPO_NAME}"
RELEASE_DIR=""
export KO_FLAGS="-P --platform=all"
VALIDATION_TESTS="./test/presubmit-tests.sh"
ARTIFACTS_TO_PUBLISH=""
FROM_NIGHTLY_RELEASE=""
FROM_NIGHTLY_RELEASE_GCS=""
SIGNING_IDENTITY=""
APPLE_CODESIGN_KEY=""
APPLE_NOTARY_API_KEY=""
APPLE_CODESIGN_PASSWORD_FILE=""
export KO_DOCKER_REPO="gcr.io/knative-nightly"
# Build stripped binary to reduce size
export GOFLAGS="-ldflags=-s -ldflags=-w"
export GITHUB_TOKEN=""
readonly IMAGES_REFS_FILE="${IMAGES_REFS_FILE:-$(mktemp -d)/images_refs.txt}"

# Convenience function to run the GitHub CLI tool `gh`.
# Parameters: $1..$n - arguments to gh.
function gh_tool() {
  go_run github.com/cli/cli/v2/cmd/gh@v2.65.0 "$@"
}

# Shortcut to "git push" that handles authentication.
# Parameters: $1..$n - arguments to "git push <repo>".
function git_push() {
  local repo_url="${REPO_UPSTREAM}"
  local git_args=$@
  # Subshell (parentheses) used to prevent GITHUB_TOKEN from printing in log
  (set +x;
   [[ -n "${GITHUB_TOKEN}}" ]] && repo_url="${repo_url/:\/\//:\/\/${GITHUB_TOKEN}@}";
   git push "${repo_url}" ${git_args} )
}

# Return the major+minor version of a release.
# For example, "v0.2.1" returns "0.2"
# Parameters: $1 - release version label.
function major_minor_version() {
  local release="${1//v/}"
  local tokens=(${release//\./ })
  echo "${tokens[0]}.${tokens[1]}"
}

# Return the short commit SHA from a release tag.
# For example, "v20010101-deadbeef" returns "deadbeef".
function hash_from_tag() {
  local tokens=(${1//-/ })
  echo "${tokens[1]}"
}

# Setup the repository upstream, if not set.
function setup_upstream() {
  # hub and checkout need the upstream URL to be set
  local upstream="$(git remote get-url upstream)"
  echo "Remote upstream URL is '${upstream}'"
  if [[ -z "${upstream}" ]]; then
    echo "Setting remote upstream URL to '${REPO_UPSTREAM}'"
    git remote add upstream "${REPO_UPSTREAM}"
  fi
}

# Fetch the release branch, so we can check it out.
function setup_branch() {
  [[ -z "${RELEASE_BRANCH}" ]] && return
  git fetch "${REPO_UPSTREAM}" "${RELEASE_BRANCH}:upstream/${RELEASE_BRANCH}"
}

# Setup version, branch and release notes for a auto release.
function prepare_auto_release() {
  echo "Auto release requested"
  TAG_RELEASE=1
  PUBLISH_RELEASE=1

  git fetch --all || abort "error fetching branches/tags from remote"
  # Support two different formats for tags
  # - knative-v1.0.0
  # - v1.0.0
  local tags="$(git tag | cut -d '-' -f2 | cut -d 'v' -f2 | cut -d '.' -f1-2 | sort -V | uniq)"
  local branches="$( { (git branch -r | grep upstream/release-) ; (git branch | grep release-); } | cut -d '-' -f2 | sort -V | uniq)"

  echo "Versions released (from tags): [" "${tags}" "]"
  echo "Versions released (from branches): [" "${branches}" "]"

  local release_number=""
  for i in ${branches}; do
    release_number="${i}"
    for j in ${tags}; do
      if [[ "${i}" == "${j}" ]]; then
        release_number=""
      fi
    done
  done

  if [[ -z "${release_number}" ]]; then
    echo "*** No new release will be generated, as no new branches exist"
    exit  0
  fi

  RELEASE_VERSION="${release_number}.0"
  RELEASE_BRANCH="release-${release_number}"
  echo "Will create release ${RELEASE_VERSION} from branch ${RELEASE_BRANCH}"
  # If --release-notes not used, add a placeholder
  if [[ -z "${RELEASE_NOTES}" ]]; then
    RELEASE_NOTES="$(mktemp)"
    echo "[add release notes here]" > "${RELEASE_NOTES}"
  fi
}

# Setup version, branch and release notes for a "dot" release.
function prepare_dot_release() {
  echo "Dot release requested"
  TAG_RELEASE=1
  PUBLISH_RELEASE=1
  git fetch --all || abort "error fetching branches/tags from remote"
  # List latest release
  local releases # don't combine with the line below, or $? will be 0
  # Support tags in two formats
  # - knative-v1.0.0
  # - v1.0.0
  releases="$(gh_tool release list --json tagName --jq '.[].tagName' | cut -d '-' -f2)"
  echo "Current releases are: ${releases}"
  [[ $? -eq 0 ]] || abort "cannot list releases"
  # If --release-branch passed, restrict to that release
  if [[ -n "${RELEASE_BRANCH}" ]]; then
    local version_filter="v${RELEASE_BRANCH##release-}"
    echo "Dot release will be generated for ${version_filter}"
    releases="$(echo "${releases}" | grep ^${version_filter})"
  fi
  local last_version="$(echo "${releases}" | grep '^v[0-9]\+\.[0-9]\+\.[0-9]\+$' | sort -r -V | head -1)"
  [[ -n "${last_version}" ]] || abort "no previous release exist"
  local major_minor_version=""
  if [[ -z "${RELEASE_BRANCH}" ]]; then
    echo "Last release is ${last_version}"
    # Determine branch
    major_minor_version="$(major_minor_version "${last_version}")"
    RELEASE_BRANCH="release-${major_minor_version}"
    echo "Last release branch is ${RELEASE_BRANCH}"
  else
    major_minor_version="${RELEASE_BRANCH##release-}"
  fi
  [[ -n "${major_minor_version}" ]] || abort "cannot get release major/minor version"
  # Ensure there are new commits in the branch, otherwise we don't create a new release
  setup_branch
  # Use the original tag (ie. potentially with a knative- prefix) when determining the last version commit sha
  local github_tag="$(gh_tool release list --json tagName --jq '.[].tagName' | grep "${last_version}")"
  local last_release_commit="$(git rev-list -n 1 "${github_tag}")"
  local last_release_commit_filtered="$(git rev-list --invert-grep --grep "\[skip-dot-release\]" -n 1 "${github_tag}")"
  local release_branch_commit="$(git rev-list -n 1 upstream/"${RELEASE_BRANCH}")"
  local release_branch_commit_filtered="$(git rev-list --invert-grep --grep "\[skip-dot-release\]" -n 1 upstream/"${RELEASE_BRANCH}")"
  [[ -n "${last_release_commit}" ]] || abort "cannot get last release commit"
  [[ -n "${release_branch_commit}" ]] || abort "cannot get release branch last commit"
  echo "Version ${last_version} is at commit ${last_release_commit}. Comparing using ${last_release_commit_filtered}. If it is different is because commits with the [skip-dot-release] flag in their commit body are not being considered."
  echo "Branch ${RELEASE_BRANCH} is at commit ${release_branch_commit}. Comparing using ${release_branch_commit_filtered}. If it is different is because commits with the [skip-dot-release] flag in their commit body are not being considered."
  if [[ "${last_release_commit_filtered}" == "${release_branch_commit_filtered}" ]]; then
    echo "*** Branch ${RELEASE_BRANCH} has no new cherry-picks (ignoring commits with [skip-dot-release]) since release ${last_version}."
    echo "*** No dot release will be generated, as no changes exist"
    exit 0
  fi
  # Create new release version number
  local last_build="$(patch_version "${last_version}")"
  RELEASE_VERSION="${major_minor_version}.$(( last_build + 1 ))"
  echo "Will create release ${RELEASE_VERSION} at commit ${release_branch_commit}"
  # If --release-notes not used, copy from the latest release
  if [[ -z "${RELEASE_NOTES}" ]]; then
    RELEASE_NOTES="$(mktemp)"
    gh_tool release view "${github_tag}" --json "body" --jq '.body' > "${RELEASE_NOTES}"
    echo "Release notes from ${last_version} copied to ${RELEASE_NOTES}"
  fi
}

# Setup source nightly image for a release.
function prepare_from_nightly_release() {
  echo "Release from nightly requested"
  SKIP_TESTS=1
  if [[ "${FROM_NIGHTLY_RELEASE}" == "latest" ]]; then
    echo "Finding the latest nightly release"
    find_latest_nightly "${NIGHTLY_GCR}" || abort "cannot find the latest nightly release"
    echo "Latest nightly is ${FROM_NIGHTLY_RELEASE}"
  fi
  readonly FROM_NIGHTLY_RELEASE_GCS="gs://knative-nightly/${REPO_NAME}/previous/${FROM_NIGHTLY_RELEASE}"
  gsutil ls -d "${FROM_NIGHTLY_RELEASE_GCS}" > /dev/null \
      || abort "nightly release ${FROM_NIGHTLY_RELEASE} doesn't exist"
}

# Build a release from an existing nightly one.
function build_from_nightly_release() {
  banner "Building the release"
  echo "Fetching manifests from nightly"
  local yamls_dir="$(mktemp -d)"
  gsutil -m cp -r "${FROM_NIGHTLY_RELEASE_GCS}/*" "${yamls_dir}" || abort "error fetching manifests"
  # Update references to release GCR
  for yaml in ${yamls_dir}/*.yaml; do
    sed -i -e "s#${NIGHTLY_GCR}#${RELEASE_GCR}#" "${yaml}"
  done
  ARTIFACTS_TO_PUBLISH="$(find "${yamls_dir}" -name '*.yaml' -printf '%p ')"
  echo "Copying nightly images"
  copy_nightly_images_to_release_gcr "${NIGHTLY_GCR}" "${FROM_NIGHTLY_RELEASE}"
  # Create a release branch from the nightly release tag.
  local commit="$(hash_from_tag "${FROM_NIGHTLY_RELEASE}")"
  echo "Creating release branch ${RELEASE_BRANCH} at commit ${commit}"
  git checkout -b "${RELEASE_BRANCH}" "${commit}" || abort "cannot create branch"
  git_push upstream "${RELEASE_BRANCH}" || abort "cannot push branch"
}

# Build a release from source.
function build_from_source() {
  run_validation_tests ${VALIDATION_TESTS}
  banner "Building the release"
  build_release
  # Do not use `||` above or any error will be swallowed.
  if [[ $? -ne 0 ]]; then
    abort "error building the release"
  fi
  sign_release || abort "error signing the release"
}

function get_images_in_yamls() {
  rm -rf "$IMAGES_REFS_FILE"
  echo "Assembling a list of image refences to sign"
  # shellcheck disable=SC2068
  for file in $@; do
    [[ "${file##*.}" != "yaml" ]] && continue
    echo "Inspecting ${file}"
    while read -r image; do
      echo "$image" >> "$IMAGES_REFS_FILE"
    done < <(grep -oh "\S*${KO_DOCKER_REPO}\S*" "${file}")
  done
  if [[ -f "$IMAGES_REFS_FILE" ]]; then
    sort -uo "$IMAGES_REFS_FILE" "$IMAGES_REFS_FILE" # Remove duplicate entries
  fi
}

# Finds a checksums file within the given list of artifacts (space delimited)
# Parameters: $n - artifact files
function find_checksums_file() {
  for arg in "$@"; do
    # kinda dirty hack needed as we pass $ARTIFACTS_TO_PUBLISH in space
    # delimiter variable, which is vulnerable to all sorts of argument quoting
    while read -r file; do
      if [[ "${file}" == *"checksums.txt" ]]; then
        echo "${file}"
        return 0
      fi
    done < <(echo "$arg" | tr ' ' '\n')
  done
  warning "cannot find checksums file"
}

# Build a release from source.
function sign_release() {
  if (( ! IS_PROW )); then # This function can't be run by devs on their laptops
    return 0
  fi
  get_images_in_yamls "${ARTIFACTS_TO_PUBLISH}"
  local checksums_file
  checksums_file="$(find_checksums_file "${ARTIFACTS_TO_PUBLISH}")"

  if ! [[ -f "${checksums_file}" ]]; then
    echo '>> No checksums file found, generating one'
    checksums_file="$(mktemp -d)/checksums.txt"
    for file in ${ARTIFACTS_TO_PUBLISH}; do
      pushd "$(dirname "$file")" >/dev/null
      sha256sum "$(basename "$file")" >> "${checksums_file}"
      popd >/dev/null
    done
    ARTIFACTS_TO_PUBLISH="${ARTIFACTS_TO_PUBLISH} ${checksums_file}"
  fi

  # Notarizing mac binaries needs to be done before cosign as it changes the checksum values
  # of the darwin binaries
 if [ -n "${APPLE_CODESIGN_KEY}" ] && [ -n "${APPLE_CODESIGN_PASSWORD_FILE}" ] && [ -n "${APPLE_NOTARY_API_KEY}" ]; then
    banner "Notarizing macOS Binaries for the release"
    local macos_artifacts
    declare -a macos_artifacts=()
    while read -r file; do
      if echo "$file" | grep -q "darwin"; then
        macos_artifacts+=("${file}")
        rcodesign sign "${file}" --p12-file="${APPLE_CODESIGN_KEY}" \
          --code-signature-flags=runtime \
          --p12-password-file="${APPLE_CODESIGN_PASSWORD_FILE}"
      fi
    done < <(echo "${ARTIFACTS_TO_PUBLISH}" | tr ' ' '\n')
    if [[ -z "${macos_artifacts[*]}" ]]; then
      warning "No macOS binaries found, skipping notarization"
    else
      local zip_file
      zip_file="$(mktemp -d)/files.zip"
      zip "$zip_file" -@ < <(printf "%s\n"  "${macos_artifacts[@]}")
      rcodesign notary-submit "$zip_file" --api-key-path="${APPLE_NOTARY_API_KEY}" --wait
      true > "${checksums_file}" # Clear the checksums file
      for file in ${ARTIFACTS_TO_PUBLISH}; do
        if echo "$file" | grep -q "checksums.txt"; then
          continue # Don't checksum the checksums file
        fi
        pushd "$(dirname "$file")" >/dev/null
        sha256sum "$(basename "$file")" >> "${checksums_file}"
        popd >/dev/null
      done
      echo "🧮     Post Notarization Checksum:"
      cat "$checksums_file"
    fi
  fi

  ID_TOKEN=$(gcloud auth print-identity-token --audiences=sigstore \
    --include-email \
    --impersonate-service-account="${SIGNING_IDENTITY}")
  echo "Signing Images with the identity ${SIGNING_IDENTITY}"
  ## Sign the images with cosign
  if [[ -f "$IMAGES_REFS_FILE" ]]; then
    COSIGN_EXPERIMENTAL=1 cosign sign $(cat "$IMAGES_REFS_FILE") \
      --recursive --identity-token="${ID_TOKEN}"
    cp "${IMAGES_REFS_FILE}" "${ARTIFACTS}"
    if  [ -n "${ATTEST_IMAGES:-}" ]; then # Temporary Feature Gate
      provenance-generator --clone-log=/logs/clone.json \
        --image-refs="$IMAGES_REFS_FILE" --output=attestation.json
      mkdir -p "${ARTIFACTS}" && cp attestation.json "${ARTIFACTS}"
      COSIGN_EXPERIMENTAL=1 cosign attest $(cat "$IMAGES_REFS_FILE") \
        --recursive --identity-token="${ID_TOKEN}" \
        --predicate=attestation.json --type=slsaprovenance
    fi
  fi

  echo "Signing checksums with the identity ${SIGNING_IDENTITY}"
  COSIGN_EXPERIMENTAL=1 cosign sign-blob "$checksums_file" \
    --output-signature="${checksums_file}.sig" \
    --output-certificate="${checksums_file}.pem" \
    --identity-token="${ID_TOKEN}"
  ARTIFACTS_TO_PUBLISH="${ARTIFACTS_TO_PUBLISH} ${checksums_file}.sig ${checksums_file}.pem"
}

# Copy tagged images from the nightly GCR to the release GCR, tagging them 'latest'.
# This is a recursive function, first call must pass $NIGHTLY_GCR as first parameter.
# Parameters: $1 - GCR to recurse into.
#             $2 - tag to be used to select images to copy.
function copy_nightly_images_to_release_gcr() {
  for entry in $(gcloud --format="value(name)" container images list --repository="$1"); do
    copy_nightly_images_to_release_gcr "${entry}" "$2"
    # Copy each image with the given nightly tag
    for x in $(gcloud --format="value(tags)" container images list-tags "${entry}" --filter="tags=$2" --limit=1); do
      local path="${entry/${NIGHTLY_GCR}}"  # Image "path" (remove GCR part)
      local dst="${RELEASE_GCR}${path}:latest"
      gcloud container images add-tag "${entry}:$2" "${dst}" || abort "error copying image"
    done
  done
}

# Recurse into GCR and find the nightly tag of the first `latest` image found.
# Parameters: $1 - GCR to recurse into.
function find_latest_nightly() {
  for entry in $(gcloud --format="value(name)" container images list --repository="$1"); do
    find_latest_nightly "${entry}" && return 0
    for tag in $(gcloud --format="value(tags)" container images list-tags "${entry}" \
        --filter="tags=latest" --limit=1); do
      local tags=( ${tag//,/ } )
      # Skip if more than one nightly tag, as we don't know what's the latest.
      if [[ ${#tags[@]} -eq 2 ]]; then
        local nightly_tag="${tags[@]/latest}"  # Remove 'latest' tag
        FROM_NIGHTLY_RELEASE="${nightly_tag// /}"  # Remove spaces
        return 0
      fi
    done
  done
  return 1
}

# Parses flags and sets environment variables accordingly.
function parse_flags() {
  local has_gcr_flag=0
  local has_gcs_flag=0
  local has_dir_flag=0
  local is_dot_release=0
  local is_auto_release=0

  cd "${REPO_ROOT_DIR}"
  while [[ $# -ne 0 ]]; do
    local parameter=$1
    case ${parameter} in
      --skip-tests) SKIP_TESTS=1 ;;
      --tag-release) TAG_RELEASE=1 ;;
      --notag-release) TAG_RELEASE=0 ;;
      --publish) PUBLISH_RELEASE=1 ;;
      --nopublish) PUBLISH_RELEASE=0 ;;
      --dot-release) is_dot_release=1 ;;
      --auto-release) is_auto_release=1 ;;
      --from-latest-nightly) FROM_NIGHTLY_RELEASE=latest ;;
      *)
        [[ $# -ge 2 ]] || abort "missing parameter after $1"
        shift
        case ${parameter} in
          --github-token)
            [[ ! -f "$1" ]] && abort "file $1 doesn't exist"
            local old="$(set +o)"
            # Prevent GITHUB_TOKEN from printing in log
            set +x
            # Remove any trailing newline/space from token
            # ^ (That's not what echo -n does, it just doesn't *add* a newline, but I'm leaving it)
            GITHUB_TOKEN="$(echo -n $(cat "$1"))"
            [[ -n "${GITHUB_TOKEN}" ]] || abort "file $1 is empty"
            eval "$old" # restore xtrace status
            ;;
          --release-gcr)
            KO_DOCKER_REPO=$1
            SIGNING_IDENTITY=$RELEASE_SIGNING_IDENTITY
            has_gcr_flag=1
            ;;
          --release-gcs)
            RELEASE_GCS_BUCKET=$1
            SIGNING_IDENTITY=$RELEASE_SIGNING_IDENTITY
            RELEASE_DIR=""
            has_gcs_flag=1
            ;;
          --release-dir)
            RELEASE_DIR=$1
            RELEASE_GCS_BUCKET=""
            has_dir_flag=1
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
          --from-nightly)
            [[ $1 =~ ^v[0-9]+-[0-9a-f]+$ ]] || abort "nightly tag must be 'vYYYYMMDD-commithash'"
            FROM_NIGHTLY_RELEASE=$1
            ;;
          --apple-codesign-key)
            APPLE_CODESIGN_KEY=$1
            ;;
          --apple-codesign-password-file)
            APPLE_CODESIGN_PASSWORD_FILE=$1
            ;;
          --apple-notary-api-key)
            APPLE_NOTARY_API_KEY=$1
            ;;
          *) abort "unknown option ${parameter}" ;;
        esac
    esac
    shift
  done

  (( has_gcs_flag )) && (( has_dir_flag )) && abort "cannot have both --release-gcs and --release-dir set simultaneously"
  [[ -n "${RELEASE_GCS_BUCKET}" && -n "${RELEASE_DIR}" ]] && abort "cannot have both GCS and release directory set"

  # Do auto release unless release is forced
  if (( is_auto_release )); then
    (( is_dot_release )) && abort "cannot have both --dot-release and --auto-release set simultaneously"
    [[ -n "${RELEASE_VERSION}" ]] && abort "cannot have both --version and --auto-release set simultaneously"
    [[ -n "${RELEASE_BRANCH}" ]] && abort "cannot have both --branch and --auto-release set simultaneously"
    [[ -n "${FROM_NIGHTLY_RELEASE}" ]] && abort "cannot have --auto-release with a nightly source"
    setup_upstream
    prepare_auto_release
  fi

  # Setup source nightly image
  if [[ -n "${FROM_NIGHTLY_RELEASE}" ]]; then
    (( is_dot_release )) && abort "dot releases are built from source"
    [[ -z "${RELEASE_VERSION}" ]] && abort "release version must be specified with --version"
    # TODO(adrcunha): "dot" releases from release branches require releasing nightlies
    # for such branches, which we don't do yet.
    [[ "${RELEASE_VERSION}" =~ ^[0-9]+\.[0-9]+\.0$ ]] || abort "version format must be 'X.Y.0'"
    RELEASE_BRANCH="release-$(major_minor_version "${RELEASE_VERSION}")"
    prepare_from_nightly_release
    setup_upstream
  fi

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
    RELEASE_GCS_BUCKET=""
    [[ -z "${RELEASE_DIR}" ]] && RELEASE_DIR="${REPO_ROOT_DIR}"
  fi

  # Set signing identity for cosign, it would already be set to the RELEASE one if the release-gcr/release-gcs flags are set
  if [[ -z "${SIGNING_IDENTITY}" ]]; then
    SIGNING_IDENTITY="${NIGHTLY_SIGNING_IDENTITY}"
  fi

  [[ -z "${RELEASE_GCS_BUCKET}" && -z "${RELEASE_DIR}" ]] && abort "--release-gcs or --release-dir must be used"
  if [[ -n "${RELEASE_DIR}" ]]; then
    mkdir -p "${RELEASE_DIR}" || abort "cannot create release dir '${RELEASE_DIR}'"
  fi

  # Get the commit, excluding any tags but keeping the "dirty" flag
  BUILD_COMMIT_HASH="$(git describe --always --dirty --match '^$')"
  [[ -n "${BUILD_COMMIT_HASH}" ]] || abort "error getting the current commit"
  BUILD_YYYYMMDD="$(date -u +%Y%m%d)"
  BUILD_TIMESTAMP="$(date -u '+%Y-%m-%d %H:%M:%S')"
  BUILD_TAG="v${BUILD_YYYYMMDD}-${BUILD_COMMIT_HASH}"

  (( TAG_RELEASE )) && TAG="${BUILD_TAG}"
  [[ -n "${RELEASE_VERSION}" ]] && TAG="v${RELEASE_VERSION}"
  [[ -n "${RELEASE_VERSION}" && -n "${RELEASE_BRANCH}" ]] && (( PUBLISH_RELEASE )) && PUBLISH_TO_GITHUB=1

  readonly BUILD_COMMIT_HASH
  readonly BUILD_YYYYMMDD
  readonly BUILD_TIMESTAMP
  readonly BUILD_TAG
  readonly SKIP_TESTS
  readonly TAG_RELEASE
  readonly PUBLISH_RELEASE
  readonly PUBLISH_TO_GITHUB
  readonly TAG
  readonly RELEASE_VERSION
  readonly RELEASE_NOTES
  readonly RELEASE_BRANCH
  readonly RELEASE_GCS_BUCKET
  readonly RELEASE_DIR
  readonly VALIDATION_TESTS
  readonly FROM_NIGHTLY_RELEASE
  readonly SIGNING_IDENTITY
}

# Run tests (unless --skip-tests was passed). Conveniently displays a banner indicating so.
# Parameters: $1 - executable that runs the tests.
function run_validation_tests() {
  (( SKIP_TESTS )) && return
  banner "Running release validation tests"
  # Unset KO_DOCKER_REPO and restore it after the tests are finished.
  # This will allow the tests to define their own KO_DOCKER_REPO.
  local old_docker_repo="${KO_DOCKER_REPO}"
  unset KO_DOCKER_REPO
  # Run tests.
  if ! $1; then
    banner "Release validation tests failed, aborting"
    abort "release validation tests failed"
  fi
  export KO_DOCKER_REPO="${old_docker_repo}"
}

# Publishes the generated artifacts to directory, GCS, GitHub, etc.
# Parameters: $1..$n - files to add to the release.
function publish_artifacts() {
  (( ! PUBLISH_RELEASE )) && return
  if [[ -n "${RELEASE_DIR}" ]]; then
    cp "${ARTIFACTS_TO_PUBLISH}" "${RELEASE_DIR}" || abort "cannot copy release to '${RELEASE_DIR}'"
  fi
  [[ -n "${RELEASE_GCS_BUCKET}" ]] && publish_to_gcs "${ARTIFACTS_TO_PUBLISH}"
  publish_to_github "${ARTIFACTS_TO_PUBLISH}"
  set_latest_to_highest_semver
  banner "New release published successfully"
}

# Sets the github release with the highest semver to 'latest'
function set_latest_to_highest_semver() {
  if ! (( PUBLISH_TO_GITHUB )); then
    return 0
  fi
  echo "Setting latest release to highest semver"
  
  local last_version release_id  # don't combine with assignment else $? will be 0

  last_version="$(gh_tool release list --json tagName --jq '.[].tagName' | cut -d'-' -f2 | grep '^v[0-9]\+\.[0-9]\+\.[0-9]\+$'| sort -r -V | head -1)"
  if ! [[ $? -eq 0 ]]; then
    abort "cannot list releases"
  fi

  gh_tool release edit "knative-${last_version}" --latest > /dev/null || abort "error setting $last_version to 'latest'"
  echo "Github release ${last_version} set as 'latest'"
}

# Entry point for a release script.
function main() {
  parse_flags "$@"

  # Checkout specific branch, if necessary
  local current_branch
  current_branch="$(git rev-parse --abbrev-ref HEAD)"
  if [[ -n "${RELEASE_BRANCH}" && -z "${FROM_NIGHTLY_RELEASE}" && "${current_branch}" != "${RELEASE_BRANCH}" ]]; then
    setup_upstream
    setup_branch
    # When it runs in Prow, the origin is identical with upstream, and previous
    # fetch already fetched release-* branches, so no need to `checkout -b`
    if (( IS_PROW )); then
      git checkout "${RELEASE_BRANCH}" || abort "cannot checkout branch ${RELEASE_BRANCH}"
    else
      git checkout -b "${RELEASE_BRANCH}" upstream/"${RELEASE_BRANCH}" || abort "cannot checkout branch ${RELEASE_BRANCH}"
    fi
    # HACK HACK HACK
    # Rerun the release script from the release branch. Fixes https://github.com/knative/test-infra/issues/1262
    ./hack/release.sh "$@"
    exit "$?"
  fi

  function_exists build_release || abort "function 'build_release()' not defined"
  [[ -x ${VALIDATION_TESTS} ]] || abort "test script '${VALIDATION_TESTS}' doesn't exist"

  banner "Environment variables"
  env
  # Log what will be done and where.
  banner "Release configuration"
  if which gcloud &>/dev/null ; then
    echo "- gcloud user: $(gcloud config get-value core/account)"
  fi
  echo "- Go path: ${GOPATH}"
  echo "- Repository root: ${REPO_ROOT_DIR}"
  echo "- Destination GCR: ${KO_DOCKER_REPO}"
  (( SKIP_TESTS )) && echo "- Tests will NOT be run" || echo "- Tests will be run"
  if (( TAG_RELEASE )); then
    echo "- Artifacts will be tagged '${TAG}'"
    # We want to add git tags to the container images built by ko
    KO_FLAGS+=" --tags=latest,${TAG}"
  else
    echo "- Artifacts WILL NOT be tagged"
  fi
  if (( PUBLISH_RELEASE )); then
    local dst="${RELEASE_DIR}"
    [[ -z "${dst}" ]] && dst="${RELEASE_GCS_BUCKET}"
    echo "- Release WILL BE published to '${dst}'"
  else
    echo "- Release will not be published"
  fi
  if (( PUBLISH_TO_GITHUB )); then
    echo "- Release WILL BE published to GitHub"
  fi
  if [[ -n "${FROM_NIGHTLY_RELEASE}" ]]; then
    echo "- Release will be A COPY OF '${FROM_NIGHTLY_RELEASE}' nightly"
  else
    echo "- Release will be BUILT FROM SOURCE"
    [[ -n "${RELEASE_BRANCH}" ]] && echo "- Release will be built from branch '${RELEASE_BRANCH}'"
  fi
  [[ -n "${RELEASE_NOTES}" ]] && echo "- Release notes are generated from '${RELEASE_NOTES}'"

  if [[ -n "${FROM_NIGHTLY_RELEASE}" ]]; then
    build_from_nightly_release
  else
    set -e -o pipefail
    build_from_source
    set +e +o pipefail
  fi
  [[ -z "${ARTIFACTS_TO_PUBLISH}" ]] && abort "no artifacts were generated"
  # Ensure no empty file will be published.
  for artifact in ${ARTIFACTS_TO_PUBLISH}; do
    [[ -s ${artifact} ]] || abort "Artifact ${artifact} is empty"
  done
  echo "New release built successfully"
  publish_artifacts
}

# Publishes a new release on GitHub, also git tagging it (unless this is not a versioned release).
# Parameters: $1..$n - files to add to the release.
function publish_to_github() {
  (( PUBLISH_TO_GITHUB )) || return 0
  local title="${TAG}"
  local attachments=()
  local description="$(mktemp)"
  local attachments_dir="$(mktemp -d)"
  local commitish=""
  local target_branch=""
  local github_tag="knative-${TAG}"

  # Copy files to a separate dir
  # shellcheck disable=SC2068
  for artifact in $@; do
    cp ${artifact} "${attachments_dir}"/
    attachments+=("${artifact}#$(basename ${artifact})")
  done
  echo -e "${title}\n" > "${description}"
  if [[ -n "${RELEASE_NOTES}" ]]; then
    cat "${RELEASE_NOTES}" >> "${description}"
  fi

  # Include a tag for the go module version
  #
  # v1.0.0 = v0.27.0
  # v1.0.1 = v0.27.1
  # v1.1.1 = v0.28.1
  #
  # See: https://github.com/knative/hack/pull/97
  if [[ "$TAG" == "v1"* ]]; then
    local release_minor=$(minor_version $TAG)
    local go_module_version="v0.$(( release_minor + 27 )).$(patch_version $TAG)"
    git tag -a "${go_module_version}" -m "${title}"
    git_push tag "${go_module_version}"
  else
    # Pre-1.0 - use the tag as the release tag
    github_tag="${TAG}"
  fi

  git tag -a "${github_tag}" -m "${title}"
  git_push tag "${github_tag}"

  [[ -n "${RELEASE_BRANCH}" ]] && target_branch="--target=${RELEASE_BRANCH}"
  for i in {2..0}; do
    # shellcheck disable=SC2068
    gh_tool release create \
      "${github_tag}" \
      --title "${title}" \
      --notes-file "${description}" \
      "${target_branch}" \
      ${attachments[@]} && return 0

    if [[ "${i}" -gt 0 ]]; then
      echo "Error publishing the release, retrying in 15s..."
      sleep 15
    fi
  done
  abort "Cannot publish release to GitHub"
}
