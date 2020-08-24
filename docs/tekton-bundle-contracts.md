<!--
---
linkTitle: "Tekton Bundle Contracts"
weight: 8
---
-->
# Tekton Bundle Contract

When using a Tekton Bundle in a task or pipeline reference, the OCI artifact backing the
bundle must adhere to the following contract.

## Contract

Only Tekton CRDs (eg, `Task` or `Pipeline`) may reside in a Tekton Bundle used as a Tekton
bundle reference.

Each layer of the image must map 1:1 with a single Tekton resource.

Each layer must contain the following annotations:

- `org.opencontainers.image.title` => `ObjectMeta.Name` of the resource
- `cdf.tekton.image.kind` => `TypeMeta.Kind` of the resource, all lowercased (eg, `task`)
- `cdf.tekton.image.apiVersion` => `TypeMeta.APIVersion` of the resource (eg 
"tekton.dev/v1alpha1")  

Each { `apiVersion`, `kind`, `title` } must be unique in the image. No resources of the
same version and kind can be named the same.

The contents of each layer must be the parsed YAML of the corresponding Tekton resource. If
the resource is missing any identifying fields (missing an `apiVersion` for instance) than
it will not be parseable.

---

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).