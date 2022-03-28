#!/usr/bin/env bash

# Copyright 2018 The Tekton Authors
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

# This is a collection of useful bash functions and constants, intended
# to be used in test scripts and the like. It doesn't do anything when
# called from command line.

# Default GKE version to be used with Tekton Serving
readonly SERVING_GKE_VERSION=gke-channel-regular
readonly SERVING_GKE_IMAGE=cos

# Conveniently set GOPATH if unset
if [[ -z "${GOPATH:-}" ]]; then
  export GOPATH="$(go env GOPATH)"
  if [[ -z "${GOPATH}" ]]; then
    echo "WARNING: GOPATH not set and go binary unable to provide it"
  fi
fi

# Useful environment variables
[[ -n "${PROW_JOB_ID:-}" ]] && IS_PROW=1 || IS_PROW=0
readonly IS_PROW
readonly REPO_ROOT_DIR="${REPO_ROOT_DIR:-$(git rev-parse --show-toplevel 2> /dev/null)}"
readonly REPO_NAME="${REPO_NAME:-$(basename ${REPO_ROOT_DIR} 2> /dev/null)}"

# Set ARTIFACTS to an empty temp dir if unset
if [[ -z "${ARTIFACTS:-}" ]]; then
  export ARTIFACTS="$(mktemp -d)"
fi

# On a Prow job, redirect stderr to stdout so it's synchronously added to log
(( IS_PROW )) && exec 2>&1

# Print error message and exit 1
# Parameters: $1..$n - error message to be displayed
function abort() {
  echo "error: $@"
  exit 1
}

# Display a box banner.
# Parameters: $1 - character to use for the box.
#             $2 - banner message.
function make_banner() {
    local msg="$1$1$1$1 $2 $1$1$1$1"
    local border="${msg//[-0-9A-Za-z _.,\/()]/$1}"
    echo -e "${border}\n${msg}\n${border}"
}

# Simple header for logging purposes.
function header() {
  local upper="$(echo $1 | tr a-z A-Z)"
  make_banner "=" "${upper}"
}

# Simple subheader for logging purposes.
function subheader() {
  make_banner "-" "$1"
}

# Simple warning banner for logging purposes.
function warning() {
  make_banner "!" "$1"
}

# Checks whether the given function exists.
function function_exists() {
  [[ "$(type -t $1)" == "function" ]]
}

# Waits until the given object doesn't exist.
# Parameters: $1 - the kind of the object.
#             $2 - object's name.
#             $3 - namespace (optional).
function wait_until_object_does_not_exist() {
  local KUBECTL_ARGS="get $1 $2"
  local DESCRIPTION="$1 $2"

  if [[ -n $3 ]]; then
    KUBECTL_ARGS="get -n $3 $1 $2"
    DESCRIPTION="$1 $3/$2"
  fi
  echo -n "Waiting until ${DESCRIPTION} does not exist"
  for i in {1..150}; do  # timeout after 5 minutes
    if ! kubectl ${KUBECTL_ARGS} > /dev/null 2>&1; then
      echo -e "\n${DESCRIPTION} does not exist"
      return 0
    fi
    echo -n "."
    sleep 2
  done
  echo -e "\n\nERROR: timeout waiting for ${DESCRIPTION} not to exist"
  kubectl ${KUBECTL_ARGS}
  return 1
}

# Waits until all pods are running in the given namespace.
# Parameters: $1 - namespace.
function wait_until_pods_running() {
  echo -n "Waiting until all pods in namespace $1 are up"
  for i in {1..150}; do  # timeout after 5 minutes
    local pods="$(kubectl get pods --no-headers -n $1 2>/dev/null)"
    # All pods must be running
    local not_running=$(echo "${pods}" | grep -v Running | grep -v Completed | wc -l)
    if [[ -n "${pods}" && ${not_running} -eq 0 ]]; then
      local all_ready=1
      while read pod ; do
        local status=(`echo -n ${pod} | cut -f2 -d' ' | tr '/' ' '`)
        # All containers must be ready
        [[ -z ${status[0]} ]] && all_ready=0 && break
        [[ -z ${status[1]} ]] && all_ready=0 && break
        [[ ${status[0]} -lt 1 ]] && all_ready=0 && break
        [[ ${status[1]} -lt 1 ]] && all_ready=0 && break
        [[ ${status[0]} -ne ${status[1]} ]] && all_ready=0 && break
      done <<< $(echo "${pods}" | grep -v Completed)
      if (( all_ready )); then
        echo -e "\nAll pods are up:\n${pods}"
        return 0
      fi
    fi
    echo -n "."
    sleep 2
  done
  echo -e "\n\nERROR: timeout waiting for pods to come up\n${pods}"
  return 1
}

