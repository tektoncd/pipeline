# Helper scripts

This directory contains helper scripts used by Prow test jobs, as well and
local development scripts.

## Using the `presubmit-tests.sh` helper script

This is a helper script to run the presubmit tests. To use it:

1. Source this script.

1. Define the functions `build_tests()` and `unit_tests()`. They should run all
tests (i.e., not fail fast), and return 0 if all passed, 1 if a failure
occurred. The environment variables `RUN_BUILD_TESTS`, `RUN_UNIT_TESTS` and
`RUN_INTEGRATION_TESTS` are set to 0 (false) or 1 (true) accordingly. If
`--emit-metrics` is passed, `EMIT_METRICS` will be set to 1.

1. [optional] Define the function `integration_tests()`, just like the previous
ones. If you don't define this function, the default action for running the
integration tests is to call the `./test/e2e-tests.sh` script (passing the
`--emit-metrics` flag if necessary).

1. [optional] Define the functions `pre_integration_tests()` or
`post_integration_tests()`. These functions will be called before or after the
integration tests (either your custom one or the default action) and will cause
the test to fail if they don't return success.

1. Call the `main()` function passing `$@` (without quotes).

Running the script without parameters, or with the `--all-tests` flag causes
all tests to be executed, in the right order (i.e., build, then unit, then
integration tests).

Use the flags `--build-tests`, `--unit-tests` and `--integration-tests` to run
a specific set of tests. The flag `--emit-metrics` is used to emit metrics when
running the tests, and is automatically handled by the default action (see
above).

### Sample presubmit test script

```bash
source vendor/github.com/knative/test-infra/scripts/presubmit-tests.sh

function build_tests() {
  go build .
}

function unit_tests() {
  report_go_test .
}

function pre_integration_tests() {
  echo "Cleaning up before integration tests"
  rm -fr ./staging-area
}

# We use the default integration test runner.

main $@
```

## Using the `e2e-tests.sh` helper script

This is a helper script for Knative E2E test scripts. To use it:

1. Source the script.

1. [optional] Write the `teardown()` function, which will tear down your test
resources.

1. [optional] Write the `dump_extra_cluster_state()` function. It will be
called when a test fails, and can dump extra information about the current state
of the cluster (tipically using `kubectl`).

1. [optional] Write the `parse_flags()` function. It will be called whenever an
unrecognized flag is passed to the script, allowing you to define your own flags.
The function must return 0 if the flag is unrecognized, or the number of items
to skip in the command line if the flag was parsed successfully. For example,
return 1 for a simple flag, and 2 for a flag with a parameter.

1. Call the `initialize()` function passing `$@` (without quotes).

1. Write logic for the end-to-end tests. Run all go tests using `go_test_e2e()`
(or `report_go_test()` if you need a more fine-grained control) and call
`fail_test()` or `success()` if any of them failed. The environment variables
`DOCKER_REPO_OVERRIDE`, `K8S_CLUSTER_OVERRIDE` and `K8S_USER_OVERRIDE` will be set
according to the test cluster. You can also use the following boolean (0 is false,
1 is true) environment variables for the logic:
    * `EMIT_METRICS`: true if `--emit-metrics` was passed.
    * `USING_EXISTING_CLUSTER`: true if the test cluster is an already existing one,
and not a temporary cluster created by `kubetest`.

    All environment variables above are marked read-only.

**Notes:**

1. Calling your script without arguments will create a new cluster in the GCP
project `$PROJECT_ID` and run the tests against it.

1. Calling your script with `--run-tests` and the variables `K8S_CLUSTER_OVERRIDE`,
`K8S_USER_OVERRIDE` and `DOCKER_REPO_OVERRIDE` set will immediately start the
tests against the cluster.

1. You can force running the tests against a specific GKE cluster version by using
the `--cluster-version` flag and passing a X.Y.Z version as the flag value.

### Sample end-to-end test script

This script will test that the latest Knative Serving nightly release works. It
defines a special flag (`--no-knative-wait`) that causes the script not to
wait for Knative Serving to be up before running the tests.

```bash
source vendor/github.com/knative/test-infra/scripts/e2e-tests.sh

function teardown() {
  echo "TODO: tear down test resources"
}

function parse_flags() {
  if [[ "$1" == "--no-knative-wait" ]]; then
    WAIT_FOR_KNATIVE=0
    return 1
  fi
  return 0
}

WAIT_FOR_KNATIVE=1

initialize $@

start_latest_knative_serving

if (( WAIT_FOR_KNATIVE )); then
  wait_until_pods_running knative-serving || fail_test "Knative Serving is not up"
fi

# TODO: use go_test_e2e to run the tests.
kubectl get pods || fail_test

success
```

## Using the `release.sh` helper script

This is a helper script for Knative release scripts. To use it:

1. Source the script.

1. Call the `initialize()` function passing `$@` (without quotes).

1. Call the `run_validation_tests()` function passing the script or executable that
runs the release validation tests. It will call the script to run the tests unless
`--skip_tests` was passed.

1. Write logic for the release process. Call `publish_yaml()` to publish the manifest(s),
`tag_releases_in_yaml()` to tag the generated images, `branch_release()` to branch
named releases. Use the following boolean (0 is false, 1 is true) and string environment
variables for the logic:
    * `RELEASE_VERSION`: contains the release version if `--version` was passed. This
also overrides the value of the `TAG` variable as `v<version>`.
    * `RELEASE_BRANCH`: contains the release branch if `--branch` was passed. Otherwise
it's empty and `master` HEAD will be considered the release branch.
    * `RELEASE_NOTES`: contains the filename with the release notes if `--release-notes`
was passed. The release notes is a simple markdown file.
    * `SKIP_TESTS`: true if `--skip-tests` was passed. This is handled automatically
by the run_validation_tests() function.
    * `TAG_RELEASE`: true if `--tag-release` was passed. In this case, the environment
variable `TAG` will contain the release tag in the form `vYYYYMMDD-<commit_short_hash>`.
    * `PUBLISH_RELEASE`: true if `--publish` was passed. In this case, the environment
variable `KO_FLAGS` will be updated with the `-L` option.
    * `BRANCH_RELEASE`: true if both `--version` and `--publish-release` were passed.

    All boolean environment variables default to false for safety.

    All environment variables above, except `KO_FLAGS`, are marked read-only once
`initialize()` is called.

### Sample release script

```bash
source vendor/github.com/knative/test-infra/scripts/release.sh

initialize $@

run_validation_tests ./test/presubmit-tests.sh

# config/ contains the manifests
KO_DOCKER_REPO=gcr.io/knative-foo
ko resolve ${KO_FLAGS} -f config/ > release.yaml

tag_images_in_yaml release.yaml $KO_DOCKER_REPO $TAG

if (( PUBLISH_RELEASE )); then
  # gs://knative-foo hosts the manifest
  publish_yaml release.yaml knative-foo $TAG
fi

branch_release "Knative Foo" release.yaml
```
