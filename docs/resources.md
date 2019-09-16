# PipelineResources

`PipelinesResources` in a pipeline are the set of objects that are going to be
used as inputs to a [`Task`](tasks.md) and can be output by a `Task`.

A `Task` can have multiple inputs and outputs.

For example:

-   A `Task`'s input could be a GitHub source which contains your application
    code.
-   A `Task`'s output can be your application container image which can be then
    deployed in a cluster.
-   A `Task`'s output can be a jar file to be uploaded to a storage bucket.

--------------------------------------------------------------------------------

-   [Syntax](#syntax)
-   [Resource types](#resource-types)
-   [Using Resources](#using-resources)

## Syntax

To define a configuration file for a `PipelineResource`, you can specify the
following fields:

-   Required:
    -   [`apiVersion`][kubernetes-overview] - Specifies the API version, for
        example `tekton.dev/v1alpha1`.
    -   [`kind`][kubernetes-overview] - Specify the `PipelineResource` resource
        object.
    -   [`metadata`][kubernetes-overview] - Specifies data to uniquely identify
        the `PipelineResource` object, for example a `name`.
    -   [`spec`][kubernetes-overview] - Specifies the configuration information
        for your `PipelineResource` resource object.
    -   [`type`](#resource-types) - Specifies the `type` of the
        `PipelineResource`
-   Optional:
    -   [`params`](#resource-types) - Parameters which are specific to each type
        of `PipelineResource`

[kubernetes-overview]:
  https://kubernetes.io/docs/concepts/overview/working-with-objects/kubernetes-objects/#required-fields

## Using Resources

Resources can be used in [Tasks](./tasks.md) and [Conditions](./conditions.md#resources).

Input resources, like source code (git) or artifacts, are dumped at path
`/workspace/task_resource_name` within a mounted
[volume](https://kubernetes.io/docs/concepts/storage/volumes/) and is available
to all [`steps`](#steps) of your `Task`. The path that the resources are mounted
at can be [overridden with the `targetPath` field](./resources.md#controlling-where-resources-are-mounted). 
Steps can use the `path`[variable substitution](#variable-substitution) key to refer to the local path to the mounted resource.

### Variable substitution 

`Task` and `Condition` specs can refer resource params as well as pre-defined variables such as 
`path` using the variable substitution syntax below where `<name>` is the Resource Name and `<key>` 
is a one of the resource's `params`:


#### In Task Spec:
For an input resource in a `Task` spec:
```shell
$(inputs.resources.<name>.<key>)
```

Or for an output resource:

```shell
$(outputs.resources.<name>.<key>)
```

#### In Condition Spec:
Input resources can be accessed by:

```shell
$(resources.<name>.<key>)
```

#### Accessing local path to resource 

The `path` key is pre-defined and refers to the local path to a resource on the mounted volume 
```shell
$(inputs.resouces.<name>.path)
```

### Controlling where resources are mounted

The optional field `targetPath` can be used to initialize a resource in specific
directory. If `targetPath` is set then resource will be initialized under
`/workspace/targetPath`. If `targetPath` is not specified then resource will be
initialized under `/workspace`. Following example demonstrates how git input
repository could be initialized in `$GOPATH` to run tests:

```yaml
apiVersion: tekton.dev/v1alpha1
kind: Task
metadata:
  name: task-with-input
  namespace: default
spec:
  inputs:
    resources:
      - name: workspace
        type: git
        targetPath: go/src/github.com/tektoncd/pipeline
  steps:
    - name: unit-tests
      image: golang
      command: ["go"]
      args:
        - "test"
        - "./..."
      workingDir: "/workspace/go/src/github.com/tektoncd/pipeline"
      env:
        - name: GOPATH
          value: /workspace/go
```

### Overriding where resources are copied from

When specifying input and output `PipelineResources`, you can optionally specify
`paths` for each resource. `paths` will be used by `TaskRun` as the resource's
new source paths i.e., copy the resource from specified list of paths. `TaskRun`
expects the folder and contents to be already present in specified paths.
`paths` feature could be used to provide extra files or altered version of
existing resource before execution of steps.

Output resource includes name and reference to pipeline resource and optionally
`paths`. `paths` will be used by `TaskRun` as the resource's new destination
paths i.e., copy the resource entirely to specified paths. `TaskRun` will be
responsible for creating required directories and copying contents over. `paths`
feature could be used to inspect the results of taskrun after execution of
steps.

`paths` feature for input and output resource is heavily used to pass same
version of resources across tasks in context of pipelinerun.

In the following example, task and taskrun are defined with input resource,
output resource and step which builds war artifact. After execution of
taskrun(`volume-taskrun`), `custom` volume will have entire resource
`java-git-resource` (including the war artifact) copied to the destination path
`/custom/workspace/`.

```yaml
apiVersion: tekton.dev/v1alpha1
kind: Task
metadata:
  name: volume-task
  namespace: default
spec:
  inputs:
    resources:
      - name: workspace
        type: git
  outputs:
    resources:
      - name: workspace
  steps:
    - name: build-war
      image: objectuser/run-java-jar #https://hub.docker.com/r/objectuser/run-java-jar/
      command: jar
      args: ["-cvf", "projectname.war", "*"]
      volumeMounts:
        - name: custom-volume
          mountPath: /custom
```

```yaml
apiVersion: tekton.dev/v1alpha1
kind: TaskRun
metadata:
  name: volume-taskrun
  namespace: default
spec:
  taskRef:
    name: volume-task
  inputs:
    resources:
      - name: workspace
        resourceRef:
          name: java-git-resource
  outputs:
    resources:
      - name: workspace
        paths:
          - /custom/workspace/
        resourceRef:
          name: java-git-resource
  volumes:
    - name: custom-volume
      emptyDir: {}
```

## Resource Types

The following `PipelineResources` are currently supported:

-   [Git Resource](#git-resource)
-   [Pull Request Resource](#pull-request-resource)
-   [Image Resource](#image-resource)
-   [Cluster Resource](#cluster-resource)
-   [Storage Resource](#storage-resource)
    -   [GCS Storage Resource](#gcs-storage-resource)
    -   [BuildGCS Storage Resource](#buildgcs-storage-resource)
-   [Cloud Event Resource](#cloud-event-resource)

### Git Resource

Git resource represents a [git](https://git-scm.com/) repository, that contains
the source code to be built by the pipeline. Adding the git resource as an input
to a Task will clone this repository and allow the Task to perform the required
actions on the contents of the repo.

To create a git resource using the `PipelineResource` CRD:

```yaml
apiVersion: tekton.dev/v1alpha1
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

1.  `url`: represents the location of the git repository, you can use this to
    change the repo, e.g. [to use a fork](#using-a-fork)
1.  `revision`: Git
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

### Pull Request Resource

Pull Request resource represents a pull request event from a source control
system.

Adding the Pull Request resource as an input to a Task will populate the
workspace with a set of files containing generic pull request related metadata
such as base/head commit, comments, and labels.

The payloads will also contain
links to raw service-specific payloads where appropriate.

Adding the Pull Request resource as an output of a Task will update the source
control system with any changes made to the pull request resource during the
pipeline.

Example file structure:

```shell
/workspace/
/workspace/<resource>/
/workspace/<resource>/labels/
/workspace/<resource>/labels/<label>
/workspace/<resource>/status/
/workspace/<resource>/status/<status>
/workspace/<resource>/comments/
/workspace/<resource>/comments/<comment>
/workspace/<resource>/head
/workspace/<resource>/base
```
More details:

Labels are empty files, named after the desired label string.

Status is represented as a set of json files, structured like this:

```json
{
  "ID": "foobar",
  "Description": "description",
  "URL": "https://www.example.com",
  "Code": "success" 
}
```

References (head and base) are a set of json files, structured like this:

```json
{
  "Repo": "https://gitlab.com/foo/bar",
  "Branch": "master",
  "SHA": "b813c8fcb1e6245dcbe5ab3c14259fac2e75a799"
}
```

Comments are a set of json files, structured like this:
```json
{
  "Text": "comment body",
  "Author": "author",
  "ID": 202131633
}
```

See [types.go](/cmd/pullrequest-init/types.go) for the full payload spec.

To create a pull request resource using the `PipelineResource` CRD:

```yaml
apiVersion: tekton.dev/v1alpha1
kind: PipelineResource
metadata:
  name: wizzbang-pr
  namespace: default
spec:
  type: pullRequest
  params:
    - name: url
      value: https://github.com/wizzbangcorp/wizzbang/pulls/1
  secrets:
    - fieldName: githubToken
      secretName: github-secrets
      secretKey: token
---
apiVersion: v1
kind: Secret
metadata:
  name: github-secrets
type: Opaque
data:
  token: github_personal_access_token_secret # in base64 encoded form

```

Params that can be added are the following:

1.  `url`: represents the location of the pull request to fetch.

#### Statuses

The following status codes are available to use for the Pull Request resource:

Status          |
--------------- |
success         |
neutral         |
queued          |
in_progress     |
failure         |
unknown         |
error           |
timeout         |
canceled        |
action_required |

Note: Status codes are currently **case sensitive**.

For more information on how Tekton Pull Request status codes convert to SCM
provider statuses, see
[pullrequest-init/README.md](/cmd/pullrequest-init/README.md).

#### GitHub

The Pull Request resource will look for GitHub OAuth authentication tokens in
spec secrets with a field name called `githubToken`.

URLs should be of the form: https://github.com/tektoncd/pipeline/pull/1

### Image Resource

An Image resource represents an image that lives in a remote repository. It is
usually used as [a `Task` `output`](tasks.md#outputs) for `Tasks` that build
images. This allows the same `Tasks` to be used to generically push to any
registry.

Params that can be added are the following:

1.  `url`: The complete path to the image, including the registry and the image
    tag
1.  `digest`: The
    [image digest](https://success.docker.com/article/images-tagging-vs-digests)
    which uniquely identifies a particular build of an image with a particular
    tag. _While this can be provided as a parameter, there is not yet a way to
    update this value after an image is built, but this is planned in
    [#216](https://github.com/tektoncd/pipeline/issues/216)._

For example:

```yaml
apiVersion: tekton.dev/v1alpha1
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

#### Surfacing the image digest built in a task

To surface the image digest in the output of the `taskRun` the builder tool
should produce this information in a
[OCI Image Spec](https://github.com/opencontainers/image-spec/blob/master/image-layout.md)
`index.json` file. This file should be placed on a location as specified in the
task definition under the resource `outputImageDir`. If there is only one image
in the `index.json` file, the digest of that image is exported; otherwise, the
digest of the whole image index would be exported.

For example this build-push task defines the `outputImageDir` for the
`builtImage` resource in `/workspace/buildImage`

```yaml
apiVersion: tekton.dev/v1alpha1
kind: Task
metadata:
  name: build-push
spec:
  inputs:
    resources:
      - name: workspace
        type: git
  outputs:
    resources:
      - name: builtImage
        type: image
        outputImageDir: /workspace/builtImage
  steps: ...
```

If no value is specified for `outputImageDir`, it will default to
`/builder/home/image-outputs/{resource-name}`.

_Please check the builder tool used on how to pass this path to create the
output file._

The `taskRun` will include the image digest in the `resourcesResult` field that
is part of the `taskRun.Status`

for example:

```yaml
status:
    ...
    resourcesResult:
    - digest: sha256:eed29cd0b6feeb1a92bc3c4f977fd203c63b376a638731c88cacefe3adb1c660
      name: skaffold-image-leeroy-web
    ...
```

If the `index.json` file is not produced, the image digest will not be included
in the `taskRun` output.

### Cluster Resource

Cluster Resource represents a Kubernetes cluster other than the current cluster
Tekton Pipelines is running on. A common use case for this resource is to deploy
your application/function on different clusters.

The resource will use the provided parameters to create a
[kubeconfig](https://kubernetes.io/docs/tasks/access-application-cluster/configure-access-multiple-clusters/)
file that can be used by other steps in the pipeline Task to access the target
cluster. The kubeconfig will be placed in
`/workspace/<your-cluster-name>/kubeconfig` on your Task container

The Cluster resource has the following parameters:

-   `name` (required): The name to be given to the target cluster, will be used
    in the kubeconfig and also as part of the path to the kubeconfig file
-   `url` (required): Host url of the master node
-   `username` (required): the user with access to the cluster
-   `password`: to be used for clusters with basic auth
-   `namespace`: The namespace to target in the cluster
-   `token`: to be used for authentication, if present will be used ahead of the
    password
-   `insecure`: to indicate server should be accessed without verifying the TLS
    certificate.
-   `cadata` (required): holds PEM-encoded bytes (typically read from a root
    certificates bundle).

Note: Since only one authentication technique is allowed per user, either a
`token` or a `password` should be provided, if both are provided, the `password`
will be ignored.

The following example shows the syntax and structure of a Cluster Resource:

```yaml
apiVersion: tekton.dev/v1alpha1
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

For added security, you can add the sensitive information in a Kubernetes
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
apiVersion: tekton.dev/v1alpha1
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

Example usage of the cluster resource in a Task, using [variable substitution](tasks.md#variable-substitution):

```yaml
apiVersion: tekton.dev/v1alpha1
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
          /workspace/$(inputs.resources.testCluster.name)/kubeconfig --context
          $(inputs.resources.testCluster.name) apply -f /workspace/service.yaml'
```

### Storage Resource

Storage resource represents blob storage, that contains either an object or
directory. Adding the storage resource as an input to a Task will download the
blob and allow the Task to perform the required actions on the contents of the
blob.

Only blob storage type
[Google Cloud Storage](https://cloud.google.com/storage/)(gcs) is supported as
of now via [GCS storage resource](#gcs-storage-resource) and
[BuildGCS storage resource](#buildgcs-storage-resource).

#### GCS Storage Resource

GCS Storage resource points to
[Google Cloud Storage](https://cloud.google.com/storage/) blob.

To create a GCS type of storage resource using the `PipelineResource` CRD:

```yaml
apiVersion: tekton.dev/v1alpha1
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
    - name: dir
      value: "y" # This can have any value to be considered "true"
```

Params that can be added are the following:

1.  `location`: represents the location of the blob storage.
1.  `type`: represents the type of blob storage. For GCS storage resource this
    value should be set to `gcs`.
1.  `dir`: represents whether the blob storage is a directory or not. By default
    storage artifact is considered not a directory.

    -   If artifact is a directory then `-r`(recursive) flag is used to copy all
        files under source directory to GCS bucket. Eg: `gsutil cp -r
        source_dir/* gs://some-bucket`
    -   If artifact is a single file like zip, tar files then copy will be only
        1 level deep(no recursive). It will not trigger copy of sub directories
        in source directory. Eg: `gsutil cp source.tar gs://some-bucket.tar`.

Private buckets can also be configured as storage resources. To access GCS
private buckets, service accounts are required with correct permissions. The
`secrets` field on the storage resource is used for configuring this
information. Below is an example on how to create a storage resource with
service account.

1.  Refer to
    [official documentation](https://cloud.google.com/compute/docs/access/service-accounts)
    on how to create service accounts and configuring IAM permissions to access
    bucket.
1.  Create a Kubernetes secret from downloaded service account json key

    ```bash
    kubectl create secret generic bucket-sa --from-file=./service_account.json
    ```

1.  To access GCS private bucket environment variable
    [`GOOGLE_APPLICATION_CREDENTIALS`](https://cloud.google.com/docs/authentication/production)
    should be set so apply above created secret to the GCS storage resource
    under `fieldName` key.

    ```yaml
    apiVersion: tekton.dev/v1alpha1
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
          value: gs://some-private-bucket
        - name: dir
          value: "y"
      secrets:
        - fieldName: GOOGLE_APPLICATION_CREDENTIALS
          secretName: bucket-sa
          secretKey: service_account.json
    ```

--------------------------------------------------------------------------------

#### BuildGCS Storage Resource

BuildGCS storage resource points to
[Google Cloud Storage](https://cloud.google.com/storage/) blob like
[GCS Storage Resource](#gcs-storage-resource) either in the form of a .zip
archive, or based on the contents of a source manifest file.

In addition to fetching an .zip archive, BuildGCS also unzips it.

A
[Source Manifest File](https://github.com/GoogleCloudPlatform/cloud-builders/tree/master/gcs-fetcher#source-manifests)
is a JSON object listing other objects in Cloud Storage that should be fetched.
The format of the manifest is a mapping of destination file path to the location
in Cloud Storage where the file's contents can be found. BuildGCS resource can
also do incremental uploads of sources via Source Manifest File.

To create a BuildGCS type of storage resource using the `PipelineResource` CRD:

```yaml
apiVersion: tekton.dev/v1alpha1
kind: PipelineResource
metadata:
  name: build-gcs-storage
  namespace: default
spec:
  type: storage
  params:
    - name: type
      value: build-gcs
    - name: location
      value: gs://build-crd-tests/rules_docker-master.zip
    - name: artifactType
      value: Archive
```

Params that can be added are the following:

1.  `location`: represents the location of the blob storage.
1.  `type`: represents the type of blob storage. For BuildGCS, this value should
    be set to `build-gcs`
1.  `artifactType`: represent the type of GCS resource. Right now, we support
    following types:

* `ZipArchive`:
  * ZipArchive indicates that resource fetched is an archive file in the zip format.
  * It unzips the archive and places all the files in the directory which is set at runtime.
  * `Archive` is also supported and is equivalent to `ZipArchive`, but is deprecated.
* `TarGzArchive`:
  * TarGzArchive indicates that resource fetched is a gzipped archive file in the tar format.
  * It unzips the archive and places all the files in the directory which is set at runtime.
* [`Manifest`](https://github.com/GoogleCloudPlatform/cloud-builders/tree/master/gcs-fetcher#source-manifests):
  * Manifest indicates that resource should be fetched using a source manifest file.

Private buckets other than ones accessible by
[TaskRun Service Account](./taskruns.md#service-account) can not be configured
as storage resources for BuildGCS Storage Resource right now. This is because
the container image
[gcr.io/cloud-builders//gcs-fetcher](https://github.com/GoogleCloudPlatform/cloud-builders/tree/master/gcs-fetcher)
does not support configuring secrets.

### Cloud Event Resource

The Cloud Event Resource represents a [cloud event](https://github.com/cloudevents/spec)
that is sent to a target `URI` upon completion of a `TaskRun`.
The Cloud Event Resource sends Tekton specific events; the body of the event includes
the entire TaskRun spec plus status; the types of events defined for now are:

- dev.tekton.event.task.unknown
- dev.tekton.event.task.successful
- dev.tekton.event.task.failed

Cloud event resources are useful to notify a third party upon the completion and
status of a `TaskRun`. In combinations with the [Tekton triggers](https://github.com/tektoncd/triggers)
project they can be used to link `Task/PipelineRuns` asynchronously.

To create a CloudEvent resource using the `PipelineResource` CRD:

```yaml
apiVersion: tekton.dev/v1alpha1
kind: PipelineResource
metadata:
  name: event-to-sink
spec:
  type: cloudevent
  params:
  - name: targetURI
    value: http://sink:8080
```

The content of an event is for example:

```yaml
Context Attributes,
  SpecVersion: 0.2
  Type: dev.tekton.event.task.successful
  Source: /apis/tekton.dev/v1alpha1/namespaces/default/taskruns/pipeline-run-api-16aa55-source-to-image-task-rpndl
  ID: pipeline-run-api-16aa55-source-to-image-task-rpndl
  Time: 2019-07-04T11:03:53.058694712Z
  ContentType: application/json
Transport Context,
  URI: /
  Host: my-sink.default.my-cluster.containers.appdomain.cloud
  Method: POST
Data,
  {
    "taskRun": {
      "metadata": {...}
      "spec": {
        "inputs": {...}
        "outputs": {...}
        "serviceAccount": "default",
        "taskRef": {
          "name": "source-to-image",
          "kind": "Task"
        },
        "timeout": "1h0m0s"
      },
      "status": {...}
    }
  }
```

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
