# Getting Started

## Introduction

This guide will take you from an empty Kubernetes cluster to a
functioning Tekton Pipelines installation and a PipelineRun executing
with a Pipeline stored in a git repo.

## Prerequisites

- A computer with
  [`kubectl`](https://kubernetes.io/docs/tasks/tools/#kubectl) and
  [`ko`](https://github.com/google/ko) installed.
- A Kubernetes cluster running at least Kubernetes 1.21. A [`kind`
  cluster](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
  should work fine for following the guide on your local machine.
- An image registry that you can push images to. If you're using `kind`
  make sure your `KO_DOCKER_REPO` environment variable is set to
  `kind.local`.
- A publicly available git repository where you can put a pipeline yaml
  file.

## Step 1: Install Tekton Pipelines

At this time a development version of Tekton Pipelines
is required to support integration with Tekton Resolution.

You can install the needed version of Tekton Pipelines with the
following commands:

```sh
# clone the pipeline repository
git clone https://github.com/tektoncd/pipeline

# cd into the fetched repo
cd pipeline
```

And then install pipelines from its main branch:

```sh
ko apply -f ./config/100-namespace
ko apply -f ./config
```

Next, install the remote resolvers:

```sh
# install the resolver resources into your kubernetes cluster
ko apply -f ./config/resolvers
```

This will install the `tekton-pipelines-remote-resolvers` service. Additional
configuration will be needed to enable the available resolvers.

```sh
cd ..
```

## Step 2: Configure Pipelines to enable alpha features and resolvers

Tekton Pipelines currently has its integration with remote resolution behind
the alpha feature gate, and enabling specific resolvers is controlled by feature 
flags as well:

```sh
# update the feature-flags configmap in the tekton-pipelines namespace
kubectl patch -n tekton-pipelines configmap feature-flags -p '{"data":{"enable-api-fields":"alpha","enable-git-resolver":"true"}}'
```

The feature flags for the built-in resolvers are:

* The `bundles` resolver: `enable-tekton-oci-bundles`
* The `git` resolver: `enable-git-resolver`
* The `hub` resolver: `enable-hub-resolver`

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
    - name: branch
      value: add-a-simple-pipeline
    - name: path
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
