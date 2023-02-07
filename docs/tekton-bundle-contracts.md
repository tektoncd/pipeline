<!--
---
linkTitle: "Tekton Bundles Contract"
weight: 402
---
-->

# Tekton Bundle Contract v0.1

When using a Tekton Bundle in a task or pipeline reference, the OCI artifact backing the
bundle must adhere to the following contract.

## Contract

Only Tekton CRDs (eg, `Task` or `Pipeline`) may reside in a Tekton Bundle used as a Tekton
bundle reference.

Each layer of the image must map 1:1 with a single Tekton resource (eg Task).

*No more than 10* individual layers (Pipelines and/or Tasks) maybe placed in a single image.

Each layer must contain all of the following annotations:

- `dev.tekton.image.name` => `ObjectMeta.Name` of the resource
- `dev.tekton.image.kind` => `TypeMeta.Kind` of the resource, all lower-cased and singular (eg, `task`)
- `dev.tekton.image.apiVersion` => `TypeMeta.APIVersion` of the resource (eg 
"tekton.dev/v1beta1")  

The union of the { `dev.tekton.image.apiVersion`, `dev.tekton.image.kind`, `dev.tekton.image.name` }
annotations on a given layer must be unique among all layers of that image. In practical terms, this means no two
"tasks" can have the same name for example.

Each layer must be compressed and stored with a supported OCI MIME type *except* for `+zstd` types. For list of the 
supported types see 
[the official spec](https://github.com/opencontainers/image-spec/blob/master/layer.md#zstd-media-types).
 
Furthermore, each layer must contain a YAML or JSON representation of the underlying resource. If the resource is 
missing any identifying fields (missing an `apiVersion` for instance) then it will be considered invalid.

Any tool creating a Tekton bundle must enforce this format and ensure that the annotations and contents all match and
conform to this spec. Additionally, the Tekton controller will reject non-conforming Tekton Bundles.

## Examples

Say you wanted to create a Tekton Bundle out of the following resources: 

```yaml
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: foo
---
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: bar
---
apiVersion: tekton.dev/v1beta1
kind: Pipeline
metadata:
  name: foobar
```

If we imagine what the contents of the resulting bundle look like, it would look something like this (YAML is just for 
illustrative purposes):
```
# my-bundle
layers:
  - annotations:
    - name: "dev.tekton.image.name"
      value: "foo"
    - name: "dev.tekton.image.kind"
      value: "Task"
    - name: "dev.tekton.image.apiVersion"
      value: "tekton.dev/v1beta1"
    contents: <compressed bytes of Task object>
  - annotations:
    - name: "dev.tekton.image.name"
      value: "bar"
    - name: "dev.tekton.image.kind"
      value: "Task"
    - name: "dev.tekton.image.apiVersion"
      value: "tekton.dev/v1beta1"
    contents: <compressed bytes of Task object>
  - annotations:
    - name: "dev.tekton.image.name"
      value: "foobar"
    - name: "dev.tekton.image.kind"
      value: "Pipeline"
    - name: "dev.tekton.image.apiVersion"
      value: "tekton.dev/v1beta1"
    contents: <compressed bytes of Pipeline object>
```

---

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
