# Helper scripts

This directory contains helper scripts used by Prow test jobs, as well and
local development scripts.

## Using the `e2e-tests.sh` helper script

This is a helper script for Knative E2E test scripts. To use it:

1. Source the script.

1. [optional] Write the `teardown()` function, which will tear down your test
resources.

1. [optional] Write the `dump_extra_cluster_state()` function. It will be
called when a test fails, and can dump extra information about the current state
of the cluster (tipically using `kubectl`).

1. Call the `initialize()` function passing `$@` (without quotes).

1. Write logic for the end-to-end tests. Run all go tests using `go_test_e2e()`
(or `report_go_test()` if you need a more fine-grained control) and call
`fail_test()` or `success()` if any of them failed. The environment variables
`DOCKER_REPO_OVERRIDE`, `K8S_CLUSTER_OVERRIDE` and `K8S_USER_OVERRIDE` will be set
according to the test cluster. You can also use the following boolean (0 is false,
1 is true) environment variables for the logic:
  * `EMIT_METRICS`: true if `--emit-metrics` is passed.
  * `USING_EXISTING_CLUSTER`: true if the test cluster is an already existing one,
and not a temporary cluster created by `kubetest`.
All environment variables above are marked read-only.

**Notes:**

1. Calling your script without arguments will create a new cluster in the GCP
project `$PROJECT_ID` and run the tests against it.

1. Calling your script with `--run-tests` and the variables `K8S_CLUSTER_OVERRIDE`,
`K8S_USER_OVERRIDE` and `DOCKER_REPO_OVERRIDE` set will immediately start the
tests against the cluster.

### A minimal end-to-end script runner

This script will test that the latest Knative Serving nightly release works.

```
source vendor/github.com/knative/test-infra/scripts/e2e-tests.sh

function teardown() {
  echo "TODO: tear down test resources"
}

initialize $@

start_latest_knative_serving

wait_until_pods_running knative-serving || fail_test "Knative Serving is not up"

# TODO: use go_test_e2e to run the tests.
kubectl get pods || fail_test

success
```
