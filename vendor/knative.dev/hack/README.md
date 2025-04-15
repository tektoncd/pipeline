# knative.dev/hack

`hack` is a collection of scripts used to bootstrap CI processes and other vital
entrypoint functionality.

## Contributing

If you are interested in contributing to Knative, take a look at [CLOTRIBUTOR](https://clotributor.dev/search?project=knative&page=1)
for a list of help wanted issues across the project.

## Using the `presubmit-tests.sh` helper script

This is a helper script to run the presubmit tests. To use it:

1. Source this script:
   ```bash
   source "$(go run knative.dev/hack/cmd/script presubmit-tests.sh)"
   ```

1. [optional] Define the function `build_tests()`. If you don't define this
   function, the default action for running the build tests is to:

   - run `go build` on the entire repo
   - run `hack/verify-codegen.sh` (if it exists)
   - check licenses in all go packages

1. [optional] Customize the default build test runner, if you're using it. Set
   the following environment variables if the default values don't fit your
   needs:

   - `PRESUBMIT_TEST_FAIL_FAST`: Fail the presubmit test immediately if a test
     fails, defaults to 0 (false).

1. [optional] Define the functions `pre_build_tests()` and/or
   `post_build_tests()`. These functions will be called before or after the
   build tests (either your custom one or the default action) and will cause the
   test to fail if they don't return success.

1. [optional] Define the function `unit_tests()`. If you don't define this
   function, the default action for running the unit tests is to run all go
   tests in the repo.

1. [optional] Define the functions `pre_unit_tests()` and/or
   `post_unit_tests()`. These functions will be called before or after the unit
   tests (either your custom one or the default action) and will cause the test
   to fail if they don't return success.

1. [optional] Define the function `integration_tests()`. If you don't define
   this function, the default action for running the integration tests is to run
   all run all `./test/e2e-*tests.sh` scripts, in sequence.

1. [optional] Define the functions `pre_integration_tests()` and/or
   `post_integration_tests()`. These functions will be called before or after
   the integration tests (either your custom one or the default action) and will
   cause the test to fail if they don't return success.

1. Call the `main()` function passing `"$@"` (with quotes).

Running the script without parameters, or with the `--all-tests` flag causes all
tests to be executed, in the right order (i.e., build, then unit, then
integration tests).

Use the flags `--build-tests`, `--unit-tests` and `--integration-tests` to run a
specific set of tests.

To run specific programs as a test, use the `--run-test` flag, and provide the
program as the argument. If arguments are required for the program, pass
everything as a single quotes argument. For example,
`./presubmit-tests.sh --run-test "test/my/test data"`. This flag can be used
repeatedly, and each one will be ran in sequential order.

The script will automatically skip all presubmit tests for PRs where all changed
files are exempt of tests (e.g., a PR changing only the `OWNERS` file).

Also, for PRs touching only markdown files, the unit and integration tests are
skipped.

### Sample presubmit test script

```bash
source "$(go run knative.dev/hack/cmd/script presubmit-tests.sh)"

function post_build_tests() {
  echo "Cleaning up after build tests"
  rm -fr ./build-cache
}

function unit_tests() {
  make -C tests test
}

function pre_integration_tests() {
  echo "Cleaning up before integration tests"
  rm -fr ./staging-area
}

# We use the default integration test runner.

main "$@"
```

## Using the `e2e-tests.sh` helper script

This is a helper script for Knative E2E test scripts. To use it:

1. [optional] Customize the test cluster. Pass the flags as described
   [here](https://github.com/knative/toolbox/blob/main/kntest/pkg/kubetest2/gke/README.md) to the `initialize` function
   call if the default values don't fit your needs.

1. Source the script:
    ```bash
    source "$(go run knative.dev/hack/cmd/script e2e-tests.sh)"
    ```

1. [optional] Write the `knative_setup()` function, which will set up your
   system under test (e.g., Knative Serving).

1. [optional] Write the `knative_teardown()` function, which will tear down your
   system under test (e.g., Knative Serving).

1. [optional] Write the `test_setup()` function, which will set up the test
   resources.

1. [optional] Write the `test_teardown()` function, which will tear down the
   test resources.

1. [optional] Write the `cluster_setup()` function, which will set up any
   resources before the test cluster is created.

1. [optional] Write the `cluster_teardown()` function, which will tear down any
   resources after the test cluster is destroyed.

1. [optional] Write the `dump_extra_cluster_state()` function. It will be called
   when a test fails, and can dump extra information about the current state of
   the cluster (typically using `kubectl`).

1. [optional] Write the `on_success` function. It will be called when a test succeeds

1. [optional] Write the `on_failure` function. It will be called when a test fails

1. [optional] Write the `parse_flags()` function. It will be called whenever an
   unrecognized flag is passed to the script, allowing you to define your own
   flags. The function must return 0 if the flag is unrecognized, or the number
   of items to skip in the command line if the flag was parsed successfully. For
   example, return 1 for a simple flag, and 2 for a flag with a parameter.

1. Call the `initialize()` function passing `"$@"`.

1. Write logic for the end-to-end tests. Run all go tests using `go_test_e2e()`
   (or `report_go_test()` if you need a more fine-grained control) and call
   `fail_test()` or `success()` if any of them failed. The environment variable
   `KO_DOCKER_REPO` and `E2E_PROJECT_ID` will be set according to the test
   cluster.

**Notes:**

1. Calling your script without arguments will create a new cluster in your
   current GCP project and run the tests against it.

1. Calling your script with `--run-tests` and the variable `KO_DOCKER_REPO` set
   will immediately start the tests against the cluster currently configured for
   `kubectl`.

1. By default `knative_teardown()` and `test_teardown()` will be called after
   the tests finish, use `--skip-teardowns` if you don't want them to be called.

1. By default Google Kubernetes Engine telemetry to Cloud Logging and Monitoring is disabled.
   This can be enabled by setting `ENABLE_GKE_TELEMETRY` to `true`.
   
1. By default Spot Worker nodes are disabled. This can be enabled by setting `ENABLE_PREEMPTIBLE_NODES`
   to `true`.
### Sample end-to-end test script

This script will test that the latest Knative Serving nightly release works. It
defines a special flag (`--no-knative-wait`) that causes the script not to wait
for Knative Serving to be up before running the tests. It also requires that the
test cluster is created in a specific region, `us-west2`.

```bash
source "$(go run knative.dev/hack/cmd/script e2e-tests.sh)"

function knative_setup() {
  start_latest_knative_serving
  if (( WAIT_FOR_KNATIVE )); then
    wait_until_pods_running knative-serving || fail_test "Knative Serving not up"
  fi
}

function parse_flags() {
  if [[ "$1" == "--no-knative-wait" ]]; then
    WAIT_FOR_KNATIVE=0
    return 1
  fi
  return 0
}

WAIT_FOR_KNATIVE=1

# This test requires a cluster in LA
initialize $@ --region=us-west2

# TODO: use go_test_e2e to run the tests.
kubectl get pods || fail_test

success
```

## Using the `performance-tests.sh` helper script

This is a helper script for Knative performance test scripts. In combination
with specific Prow jobs, it can automatically manage the environment for running
benchmarking jobs for each repo. To use it:

1. Source the script:
   ```bash
   source "$(go run knative.dev/hack/cmd/script performance-tests.sh)"
   ```

1. [optional] Customize GCP project settings for the benchmarks. Set the
   following environment variables if the default value doesn't fit your needs:

   - `PROJECT_NAME`: GCP project name for keeping the clusters that run the
     benchmarks. Defaults to `knative-performance`.
   - `SERVICE_ACCOUNT_NAME`: Service account name for controlling GKE clusters
     and interacting with [Mako](https://github.com/google/mako) server. It MUST
     have `Kubernetes Engine Admin` and `Storage Admin` role, and be
     [allowed](https://github.com/google/mako/blob/master/docs/ACCESS.md) by
     Mako admin. Defaults to `mako-job`.

1. [optional] Customize root path of the benchmarks. This root folder should
   contain and only contain all benchmarks you want to run continuously. Set the
   following environment variable if the default value doesn't fit your needs:

   - `BENCHMARK_ROOT_PATH`: Benchmark root path, defaults to
     `test/performance/benchmarks`. Each repo can decide which folder to put its
     benchmarks in, and override this environment variable to be the path of
     that folder.

1. [optional] Write the `update_knative` function, which will update your system
   under test (e.g. Knative Serving).

1. [optional] Write the `update_benchmark` function, which will update the
   underlying resources for the benchmark (usually Knative resources and
   Kubernetes cronjobs for benchmarking). This function accepts a parameter,
   which is the benchmark name in the current repo.

1. Call the `main()` function with all parameters (e.g. `$@`).

### Sample performance test script

This script will update `Knative serving` and the given benchmark.

```bash
source "$(go run knative.dev/hack/cmd/script performance-tests.sh)"

function update_knative() {
  echo ">> Updating serving"
  ko apply -f config/ || abort "failed to apply serving"
}

function update_benchmark() {
  echo ">> Updating benchmark $1"
  ko apply -f ${BENCHMARK_ROOT_PATH}/$1 || abort "failed to apply benchmark $1"
}

main $@
```

## Using the `release.sh` helper script

This is a helper script for Knative release scripts. To use it:

1. Source the script:
    ```bash
    source "$(go run knative.dev/hack/cmd/script release.sh)"
    ```

1. [optional] By default, the release script will run
   `./test/presubmit-tests.sh` as the release validation tests. If you need to
   run something else, set the environment variable `VALIDATION_TESTS` to the
   executable to run.

1. Write logic for building the release in a function named `build_release()`.
   Set the environment variable `ARTIFACTS_TO_PUBLISH` to the list of files
   created, space separated. Use the following boolean (0 is false, 1 is true)
   and string environment variables for the logic:

   - `RELEASE_VERSION`: contains the release version if `--version` was passed.
     This also overrides the value of the `TAG` variable as `v<version>`.
   - `RELEASE_BRANCH`: contains the release branch if `--branch` was passed.
     Otherwise it's empty and `main` HEAD will be considered the release
     branch.
   - `RELEASE_NOTES`: contains the filename with the release notes if
     `--release-notes` was passed. The release notes is a simple markdown file.
   - `RELEASE_GCS_BUCKET`: contains the GCS bucket name to store the manifests
     if `--release-gcs` was passed, otherwise the default value
     `knative-nightly/<repo>` will be used. It is empty if `--publish` was not
     passed.
   - `RELEASE_DIR`: contains the directory to store the manifests if
     `--release-dir` was passed. Defaults to empty value, but if `--nopublish`
     was passed then points to the repository root directory.
   - `BUILD_COMMIT_HASH`: the commit short hash for the current repo. If the
     current git tree is dirty, it will have `-dirty` appended to it.
   - `BUILD_YYYYMMDD`: current UTC date in `YYYYMMDD` format.
   - `BUILD_TIMESTAMP`: human-readable UTC timestamp in `YYYY-MM-DD HH:MM:SS`
     format.
   - `BUILD_TAG`: a tag in the form `v$BUILD_YYYYMMDD-$BUILD_COMMIT_HASH`.
   - `KO_DOCKER_REPO`: contains the GCR to store the images if `--release-gcr`
     was passed, otherwise the default value `gcr.io/knative-nightly` will be
     used. It is set to `ko.local` if `--publish` was not passed.
   - `SKIP_TESTS`: true if `--skip-tests` was passed. This is handled
     automatically.
   - `TAG_RELEASE`: true if `--tag-release` was passed. In this case, the
     environment variable `TAG` will contain the release tag in the form
     `v$BUILD_TAG`.
   - `PUBLISH_RELEASE`: true if `--publish` was passed. In this case, the
     environment variable `KO_FLAGS` will be updated with the `-L` option and
     `TAG` will contain the release tag in the form `v$RELEASE_VERSION`.
   - `PUBLISH_TO_GITHUB`: true if `--version`, `--branch` and
     `--publish-release` were passed.

   All boolean environment variables default to false for safety.

   All environment variables above, except `KO_FLAGS`, are marked read-only once
   `main()` is called (see below).

1. Call the `main()` function passing `"$@"` (with quotes).

### Sample release script

```bash
source "$(go run knative.dev/hack/cmd/script release.sh)"

function build_release() {
  # config/ contains the manifests
  ko resolve ${KO_FLAGS} -f config/ > release.yaml
  ARTIFACTS_TO_PUBLISH="release.yaml"
}

main "$@"
```

# Origins of `hack`

When Kubernetes was first getting started, someone was trying to introduce some
quick shell scripts and land them into the `./scripts` folder. But there was one
that opposed this: Ville Aikas. The compromise was to put those quick scripts in
a folder called `hack` to remind users and developers that there is likely a
better way to perform the task you are attempting that is not using a shell
script, like a tested python script.

> "I was like fine, put them in hack not scripts, cause they are hacks." - Ville Aikas
