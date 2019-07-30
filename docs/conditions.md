# Conditions

This document defines `Conditions` and their capabilities.

*NOTE*: This feature is currently a WIP being tracked in [#1137](https://github.com/tektoncd/pipeline/issues/1137)

---

- [Syntax](#syntax)
  - [Check](#check)
- [Examples](#examples)

## Syntax

To define a configuration file for a `Condition` resource, you can specify the
following fields:

- Required:
  - [`apiVersion`][kubernetes-overview] - Specifies the API version, for example
    `tekton.dev/v1alpha1`.
  - [`kind`][kubernetes-overview] - Specify the `Condition` resource object.
  - [`metadata`][kubernetes-overview] - Specifies data to uniquely identify the
    `Condition` resource object, for example a `name`.
  - [`spec`][kubernetes-overview] - Specifies the configuration information for
    your `Condition` resource object. In order for a `Condition` to do anything,
    the spec must include:
    - [`check`](#check) - Specifies a container that you want to run for evaluating the condition 

[kubernetes-overview]:
  https://kubernetes.io/docs/concepts/overview/working-with-objects/kubernetes-objects/#required-fields

### Check

The `check` field is required. You define a single check to define the body of a `Condition`. The 
check must specify a container image that adheres to the [container contract](./container-contract.md). The container image 
runs till completion. The container must exit successfully i.e. with an exit code 0 for the 
condition evaluation to be successful. All other exit codes are considered to be a condition check
failure.

## Examples

For complete examples, see
[the examples folder](https://github.com/tektoncd/pipeline/tree/master/examples).

---

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).