# Waits until all batch jobs complete in the given namespace.
# Parameters: $1 - namespace.
function wait_until_batch_job_complete() {
  echo -n "Waiting until all batch jobs in namespace $1 run to completion."
  for i in {1..150}; do  # timeout after 5 minutes
    local jobs=$(kubectl get jobs -n $1 --no-headers \
                 -ocustom-columns='n:{.metadata.name},c:{.spec.completions},s:{.status.succeeded}')
    # All jobs must be complete
    local not_complete=$(echo "${jobs}" | awk '{if ($2!=$3) print $0}' | wc -l)
    if [[ ${not_complete} -eq 0 ]]; then
      echo -e "\nAll jobs are complete:\n${jobs}"
      return 0
    fi
    echo -n "."
    sleep 2
  done
  echo -e "\n\nERROR: timeout waiting for jobs to complete\n${jobs}"
  return 1
}

# Waits until the given service has an external address (IP/hostname).
# Parameters: $1 - namespace.
#             $2 - service name.
function wait_until_service_has_external_ip() {
  echo -n "Waiting until service $2 in namespace $1 has an external address (IP/hostname)"
  for i in {1..150}; do  # timeout after 15 minutes
    local ip=$(kubectl get svc -n $1 $2 -o jsonpath="{.status.loadBalancer.ingress[0].ip}")
    if [[ -n "${ip}" ]]; then
      echo -e "\nService $2.$1 has IP $ip"
      return 0
    fi
    local hostname=$(kubectl get svc -n $1 $2 -o jsonpath="{.status.loadBalancer.ingress[0].hostname}")
    if [[ -n "${hostname}" ]]; then
      echo -e "\nService $2.$1 has hostname $hostname"
      return 0
    fi
    echo -n "."
    sleep 6
  done
  echo -e "\n\nERROR: timeout waiting for service $2.$1 to have an external address"
  kubectl get pods -n $1
  return 1
}

# Waits for the endpoint to be routable.
# Parameters: $1 - External ingress IP address.
#             $2 - cluster hostname.
function wait_until_routable() {
  echo -n "Waiting until cluster $2 at $1 has a routable endpoint"
  for i in {1..150}; do  # timeout after 5 minutes
    local val=$(curl -H "Host: $2" "http://$1" 2>/dev/null)
    if [[ -n "$val" ]]; then
      echo -e "\nEndpoint is now routable"
      return 0
    fi
    echo -n "."
    sleep 2
  done
  echo -e "\n\nERROR: Timed out waiting for endpoint to be routable"
  return 1
}

# Returns the name of the first pod of the given app.
# Parameters: $1 - app name.
#             $2 - namespace (optional).
function get_app_pod() {
  local pods=($(get_app_pods $1 $2))
  echo "${pods[0]}"
}

# Returns the name of all pods of the given app.
# Parameters: $1 - app name.
#             $2 - namespace (optional).
function get_app_pods() {
  local namespace=""
  [[ -n $2 ]] && namespace="-n $2"
  kubectl get pods ${namespace} --selector=app=$1 --output=jsonpath="{.items[*].metadata.name}"
}

# Capitalize the first letter of each word.
# Parameters: $1..$n - words to capitalize.
function capitalize() {
  local capitalized=()
  for word in $@; do
    local initial="$(echo ${word:0:1}| tr 'a-z' 'A-Z')"
    capitalized+=("${initial}${word:1}")
  done
  echo "${capitalized[@]}"
}

# Dumps pod logs for the given app.
# Parameters: $1 - app name.
#             $2 - namespace.
function dump_app_logs() {
  echo ">>> ${REPO_NAME_FORMATTED} $1 logs:"
  for pod in $(get_app_pods "$1" "$2")
  do
    echo ">>> Pod: $pod"
    kubectl -n "$2" logs "$pod" -c "$1"
  done
}

