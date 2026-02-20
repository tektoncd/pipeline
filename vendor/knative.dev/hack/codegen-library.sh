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

# Setup the env for doing Knative style codegen.

# Store Bash options
oldstate="$(set +o)"

set -Eeuo pipefail

export repodir kn_hack_dir kn_hack_library \
  MODULE_NAME CODEGEN_TMP_GOPATH CODEGEN_ORIGINAL_GOPATH GOPATH GOBIN \
  CODEGEN_PKG KNATIVE_CODEGEN_PKG

kn_hack_dir="$(realpath "$(dirname "${BASH_SOURCE[0]:-$0}")")"
kn_hack_library=${kn_hack_library:-"${kn_hack_dir}/library.sh"}

if [[ -f "$kn_hack_library" ]]; then
  # shellcheck disable=SC1090
  source "$kn_hack_library"
else
  echo "The \$kn_hack_library points to a non-existent file: $kn_hack_library" >&2
  exit 42
fi

repodir="$(go_run knative.dev/toolbox/modscope@latest current --path)"

function go-resolve-pkg-dir() {
  local pkg="${1:?Pass the package name}"
  local pkgdir
  if [ -d "${repodir}/vendor" ]; then
    pkgdir="${repodir}/vendor/${pkg}"
    if [ -d "${pkgdir}" ]; then
      echo "${pkgdir}"
      return 0
    else
      return 1
    fi
  else
    go mod download -x > /dev/stderr
    go list -m -f '{{.Dir}}' "${pkg}" 2>/dev/null
    return $?
  fi
}

# Change dir to the original executing script's directory, not the current source!
pushd "$(dirname "$(realpath "$0")")" > /dev/null

if ! CODEGEN_PKG="${CODEGEN_PKG:-"$(go-resolve-pkg-dir k8s.io/code-generator)"}"; then
  warning "Failed to determine the k8s.io/code-generator package"
fi
if ! KNATIVE_CODEGEN_PKG="${KNATIVE_CODEGEN_PKG:-"$(go-resolve-pkg-dir knative.dev/pkg)"}"; then
  warning "Failed to determine the knative.dev/pkg package"
fi

popd > /dev/null

CODEGEN_ORIGINAL_GOPATH="$(go env GOPATH)"
CODEGEN_TMP_GOPATH=$(go_mod_gopath_hack)
GOPATH="${CODEGEN_TMP_GOPATH}"
GOBIN="${GOPATH}/bin" # Set GOBIN explicitly as k8s-gen' are installed by go install.

if [[ -n "${CODEGEN_PKG}" ]] && ! [ -x "${CODEGEN_PKG}/generate-groups.sh" ]; then
  chmod +x "${CODEGEN_PKG}/generate-groups.sh"
  chmod +x "${CODEGEN_PKG}/generate-internal-groups.sh"
fi
if [[ -n "${KNATIVE_CODEGEN_PKG}" ]] && ! [ -x "${KNATIVE_CODEGEN_PKG}/hack/generate-knative.sh" ]; then
  chmod +x "${KNATIVE_CODEGEN_PKG}/hack/generate-knative.sh"
fi

# Generate boilerplate file with the current year
function boilerplate() {
  local go_header_file="${kn_hack_dir}/boilerplate.go.txt"
  local current_boilerplate_file="${TMPDIR}/boilerplate.go.txt"
  # Replace #{YEAR} with the current year
  sed "s/#{YEAR}/$(date +%Y)/" \
    < "${go_header_file}" \
    > "${current_boilerplate_file}"
  echo "${current_boilerplate_file}"
}

# Generate K8s' groups codegen
function generate-groups() {
  if [[ -z "${CODEGEN_PKG}" ]]; then
    abort "CODEGEN_PKG is not set"
  fi
  "${CODEGEN_PKG}"/generate-groups.sh \
    "$@" \
    --go-header-file "$(boilerplate)"
}

# Generate K8s' internal groups codegen
function generate-internal-groups() {
  if [[ -z "${CODEGEN_PKG}" ]]; then
    abort "CODEGEN_PKG is not set"
  fi
  "${CODEGEN_PKG}"/generate-internal-groups.sh \
    "$@" \
    --go-header-file "$(boilerplate)"
}

# Generate Knative style codegen
function generate-knative() {
  if [[ -z "${KNATIVE_CODEGEN_PKG}" ]]; then
    abort "KNATIVE_CODEGEN_PKG is not set"
  fi
  "${KNATIVE_CODEGEN_PKG}/hack/generate-knative.sh" \
    "$@" \
    --go-header-file "$(boilerplate)"
}

# Cleanup after generating code
function cleanup-codegen() {
  restore-changes-if-its-copyright-year-only
  restore-gopath
}

# Restore changes if the file contains only the change in the copyright year
function restore-changes-if-its-copyright-year-only() {
  local difflist
  log "Cleaning up generated code"
  difflist="$(mktemp)"
  if ! git diff --exit-code --name-only > /dev/null; then
    # list git changes and skip those which differ only in the boilerplate year
    git diff --name-only > "$difflist"  
    while read -r file; do
      # check if the file contains just the change in the boilerplate year
      if [ "$(LANG=C git diff --ignore-space-change --exit-code --shortstat -- "$file")" = ' 1 file changed, 1 insertion(+), 1 deletion(-)' ] && \
          [[ "$(git diff --ignore-space-change --exit-code -U1 -- "$file" | grep -Ec '^[+-]\s*[*#]?\s*Copyright 2[0-9]{3}')" -eq 2 ]]; then
        # restore changes to that file
        git checkout -- "$file"
      fi
    done < "$difflist"
  fi
  rm -f "$difflist"
}

# Restore the GOPATH and clean up the temporary directory
function restore-gopath() {
  # Skip this if the directory is already checked out onto the GOPATH.
  if __is_checkout_onto_gopath; then
    return
  fi
  if [ -d "$CODEGEN_TMP_GOPATH" ]; then
    chmod -R u+w "${CODEGEN_TMP_GOPATH}"
    rm -rf "${CODEGEN_TMP_GOPATH}"
    unset CODEGEN_TMP_GOPATH
  fi
  unset GOPATH GOBIN
  # Restore the original GOPATH, if it was different from the default one.
  if [ "$CODEGEN_ORIGINAL_GOPATH" != "$(go env GOPATH)" ]; then
    export GOPATH="$CODEGEN_ORIGINAL_GOPATH"
  fi
  unset CODEGEN_ORIGINAL_GOPATH
}

add_trap cleanup-codegen EXIT

# Restore Bash options
eval "$oldstate"
