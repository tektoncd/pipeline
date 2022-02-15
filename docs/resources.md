<!--
---
linkTitle: "PipelineResources"
weight: 2000
---
-->
# PipelineResources

> :warning: **`PipelineResources` are [deprecated](deprecations.md#deprecation-table).**
>
> Consider using replacement features instead. Read more in [documentation](migrating-v1alpha1-to-v1beta1.md#replacing-pipelineresources-with-tasks)
> and [TEP-0074](https://github.com/tektoncd/community/blob/main/teps/0074-deprecate-pipelineresources.md).

`PipelineResources` in a pipeline are the set of objects that are going to be
used as inputs to a [`Task`](tasks.md) and can be output by a `Task`.

A `Task` can have multiple inputs and outputs.

For example:

-   A `Task`'s input could be a GitHub source which contains your application
    code.
-   A `Task`'s output can be your application container image which can be then
    deployed in a cluster.
-   A `Task`'s output can be a jar file to be uploaded to a storage bucket.

> **Note**: `PipelineResources` have not been promoted to Beta in tandem with Pipeline's other CRDs. 
> This means that the level of support for `PipelineResources` remains Alpha.
> `PipelineResources` are now [deprecated](deprecations.md#deprecation-table).** 
>
> For Beta-supported alternatives to PipelineResources see
> [the v1alpha1 to v1beta1 migration guide](./migrating-v1alpha1-to-v1beta1.md#pipelineresources-and-catalog-tasks)
> which lists each PipelineResource type and a suggested option for replacing it.
>
> For more information on why PipelineResources are remaining alpha [see the description
> of their problems, along with next steps, below](#why-aren-t-pipelineresources-in-beta).

--------------------------------------------------------------------------------

-   [Syntax](#syntax)
-   [Using Resources](#using-resources)
    -   [Variable substitution](#variable-substitution)
    -   [Controlling where resources are mounted](#controlling-where-resources-are-mounted)
    -   [Overriding where resources are copied from](#overriding-where-resources-are-copied-from)
    -   [Resource Status](#resource-status)
    -   [Optional Resources](#optional-resources)
-   [Resource types](#resource-types)
    -   [Git Resource](#git-resource)
    -   [Pull Request Resource](#pull-request-resource)
    -   [Image Resource](#image-resource)
    -   [Cluster Resource](#cluster-resource)
    -   [Storage Resource](#storage-resource)
        -   [GCS Storage Resource](#gcs-storage-resource)
    -   [Cloud Event Resource](#cloud-event-resource)
-   [Why Aren't PipelineResources in Beta?](#why-aren-t-pipelineresources-in-beta)

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
    -   [`description`](#description) - Description of the Resource.
    -   [`params`](#resource-types) - Parameters which are specific to each type
        of `PipelineResource`
    -   [`optional`](#optional-resources) - Boolean flag to mark a resource
        optional (by default, `optional` is set to `false` making resources
        mandatory).

[kubernetes-overview]:
  https://kubernetes.io/docs/concepts/overview/working-with-objects/kubernetes-objects/#required-fields

## Using Resources

> :warning: **`PipelineResources` are [deprecated](deprecations.md#deprecation-table).**
>
> Consider using replacement features instead. Read more in [documentation](migrating-v1alpha1-to-v1beta1.md#replacing-pipelineresources-with-tasks)
> and [TEP-0074](https://github.com/tektoncd/community/blob/main/teps/0074-deprecate-pipelineresources.md).

Resources can be used in [Tasks](./tasks.md) and
[Conditions](./conditions.md#resources).

Input resources, like source code (git) or artifacts, are dumped at path
`/workspace/task_resource_name` within a mounted
[volume](https://kubernetes.io/docs/concepts/storage/volumes/) and are available
to all [`steps`](#steps) of your `Task`. The path that the resources are mounted
at can be
[overridden with the `targetPath` field](./resources.md#controlling-where-resources-are-mounted).
Steps can use the `path`[variable substitution](#variable-substitution) key to
refer to the local path to the mounted resource.

### Variable substitution

`Task` and `Condition` specs can refer resource params as well as predefined
variables such as `path` using the variable substitution syntax below where
`<name>` is the resource's `name` and `<key>` is one of the resource's `params`:

#### In Task Spec:

For an input resource in a `Task` spec: `shell $(resources.inputs.<name>.<key>)`

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

The `path` key is predefined and refers to the local path to a resource on the
mounted volume `shell $(resources.inputs.<name>.path)`

### Controlling where resources are mounted

The optional field `targetPath` can be used to initialize a resource in a
specific directory. If `targetPath` is set, the resource will be initialized
under `/workspace/targetPath`. If `targetPath` is not specified, the resource
will be initialized under `/workspace`. The following example demonstrates how
git input repository could be initialized in `$GOPATH` to run tests:

```yaml
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: task-with-input
  namespace: default
spec:
  resources:
    inputs:
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
new source paths i.e., copy the resource from a specified list of paths.
`TaskRun` expects the folder and contents to be already present in specified
paths. The `paths` feature could be used to provide extra files or altered
version of existing resources before the execution of steps.

The output resource includes the name and reference to the pipeline resource and
optionally `paths`. `paths` will be used by `TaskRun` as the resource's new
destination paths i.e., copy the resource entirely to specified paths. `TaskRun`
will be responsible for the creation of required directories and content
transition. The `paths` feature could be used to inspect the results of
`TaskRun` after the execution of steps.

`paths` feature for input and output resources is heavily used to pass the same
version of resources across tasks in context of `PipelineRun`.

In the following example, `Task` and `TaskRun` are defined with an input
resource, output resource and step, which builds a war artifact. After the
execution of `TaskRun`(`volume-taskrun`), `custom` volume will have the entire
resource `java-git-resource` (including the war artifact) copied to the
destination path `/custom/workspace/`.

```yaml
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: volume-task
  namespace: default
spec:
  resources:
    inputs:
      - name: workspace
        type: git
    outputs:
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
apiVersion: tekton.dev/v1beta1
kind: TaskRun
metadata:
  name: volume-taskrun
  namespace: default
spec:
  taskRef:
    name: volume-task
  resources:
    inputs:
      - name: workspace
        resourceRef:
          name: java-git-resource
    outputs:
      - name: workspace
        paths:
          - /custom/workspace/
        resourceRef:
          name: java-git-resource
  podTemplate:
    volumes:
      - name: custom-volume
        emptyDir: {}
```

### Resource Status

When resources are bound inside a `TaskRun`, they can include extra information
in the `TaskRun` Status.ResourcesResult field. This information can be useful
for auditing the exact resources used by a `TaskRun` later. Currently the Image
and Git resources use this mechanism.

For an example of what this output looks like:

```yaml
resourcesResult:
  - key: digest
    value: sha256:a08412a4164b85ae521b0c00cf328e3aab30ba94a526821367534b81e51cb1cb
    resourceRef:
      name: skaffold-image-leeroy-web
```

### Description

The `description` field is an optional field and can be used to provide description of the Resource.

### Optional Resources

By default, a resource is declared as mandatory unless `optional` is set to `true`
for that resource. Resources declared as `optional` in a `Task` does not have be
specified in `TaskRun`.

```yaml
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: task-check-optional-resources
spec:
  resources:
    inputs:
      - name: git-repo
        type: git
        optional: true
```

Similarly, resources declared as `optional` in a `Pipeline` does not have to be
specified in `PipelineRun`.

```yaml
apiVersion: tekton.dev/v1beta1
kind: Pipeline
metadata:
  name: pipeline-build-image
spec:
  resources:
    - name: workspace
      type: git
      optional: true
  tasks:
    - name: check-workspace
# ...
```

You can refer to different examples demonstrating usage of optional resources in
`Task`, `Condition`, and `Pipeline`:

-   [Task](../examples/v1beta1/taskruns/optional-resources.yaml)
-   [Cluster Task](../examples/v1beta1/taskruns/optional-resources-with-clustertask.yaml)
-   [Condition](../examples/v1beta1/pipelineruns/conditional-pipelinerun-with-optional-resources.yaml)
-   [Pipeline](../examples/v1beta1/pipelineruns/demo-optional-resources.yaml)

## Resource Types

### Git Resource

The `git` resource represents a [git](https://git-scm.com/) repository, that
contains the source code to be built by the pipeline. Adding the `git` resource
as an input to a `Task` will clone this repository and allow the `Task` to
perform the required actions on the contents of the repo.

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
1.  `revision`: Git [revision][git-rev] (branch, tag, commit SHA or ref) to
    clone. You can use this to control what commit [or branch](#using-a-branch)
    is used. [git checkout][git-checkout] is used to switch to the
    revision, and will result in a detached HEAD in most cases. Use refspec
    along with revision if you want to checkout a particular branch without a
    detached HEAD. _If no revision is specified, the resource inspects remote repository to determine the correct default branch._
1.  `refspec`: (Optional) specify a git [refspec][git-refspec] to pass to git-fetch.
     Note that if this field is specified, it must specify all refs, branches, tags,
     or commits required to checkout the specified `revision`. An additional fetch
     will not be run to obtain the contents of the revision field. If no refspec
     is specified, the value of the `revision` field will be fetched directly.
     The refspec is useful in manipulating the repository in several cases:
     * when the server does not support fetches via the commit SHA (i.e. does
       not have `uploadpack.allowReachableSHA1InWant` enabled) and you want
       to fetch and checkout a specific commit hash from a ref chain.
     * when you want to fetch several other refs alongside your revision
       (for instance, tags)
     * when you want to checkout a specific branch, the revision and refspec
       fields can work together to be able to set the destination of the incoming
       branch and switch to the branch.

        Examples:
         - Check out a specified revision commit SHA1 after fetching ref (detached) <br>
           &nbsp;&nbsp;`revision`: cb17eba165fe7973ef9afec20e7c6971565bd72f <br>
           &nbsp;&nbsp;`refspec`: refs/smoke/myref <br>
         - Fetch all tags alongside refs/heads/master and switch to the master branch
           (not detached) <br>
           &nbsp;&nbsp;`revision`: master <br>
           &nbsp;&nbsp;`refspec`: "refs/tags/\*:refs/tags/\* +refs/heads/master:refs/heads/master"<br>
         - Fetch the develop branch and switch to it (not detached) <br>
           &nbsp;&nbsp;`revision`: develop <br>
           &nbsp;&nbsp;`refspec`: refs/heads/develop:refs/heads/develop <br>
         - Fetch refs/pull/1009/head into the master branch and switch to it (not detached) <br>
           &nbsp;&nbsp;`revision`: master <br>
           &nbsp;&nbsp;`refspec`: refs/pull/1009/head:refs/heads/master <br>

1.  `submodules`: defines if the resource should initialize and fetch the
    submodules, value is either `true` or `false`. _If not specified, this will
    default to true_
1.  `depth`: performs a [shallow clone][git-depth] where only the most recent
    commit(s) will be fetched. This setting also applies to submodules. If set to
     `'0'`, all commits will be fetched. _If not specified, the default depth is 1._
1.  `sslVerify`: defines if [http.sslVerify][git-http.sslVerify] should be set
    to `true` or `false` in the global git config. _Defaults to `true` if
    omitted._

[git-rev]: https://git-scm.com/docs/gitrevisions#_specifying_revisions
[git-checkout]: https://git-scm.com/docs/git-checkout
[git-refspec]: https://git-scm.com/book/en/v2/Git-Internals-The-Refspec
[git-depth]: https://git-scm.com/docs/git-clone#Documentation/git-clone.txt---depthltdepthgt
[git-http.sslVerify]: https://git-scm.com/docs/git-config#Documentation/git-config.txt-httpsslVerify

When used as an input, the Git resource includes the exact commit fetched in the
`resourceResults` section of the `taskRun`'s status object:

```yaml
resourceResults:
  - key: commit
    value: 6ed7aad5e8a36052ee5f6079fc91368e362121f7
    resourceRef:
      name: skaffold-git
```

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

#### Using HTTP/HTTPS Proxy

The `httpProxy` and `httpsProxy` parameter can be used to proxy non-SSL/SSL requests, for example to use an enterprise
proxy server for SSL requests:

```yaml
spec:
  type: git
  params:
    - name: url
      value: https://github.com/bobcatfish/wizzbang.git
    - name: httpsProxy
      value: "my-enterprise.proxy.com"
```

#### Using No Proxy

The `noProxy` parameter can be used to opt out of proxying, for example, to not proxy HTTP/HTTPS requests to
`no.proxy.com`:

```yaml
spec:
  type: git
  params:
    - name: url
      value: https://github.com/bobcatfish/wizzbang.git
    - name: noProxy
      value: "no.proxy.com"
```

Note: `httpProxy`, `httpsProxy`, and `noProxy` are all optional but no validation done if all three are specified.

### Pull Request Resource

The `pullRequest` resource represents a pull request event from a source control
system.

Adding the Pull Request resource as an input to a `Task` will populate the
workspace with a set of files containing generic pull request related metadata
such as base/head commit, comments, and labels.

The payloads will also contain links to raw service-specific payloads where
appropriate.

Adding the Pull Request resource as an output of a `Task` will update the source
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
/workspace/<resource>/head.json
/workspace/<resource>/base.json
/workspace/<resource>/pr.json
```

More details:

Labels are empty files, named after the desired label string.

Statuses describe pull request statuses. It is represented as a set of json
files.

References (head and base) describe Git references. They are represented as a
set of json files.

Comments describe a pull request comment. They are represented as a set of json
files. Add a file or modify the `Body` field in an existing json comment file to
interact with the PR. Files with json extension will be parsed as such.
The content of any comments file(s) with other/no extensions will be treated as
body field of the comment.

Other pull request information can be found in `pr.json`. This is a read-only
resource. Users should use other subresources (labels, comments, etc) to
interact with the PR.

For an example of the output this resource provides, see
[`example`](../cmd/pullrequest-init/example).

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
    - fieldName: authToken
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
1.  `provider`: represents the SCM provider to use. This will be "guessed" based
    on the url if not set. Valid values are `github` or `gitlab` today.
1.  `insecure-skip-tls-verify`: represents whether to skip verification of certificates
    from the git server. Valid values are `"true"` or `"false"`, the default being
    `"false"`.

#### Statuses

The following status codes are available to use for the Pull Request resource:
https://godoc.org/github.com/jenkins-x/go-scm/scm#State

#### Pull Request

The `pullRequest` resource will look for GitHub or GitLab OAuth authentication
tokens in spec secrets with a field name called `authToken`.

URLs should be of the form: https://github.com/tektoncd/pipeline/pull/1

#### Self hosted / Enterprise instances

The PullRequest resource works with self hosted or enterprise GitHub/GitLab
instances. Simply provide the pull request URL and set the `provider` parameter.
If you need to skip certificate validation set the `insecure-skip-tls-verify`
parameter to `"true"`.

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
      value: https://github.example.com/wizzbangcorp/wizzbang/pulls/1
    - name: provider
      value: github
```

### Image Resource

An `image` resource represents an image that lives in a remote repository. It is
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
[OCI Image Layout](https://github.com/opencontainers/image-spec/blob/master/image-layout.md)
`index.json` file. This file should be placed on a location as specified in the
task definition under the default resource directory, or the specified
`targetPath`. If there is only one image in the `index.json` file, the digest of
that image is exported; otherwise, the digest of the whole image index would be
exported. For example this build-push task defines the `outputImageDir` for the
`builtImage` resource in `/workspace/buildImage`

```yaml
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: build-push
spec:
  resources:
    inputs:
      - name: workspace
        type: git
    outputs:
      - name: builtImage
        type: image
        targetPath: /workspace/builtImage
  steps: ...
```

If no value is specified for `targetPath`, it will default to
`/workspace/output/{resource-name}`.

_Please check the builder tool used on how to pass this path to create the
output file._

The `taskRun` will include the image digest and URL in the `resourcesResult` field that
is part of the `taskRun.Status`

for example:

```yaml
status:
    # ...
    resourcesResult:
      - key: "digest"
        value: "sha256:eed29cd0b6feeb1a92bc3c4f977fd203c63b376a638731c88cacefe3adb1c660"
        resourceRef:
          name: skaffold-image-leeroy-web
    # ...
```

If the `index.json` file is not produced, the image digest will not be included
in the `taskRun` output.

### Cluster Resource

A `cluster` resource represents a Kubernetes cluster other than the current
cluster Tekton Pipelines is running on. A common use case for this resource is
to deploy your application/function on different clusters.

The resource will use the provided parameters to create a
[kubeconfig](https://kubernetes.io/docs/tasks/access-application-cluster/configure-access-multiple-clusters/)
file that can be used by other steps in the pipeline `Task` to access the target
cluster. The kubeconfig will be placed in
`/workspace/<your-cluster-name>/kubeconfig` on your `Task` container

The Cluster resource has the following parameters:

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
-   `clientKeyData`: contains PEM-encoded data from a client key file 
        for TLS 
-   `clientCertificateData`: contains PEM-encoded data from a client cert file for TLS


Note: Since only one authentication technique is allowed per user, either a
`token` or a `password` should be provided, if both are provided, the `password`
will be ignored.

`clientKeyData` and `clientCertificateData` are only required if `token` or 
`password` is not provided for authentication to cluster.

The following example shows the syntax and structure of a `cluster` resource:

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

Example usage of the `cluster` resource in a `Task`, using
[variable substitution](tasks.md#variable-substitution):

```yaml
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: deploy-image
  namespace: default
spec:
  resources:
    inputs:
      - name: workspace
        type: git
      - name: dockerimage
        type: image
      - name: test-cluster
        type: cluster
  steps:
    - name: deploy
      image: image-with-kubectl
      command: ["bash"]
      args:
        - "-c"
        - kubectl --kubeconfig
          /workspace/$(resources.inputs.test-cluster.name)/kubeconfig --context
          $(resources.inputs.test-cluster.name) apply -f /workspace/service.yaml'
```

To use the `cluster` resource with Google Kubernetes Engine, you should use the
`cadata` authentication mechanism.

To determine the caData, you can use the following `gcloud` commands:

```shell
gcloud container clusters describe <cluster-name> --format='value(masterAuth.clusterCaCertificate)'
```

To create a secret with this information, you can use:

```shell
CADATA=$(gcloud container clusters describe <cluster-name> --format='value(masterAuth.clusterCaCertificate)')
kubectl create secret generic cluster-ca-data --from-literal=cadata=$CADATA
```

To retrieve the URL, you can use this gcloud command:

```shell
gcloud container clusters describe <cluster-name> --format='value(endpoint)'
```

Then to use these in a resource, reference the cadata from the secret you
created above, and use the IP address from the gcloud command as your url
(prefixed with https://):

```yaml
spec:
  type: cluster
  params:
    - name: url
      value: https://<ip address determined above>
  secrets:
    - fieldName: cadata
      secretName: cluster-ca-data
      secretKey: cadata
```

### Storage Resource

The `storage` resource represents blob storage, that contains either an object
or directory. Adding the storage resource as an input to a `Task` will download
the blob and allow the `Task` to perform the required actions on the contents of
the blob.

Only blob storage type
[Google Cloud Storage](https://cloud.google.com/storage/)(gcs) is supported as
of now via [GCS storage resource](#gcs-storage-resource).

#### GCS Storage Resource

The `gcs` storage resource points to
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
    a storage artifact is not considered a directory.

    -   If the artifact is a directory then `-r`(recursive) flag is used, to
        copy all files under the source directory to a GCS bucket. Eg: `gsutil
        cp -r source_dir/* gs://some-bucket`
    -   If an artifact is a single file like a zip or tar, then the copy will be
        only 1 level deep(not recursive). It will not trigger a copy of sub
        directories in the source directory. Eg: `gsutil cp source.tar
        gs://some-bucket.tar`.

Private buckets can also be configured as storage resources. To access GCS
private buckets, service accounts with correct permissions are required. The
`secrets` field on the storage resource is used for configuring this
information. Below is an example on how to create a storage resource with a
service account.

1.  Refer to the
    [official documentation](https://cloud.google.com/compute/docs/access/service-accounts)
    on how to create service accounts and configuring
    [IAM permissions](https://cloud.google.com/storage/docs/access-control/iam-permissions)
    to access buckets.

1.  Create a Kubernetes secret from a downloaded service account json key

    ```bash
    kubectl create secret generic bucket-sa --from-file=./service_account.json
    ```

1.  To access the GCS private bucket environment variable
    [`GOOGLE_APPLICATION_CREDENTIALS`](https://cloud.google.com/docs/authentication/production)
    should be set, so apply the above created secret to the GCS storage resource
    under the `fieldName` key.

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

### Cloud Event Resource

The `cloudevent` resource represents a
[cloud event](https://github.com/cloudevents/spec) that is sent to a target
`URI` upon completion of a `TaskRun`. The `cloudevent` resource sends Tekton
specific events; the body of the event includes the entire `TaskRun` spec plus
status; the types of events defined for now are:

-   dev.tekton.event.task.unknown
-   dev.tekton.event.task.successful
-   dev.tekton.event.task.failed

`cloudevent` resources are useful to notify a third party upon the completion
and status of a `TaskRun`. In combinations with the
[Tekton triggers](https://github.com/tektoncd/triggers) project they can be used
to link `Task/PipelineRuns` asynchronously.

To create a CloudEvent resource using the `PipelineResource` CRD:

```yaml
apiVersion: tekton.dev/v1alpha1
kind: PipelineResource
metadata:
  name: event-to-sink
spec:
  type: cloudEvent
  params:
    - name: targetURI
      value: http://sink:8080
```

The content of an event is for example:

```yaml
Context Attributes,
  SpecVersion: 0.2
  Type: dev.tekton.event.task.successful
  Source: /apis/tekton.dev/v1beta1/namespaces/default/taskruns/pipeline-run-api-16aa55-source-to-image-task-rpndl
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

## Why Aren't PipelineResources in Beta?

The short answer is that they're not ready to be given a Beta level of support by Tekton's developers. The long answer is, well, longer:

- Their behaviour can be opaque.

    They're implemented as a mixture of injected Task Steps, volume configuration and type-specific code in Tekton
    Pipeline's controller. This means errors from `PipelineResources` can manifest in quite a few different ways
    and it's not always obvious whether an error directly relates to `PipelineResource` behaviour. This problem
    is compounded by the fact that, while our docs explain each Resource type's "happy path", there never seems to
    be enough info available to explain error cases sufficiently.

- When they fail they're difficult to debug.

    Several PipelineResources inject their own Steps before a `Task's` Steps. It's extremely difficult to manually
    insert Steps before them to inspect the state of a container before they run.

- There aren't enough of them.

    The six types of existing PipelineResources only cover a tiny subset of the possible systems and side-effects we
    want to support with Tekton Pipelines.

- Adding extensibility to them makes them really similar to `Tasks`:
    - User-definable `Steps`? This is what `Tasks` provide.
    - User-definable params? Tasks already have these.
    - User-definable "resource results"? `Tasks` have `Task` Results.
    - Sharing data between Tasks using PVCs? `workspaces` provide this for `Tasks`.
- They make `Tasks` less reusable.
    - A `Task` has to choose the `type` of `PipelineResource` it will accept.
    - If a `Task` accepts a `git` `PipelineResource` then it's not able to accept a `gcs` `PipelineResource` from a
      `TaskRun` or `PipelineRun` even though both the `git` and `gcs` `PipelineResources` fetch files. They should
      technically be interchangeable: all they do is write files from somewhere remote onto disk. Yet with the existing
      `PipelineResources` implementation they aren't interchangeable.

They also present challenges from a documentation perspective:

- Their purpose is ambiguous and it's difficult to articulate what the CRD is precisely for.
- Four of the types interact with external systems (git, pull-request, gcs, gcs-build).
- Five of them write files to a Task's disk (git, pull-request, gcs, gcs-build, cluster).
- One tells the Pipelines controller to emit CloudEvents to a specific endpoint (cloudEvent).
- One writes config to disk for a `Task` to use (cluster).
- One writes a digest in one `Task` and then reads it back in another `Task` (image).
- Perhaps the one thing you can say about the `PipelineResource` CRD is that it can create
  side-effects for your `Tasks`.

### What's still missing

So what are PipelineResources still good for?  We think we've identified some of the most important things:

1. You can augment `Task`-only workflows with `PipelineResources` that, without them, can only be done with `Pipelines`.
    - For example, let's say you want to checkout a git repo for your Task to test. You have two options. First, you could use a `git` PipelineResource and add it directly to your test `Task`. Second, you could write a `Pipeline` that has a `git-clone` `Task` which checks out the code onto a PersistentVolumeClaim `workspace` and then passes that PVC `workspace` to your test `Task`. For a lot of users the second workflow is totally acceptable but for others it isn't. Some of the most notable reasons we've heard are:
      - Some users simply cannot allocate storage on their platform, meaning `PersistentVolumeClaims` are out of the question.
      - Expanding a single `Task` workflow into a `Pipeline` is labor-intensive and feels unnecessary.
2. Despite being difficult to explain the whole CRD clearly each individual `type` is relatively easy to explain.
    - For example, users can build a pretty good "hunch" for what a `git` `PipelineResource` is without really reading any docs.
3. Configuring CloudEvents to be emitted by the Tekton Pipelines controller.
    - Work is ongoing to get notifications support into the Pipelines controller which should hopefully be able to replace the `cloudEvents` `PipelineResource`.

For each of these there is some amount of ongoing work or discussion. We have deprecated `PipelineResources` and are 
actively working to cover the missing replacement features. Read more about the deprecation in [TEP-0074](https://github.com/tektoncd/community/blob/main/teps/0074-deprecate-pipelineresources.md).

For Beta-supported alternatives to PipelineResources see
[the v1alpha1 to v1beta1 migration guide](./migrating-v1alpha1-to-v1beta1.md#pipelineresources-and-catalog-tasks)
which lists each PipelineResource type and a suggested option for replacing it.

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
