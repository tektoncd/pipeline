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

# Print error message and exit 1
# Parameters: $1..$n - error message to be displayed
function abort() {
    echo "error: $*" >&2
    exit 1
}

# Check if crane is available
if ! command -v crane &>/dev/null; then
    abort "crane command not found. Please install it from https://github.com/google/go-containerregistry"
fi

# Check if .ko.yaml exists
if [[ ! -f ".ko.yaml" ]]; then
    abort ".ko.yaml file not found in current directory"
fi

# Set default KOCACHE if not set
KOCACHE="${KOCACHE:-${HOME}/.ko}"

# Create output directory if it doesn't exist
IMG_DIR="${KOCACHE}/img"
mkdir -p "${IMG_DIR}"

# Extract images with digests from .ko.yaml
# Look for lines containing @sha256: or @sha512: followed by a digest
# Extract the image reference (everything from start of image to end of digest, ignoring comments)
DIGEST_IMAGES=()

while IFS= read -r line; do
    # Skip comments and empty lines
    line_trimmed=$(echo "${line}" | sed 's/#.*$//' | sed 's/^[[:space:]]*//' | sed 's/[[:space:]]*$//')
    if [[ -z "${line_trimmed}" ]]; then
        continue
    fi

    # Extract image references with digests
    # Pattern: match image refs like registry/repo:tag@sha256:digest or registry/repo@sha256:digest
    if echo "${line_trimmed}" | grep -qE '@sha(256|512):[a-f0-9]+'; then
        # Extract the value part after the colon (for YAML key: value)
        if echo "${line_trimmed}" | grep -qE '^[^:]+:[[:space:]]+'; then
            # YAML key: value format - extract the value
            image=$(echo "${line_trimmed}" | sed -E 's/^[^:]+:[[:space:]]+//' | sed 's/#.*$//' | sed 's/[[:space:]]*$//')
            # Check if it's an image with digest
            if echo "${image}" | grep -qE '@sha(256|512):[a-f0-9]+'; then
                DIGEST_IMAGES+=("${image}")
            fi
        else
            # Might be standalone or in a different format, try to extract directly
            image=$(echo "${line_trimmed}" | grep -oE '[^[:space:]]+@sha(256|512):[a-f0-9]+' | head -1)
            if [[ -n "${image}" ]]; then
                DIGEST_IMAGES+=("${image}")
            fi
        fi
    fi
done <.ko.yaml

if [[ ${#DIGEST_IMAGES[@]} -eq 0 ]]; then
    echo "No images with digests found in .ko.yaml"
    exit 0
fi

# Remove duplicates
IFS=$'\n' sorted_unique_images=($(sort -u <<<"${DIGEST_IMAGES[*]}"))
unset IFS

echo "Found ${#sorted_unique_images[@]} unique image(s) with digests in .ko.yaml:"
for img in "${sorted_unique_images[@]}"; do
    echo "  - ${img}"
done
echo ""

# Check if cache directory already has content
CACHE_POPULATED=false
if [[ -d "${IMG_DIR}" ]] && [[ -n "$(ls -A "${IMG_DIR}" 2>/dev/null)" ]]; then
    echo "Cache directory ${IMG_DIR} already contains images, skipping download"
    CACHE_POPULATED=true
fi

# Pull each image only if cache is not already populated
if [[ "${CACHE_POPULATED}" == "false" ]]; then
    for image in "${sorted_unique_images[@]}"; do
        echo "Pulling ${image}..."
        if crane pull --format oci "${image}" "${IMG_DIR}"; then
            echo "Successfully pulled ${image} to ${IMG_DIR}"
        else
            abort "Failed to pull ${image}"
        fi
    done
    echo ""
    echo "All images have been pulled to ${IMG_DIR}"
else
    echo "Using cached images from ${IMG_DIR}"
fi
