# Examples

This directory contains examples of [the Pipeline CRDs](../README.md) in action.

To deploy them to your cluster (after
[installing the CRDs and running the controller](../DEVELOPMENT.md#getting-started)):

```bash
# To setup all the Tasks/Pipelines etc.
kubectl apply -f examples/

# To invoke the build-push Task only
kubectl apply -f examples/run/taskrun.yaml

# To invoke the simple Pipeline
kubectl apply -f examples/run/pipelinerun.yaml

# To invoke the Pipeline that links outputs
kubectl apply -f examples/run/output-pipelinerun.yaml

# To invoke the TaskRun with embedded Resource spec and task Spec
kubectl apply -f examples/run/resource-spec-taskrun.yaml
```

## Example Pipelines

### Simple Pipelines

[The simple Pipeline](pipeline.yaml) Builds
[two microservice images](https://github.com/GoogleContainerTools/skaffold/tree/master/examples/microservices)
from [the Skaffold repo](https://github.com/GoogleContainerTools/skaffold) and
deploys them to the repo currently running the Pipeline CRD.

It does this using the k8s `Deployment` in the skaffold repos's existing yaml
files, so at the moment there is no guarantee that the image that are built and
pushed are the ones that are deployed (that would require using the digest of
the built image, see https://github.com/knative/build-pipeline/issues/216).

To run this yourself, you will need to change the values of
`gcr.io/christiewilson-catfactory` to a registry you can push to from inside
your cluster.

Since this demo modifies the cluster (deploys to it) you must use a service
account with permission to admin the cluster (or make your default user an admin
of the `default` namespace with
[default-cluster-admin.yaml](default-cluster-admin.yaml)).

#### Simple Tasks

The [Tasks](../docs/tasks.md) used by the simple examples are:

- [build-task.yaml](build-task.yaml): Builds an image via
  [kaniko](https://github.com/GoogleContainerTools/kaniko) and pushes it to
  registry.
- [deploy-task.yaml](deploy-task.yaml): This task deploys with
  `kubectl apply -f <filename>`

#### Simple Runs

The [run](./run/) directory contains an example [TaskRun](../docs/taskruns.md)
and an example [PipelineRun](../docs/pipelineruns.md):

- [task-run.yaml](./run/task-run.yaml) shows an example of how to manually run
  the `build-push` task
- [pipeline-run.yaml](./run/pipeline-run.yaml) invokes
  [the example pipeline](#example-pipeline)
- [embed-resource-spec.yaml](./run/task-run-resource-spec.yaml) shows an example
  how to create TaskRun with embedded Task spec and resource spec.

### Pipeline with outputs

[The Pipeline with outputs](output-pipeline.yaml) contains a Pipeline that
demonstrates how the outputs of a `Task` can be given as inputs to the next
`Task`. It does this by:

1. Running a `Task` that writes to a `PipelineResource`
2. Running a `Task` that reads the written value from the `PipelineResource`

The [`Output`](../docs/tasks.md#outputs) of the first `Task` is given as an
[`Input`](../docs/tasks.md#inputs) to the next `Task` thanks to the
[`from`](../docs/pipelines.md#from) clause.

#### Output Tasks

The two [Tasks](../docs/tasks.md) used by the output Pipeline are in
[output-tasks.yaml](output-tasks.yaml):

- `create-file`: Writes "some stuff" to a predefined path in the `workspace`
  `git` `PipelineResource`
- `check-stuff-file-exists`: Reads a file from a predefined path in the
  `workspace` `git` `PipelineResource`

These work together when combined in a `Pipeline` because the git resource used
as an [`Output`](../docs/tasks.md#outputs) of the `create-file` `Task` can be an
[`Input`](../docs/tasks.md#inputs) of the `check-stuff-file-exists` `Task`.

#### Output Runs

The [run](./run/) directory contains an example
[PipelineRun](../docs/pipelineruns.md) that invokes this `Pipeline` in
[`run/output-pipeline-run.yaml`](./run/output-pipeline-run.yaml).

### Accessing private docker image

The [run](./run/) directory contains an example
[TaskRun](../docs/Concepts.md#taskrun) with an embedded taskSpec, that pull a
private image from `gcr.io`, see
[`run/private-taskrun.yaml`](./run/private-taskrun.yaml).

This _run_ requires the secrets from [`0-secrets.yaml`](`0-secrets.yaml`) and
service accounts from [`1-bots.yaml`](`1-bots.yaml`) to be able to pull the
private image.

It uses `kubernetes.io/dockercfg` secret type but,
`kubernetes.io/dockerconfigjson` and
[Knative flavored credentials](https://github.com/knative/docs/blob/master/build/auth.md#guiding-credential-selection)
are supported too.
