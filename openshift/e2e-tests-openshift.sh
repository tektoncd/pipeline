#!/usr/bin/env bash
set -e
source $(dirname $0)/../vendor/github.com/tektoncd/plumbing/scripts/e2e-tests.sh
source $(dirname $0)/resolve-yamls.sh

set -x

readonly API_SERVER=$(oc config view --minify | grep server | awk -F'//' '{print $2}' | awk -F':' '{print $1}')
readonly OPENSHIFT_REGISTRY="${OPENSHIFT_REGISTRY:-"registry.svc.ci.openshift.org"}"
readonly TEST_NAMESPACE=tekton-pipeline-tests
readonly TEST_YAML_NAMESPACE=tekton-pipeline-tests-yaml
readonly TEKTON_PIPELINE_NAMESPACE=tekton-pipelines
readonly IGNORES="pipelinerun.yaml|pull-private-image.yaml|build-push-kaniko.yaml|gcs|git-volume.yaml"
readonly KO_DOCKER_REPO=image-registry.openshift-image-registry.svc:5000/tektoncd-pipeline
# Where the CRD will install the pipelines
readonly TEKTON_NAMESPACE=tekton-pipelines
# Variable usually set by openshift CI but generating one if not present when running it locally
readonly OPENSHIFT_BUILD_NAMESPACE=${OPENSHIFT_BUILD_NAMESPACE:-tektoncd-build-$$}
# Yaml test skipped due of not being able to run on openshift CI, usually becaus
# of rights.
# test-git-volume: `"gitRepo": gitRepo volumes are not allowed to be used]'
# dind-sidecar-taskrun-1: securityContext.privileged: Invalid value: true: Privileged containers are not allowed]
# gcs: google container storage
declare -ar SKIP_YAML_TEST=(test-git-volume dind-sidecar-taskrun-1 build-gcs-targz build-gcs-zip gcs-resource)

function install_tekton_pipeline() {
  header "Installing Tekton Pipeline"

  create_pipeline

  wait_until_pods_running $TEKTON_PIPELINE_NAMESPACE || return 1

  header "Tekton Pipeline Installed successfully"
}

function create_pipeline() {
  resolve_resources config/ tekton-pipeline-resolved.yaml "nothing" $OPENSHIFT_REGISTRY/$OPENSHIFT_BUILD_NAMESPACE/stable

  # NOTE(chmou): This is a very cheeky hack, sidecar is currently broken with
  # our nop image so we just use nightly `nop` from upstream CI. `nop` should
  # not change or do anything differently with a different base so we should be
  # safe until https://github.com/tektoncd/pipeline/issues/1347 gets fixed
  sed -i 's%"-nop-image.*%"-nop-image", "gcr.io/tekton-nightly/github.com/tektoncd/pipeline/cmd/nop:latest",%' tekton-pipeline-resolved.yaml

  oc apply -f tekton-pipeline-resolved.yaml
}

function create_test_namespace() {
  for ns in ${TEKTON_NAMESPACE} ${OPENSHIFT_BUILD_NAMESPACE} ${TEST_YAML_NAMESPACE} ${TEST_NAMESPACE};do
     oc get project ${ns} >/dev/null 2>/dev/null || oc new-project ${ns}
  done

  oc policy add-role-to-group system:image-puller system:serviceaccounts:$TEST_YAML_NAMESPACE -n $OPENSHIFT_BUILD_NAMESPACE
  oc policy add-role-to-group system:image-puller system:serviceaccounts:$TEST_NAMESPACE -n $OPENSHIFT_BUILD_NAMESPACE
}

function run_go_e2e_tests() {
  header "Running Go e2e tests"
  go test -v -failfast -count=1 -tags=e2e -ldflags '-X github.com/tektoncd/pipeline/test.missingKoFatal=false' ./test -timeout=20m --kubeconfig $KUBECONFIG || return 1
}

function run_yaml_e2e_tests() {
  header "Running YAML e2e tests"
  oc project $TEST_YAML_NAMESPACE
  resolve_resources examples/ tests-resolved.yaml $IGNORES $OPENSHIFT_REGISTRY/$OPENSHIFT_BUILD_NAMESPACE/stable
  oc apply -f tests-resolved.yaml

  # The rest of this function copied from test/e2e-common.sh#run_yaml_tests()
  # The only change is "kubectl get builds" -> "oc get builds.build.knative.dev"
  oc get project

  # Wait for tests to finish.
  echo ">> Waiting for tests to finish"
  for test in taskrun pipelinerun; do
    if validate_run ${test}; then
      echo "ERROR: tests timed out"
    fi
  done

  # Check that tests passed.
  echo ">> Checking test results"
  for test in taskrun pipelinerun; do
    if check_results ${test}; then
      echo ">> All YAML tests passed"
      return 0
    fi
  done

  # it failed, display logs
  for test in taskrun pipelinerun; do
    echo "<< State and Logs for ${test}"
    output_yaml_test_results ${test}
    output_pods_logs ${test}
  done
  return 1
}

