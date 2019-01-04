# How to use the Pipeline CRD

- [How do I create a new Pipeline?](#creating-a-pipeline)
- [How do I make a Task?](#creating-a-task)
- [How do I make Resources?](#creating-resources)
- [How do I run a Pipeline?](#running-a-pipeline)
- [How do I run a Task on its own?](#running-a-task)
- [How do I troubleshoot a PipelineRun?](#troubleshooting)
- [How do I follow logs](../test/logs/README.md)

## Creating a Pipeline

1. Create or copy [Task definitions](#creating-a-task) for the tasks you’d like
   to run. Some can be generic and reused (e.g. building with Kaniko) and others
   will be specific to your project (e.g. running your particular set of unit
   tests).
2. Create a `Pipeline` which expresses the Tasks you would like to run and what
   [Resources](#creating-resources) the Tasks need. Use
   [`providedBy`](#providedBy) to express the order the `Tasks` should run in.

See [the example Pipeline](../examples/pipeline.yaml).

### ProvidedBy

When you need to execute `Tasks` in a particular order, it will likely be
because they are operating over the same `Resources` (e.g. your unit test task
must run first against your git repo, then you build an image from that repo,
then you run integration tests against that image).

We express this ordering by adding `providedBy` on `Resources` that our `Tasks`
need.

- The (optional) `providedBy` key on an `input source` defines a set of previous
  task names.
- When the `providedBy` key is specified on an input source, only the version of
  the resource that is provided by the defined list of tasks is used.
- The `providedBy` allows for `Tasks` to fan in and fan out, and ordering can be
  expressed explicitly using this key since a task needing a resource from a
  another task would have to run after.
- The name used in the `providedBy` is the name of `PipelineTask`
- The name of the `PipelineResource` must correspond to a `PipelineResource`
  from the `Task` that the referenced `PipelineTask` provides as an output

For example see this `Pipeline` spec:

```yaml
- name: build-skaffold-app
  taskRef:
    name: build-push
  params:
    - name: pathToDockerFile
      value: Dockerfile
    - name: pathToContext
      value: /workspace/examples/microservices/leeroy-app
- name: deploy-app
  taskRef:
    name: demo-deploy-kubectl
  resources:
    - name: image
      providedBy:
        - build-skaffold-app
```

The `image` resource is expected to be provided to the `deploy-app` `Task` from
the `build-skaffold-app` `Task`. This means that the `PipelineResource` bound to
the `image` input for `deploy-app` must be bound to the same `PipelineResource`
as an output from `build-skaffold-app`.

This is the corresponding `PipelineRun` spec:

```yaml
  - name: build-skaffold-app
    ...
    outputs:
    - name: builtImage
      resourceRef:
        name: skaffold-image-leeroy-app
  - name: deploy-app
    ...
    inputs:
    - name: image
      resourceRef:
        name: skaffold-image-leeroy-app
```

You can see that the `builtImage` output from `build-skaffold-app` is bound to
the `skaffold-image-leeroy-app` `PipelineResource`, and the same
`PipelineResource` is bound to `image` for `deploy-app`.

This controls two things:

1. The order the `Tasks` are executed in: `deploy-app` must come after
   `build-skaffold-app`
2. The state of the `PipelineResources`: the image provided to `deploy-app` may
   be changed by `build-skaffold-app` (WIP, see
   [#216](https://github.com/knative/build-pipeline/issues/216))

## Creating a Task

To create a Task, you must:

- Define [parameters](task-parameters.md) (i.e. string inputs) for your `Task`
- Define the inputs and outputs of the `Task` as
  [`Resources`](./Concepts.md#pipelineresources)
- Create a `Step` for each action you want to take in the `Task`

`Steps` are images which comply with the
[container contract](#container-contract).

### Container Contract

Each container image used as a step in a [`Task`](#task) must comply with a
specific contract.

When containers are run in a `Task`, the `entrypoint` of the container will be
overwritten with a custom binary that redirects the logs to a separate location
for aggregating the log output. As such, it is always recommended to explicitly
specify a command.

When `command` is not explicitly set, the controller will attempt to lookup the
entrypoint from the remote registry.

Due to this metadata lookup, if you use a private image as a step inside a
`Task`, the build-pipeline controller needs to be able to access that registry.
The simplest way to accomplish this is to add a `.docker/config.json` at
`$HOME/.docker/config.json`, which will then be used by the controller when
performing the lookup

For example, in the following Task with the images,
`gcr.io/cloud-builders/gcloud` and `gcr.io/cloud-builders/docker`, the
entrypoint would be resolved from the registry, resulting in the tasks running
`gcloud` and `docker` respectively.

```yaml
spec:
  steps:
    - image: gcr.io/cloud-builders/gcloud
      command: [gcloud]
    - image: gcr.io/cloud-builders/docker
      command: [docker]
```

However, if the steps specified a custom `command`, that is what would be used.

```yaml
spec:
  steps:
    - image: gcr.io/cloud-builders/gcloud
      command:
        - bash
        - -c
        - echo "Hello!"
```

You can also provide `args` to the image's `command`:

```yaml
steps:
  - image: ubuntu
    command: ["/bin/bash"]
    args: ["-c", "echo hello $FOO"]
    env:
      - name: "FOO"
        value: "world"
```

### Resource shared between tasks

Pipeline Tasks are allowed to pass resources from previous tasks via
`providedBy` field. This feature is implemented using Persistent Volume Claim
under the hood but however has an implication that tasks cannot have any volume
mounted under path `/pvc`.

### Outputs

`Task` definition can include inputs and outputs resource declaration. If
specific set of resources are only declared in output then a copy of resource to
be uploaded or shared for next task is expected to be present under the path
`/workspace/output/resource_name/`.

```yaml
resources:
  outputs:
    name: storage-gcs
steps:
  - image: objectuser/run-java-jar #https://hub.docker.com/r/objectuser/run-java-jar/
    command: [jar]
    args:
      ["-cvf", "-o", "/workspace/output/storage-gcs/", "projectname.war", "*"]
    env:
      - name: "FOO"
        value: "world"
```

**Note**: If task is relying on output resource functionality then they cannot
mount anything in file path `/workspace/output`.

If resource is declared in both input and output then input resource, then
destination path of input resource is used instead of
`/workspace/output/resource_name`.

In the following example Task `tar-artifact` resource is used both as input and
output so input resource is downloaded into directory `customworkspace`(as
specified in `targetPath`). Step `untar` extracts tar file into
`tar-scratch-space` directory , `edit-tar` adds a new file and last step
`tar-it-up` creates new tar file and places in `/workspace/customworkspace/`
directory. After execution of task steps, (new) tar file in directory
`/workspace/customworkspace` will be uploaded to the bucket defined in
`tar-artifact` resource definition.

```yaml
resources:
  inputs:
    name: tar-artifact
    targetPath: customworkspace
  outputs:
    name: tar-artifact
steps:
 - name: untar
    image: ubuntu
    command: ["/bin/bash"]
    args: ['-c', 'mkdir -p /workspace/tar-scratch-space/ && tar -xvf /workspace/customworkspace/rules_docker-master.tar -C /workspace/tar-scratch-space/']
 - name: edit-tar
    image: ubuntu
    command: ["/bin/bash"]
    args: ['-c', 'echo crazy > /workspace/tar-scratch-space/rules_docker-master/crazy.txt']
 - name: tar-it-up
   image: ubuntu
   command: ["/bin/bash"]
   args: ['-c', 'cd /workspace/tar-scratch-space/ && tar -cvf /workspace/customworkspace/rules_docker-master.tar rules_docker-master']
```

### Conventions

- `/workspace/<resource-name>`:
  [`PipelineResources` are made available in this mounted dir](#creating-resources)
- `/builder/home`: This volume is exposed to steps via `$HOME`.
- Credentials attached to the Build's service account may be exposed as Git or
  Docker credentials as outlined
  [in the auth docs](https://github.com/knative/docs/blob/master/build/auth.md#authentication).

### Templating

Tasks support templating using values from all `inputs` and `outputs`. Both
`Resources` and `Params` can be used inside the `Spec` of a `Task`.

`Resources` can be referenced in a `Task` spec like this, where `NAME` is the
Resource Name and `KEY` is one of `name`, `url`, `type` or `revision`:

```shell
${inputs.resources.NAME.KEY}
```

To access a `Param`, replace `resources` with `params` as below:

```shell
${inputs.params.NAME}
```

## Cluster Task

Similar to Task, but with a cluster scope.

In case of using a ClusterTask, the `TaskRef` kind should be added. The default
kind is Task which represents a namespaced Task

```yaml
apiVersion: pipeline.knative.dev/v1alpha1
kind: Pipeline
metadata:
  name: demo-pipeline
  namespace: default
spec:
  tasks:
    - name: build-skaffold-web
      taskRef:
        name: build-push
        kind: ClusterTask
      params: ....
```

## Running a Pipeline

In order to run a Pipeline, you will need to provide:

1. A Pipeline to run (see [creating a Pipeline](#creating-a-pipeline))
2. The `PipelineResources` to use with this Pipeline.

On its own, a `Pipeline` declares what `Tasks` to run, and what order to run
them in (implied by [`providedBy`](#providedby)). When running a `Pipeline`, you
will need to specify the `PipelineResources` to use with it. One `Pipeline` may
need to be run with different `PipelineResources` in cases such as:

- When triggering the run of a `Pipeline` against a pull request, the triggering
  system must specify the commitish of a git `PipelineResource` to use
- When invoking a `Pipeline` manually against one's own setup, one will need to
  ensure that one's own github fork (via the git `PipelineResource`), image
  registry (via the image `PipelineResource`) and kubernetes cluster (via the
  cluster `PipelineResource`).

Specify the `PipelineResources` in the PipelineRun using the `resources`
section, for example:

```yaml
resources:
  - name: push-kritis
    inputs:
      - key: workspace
        resourceRef:
          name: kritis-resources-git
    outputs:
      - key: builtImage
        resourceRef:
          name: kritis-resources-image
```

This example section says:

- For the `Task` in the `Pipeline` called `push-kritis`
- For the input called `workspace`, use the existing resource called
  `kritis-resources-git`
- For the output called `builtImage`, use the existing resource called
  `kritis-resources-image`

Creation of a `PipelineRun` will trigger the creation of
[`TaskRuns`](#running-a-task) for each `Task` in your pipeline.

See [the example PipelineRun](../examples/runs/pipeline-run.yaml).

### Using a ServiceAccount

In order to access to private resources, you may need to provide a
`ServiceAccount` to the build-pipeline objects.

```yaml
apiVersion: pipeline.knative.dev/v1alpha1
kind: Pipeline
metadata:
  name: demo-pipeline
  namespace: default
spec:
  serviceAccount: test-build-robot-git-ssh
  tasks:
    - name: build-skaffold-web
      taskRef:
        name: build-push
        kind: ClusterTask
      params: ....
```

Where `serviceAccount: test-build-robot-git-ssh` references to the following
`ServiceAccount`:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: test-build-robot-git-ssh
secrets:
  - name: test-git-ssh
```

## Running a Task

1. To run a `Task`, create a new `TaskRun` which defines all inputs, outputs
   that the `Task` needs to run.
2. The `TaskRun` will also serve as a record of the history of the invocations
   of the `Task`.
3. Another way of running a Task is embedding the TaskSpec in the taskRun yaml
   as shown in the following example

```yaml
apiVersion: pipeline.knative.dev/v1alpha1
kind: PipelineResource
metadata:
  name: go-example-git
spec:
  type: git
  params:
    - name: url
      value: https://github.com/pivotal-nader-ziada/gohelloworld
---
apiVersion: pipeline.knative.dev/v1alpha1
kind: TaskRun
metadata:
  name: build-push-task-run-2
spec:
  trigger:
    type: manual
  inputs:
    resources:
      - name: workspace
        resourceRef:
          name: go-example-git
  taskSpec:
    inputs:
      resources:
        - name: workspace
          type: git
    steps:
      - name: build-and-push
        image: gcr.io/kaniko-project/executor
        command:
          - /kaniko/executor
        args:
          - --destination=gcr.io/my-project/gohelloworld
```

If the TaskSpec is provided, TaskRef is not allowed.

See [the example TaskRun](../examples/runs/task-run.yaml).

### Using a ServiceAccount

In order to access to private resources, you may need to provide a
`ServiceAccount` to the build-pipeline objects.

```yaml
apiVersion: pipeline.knative.dev/v1alpha1
kind: TaskRun
metadata:
  name: build-push-task-run-2
spec:
  sericeAccount: test-build-robot-git-ssh
  trigger:
    type: manual
  inputs:
    resources:
      - name: workspace
        resourceRef:
          name: go-example-git
```

Where `serviceAccount: test-build-robot-git-ssh` references to the following
`ServiceAccount`:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: test-build-robot-git-ssh
secrets:
  - name: test-git-ssh
```

## Creating Resources

We current support these `PipelineResources`:

- [Git resource](#git-resource)
- [Image resource](#image-resource)
- [Cluster resource](#cluster-resource)

When used as inputs, these resources will be made available in a mounted
directory called `/workspace` at the path /workspace/<resource-name>`.

### Git Resource

Git resource represents a [git](https://git-scm.com/) repository, that contains
the source code to be built by the pipeline. Adding the git resource as an input
to a task will clone this repository and allow the task to perform the required
actions on the contents of the repo.

To create a git resource using the `PipelineResource` CRD:

```yaml
apiVersion: pipeline.knative.dev/v1alpha1
kind: PipelineResource
metadata:
  name: wizzbang-git
  namespace: default
spec:
  type: git
  params:
    - name: url
      value: https://github.com/wizzbangcorp/wizzbang.git
    - name: revision
      value: master
```

Params that can be added are the following:

1. `url`: represents the location of the git repository, you can use this to
   change the repo, e.g. [to use a fork](#using-a-fork)
1. `revision`: Git
   [revision](https://git-scm.com/docs/gitrevisions#_specifying_revisions)
   (branch, tag, commit SHA or ref) to clone. You can use this to control what
   commit [or branch](#using-a-branch) is used. _If no revision is specified,
   the resource will default to `latest` from `master`._

#### Using a fork

The `Url` parameter can be used to point at any git repository, for example to
use a GitHub fork at master:

```yaml
spec:
  type: git
  params:
    - name: url
      value: https://github.com/bobcatfish/wizzbang.git
```

#### Using a branch

The `revision` can be any
[git commit-ish (revision)](https://git-scm.com/docs/gitrevisions#_specifying_revisions).
You can use this to create a git `PipelineResource` that points at a branch, for
example:

```yaml
spec:
  type: git
  params:
    - name: url
      value: https://github.com/wizzbangcorp/wizzbang.git
    - name: revision
      value: some_awesome_feature
```

To point at a pull request, you can use
[the pull requests's branch](https://help.github.com/articles/checking-out-pull-requests-locally/):

```yaml
spec:
  type: git
  params:
    - name: url
      value: https://github.com/wizzbangcorp/wizzbang.git
    - name: revision
      value: refs/pull/52525/head
```

### Image Resource

An Image resource represents an image that lives in a remote repository. It is
usually used as [a `Task` `output`](concepts.md#task) for `Tasks` that build
images. This allows the same `Tasks` to be used to generically push to any
registry.

Params that can be added are the following:

1. `url`: The complete path to the image, including the registry and the image
   tag
2. `digest`: The
   [image digest](https://success.docker.com/article/images-tagging-vs-digests)
   which uniquely identifies a particular build of an image with a particular
   tag.

For example:

```yaml
apiVersion: pipeline.knative.dev/v1alpha1
kind: PipelineResource
metadata:
  name: kritis-resources-image
  namespace: default
spec:
  type: image
  params:
    - name: url
      value: gcr.io/staging-images/kritis
```

### Cluster Resource

Cluster Resource represents a Kubernetes cluster other than the current cluster
the pipleine CRD is running on. A common use case for this resource is to deploy
your application/function on different clusters.

The resource will use the provided parameters to create a
[kubeconfig](https://kubernetes.io/docs/tasks/access-application-cluster/configure-access-multiple-clusters/)
file that can be used by other steps in the pipeline task to access the target
cluster. The kubeconfig will be placed in
`/workspace/<your-cluster-name>/kubeconfig` on your task container

The Cluster resource has the following parameters:

- Name: The name of the Resource is also given to cluster, will be used in the
  kubeconfig and also as part of the path to the kubeconfig file
- URL (required): Host url of the master node
- Username (required): the user with access to the cluster
- Password: to be used for clusters with basic auth
- Token: to be used for authenication, if present will be used ahead of the
  password
- Insecure: to indicate server should be accessed without verifying the TLS
  certificate.
- CAData (required): holds PEM-encoded bytes (typically read from a root
  certificates bundle).

Note: Since only one authentication technique is allowed per user, either a
token or a password should be provided, if both are provided, the password will
be ignored.

The following example shows the syntax and structure of a Cluster Resource:

```yaml
apiVersion: pipeline.knative.dev/v1alpha1
kind: PipelineResource
metadata:
  name: test-cluster
spec:
  type: cluster
  params:
    - name: url
      value: https://10.10.10.10 # url to the cluster master node
    - name: cadata
      value: LS0tLS1CRUdJTiBDRVJ.....
    - name: token
      value: ZXlKaGJHY2lPaU....
```

For added security, you can add the sensetive information in a Kubernetes
[Secret](https://kubernetes.io/docs/concepts/configuration/secret/) and populate
the kubeconfig from them.

For example, create a secret like the following example:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: target-cluster-secrets
data:
  cadatakey: LS0tLS1CRUdJTiBDRVJUSUZ......tLQo=
  tokenkey: ZXlKaGJHY2lPaUpTVXpJMU5pSXNJbX....M2ZiCg==
```

and then apply secrets to the cluster resource

```yaml
apiVersion: pipeline.knative.dev/v1alpha1
kind: PipelineResource
metadata:
  name: test-cluster
spec:
  type: cluster
  params:
    - name: url
      value: https://10.10.10.10
    - name: username
      value: admin
  secrets:
    - fieldName: token
      secretKey: tokenKey
      secretName: target-cluster-secrets
    - fieldName: cadata
      secretKey: cadataKey
      secretName: target-cluster-secrets
```

Example usage of the cluster resource in a task:

```yaml
apiVersion: pipeline.knative.dev/v1alpha1
kind: Task
metadata:
  name: deploy-image
  namespace: default
spec:
  inputs:
    resources:
      - name: workspace
        type: git
      - name: dockerimage
        type: image
      - name: testcluster
        type: cluster
  steps:
    - name: deploy
      image: image-wtih-kubectl
      command: ["bash"]
      args:
        - "-c"
        - kubectl --kubeconfig
          /workspace/${inputs.resources.testCluster.Name}/kubeconfig --context
          ${inputs.resources.testCluster.Name} apply -f /workspace/service.yaml'
```

### Storage Resource

Storage resource represents blob storage, that contains either an object or
directory. Adding the storage resource as an input to a task will download the
blob and allow the task to perform the required actions on the contents of the
blob. Blob storage type "Google Cloud Storage"(gcs) is supported as of now.

#### GCS Storage Resource

GCS Storage resource points to "Google Cloud Storage" blob.

To create a GCS type of storage resource using the `PipelineResource` CRD:

```yaml
apiVersion: pipeline.knative.dev/v1alpha1
kind: PipelineResource
metadata:
  name: wizzbang-storage
  namespace: default
spec:
  type: storage
  params:
    - name: type
      value: gcs
    - name: location
      value: gs://some-bucket
```

Params that can be added are the following:

1. `location`: represents the location of the blob storage.
2. `type`: represents the type of blob storage. Currently there is
   implementation for only `gcs`.
3. `dir`: represents whether the blob storage is a directory or not. By default
   storage artifact is considered not a directory.
   - If artifact is a directory then `-r`(recursive) flag is used to copy all
     files under source directory to GCS bucket. Eg:
     `gsutil cp -r source_dir gs://some-bucket`
   - If artifact is a single file like zip, tar files then copy will be only 1
     level deep(no recursive). It will not trigger copy of sub directories in
     source directory. Eg: `gsutil cp source.tar gs://some-bucket.tar`.

Private buckets can also be configured as storage resources. To access GCS
private buckets, service accounts are required with correct permissions.
`secrets` field on storage resource is used for configuring this information.
Below is an example on how to create a storage resource with service account.

1. Refer to
   [official documentation](https://cloud.google.com/compute/docs/access/service-accounts)
   on how to create service accounts and configuring IAM permissions to access
   bucket.
2. Create a kubernetes secret from downloaded service account json key

   ```bash
   $ kubectl create secret generic bucket-sa --from-file=./service_account.json
   ```

3. To access GCS private bucket environment variable
   [`GOOGLE_APPLICATION_CREDENTIALS`](https://cloud.google.com/docs/authentication/production)
   should be set so apply above created secret to the GCS storage resource under
   `fieldName` key.

   ```yaml
   apiVersion: pipeline.knative.dev/v1alpha1
   kind: PipelineResource
   metadata:
     name: wizzbang-storage
     namespace: default
   spec:
     type: storage
     params:
       - name: type
         value: gcs
       - name: url
         value: gs://some-private-bucket
       - name: dir
         value: "directory"
     secrets:
       - fieldName: GOOGLE_APPLICATION_CREDENTIALS
         secretKey: bucket-sa
         secretName: service_account.json
   ```

## Troubleshooting

All objects created by the build-pipeline controller show the lineage of where
that object came from through labels, all the way down to the individual build.

There are a common set of labels that are set on objects. For `TaskRun` objects,
it will receive two labels:

- `pipeline.knative.dev/pipeline`, which will be set to the name of the owning
  pipeline
- `pipeline.knative.dev/pipelineRun`, which will be set to the name of the
  PipelineRun

When the underlying `Build` is created, it will receive each of the `pipeline`
and `pipelineRun` labels, as well as `pipeline.knative.dev/taskRun` which will
contain the `TaskRun` which caused the `Build` to be created.

In the end, this allows you to easily find the `Builds` and `TaskRuns` that are
associated with a given pipeline.

For example, to find all `Builds` created by a `Pipeline` named "build-image",
you could use the following command:

```shell
kubectl get builds --all-namespaces -l pipeline.knative.dev/pipeline=build-image
```
