<!--
---
linkTitle: "Variable Substitutions"
weight: 900
---
-->
# Variable Substitutions Supported by `Tasks` and `Pipelines`

This page documents the variable substitions supported by `Tasks` and `Pipelines`.

## Variables available in a `Pipeline`

| Variable | Description |
| -------- | ----------- |
| `params.<param name>` | The value of the parameter at runtime. |
| `tasks.<taskName>.results.<resultName>` | The value of the `Task's` result. Can alter `Task` execution order within a `Pipeline`.) |
| `context.pipelineRun.name` | The name of the `PipelineRun` that this `Pipeline` is running in. |
| `context.pipeline.name` | The name of this `Pipeline` . |


## Variables available in a `Task`

| Variable | Description |
| -------- | ----------- |
| `params.<param name>` | The value of the parameter at runtime. |
| `resources.inputs.<resourceName>.path` | The path to the input resource's directory. |
| `resources.outputs.<resourceName>.path` | The path to the output resource's directory. |
| `results.<resultName>.path` | The path to the file where the `Task` writes its results data. |
| `workspaces.<workspaceName>.path` | The path to the mounted `Workspace`. |
| `workspaces.<workspaceName>.claim` | The name of the `PersistentVolumeClaim` specified as a volume source for the `Workspace`. Empty string for other volume types. |
| `workspaces.<workspaceName>.volume` | The name of the volume populating the `Workspace`. |
| `credentials.path` | The path to the credentials written by the `creds-init` init container. |
| `context.taskRun.name` | The name of the `TaskRun` that this `Task` is running in. |
| `context.task.name` | The name of this `Task`. |

### `PipelineResource` variables available in a `Task`

Each supported type of `PipelineResource` specified within a `Task` exposes a unique set
of variables. This section lists the variables exposed by each type. You can access a
variable via `resources.inputs.<resourceName>.<variableName>` or
`resources.outputs.<resourceName>.<variableName>`.

#### Variables for the `Git` type

| Variable | Description |
| -------- | ----------- |
| `name` | The name of the resource. |
| `type` | Type value of `"git"`. |
| `url` | The URL of the Git repository. |
| `revision` | The revision to check out. |
| `refspec` | The value of the resource's `refspec` parameter. |
| `depth` | The integer value of the resource's `depth` parameter. |
| `sslVerify` | The value of the resource's `sslVerify` parameter, either `"true"` or `"false"`. |
| `httpProxy` | The value of the resource's `httpProxy` parameter. |
| `httpsProxy` | The value of the resource's `httpsProxy` parameter. |
| `noProxy` | The value of the resource's `noProxy` parameter. |

#### Variables for the `PullRequest` type

| Variable | Description |
| -------- | ----------- |
| `name` | The name of the resource. |
| `type` | Type value of `"pullRequest"`.|
| `url` | The URL of the pull request. |
| `provider` | Provider value, either `"github"` or `"gitlab"`. |
| `insecure-skip-tls-verify` | The value of the resource's `insecure-skip-tls-verify` parameter, either `"true"` or `"false"`. |

#### Variables for the `Image` type

| Variable | Description |
| -------- | ----------- |
| `name` | The name of the resource. |
| `type` | Type value of `"image"`. |
| `url` | The complete path to the image. |
| `digest` | The digest of the image. |

#### Variables for the `GCS` type

| Variable | Description |
| -------- | ----------- |
| `name` | The name of the resource. |
| `type` | Type value of `"gcs"`. |
| `location` | The fully qualified address of the blob storage. |

#### Variables for the  `BuildGCS` type

| Variable | Description |
| -------- | ----------- |
| `name` | The name of the resource. |
| `type` | Type value of `"build-gcs"`. |
| `location` | The fully qualified address of the blob storage. |

#### Variables for the `Cluster` type

| Variable | Description |
| -------- | ----------- |
| `name` | The name of the resource. |
| `type` | Type value of `"cluster"`. |
| `url` | Host URL of the master node. |
| `username` | The user with access to the cluster. |
| `password` | The password for the user specified in `username`. |
| `namespace` | The namespace to target in the cluster. |
| `token` | The bearer token. |
| `insecure` | Whether to verify the TLS connection to the server, either `"true"` or `"false"`. |
| `cadata` | Stringified PEM-encoded bytes from the relevant root certificate bundle. |
| `clientKeyData` | Stringified PEM-encoded bytes from the client key file for TLS. |
| `clientCertificateData` | Stringified PEM-encoded bytes from the client certificate file for TLS. |

#### Variables for the `CloudEvent` type

| Variable | Description |
| -------- | ----------- |
| `name` | The name of the resource. |
| `type` | Type value of `"cloudEvent"`. |
| `target-uri` | The URI to hit with cloud event payloads. |
