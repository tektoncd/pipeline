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

| Feature Being Deprecated                                                                                                                                            | Deprecation Announcement                                             | [API Compatibility Policy](https://github.com/tektoncd/pipeline/tree/main/api_compatibility_policy.md) | Earliest Date or Release of Removal |
|---------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------|-------------------------------------|
| [The `TaskRun.Status.ResourceResults.ResourceRef` field is deprecated and will be removed.](https://github.com/tektoncd/pipeline/issues/2694)                       | [v0.14.0](https://github.com/tektoncd/pipeline/releases/tag/v0.14.0) | Beta                                                                                                   | April 30 2021                       |
| [The `PipelineRun.Spec.ServiceAccountNames` field is deprecated and will be removed.](https://github.com/tektoncd/pipeline/issues/2614)                             | [v0.15.0](https://github.com/tektoncd/pipeline/releases/tag/v0.15.0) | Beta                                                                                                   | May 15 2021                         |
| [`Conditions` CRD is deprecated and will be removed. Use `when` expressions instead.](https://github.com/tektoncd/community/blob/main/teps/0007-conditions-beta.md) | [v0.16.0](https://github.com/tektoncd/pipeline/releases/tag/v0.16.0) | Alpha                                                                                                  | Nov 02 2020                         |
| [`PipelineRunCancelled` is deprecated and will be removed](https://github.com/tektoncd/pipeline/issues/4611)                                                        | [v0.25.0](https://github.com/tektoncd/pipeline/releases/tag/v0.25.0) | Beta                                                                                                   | July 12 2022                       |
| [`PipelineResources` are deprecated.](https://github.com/tektoncd/community/blob/main/teps/0074-deprecate-pipelineresources.md)                                     | [v0.30.0](https://github.com/tektoncd/pipeline/releases/tag/v0.30.0) | Alpha                                                                                                  | Dec 20 2021                         |
| [The `PipelineRun.Status.TaskRuns` and `PipelineRun.Status.Runs` fields are deprecated and will be removed.](https://github.com/tektoncd/community/blob/main/teps/0100-embedded-taskruns-and-runs-status-in-pipelineruns.md) | v0.35.0 (to be released)                                             | Beta                                                                                                   | Jan 25, 2023                        |
