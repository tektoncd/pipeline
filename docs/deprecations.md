<!--
---
linkTitle: "Deprecations"
weight: 107
---
-->

# Deprecations

- [Introduction](#introduction)
- [Deprecation Table](#deprecation-table)

## Introduction

This doc provides a list of features in Tekton Pipelines that are
deprecated or recently removed.

## Deprecation Table

The following features are deprecated but have not yet been removed.

| Deprecated Features                                                                                                                                                                                                      | Deprecation Announcement                                             | [API Compatibility Policy](https://github.com/tektoncd/pipeline/tree/main/api_compatibility_policy.md) | Earliest Date or Release of Removal |
|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------|-------------------------------------|
| [Several fields of Task.Step are deprecated](https://github.com/tektoncd/pipeline/issues/4737)                                                                                                                               | v0.36.0                                                              | Beta                                                                                                   | Feb 25, 2023                        |
| [ClusterTask is deprecated](https://github.com/tektoncd/pipeline/issues/4476)                                                                                                                                                | v0.41.0                                                              | Beta                                                                                                   | July 13, 2023                       |
| [`pipelineRef.bundle` and `taskRef.bundle` are deprecated](https://github.com/tektoncd/pipeline/issues/5514)                                                                                                                 | v0.41.0                                                              | Alpha                                                                                                  | July 13, 2023                       |
| [The `config-trusted-resources` configMap is deprecated](https://github.com/tektoncd/pipeline/issues/5852)                                                                                                                 | v0.45.0                                                              | Alpha                                                                                                  | v0.46.0                       |

## Removed features

The features listed below have been removed but may still be supported in releases that have not reached their EOL.

| Removed Feature                                                                                                                                                                                                   | Removal Pull Request  | Removal Date | Latest LTS Release with Support | EOL of Supported Release |
|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------|-------------------------------------|-------------------------------------|
| [The `PipelineRun.Status.TaskRuns` and `PipelineRun.Status.Runs` fields and the `embedded-status` feature flag along with their functionalities have been tombstoned since v0.45.](https://github.com/tektoncd/community/blob/main/teps/0100-embedded-taskruns-and-runs-status-in-pipelineruns.md)                                                             | [[TEP100] Remove Taskruns and Runs Fields for PipelineRunStatus](https://github.com/tektoncd/pipeline/pull/6099)         | Jan 25, 2023 | v0.44.0 | Jan 24, 2024 |
| PipelineResources are removed, along with the components of the API that rely on them as proposed in [TEP-0074](https://github.com/tektoncd/community/blob/main/teps/0074-deprecate-pipelineresources.md). See [Removed `PipelineResources` related features](#removed-pipelineresources-related-features) for more info. | [[TEP074] Remove Generic PipelineResources with Rest of Resources Types](https://github.com/tektoncd/pipeline/pull/6150) | Mar 8, 2023  | v0.44.0 | Jan 24, 2024 |
| v1alpha1 Runs are removed, as proposed in [TEP-0114](https://github.com/tektoncd/community/blob/main/teps/0114-custom-tasks-beta.md), along with the feature flags `enable-custom-task` and `custom-task-version`. | [TEP-0114: Remove support for v1alpha1.Run](https://github.com/tektoncd/pipeline/pull/6508) | April 7, 2023  | v0.44.0 | Jan 24, 2024 |

### Removed PipelineResources related features:

The following features are removed as part of the deprecation of PipelineResources.
See [TEP-0074](https://github.com/tektoncd/community/blob/main/teps/0074-deprecate-pipelineresources.md) for more information.

- the fields`task.spec.resources`, `taskRun.spec.resources`, `pipeline.spec.resources`, `pipelineRun.spec.resources`, and `taskRun.status.cloudEvents`

- images built upon PipelineResources
  - the [kubeconfigwriter](https://github.com/tektoncd/pipeline/blob/release-v0.43.x/pkg/apis/pipeline/images.go#L36) image used with Cluster PipelineResource
  - the [imagedigestexporter](https://github.com/tektoncd/pipeline/blob/release-v0.43.x/pkg/apis/pipeline/images.go#L46) image of Image PipelineResource
  - the [pullrequest-init](https://github.com/tektoncd/pipeline/blob/c95d34f2d09854d58b4f24663a026740a5543a88/pkg/apis/pipeline/images.go#L44) image used with Pullrequest PipelineResource
  - the [gsutil](https://github.com/tektoncd/pipeline/blob/c95d34f2d09854d58b4f24663a026740a5543a88/pkg/apis/pipeline/images.go#L42) image used with Storage PipelineResource

- The [`tekton_pipelines_controller_cloudevent_count`](https://github.com/tektoncd/pipeline/blob/main/docs/metrics.md) metric

- The artifacts bucket/pvc setup by the `pkg/artifacts` package related with Storage PipelineResources

- The generic pipelineResources functions including inputs and outputs resources and the `from` type

- [TaskRun.Status.ResourcesResult is deprecated and tombstoned #6301](https://github.com/tektoncd/pipeline/issues/6325)