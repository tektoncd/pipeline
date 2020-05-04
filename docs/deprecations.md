
<!--
---
linkTitle: "Deprecations"
weight: 16
---
-->

# Deprecations

- [Introduction](#introduction)
- [Deprecation Table](#deprecation-table)

## Introduction

This doc provides a list of features in Tekton Pipelines that are
being deprecated.

## Deprecation Table

| Feature Being Deprecated | Deprecation Announcement | [API Compatibility Policy](https://github.com/tektoncd/pipeline/tree/master/api_compatibility_policy.md) | Earliest Date of Removal |
| ------------------------ | ------------------------ | -------------------------------------------------------------------------------------------------------- | ------------------------ |
| [`tekton.dev/task` label on ClusterTasks](https://github.com/tektoncd/pipeline/issues/2533) | [v0.12.0](https://github.com/tektoncd/pipeline/releases/tag/v0.12.0) | Beta | January 30 2021 |
| [Step `$HOME` env var defaults to `/tekton/home`](https://github.com/tektoncd/pipeline/issues/2013) | [v0.11.0-rc1](https://github.com/tektoncd/pipeline/releases/tag/v0.11.0-rc1) | Beta | December 4 2020 |
| [Step `workingDir` defaults to `/workspace`](https://github.com/tektoncd/pipeline/issues/1836) | [v0.11.0-rc1](https://github.com/tektoncd/pipeline/releases/tag/v0.11.0-rc1) | Beta | December 4 2020 |
