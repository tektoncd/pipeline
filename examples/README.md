# Examples

This directory contains examples of [Tekton Pipelines](../README.md) in action.

To deploy them to your cluster (after
[installing the CRDs and running the controller](../DEVELOPMENT.md#getting-started)):

Some examples have `metadata:generateName` instead of `metadata:name`.  These examples
can not be executed by `kubectl apply`.  Please use `kubectl create` instead.

In few examples to demonstrate tasks that push image to registry, sample URL
`gcr.io/christiewilson-catfactory` is used. To run these examples yourself, you
will need to change the values of this sample registry URL to a registry you can
push to from inside your cluster. If you are following instructions
[here](../DEVELOPMENT.md#getting-started) to setup then use the value of
`$KO_DOCKER_REPO` instead of `gcr.io/christiewilson-catfactory`.

```bash
# To invoke the build-push Task only
kubectl apply -f examples/v1beta1/taskruns/task-output-image.yaml

# To invoke the simple Pipeline
kubectl apply -f examples/v1beta1/pipelineruns/pipelinerun.yaml

# To invoke the Pipeline that links outputs
kubectl apply -f examples/v1beta1/pipelineruns/output-pipelinerun.yaml

# To invoke the TaskRun with embedded Resource spec and task Spec
kubectl apply -f examples/v1beta1/taskruns/git-resource.yaml
```

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
