<!--
---
linkTitle: "Get started with Resolvers"
weight: 103
---
-->


# Getting Started with Resolvers

## Introduction

This guide will take you from an empty Kubernetes cluster to a
functioning Tekton Pipelines installation and a PipelineRun executing
with a Pipeline stored in a git repo.

## Prerequisites

- A computer with
  [`kubectl`](https://kubernetes.io/docs/tasks/tools/#kubectl).
- A Kubernetes cluster running at least Kubernetes 1.24. A [`kind`
  cluster](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
  should work fine for following the guide on your local machine.
- An image registry that you can push images to. If you're using `kind`
  make sure your `KO_DOCKER_REPO` environment variable is set to
  `kind.local`.
- A publicly available git repository where you can put a pipeline yaml
  file.

## Step 1: Install Tekton Pipelines and the Resolvers

See [the installation instructions for Tekton Pipeline](./install.md#installing-tekton-pipelines-on-kubernetes), and
[the installation instructions for the built-in resolvers](./install.md#installing-and-configuring-remote-task-and-pipeline-resolution).

## Step 2: Ensure Pipelines is configured to enable resolvers

Starting with v0.41.0, remote resolvers for Tekton Pipelines are enabled by default, 
but can be disabled via feature flags in the `resolvers-feature-flags` configmap in 
the `tekton-pipelines-resolvers` namespace. Check that configmap to verify that the
resolvers you wish to have enabled are set to `"true"`.

The feature flags for the built-in resolvers are:

* The `bundles` resolver: `enable-bundles-resolver`
* The `git` resolver: `enable-git-resolver`
* The `hub` resolver: `enable-hub-resolver`
* The `cluster` resolver: `enable-cluster-resolver`

## Step 3: Try it out!

In order to test out your install you'll need a Pipeline stored in a
public git repository. First cd into a clone of your repo and then
create a new branch:

```sh
# checkout a new branch in the public repo you're using
git checkout -b add-a-simple-pipeline
```

Then create a basic pipeline:

```sh
cat <<"EOF" > pipeline.yaml
kind: Pipeline
apiVersion: tekton.dev/v1beta1
metadata:
  name: a-simple-pipeline
spec:
  params:
  - name: username
  tasks:
  - name: task-1
    params:
    - name: username
      value: $(params.username)
    taskSpec:
      params:
      - name: username
      steps:
      - image: alpine:3.15
        script: |
          echo "hello $(params.username)"
EOF
```

Commit the pipeline and push it to your git repo:

```sh
git add ./pipeline.yaml
git commit -m "Add a basic pipeline to test Tekton Pipeline remote resolution"

# push to your publicly accessible repository, replacing origin with
# your git remote's name
git push origin add-a-simple-pipeline
```

And finally create a `PipelineRun` that uses your pipeline:

```sh
# first assign your public repo's url to an environment variable
REPO_URL=# insert your repo's url here

# create a pipelinerun yaml file
cat <<EOF > pipelinerun.yaml
kind: PipelineRun
apiVersion: tekton.dev/v1beta1
metadata:
  name: run-basic-pipeline-from-git
spec:
  pipelineRef:
    resolver: git
    params:
    - name: url
      value: ${REPO_URL}
    - name: revision
      value: add-a-simple-pipeline
    - name: pathInRepo
      value: pipeline.yaml
  params:
  - name: username
    value: liza
EOF

# execute the pipelinerun
kubectl apply -f ./pipelinerun.yaml
```

## Step 6: Monitor the PipelineRun

First let's watch the PipelineRun to see if it succeeds:

```sh
kubectl get pipelineruns -w
```

Shortly the PipelineRun should move into a Succeeded state.

Now we can check the logs of the PipelineRun's only task:

```sh
kubectl logs run-basic-pipeline-from-git-task-1-pod
# This should print "hello liza"
```

---

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
