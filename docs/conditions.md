# Conditions

This document defines `Conditions` and their capabilities.

*NOTE*: This feature is currently a WIP being tracked in [#1137](https://github.com/tektoncd/pipeline/issues/1137)

---

- [Syntax](#syntax)
  - [Check](#check)
  - [Parameters](#parameters)
  - [Resources](#resources)
- [Labels](#labels)
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
check must specify a [`Step`](./tasks.md#steps). The container image runs till completion. The container 
must exit successfully i.e. with an exit code 0 for the condition evaluation to be successful. All other 
exit codes are considered to be a condition check failure.

### Parameters

A Condition can declare parameters that must be supplied to it during a PipelineRun. Sub-fields
within the check field can access the parameter values using the templating syntax:

```yaml
spec:
  parameters:
    - name: image
      default: ubuntu
  check:
    image: $(params.image)
```

Parameters name are limited to alpha-numeric characters, `-` and `_` and can
only start with alpha characters and `_`. For example, `fooIs-Bar_` is a valid
parameter name, `barIsBa$` or `0banana` are not.
 
Each declared parameter has a type field, assumed to be string if not provided by the user. 
The other possible type is array — useful, for instance, checking that a pushed branch name doesn't match any of 
multiple protected branch names. When the actual parameter value is supplied, its parsed type 
is validated against the type field.

### Resources

Conditions can declare input [`PipelineResources`](resources.md)  via the `resources` field to 
provide the Condition container step with data or context that is needed to perform the check.

Resources in Conditions work similar to the way they work in `Tasks` i.e. they can be accessed using
[variable substitution](./resources.md#variable-substitution) and the `targetPath` field can be used
to [control where the resource is mounted](./resources.md#controlling-where-resources-are-mounted)

## Labels

[Labels](labels.md) defined as part of the `Condition` metadata will be automatically propagated to the `Pod`.

## Examples

For complete examples, see
[the examples folder](https://github.com/tektoncd/pipeline/tree/master/examples).

---

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