# Runs a go test and generate a junit summary.
# Parameters: $1... - parameters to go test
function report_go_test() {
  # Run tests in verbose mode to capture details.
  # go doesn't like repeating -v, so remove if passed.
  local args=" $@ "
  local go_test="go test -race -v ${args/ -v / }"
  # Just run regular go tests if not on Prow.
  echo "Running tests with '${go_test}'"
  local report=$(mktemp)
  ${go_test} | tee ${report}
  local failed=( ${PIPESTATUS[@]} )
  [[ ${failed[0]} -eq 0 ]] && failed=${failed[1]} || failed=${failed[0]}
  echo "Finished run, return code is ${failed}"
  # Install go-junit-report if necessary.
  run_go_tool github.com/jstemmer/go-junit-report go-junit-report --help > /dev/null 2>&1
  local xml="$(mktemp ${ARTIFACTS}/junit_XXXXXXXX).xml"
  cat "${report}" \
      | go-junit-report \
      | sed -e "s#\"github.com/tektoncd/${REPO_NAME}/#\"#g" \
      > ${xml}
  echo "XML report written to ${xml}"
  if (( ! IS_PROW )); then
    # Keep the suffix, so files are related.
    local logfile=${xml/junit_/go_test_}
    logfile=${logfile/.xml/.log}
    cp ${report} ${logfile}
    echo "Test log written to ${logfile}"
  fi
  return ${failed}
}

# Run a go tool, installing it first if necessary.
# Parameters: $1 - tool package/dir for go get/install.
#             $2 - tool to run.
#             $3..$n - parameters passed to the tool.
function run_go_tool() {
  local tool=$2
  if [[ -z "$(which ${tool})" ]]; then
    local action=get
    [[ $1 =~ ^[\./].* ]] && action=install
    go ${action} $1
  fi
  shift 2
  ${tool} "$@"
}

# Update licenses.
# Parameters: $1 - output file, relative to repo root dir.
#             $2...$n - directories and files to inspect.
function update_licenses() {
  cd ${REPO_ROOT_DIR} || return 1
  local dst=$1
  shift
  go-licenses save ./... --save_path=${dst} --force
  # Hack to make sure directories retain write permissions after save. This
  # can happen if the directory being copied is a Go module.
  # See https://github.com/google/go-licenses/issues/11
   chmod +w $(find ${dst} -type d)
}

# Check for forbidden liceses.
# Parameters: $1...$n - directories and files to inspect.
function check_licenses() {
  go-licenses check ./...
}

# Run the given linter on the given files, checking it exists first.
# Parameters: $1 - tool
#             $2 - tool purpose (for error message if tool not installed)
#             $3 - tool parameters (quote if multiple parameters used)
#             $4..$n - files to run linter on
function run_lint_tool() {
  local checker=$1
  local params=$3
  if ! hash ${checker} 2>/dev/null; then
    warning "${checker} not installed, not $2"
    return 127
  fi
  shift 3
  local failed=0
  for file in $@; do
    ${checker} ${params} ${file} || failed=1
  done
  return ${failed}
}

# Check links in the given markdown files.
# Parameters: $1...$n - files to inspect
function check_links_in_markdown() {
  # https://github.com/raviqqe/liche
  local config="${REPO_ROOT_DIR}/test/markdown-link-check-config.rc"
  [[ ! -e ${config} ]] && config="${_PLUMBING_SCRIPTS_DIR}/markdown-link-check-config.rc"
  local options="$(grep '^-' ${config} | tr \"\n\" ' ')"
  run_lint_tool liche "checking links in markdown files" "-d ${REPO_ROOT_DIR} ${options}" $@
}

# Check format of the given markdown files.
# Parameters: $1..$n - files to inspect
function lint_markdown() {
  # https://github.com/markdownlint/markdownlint
  local config="${REPO_ROOT_DIR}/test/markdown-lint-config.rc"
  [[ ! -e ${config} ]] && config="${_PLUMBING_SCRIPTS_DIR}/markdown-lint-config.rc"
  run_lint_tool mdl "linting markdown files" "-c ${config}" $@
}

# Return whether the given parameter is an integer.
# Parameters: $1 - integer to check
function is_int() {
  [[ -n $1 && $1 =~ ^[0-9]+$ ]]
}

# Return whether the given parameter is the tekton release/nightly GCF.
# Parameters: $1 - full GCR name, e.g. gcr.io/tekton-foo-bar
function is_protected_gcr() {
  [[ -n $1 && "$1" =~ "^gcr.io/tekton-(releases|nightly)/?$" ]]
}

