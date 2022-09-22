<!--
---
linkTitle: "Deprecations"
weight: 5000
---
-->

# Deprecations

- [Introduction](#introduction)
- [Deprecation Table](#deprecation-table)

## Introduction

This doc provides a list of features in Tekton Pipelines that are
being deprecated.

## Deprecation Table

| Feature Being Deprecated                                                                                                                                                                                                     | Deprecation Announcement                                             | [API Compatibility Policy](https://github.com/tektoncd/pipeline/tree/main/api_compatibility_policy.md) | Earliest Date or Release of Removal |
|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------|-------------------------------------|
| [`PipelineResources` are deprecated.](https://github.com/tektoncd/community/blob/main/teps/0074-deprecate-pipelineresources.md)                                                                                              | [v0.30.0](https://github.com/tektoncd/pipeline/releases/tag/v0.30.0) | Alpha                                                                                                  | Dec 20 2021                         |
| [The `PipelineRun.Status.TaskRuns` and `PipelineRun.Status.Runs` fields are deprecated and will be removed.](https://github.com/tektoncd/community/blob/main/teps/0100-embedded-taskruns-and-runs-status-in-pipelineruns.md) | v0.35.0                                                              | Beta                                                                                                   | Jan 25, 2023                        |
| [PipelineRun.Timeout is deprecated and will be removed](https://github.com/tektoncd/community/blob/main/teps/0046-finallytask-execution-post-timeout.md)                                                                     | v0.36.0                                                              | Beta                                                                                                   | Feb 25, 2023                        |
| [Several fields of Task.Step are deprecated](https://github.com/tektoncd/pipeline/issues/4737)                                                                                                                               | v0.36.0                                                              | Beta                                                                                                   | Feb 25, 2023                        |
| [ClusterTask is deprecated](https://github.com/tektoncd/pipeline/issues/4476)                                                                                                                               | v0.41.0                                                              | Beta                                                                                                   | July 13, 2023                        |
