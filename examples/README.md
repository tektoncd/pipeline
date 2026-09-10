# Examples

This directory contains examples of [Tekton Pipelines](../README.md) in action.

To deploy them to your cluster (after
[installing the CRDs and running the controller](../DEVELOPMENT.md#getting-started)):

Some examples have `metadata:generateName` instead of `metadata:name`.  These examples
can not be executed by `kubectl apply`.  Please use `kubectl create` instead.

In few examples to demonstrate tasks that push image to registry, sample URL
`gcr.io/christiewilson-catfactory` is used. To run these examples yourself, you
will need to change the values of this sample registry URL to a registry you can
push to from inside your cluster.

## Using the run-example.sh Helper Script

We provide a helper script to automatically substitute the registry URL before
applying examples. This is useful for:
- Using local registries to avoid rate limiting
- Testing in air-gapped environments
- Speeding up development with cached images

Set the `KO_DOCKER_REPO` environment variable to your registry and use the script:

```bash
# Set your registry
export KO_DOCKER_REPO=kind.local
# or: export KO_DOCKER_REPO=localhost:5000
# or: export KO_DOCKER_REPO=your-registry.example.com

# Run examples using the helper script
./hack/run-example.sh examples/v1/pipelineruns/pipelinerun.yaml
./hack/run-example.sh examples/v1beta1/taskruns/task-output-image.yaml
```

## Manual Application

Alternatively, you can manually apply examples with `kubectl apply`:

```bash
# To invoke a TaskRun
kubectl apply -f examples/v1/taskruns/

# To invoke a PipelineRun
kubectl apply -f examples/v1/pipelineruns/
```

**Note:** When using manual application with examples that contain registry URLs,
you'll need to edit the files or use `sed` to substitute the registry before applying.

## Results

You can track the progress of your taskruns and pipelineruns with this command,
which will also format the output nicely.

```shell
$ kubectl get taskruns -o=custom-columns-file=./test/columns.txt
NAME              TYPE        STATUS    START
test-git-branch   Succeeded   True      2019-02-11T21:21:03Z
test-git-ref      Succeeded   True      2019-02-11T21:21:02Z
test-git-tag      Succeeded   True      2019-02-11T21:21:02Z
```

```shell
$ kubectl get pipelineruns -o=custom-columns-file=./test/columns.txt
NAME                  TYPE        STATUS    START
demo-pipeline-run-1   Succeeded   True      2019-02-11T21:21:03Z
output-pipeline-run   Succeeded   True      2019-02-11T21:35:43Z
```

You can also use `kubectl get tr` or `kubectl get pr` to query all `taskruns` or
`pipelineruns` respectively.