function validate_run() {
  local tests_finished=0
  for i in {1..120}; do
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
    reltestname=${result/=*Succeeded*/}
    skipit=
    for skip in ${SKIP_YAML_TEST[@]};do
        [[ ${reltestname} == ${skip} ]] && skipit=True
    done
    [[ -n ${skipit} ]] && {
        echo "INFO: skipping yaml test ${reltestname}"
        continue
    }
    if [[ ! "${result,,}" == *"=succeededtrue" ]]; then
      echo "ERROR: test ${result} but should be succeededtrue"
      kubectl get $1.tekton.dev ${reltestname} -o yaml
      failed=1
    fi
  done

  return ${failed}
}

function output_yaml_test_results() {
  # If formatting fails for any reason, use yaml as a fall back.
  oc get $1.tekton.dev -o=custom-columns-file=${REPO_ROOT_DIR}/test/columns.txt ||
    oc get $1.tekton.dev -oyaml
}

function output_pods_logs() {
  echo ">>> $1"
  oc get $1.tekton.dev -o yaml
  local runs=$(kubectl get $1.tekton.dev --output=jsonpath="{.items[*].metadata.name}")
  set +e
  for run in ${runs}; do
    echo ">>>> $1 ${run}"
    case "$1" in
    "taskrun")
      go run ./test/logs/main.go -tr ${run}
      ;;
    "pipelinerun")
      go run ./test/logs/main.go -pr ${run}
      ;;
    esac
  done
  set -e
  echo ">>>> Pods"
  kubectl get pods -o yaml
}

function delete_build_pipeline_openshift() {
  echo ">> Bringing down Build"
  # Make sure that are no residual object in the tekton-pipelines namespace.
  oc delete --ignore-not-found=true taskrun.tekton.dev --all -n $TEKTON_PIPELINE_NAMESPACE
  oc delete --ignore-not-found=true pipelinerun.tekton.dev --all -n $TEKTON_PIPELINE_NAMESPACE
  oc delete --ignore-not-found=true task.tekton.dev --all -n $TEKTON_PIPELINE_NAMESPACE
  oc delete --ignore-not-found=true clustertask.tekton.dev --all -n $TEKTON_PIPELINE_NAMESPACE
  oc delete --ignore-not-found=true pipeline.tekton.dev --all -n $TEKTON_PIPELINE_NAMESPACE
  oc delete --ignore-not-found=true pipelineresources.tekton.dev --all -n $TEKTON_PIPELINE_NAMESPACE
  oc delete --ignore-not-found=true -f tekton-pipeline-resolved.yaml
}

function delete_test_resources_openshift() {
  echo ">> Removing test resources (test/)"
  # ignore any errors while deleting tests-resolved.yaml
  # some of the resources use `GenerateName` instead of `Name`
  oc delete --ignore-not-found=true -f tests-resolved.yaml || true
}

function delete_test_namespace() {
  echo ">> Deleting test namespace $TEST_NAMESPACE"
  #oc policy remove-role-from-group system:image-puller system:serviceaccounts:$TEST_NAMESPACE -n $OPENSHIFT_BUILD_NAMESPACE
  #oc delete project $TEST_NAMESPACE
  oc policy remove-role-from-group system:image-puller system:serviceaccounts:$TEST_YAML_NAMESPACE -n $OPENSHIFT_BUILD_NAMESPACE
  oc delete project $TEST_YAML_NAMESPACE
}

function teardown() {
  delete_test_namespace
  delete_test_resources_openshift
  delete_build_pipeline_openshift
}

create_test_namespace

## If we want to debug the E2E script we don't want to use the images from the
## CI, let the user do this by herself in the `tekton-pipelines` namespace and
## use the deployed controller/webhook from there.
[[ -z ${E2E_DEBUG} ]] && install_tekton_pipeline

failed=0

run_go_e2e_tests || failed=1

run_yaml_e2e_tests || failed=1

((failed)) && dump_cluster_state

teardown

((failed)) && exit 1

success
