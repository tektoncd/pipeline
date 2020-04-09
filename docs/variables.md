<!--
---
linkTitle: "Variable Substitutions"
weight: 15
---
-->
# Variable Substitutions

This doc aggregates the complete set of variable substitions available
in `Tasks` and `Pipelines`.

## Variables Available in a Pipeline

| Variable | Description |
| -------- | ----------- |
| `params.<param name>` | The value of the param at runtime. |
| `tasks.<task name>.results.<result name>` | The value of a Task's result (**Note**: Affects Task ordering in a Pipeline!) |

## Variables Available in a Task

| Variable | Description |
| -------- | ----------- |
| `params.<param name>` | The value of the param at runtime. |
| `resources.inputs.<resource name>.path` | The path to the input resource's directory. |
| `resources.outputs.<resource name>.path` | The path to the output resource's directory. |
| `results.<result name>.path` | The path to the file where a Task's result should be written. |
| `workspaces.<workspace name>.path` | The path to the mounted workspace. |
| `workspaces.<workspace name>.volume` | The name of the volume populating the workspace. |
| `credentials.path` | The path to the location of credentials written by the `creds-init` init container. |

### PipelineResource Variables

Each PipelineResource exposes its own set of variables. Below the variables are grouped by
PipelineResource type. These are available in `Tasks`.

Each variable is accessible via `resources.inputs.<resource name>.<variable name>` or
`resources.outputs.<resource name>.<variable name>`.

#### Git PipelineResource

| Variable | Description |
| -------- | ----------- |
| `name` | The resource's name |
| `type` | `"git"` |
| `url` | The URL to the Git repo |
| `revision` | The revision to be checked out. |
| `depth` | The integer value of the resource's `depth` param. |
| `sslVerify` | The value of the resource's `sslVerify` param: `"true"` or `"false"`. |
| `httpProxy` | The value of the resource's `httpProxy` param. |
| `httpsProxy` | The value of the resource's `httpsProxy` param. |
| `noProxy` | The value of the resource's `noProxy` param. |

#### PullRequest PipelineResource

| Variable | Description |
| -------- | ----------- |
| `name` | The resource's name |
| `type` | `"pullRequest"` |
| `url` | The URL pointing to the pull request. |
| `provider` | `"github"` or `"gitlab"`. |
| `insecure-skip-tls-verify` | The value of the resource's `insecure-skip-tls-verify` param: `"true"` or `"false"`. |

#### Image PipelineResource

| Variable | Description |
| -------- | ----------- |
| `name` | The resource's name |
| `type` | `"image"` |
| `url` | The complete path to the image. |
| `digest` | The image's digest. |

#### GCS PipelineResource

| Variable | Description |
| -------- | ----------- |
| `name` | The resource's name |
| `type` | `"gcs"` |
| `location` | The location of the blob storage. |

#### BuildGCS PipelineResource

| Variable | Description |
| -------- | ----------- |
| `name` | The resource's name |
| `type` | `"build-gcs"` |
| `location` | The location of the blob storage. |

#### Cluster PipelineResource

| Variable | Description |
| -------- | ----------- |
| `name` | The resource's name |
| `type` | `"cluster"` |
| `url` | Host url of the master node. |
| `username` | The user with access to the cluster. |
| `password` | The password to be used for clusters with basic auth. |
| `namespace` | The namespace to target in the cluster. |
| `token` | Bearer token. |
| `insecure` | Whether TLS connection to server should be verified: `"true"` or `"false"`. |
| `cadata` | Stringified PEM-encoded bytes typically read from a root certificates bundle. |

#### CloudEvent PipelineResource

| Variable | Description |
| -------- | ----------- |
| `name` | The resource's name |
| `type` | `"cloudEvent"` |
| `target-uri` | The URI that will be hit with cloud event payloads. |
