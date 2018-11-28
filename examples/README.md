# Examples

This directory contains examples of [the Pipeline CRDs](../README.md) in action.

To deploy them to your cluster (after
[installing the CRDs and running the controller](../DEVELOPMENT.md#getting-started)):

```bash
kubectl apply -f examples/
kubectl apply -f examples/pipelines
kubectl apply -f examples/demo

# To invoke them:
kubectl apply -f examples/demo/run

# TODO(#108): these examples are not known to work
kubectl apply -f examples/invocations
```

## Example Pipelines

We have 3 example [Pipelines](../README.md#pipeline) in [./pipelines](./pipelines)

### Demo Pipeline

[`demo`](./demo): This simple Pipeline Builds
[two microservice images](https://github.com/GoogleContainerTools/skaffold/tree/master/examples/microservices)
from [the Skaffold repo](https://github.com/GoogleContainerTools/skaffold) and deploys them
to the repo currently running the Pipeline CRD.

It does this using the `Deployment` in the existing yaml files, so at the moment there
is no guarantee that the image that are built and pushed are the ones that are deployed
(that would require using the digest of the built image, see
https://github.com/knative/build-pipeline/issues/216).

To run this yourself, you will need to change the values of
`gcr.io/christiewilson-catfactory` to a registry you can push to from inside your cluster.

Since this demo modifies the cluster (deploys to it) you must use a service account with
permission to admin the cluster (or make your default user an admin of the `default`
namespace with [default-cluster-admin.yaml](demo/default-cluster-admin.yaml)).

### Kritis Pipeline

[The Kritis Pipeline](./pipelines/kritis.yaml): This example builds a Pipeline for the
[kritis project](https://github.com/grafeas/kritis), and demonstrates how to configure
a pipeline which:

1. Runs unit tests
2. Build an image
3. Deploys it to a test environment
4. Runs integration tests

![Pipeline Configuration](./pipelines/kritis-pipeline.png)

### Guestbook Pipeline

[Guestbook](./pipelines/guestbook.yaml): This Pipeline is based on example application in
[the Kubernetes example Repo](https://github.com/kubernetes/examples/tree/master/guestbook)
This pipeline demonstartes how to integrate frontend
[guestbook app code](https://github.com/kubernetes/examples/tree/master/guestbook-go) with
backend [redis-docker image](https://github.com/GoogleCloudPlatform/redis-docker/tree/master/4) provided by GCP.

![Pipeline Configuration](./pipelines/guestbook-pipeline.png)

## Example Tasks

* Example [Tasks](../docs/Concepts.md#task) are in:
  * [build_task.yaml](build_task.yaml)
  * [deploy_tasks.yaml](deploy_tasks.yaml)
  * [test_tasks.yaml](test_tasks.yaml)

Here are the Task Types that are defined.

1. `build-push`: This task as the name suggests build an image via [kaniko](https://github.com/GoogleContainerTools/kaniko) and pushes it to registry.
2. `make`:  Runs make target.
3. `integration-test-in-docker`: This is a new task that is used in the sample pipelines to test an app in using `docker build` command to build an image with has the integration test code.
This task then calls `docker run` which will run the test code. This follows the steps we have for [kritis integration test](https://github.com/grafeas/kritis/blob/4f83f99ca58751c28c0ec40016ed0bba5867d70f/Makefile#L152)
4. `deploy-with-helm`: This task deploys a kubernetes app with helm.
5. `deploy-with-kubectl`: This task deploys with kubectl apply -f <filename>

### Example Runs

The [invocations](./invocations/) directory contains an example [TaskRun](../docs/Concepts.md#taskrun) and two example [PipelineRuns](../docs/Concepts.md#pipelinerun).

[run-kritis-test.yaml](./invocations/run-kritis-test.yaml) shows an example of how to manually run kritis unit test off your development branch.

The example `PipelineRuns are`:

* [kritis-pipeline-run.yaml](./invocations/kritis-pipeline-run.yaml) invokes [the kritis example pipeline](#kritis-pipeline)
* [guestbook-pipeline-run.yaml](./invocations/guestbook-pipeline-run.yaml) invokes [the guestbook example pipeline](#guestbook-pipeline)
