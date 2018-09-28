# Knative Test Infrastructure

The `test-infra` repository contains a collection of tools for testing Knative, collecting metrics
and displaying test results.

## High level architecture

Knative uses [Prow](https://github.com/kubernetes/test-infra/tree/master/prow) to schedule testing and update issues.

### E2E Testing

Our E2E testing uses [kubetest](https://github.com/kubernetes/test-infra/blob/master/kubetest) to build/deploy/test Knative clusters.
