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

# This is a helper script for Knative release scripts. To use it:
# 1. Source this script.
# 2. Call the parse_flags() function passing $@ (without quotes).
# 3. Call the run_validation_tests() passing the script or executable that
#    runs the release validation tests.
# 4. Write logic for the release process. Use the following boolean (0 is
#    false, 1 is true) environment variables for the logic:
#    SKIP_TESTS: true if --skip-tests is passed. This is handled automatically
#                by the run_validation_tests() function.
#    TAG_RELEASE: true if --tag-release is passed. In this case, the
#                 environment variable TAG will contain the release tag in the
#                 form vYYYYMMDD-<commit_short_hash>.
#    PUBLISH_RELEASE: true if --publish is passed. In this case, the environment
#                     variable KO_FLAGS will be updated with the -L option.
#    SKIP_TESTS, TAG_RELEASE and PUBLISH_RELEASE default to false for safety.
#    All environment variables above, except KO_FLAGS, are marked read-only once
#    parse_flags() is called.

source $(dirname ${BASH_SOURCE})/library.sh

# Simple banner for logging purposes.
function banner() {
    make_banner "@" "$1"
}

# Tag images in the yaml file with a tag. If not tag is passed, does nothing.
# Parameters: $1 - yaml file to parse for images.
#             $2 - registry where the images are stored.
#             $3 - tag to apply (optional).
function tag_images_in_yaml() {
  [[ -z $3 ]] && return 0
  echo "Tagging images with $3"
  for image in $(grep -o "$2/[a-z\./-]\+@sha256:[0-9a-f]\+" $1); do
    gcloud -q container images add-tag ${image} ${image%%@*}:$3
  done
}

# Copy the given yaml file to a GCS bucket. Image is tagged :latest, and optionally :$2.
# Parameters: $1 - yaml file to copy.
#             $2 - destination bucket name.
#             $3 - tag to apply (optional).
function publish_yaml() {
  gsutil cp $1 gs://$2/latest/
  [[ -n $3 ]] && gsutil cp $1 gs://$2/previous/$3/
}

SKIP_TESTS=0
TAG_RELEASE=0
PUBLISH_RELEASE=0
TAG=""
KO_FLAGS="-P -L"

# Parses flags and sets environment variables accordingly.
function parse_flags() {
  cd ${REPO_ROOT_DIR}
  for parameter in $@; do
    case $parameter in
      --skip-tests) SKIP_TESTS=1 ;;
      --tag-release) TAG_RELEASE=1 ;;
      --notag-release) TAG_RELEASE=0 ;;
      --publish)
        PUBLISH_RELEASE=1
        # Remove -L from ko flags
        KO_FLAGS="${KO_FLAGS/-L}"
        ;;
      --nopublish)
        PUBLISH_RELEASE=0
        # Add -L to ko flags
        KO_FLAGS="-L ${KO_FLAGS}"
        shift
        ;;
      *)
        echo "error: unknown option ${parameter}"
        exit 1
        ;;
    esac
    shift
  done

  TAG=""
  if (( TAG_RELEASE )); then
    # Currently we're not considering the tags in refs/tags namespace.
    commit=$(git describe --always --dirty)
    # Like kubernetes, image tag is vYYYYMMDD-commit
    TAG="v$(date +%Y%m%d)-${commit}"
  fi

  readonly SKIP_TESTS
  readonly TAG_RELEASE
  readonly PUBLISH_RELEASE
  readonly TAG
}

# Run tests (unless --skip-tests was passed). Conveniently displays a banner indicating so.
# Parameters: $1 - executable that runs the tests.
function run_validation_tests() {
  if (( ! SKIP_TESTS )); then
    banner "Running release validation tests"
    # Run tests.
    $1
  fi
}
