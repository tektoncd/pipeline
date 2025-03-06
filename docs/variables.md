<!--
---
linkTitle: "Variable Substitutions"
weight: 407
---
-->

# Variable Substitutions Supported by `Tasks` and `Pipelines`

This page documents the variable substitutions supported by `Tasks` and `Pipelines`.

For instructions on using variable substitutions see the relevant section of [the Tasks doc](tasks.md#using-variable-substitution).

**Note:** Tekton does not escape the contents of variables. Task authors are responsible for properly escaping a variable's value according to the shell, image or scripting language that the variable will be used in.

## Variables available in a `Pipeline`

| Variable                                           | Description                                                                                                                                                                                                                                                                                                                         |
|----------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `params.<param name>`                              | The value of the parameter at runtime.                                                                                                                                                                                                                                                                                              |
| `params['<param name>']`                           | (see above)                                                                                                                                                                                                                                                                                                                         |
| `params["<param name>"]`                           | (see above)                                                                                                                                                                                                                                                                                                                         |
| `params.<param name>[*]`                           | Get the whole param array or object.                                                                                                                                                                                                                                                                                                |
| `params['<param name>'][*]`                        | (see above)                                                                                                                                                                                                                                                                                                                         |
| `params["<param name>"][*]`                        | (see above)                                                                                                                                                                                                                                                                                                                         |
| `params.<param name>[i]`                           | Get the i-th element of param array. This is alpha feature, set `enable-api-fields` to `alpha`  to use it.                                                                                                                                                                                                                          |
| `params['<param name>'][i]`                        | (see above)                                                                                                                                                                                                                                                                                                                         |
| `params["<param name>"][i]`                        | (see above)                                                                                                                                                                                                                                                                                                                         |
| `params.<object-param-name>[*]`                    | Get the value of the whole object param. This is alpha feature, set `enable-api-fields` to `alpha`  to use it.                                                                                                                                                                                                                      |
| `params.<object-param-name>.<individual-key-name>` | Get the value of an individual child of an object param. This is alpha feature, set `enable-api-fields` to `alpha`  to use it.                                                                                                                                                                                                      |
| `tasks.<taskName>.matrix.length`                   | The length of the `Matrix` combination count.                                                                                                                                                                                                                                                                                       |
| `tasks.<taskName>.results.<resultName>`            | The value of the `Task's` result. Can alter `Task` execution order within a `Pipeline`.)                                                                                                                                                                                                                                            |
| `tasks.<taskName>.results.<resultName>[i]`         | The ith value of the `Task's` array result. Can alter `Task` execution order within a `Pipeline`.)                                                                                                                                                                                                                                  |
| `tasks.<taskName>.results.<resultName>[*]`         | The array value of the `Task's` result. Can alter `Task` execution order within a `Pipeline`. Cannot be used in `script`.)                                                                                                                                                                                                          |
| `tasks.<taskName>.results.<resultName>.key`        | The `key` value of the `Task's` object result. Can alter `Task` execution order within a `Pipeline`.)                                                                                                                                                                                                                               |
| `tasks.<taskName>.matrix.<resultName>.length`      | The length of the matrixed `Task's` results. (Can alter `Task` execution order within a `Pipeline`.)                                                                                                                                                                                                                                |
| `workspaces.<workspaceName>.bound`                 | Whether a `Workspace` has been bound or not. "false" if the `Workspace` declaration has `optional: true` and the Workspace binding was omitted by the PipelineRun.                                                                                                                                                                  |
| `context.pipelineRun.name`                         | The name of the `PipelineRun` that this `Pipeline` is running in.                                                                                                                                                                                                                                                                   |
| `context.pipelineRun.namespace`                    | The namespace of the `PipelineRun` that this `Pipeline` is running in.                                                                                                                                                                                                                                                              |
| `context.pipelineRun.uid`                          | The uid of the `PipelineRun` that this `Pipeline` is running in.                                                                                                                                                                                                                                                                    |
| `context.pipeline.name`                            | The name of this `Pipeline` .                                                                                                                                                                                                                                                                                                       |
| `tasks.<pipelineTaskName>.status`                  | The execution status of the specified `pipelineTask`, only available in `finally` tasks. The execution status can be set to any one of the values (`Succeeded`, `Failed`, or `None`) described [here](pipelines.md#using-execution-status-of-pipelinetask).                                                                         |
| `tasks.<pipelineTaskName>.reason`                  | The execution reason of the specified `pipelineTask`, only available in `finally` tasks. The reason can be set to any one of the values (`Failed`, `TaskRunCancelled`, `TaskRunTimeout`, `FailureIgnored`, etc ) described [here](taskruns.md#monitoring-execution-status).                                                         |
| `tasks.status`                                     | An aggregate status of all the `pipelineTasks` under the `tasks` section (excluding the `finally` section). This variable is only available in the `finally` tasks and can have any one of the values (`Succeeded`, `Failed`, `Completed`, or `None`) described [here](pipelines.md#using-aggregate-execution-status-of-all-tasks). |
| `context.pipelineTask.retries`                     | The retries of this `PipelineTask`.                                                                                                                                                                                                                                                                                                 |
| `tasks.<taskName>.outputs.<artifactName>`          | The value of a specific output artifact of the `Task`                                                                                                                                                                                                                                                                               |
| `tasks.<taskName>.inputs.<artifactName>`           | The value of a specific input artifact of the `Task`                                                                                                                                                                                                                                                                                |

## Variables available in a `Task`

| Variable                                           | Description                                                                                                                    |
|----------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------|
| `params.<param name>`                              | The value of the parameter at runtime.                                                                                         |
| `params['<param name>']`                           | (see above)                                                                                                                    |
| `params["<param name>"]`                           | (see above)                                                                                                                    |
| `params.<param name>[*]`                           | Get the whole param array or object.                                                                                           |
| `params['<param name>'][*]`                        | (see above)                                                                                                                    |
| `params["<param name>"][*]`                        | (see above)                                                                                                                    |
| `params.<param name>[i]`                           | Get the i-th element of param array. This is alpha feature, set `enable-api-fields` to `alpha`  to use it.                     |
| `params['<param name>'][i]`                        | (see above)                                                                                                                    |
| `params["<param name>"][i]`                        | (see above)                                                                                                                    |
| `params.<object-param-name>.<individual-key-name>` | Get the value of an individual child of an object param. This is alpha feature, set `enable-api-fields` to `alpha`  to use it. |
| `results.<resultName>.path`                        | The path to the file where the `Task` writes its results data.                                                                 |
| `results['<resultName>'].path`                     | (see above)                                                                                                                    |
| `results["<resultName>"].path`                     | (see above)                                                                                                                    |
| `workspaces.<workspaceName>.path`                  | The path to the mounted `Workspace`. Empty string if an optional `Workspace` has not been provided by the TaskRun.             |
| `workspaces.<workspaceName>.bound`                 | Whether a `Workspace` has been bound or not. "false" if an optional`Workspace` has not been provided by the TaskRun.           |
| `workspaces.<workspaceName>.claim`                 | The name of the `PersistentVolumeClaim` specified as a volume source for the `Workspace`. Empty string for other volume types. |
| `workspaces.<workspaceName>.volume`                | The name of the volume populating the `Workspace`.                                                                             |
| `credentials.path`                                 | The path to credentials injected from Secrets with matching annotations.                                                       |
| `context.taskRun.name`                             | The name of the `TaskRun` that this `Task` is running in.                                                                      |
| `context.taskRun.namespace`                        | The namespace of the `TaskRun` that this `Task` is running in.                                                                 |
| `context.taskRun.uid`                              | The uid of the `TaskRun` that this `Task` is running in.                                                                       |
| `context.task.name`                                | The name of this `Task`.                                                                                                       |
| `context.task.retry-count`                         | The current retry number of this `Task`.                                                                                       |
| `steps.step-<stepName>.exitCode.path`              | The path to the file where a Step's exit code is stored.                                                                       |
| `steps.step-unnamed-<stepIndex>.exitCode.path`     | The path to the file where a Step's exit code is stored for a step without any name.                                           |
| `artifacts.path`                                   | The path to the file where the `Task` writes its artifacts data.                                                               |

## Fields that accept variable substitutions

| CRD           | Field                                                           |
|---------------|-----------------------------------------------------------------|
| `Task`        | `spec.steps[].name`                                             |
| `Task`        | `spec.steps[].image`                                            |
| `Task`        | `spec.steps[].imagePullPolicy`                                  |
| `Task`        | `spec.steps[].command`                                          |
| `Task`        | `spec.steps[].args`                                             |
| `Task`        | `spec.steps[].script`                                           |
| `Task`        | `spec.steps[].onError`                                          |
| `Task`        | `spec.steps[].env.value`                                        |
| `Task`        | `spec.steps[].env.valueFrom.secretKeyRef.name`                  |
| `Task`        | `spec.steps[].env.valueFrom.secretKeyRef.key`                   |
| `Task`        | `spec.steps[].env.valueFrom.configMapKeyRef.name`               |
| `Task`        | `spec.steps[].env.valueFrom.configMapKeyRef.key`                |
| `Task`        | `spec.steps[].volumeMounts.name`                                |
| `Task`        | `spec.steps[].volumeMounts.mountPath`                           |
| `Task`        | `spec.steps[].volumeMounts.subPath`                             |
| `Task`        | `spec.volumes[].name`                                           |
| `Task`        | `spec.volumes[].configMap.name`                                 |
| `Task`        | `spec.volumes[].configMap.items[].key`                          |
| `Task`        | `spec.volumes[].configMap.items[].path`                         |
| `Task`        | `spec.volumes[].secret.secretName`                              |
| `Task`        | `spec.volumes[].secret.items[].key`                             |
| `Task`        | `spec.volumes[].secret.items[].path`                            |
| `Task`        | `spec.volumes[].persistentVolumeClaim.claimName`                |
| `Task`        | `spec.volumes[].projected.sources.configMap.name`               |
| `Task`        | `spec.volumes[].projected.sources.secret.name`                  |
| `Task`        | `spec.volumes[].projected.sources.serviceAccountToken.audience` |
| `Task`        | `spec.volumes[].csi.nodePublishSecretRef.name`                  |
| `Task`        | `spec.volumes[].csi.volumeAttributes.* `                        |
| `Task`        | `spec.sidecars[].name`                                          |
| `Task`        | `spec.sidecars[].image`                                         |
| `Task`        | `spec.sidecars[].imagePullPolicy`                               |
| `Task`        | `spec.sidecars[].env.value`                                     |
| `Task`        | `spec.sidecars[].env.valueFrom.secretKeyRef.name`               |
| `Task`        | `spec.sidecars[].env.valueFrom.secretKeyRef.key`                |
| `Task`        | `spec.sidecars[].env.valueFrom.configMapKeyRef.name`            |
| `Task`        | `spec.sidecars[].env.valueFrom.configMapKeyRef.key`             |
| `Task`        | `spec.sidecars[].volumeMounts.name`                             |
| `Task`        | `spec.sidecars[].volumeMounts.mountPath`                        |
| `Task`        | `spec.sidecars[].volumeMounts.subPath`                          |
| `Task`        | `spec.sidecars[].command`                                       |
| `Task`        | `spec.sidecars[].args`                                          |
| `Task`        | `spec.sidecars[].script`                                        |
| `Task`        | `spec.workspaces[].mountPath`                                   |
| `TaskRun`     | `spec.workspaces[].subPath`                                     |
| `TaskRun`     | `spec.workspaces[].persistentVolumeClaim.claimName`             |
| `TaskRun`     | `spec.workspaces[].configMap.name`                              |
| `TaskRun`     | `spec.workspaces[].configMap.items[].key`                       |
| `TaskRun`     | `spec.workspaces[].configMap.items[].path`                      |
| `TaskRun`     | `spec.workspaces[].secret.secretName`                           |
| `TaskRun`     | `spec.workspaces[].secret.items[].key`                          |
| `TaskRun`     | `spec.workspaces[].secret.items[].path`                         |
| `TaskRun`     | `spec.workspaces[].projected.sources[].secret.name`             |
| `TaskRun`     | `spec.workspaces[].projected.sources[].secret.items[].key`      |
| `TaskRun`     | `spec.workspaces[].projected.sources[].secret.items[].path`     |
| `TaskRun`     | `spec.workspaces[].projected.sources[].configMap.name`          |
| `TaskRun`     | `spec.workspaces[].projected.sources[].configMap.items[].key`   |
| `TaskRun`     | `spec.workspaces[].projected.sources[].configMap.items[].path`  |
| `TaskRun`     | `spec.workspaces[].csi.driver`                                  |
| `TaskRun`     | `spec.workspaces[].csi.nodePublishSecretRef.name`               |
| `Pipeline`    | `spec.tasks[].params[].value`                                   |
| `Pipeline`    | `spec.tasks[].matrix.params[].value`                            |
| `Pipeline`    | `spec.tasks[].matrix.include[].params[].value`                  |
| `Pipeline`    | `spec.tasks[].displayName`                                      |
| `Pipeline`    | `spec.tasks[].workspaces[].subPath`                             |
| `Pipeline`    | `spec.tasks[].when[].input`                                     |
| `Pipeline`    | `spec.tasks[].when[].values`                                    |
| `Pipeline`    | `spec.tasks[].taskRef.params[].values`                          |
| `Pipeline`    | `spec.tasks[].taskRef.name`                                     |
| `Pipeline`    | `spec.tasks[].onError`                                          |
| `Pipeline`    | `spec.finally[].params[].value`                                 |
| `Pipeline`    | `spec.finally[].matrix.params[].value`                          |
| `Pipeline`    | `spec.finally[].matrix.include[].params[].value`                |
| `Pipeline`    | `spec.finally[].displayName`                                    |
| `Pipeline`    | `spec.finally[].workspaces[].subPath`                           |
| `Pipeline`    | `spec.finally[].when[].input`                                   |
| `Pipeline`    | `spec.finally[].when[].values`                                  |
| `Pipeline`    | `spec.finally[].taskRef.params[].values`                        |
| `Pipeline`    | `spec.finally[].taskRef.name`                                   |
| `Pipeline`    | `spec.finally[].onError`                                        |
| `PipelineRun` | `spec.workspaces[].subPath`                                     |
| `PipelineRun` | `spec.workspaces[].persistentVolumeClaim.claimName`             |
| `PipelineRun` | `spec.workspaces[].configMap.name`                              |
| `PipelineRun` | `spec.workspaces[].configMap.items[].key`                       |
| `PipelineRun` | `spec.workspaces[].configMap.items[].path`                      |
| `PipelineRun` | `spec.workspaces[].secret.secretName`                           |
| `PipelineRun` | `spec.workspaces[].secret.items[].key`                          |
| `PipelineRun` | `spec.workspaces[].secret.items[].path`                         |
| `PipelineRun` | `spec.workspaces[].projected.sources[].secret.name`             |
| `PipelineRun` | `spec.workspaces[].projected.sources[].secret.items[].key`      |
| `PipelineRun` | `spec.workspaces[].projected.sources[].secret.items[].path`     |
| `PipelineRun` | `spec.workspaces[].projected.sources[].configMap.name`          |
| `PipelineRun` | `spec.workspaces[].projected.sources[].configMap.items[].key`   |
| `PipelineRun` | `spec.workspaces[].projected.sources[].configMap.items[].path`  |
| `PipelineRun` | `spec.workspaces[].csi.driver`                                  |
| `PipelineRun` | `spec.workspaces[].csi.nodePublishSecretRef.name`               |