# Remove symlinks in a path that are broken or lead outside the repo.
# Parameters: $1 - path name, e.g. vendor
function remove_broken_symlinks() {
  for link in $(find $1 -type l); do
    # Remove broken symlinks
    if [[ ! -e ${link} ]]; then
      unlink ${link}
      continue
    fi
    # Get canonical path to target, remove if outside the repo
    local target="$(ls -l ${link})"
    target="${target##* -> }"
    [[ ${target} == /* ]] || target="./${target}"
    target="$(cd `dirname ${link}` && cd ${target%/*} && echo $PWD/${target##*/})"
    if [[ ${target} != *github.com/tektoncd/* ]]; then
      unlink ${link}
      continue
    fi
  done
}

# Return whether the given parameter is tekton-tests.
# Parameters: $1 - project name
function is_protected_project() {
  [[ -n "$1" && "$1" == "tekton-tests" ]]
}

# Returns the canonical path of a filesystem object.
# Parameters: $1 - path to return in canonical form
#             $2 - base dir for relative links; optional, defaults to current
function get_canonical_path() {
  # We don't use readlink because it's not available on every platform.
  local path=$1
  local pwd=${2:-.}
  [[ ${path} == /* ]] || path="${pwd}/${path}"
  echo "$(cd ${path%/*} && echo $PWD/${path##*/})"
}

# Initializations that depend on previous functions.
# These MUST come last.

readonly _PLUMBING_SCRIPTS_DIR="$(dirname $(get_canonical_path ${BASH_SOURCE[0]}))"
readonly REPO_NAME_FORMATTED="Tekton $(capitalize ${REPO_NAME//-/})"

# Helper functions to run YAML tests
# Taken from tektoncd/pipeline test/e2e-common.sh
function validate_run() {
  local tests_finished=0
  for i in {1..60}; do
    local finished="$(kubectl get $1.tekton.dev --output=jsonpath='{.items[*].status.conditions[*].status}')"
    if [[ ! "$finished" == *"Unknown"* ]]; then
      tests_finished=1
      break
    fi
    sleep 10
  done

  return ${tests_finished}
}

function check_results() {
  local failed=0
  results="$(kubectl get $1.tekton.dev --output=jsonpath='{range .items[*]}{.metadata.name}={.status.conditions[*].type}{.status.conditions[*].status}{" "}{end}')"
  for result in ${results}; do
    if [[ ! "${result,,}" == *"=succeededtrue" ]]; then
      echo "ERROR: test ${result} but should be succeededtrue"
      failed=1
    fi
  done

  return ${failed}
}

function create_resources() {
  local resource=$1
  echo ">> Creating resources ${resource}"

  # Applying the resources, either *taskruns or * *pipelineruns
  for file in $(find ${REPO_ROOT_DIR}/examples/${resource}s/ -name *.yaml | sort); do
    perl -p -e 's/gcr.io\/christiewilson-catfactory/$ENV{KO_DOCKER_REPO}/g' ${file} | ko apply -f - || return 1
  done
}

function run_tests() {
  local resource=$1

  # Wait for tests to finish.
  echo ">> Waiting for tests to finish for ${resource}"
  if validate_run $resource; then
    echo "ERROR: tests timed out"
  fi

  # Check that tests passed.
  echo ">> Checking test results for ${resource}"
  if check_results $resource; then
    echo ">> All YAML tests passed"
    return 0
  fi
  return 1
}

function run_yaml_tests() {
  echo ">> Starting tests for the resource ${1}"
  create_resources ${1}
  if ! run_tests ${1}; then
    return 1
  fi
  return 0
}

function output_yaml_test_results() {
  # If formatting fails for any reason, use yaml as a fall back.
  kubectl get $1.tekton.dev -o=custom-columns-file=${REPO_ROOT_DIR}/test/columns.txt || \
    kubectl get $1.tekton.dev -oyaml
}

function output_pods_logs() {
    echo ">>> $1"
    kubectl get $1.tekton.dev -o yaml
    local runs=$(kubectl get $1.tekton.dev --output=jsonpath="{.items[*].metadata.name}")
    set +e
    for run in ${runs}; do
	echo ">>>> $1 ${run}"
	case "$1" in
	    "taskrun")
		tkn taskrun logs ${run}
		;;
	    "pipelinerun")
		tkn pipelinerun logs ${run}
		;;
	esac
    done
    set -e
    echo ">>>> Pods"
    kubectl get pods -o yaml
}
