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
| PipelineResources are deprecated, along with the components of the API that rely on them as proposed in [TEP-0074](https://github.com/tektoncd/community/blob/main/teps/0074-deprecate-pipelineresources.md) See [Deprecated `PipelineResources` related features](#deprecated-pipelineresources-related-features) for more info. | [v0.30.0](https://github.com/tektoncd/pipeline/releases/tag/v0.30.0) | Alpha | Dec 20 2021 |
| [PipelineRun.Timeout is deprecated and will be removed](https://github.com/tektoncd/community/blob/main/teps/0046-finallytask-execution-post-timeout.md)                                                                     | v0.36.0                                                              | Beta                                                                                                   | Feb 25, 2023                        |
| [Several fields of Task.Step are deprecated](https://github.com/tektoncd/pipeline/issues/4737)                                                                                                                               | v0.36.0                                                              | Beta                                                                                                   | Feb 25, 2023                        |
| [`v1alpha1.Run` is deprecated, feature flag `enable-custom-task` is removed in v0.44.0, `custom-task-version` will be removed together with `v1alpha1.Run`](https://github.com/tektoncd/community/blob/main/teps/0114-custom-tasks-beta.md) | v0.43.0                                                              | Alpha                                                                                                 | April 10, 2023 or v0.47.0           |
| [ClusterTask is deprecated](https://github.com/tektoncd/pipeline/issues/4476)                                                                                                                                                | v0.41.0                                                              | Beta                                                                                                   | July 13, 2023                       |
| [`pipelineRef.bundle` and `taskRef.bundle` are deprecated](https://github.com/tektoncd/pipeline/issues/5514)                                                                                                                 | v0.41.0                                                              | Alpha                                                                                                  | July 13, 2023                       |
| [The `config-trusted-resources` configMap is deprecated](https://github.com/tektoncd/pipeline/issues/5852)                                                                                                                 | v0.45.0                                                              | Alpha                                                                                                  | v0.46.0                       |
### Deprecated PipelineResources related features:

The following features are deprecated as part of the deprecation of PipelineResources.
See [TEP-0074](https://github.com/tektoncd/community/blob/main/teps/0074-deprecate-pipelineresources.md) for more information.

- the fields`task.spec.resources`, `taskRun.spec.resources`, `pipeline.spec.resources`, `pipelineRun.spec.resources`, and `taskRun.status.cloudEvents`

- The [kubeconfigwriter](https://github.com/tektoncd/pipeline/blob/release-v0.43.x/pkg/apis/pipeline/images.go#L36) and [imagedigestexporter](https://github.com/tektoncd/pipeline/blob/release-v0.43.x/pkg/apis/pipeline/images.go#L46) images, used by the Cluster PipelineResource and Image PipelineResource, respectively

- The [`tekton_pipelines_controller_cloudevent_count`](https://github.com/tektoncd/pipeline/blob/main/docs/metrics.md) metric

## Removed features

The features listed below have been removed but may still be supported in releases that have not reached their EOL.

| Removed Feature                                                                                                                                                                                                   | Removal Pull Request  | Removal Date | Latest Release with Support | EOL of Supported Release |
|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------|-------------------------------------|-------------------------------------|
| [The `PipelineRun.Status.TaskRuns` and `PipelineRun.Status.Runs` fields and the `embedded-status` feature flag along with their functionalities have been tombstoned since v0.45.](https://github.com/tektoncd/community/blob/main/teps/0100-embedded-taskruns-and-runs-status-in-pipelineruns.md) | [[TEP100] Remove Taskruns and Runs Fields for PipelineRunStatus](https://github.com/tektoncd/pipeline/pull/6099)  | Jan 25, 2023 | v0.44.0 | Jan 24, 2024 |