<!--
---
linkTitle: "Artifacts"
weight: 201
---
-->

# Artifacts

- [Overview](#overview)
- [Artifact Provenance Data](#passing-artifacts-between-steps)
  - [Passing Artifacts between Steps](#passing-artifacts-between-steps)



## Overview
> :seedling: **`Artifacts` is an [alpha](additional-configs.md#alpha-features) feature.**
> The `enable-artifacts` feature flag must be set to `"true"` to read or write artifacts in a step.

Artifacts provide a way to track the origin of data produced and consumed within your Tekton Tasks.

## Artifact Provenance Data
Artifacts fall into two categories:

 - Inputs: Artifacts downloaded and used by the Step/Task.
 - Outputs: Artifacts created and uploaded by the Step/Task.
Example Structure:
```json
{
  "inputs":[
    {
      "name": "<input-category-name>", 
      "values": [
        {
          "uri": "pkg:github/package-url/purl-spec@244fd47e07d1004f0aed9c", 
          "digest": { "sha256": "b35caccc..." }
        }
      ]
    }
  ],
  "outputs": [
    {
      "name": "<output-category-name>",
      "values": [
        {
          "uri": "pkg:oci/nginx:stable-alpine3.17-slim?repository_url=docker.io/library",
          "digest": {
            "sha256": "df85b9e3...",
            "sha1": "95588b8f..."
          }
        }
      ]
    }
  ]
}

```

The content is written by the `Step` to a file `$(step.artifacts.path)`:

```yaml
apiVersion: tekton.dev/v1
kind: TaskRun
metadata:
  generateName: step-artifacts-
spec:
  taskSpec:
    description: |
      A simple task that populates artifacts to TaskRun stepState
    steps:
      - name: artifacts-producer
        image: bash:latest
        script: |
          cat > $(step.artifacts.path) << EOF
          {
            "inputs":[
              {
                "name":"source",
                "values":[
                  {
                    "uri":"pkg:github/package-url/purl-spec@244fd47e07d1004f0aed9c",
                    "digest":{
                      "sha256":"b35cacccfdb1e24dc497d15d553891345fd155713ffe647c281c583269eaaae0"
                    }
                  }
                ]
              }
            ],
            "outputs":[
              {
                "name":"image",
                "values":[
                  {
                    "uri":"pkg:oci/nginx:stable-alpine3.17-slim?repository_url=docker.io/library",
                    "digest":{
                      "sha256":"df85b9e3983fe2ce20ef76ad675ecf435cc99fc9350adc54fa230bae8c32ce48",
                      "sha1":"95588b8f34c31eb7d62c92aaa4e6506639b06ef2"
                    }
                  }
                ]
              }
            ]
          }
          EOF
```

It is recommended to use [purl format](https://github.com/package-url/purl-spec/blob/master/PURL-SPECIFICATION.rst) for artifacts uri as shown in the example. 

### Passing Artifacts between Steps
You can pass artifacts from one step to the next using:

- Specific Artifact: `$(steps.<step-name>.inputs.<artifact-category-name>)` or `$(steps.<step-name>.outputs.<artifact-category-name>)`
- Default (First) Artifact: `$(steps.<step-name>.inputs)` or `$(steps.<step-name>.outputs)` (if <artifact-category-name> is omitted)

The example below shows how to access the previous' step artifacts from another step in the same task

```yaml
apiVersion: tekton.dev/v1
kind: TaskRun
metadata:
  generateName: step-artifacts-
spec:
  taskSpec:
    description: |
      A simple task that populates artifacts to TaskRun stepState
    steps:
      - name: artifacts-producer
        image: bash:latest
        script: |
          # the script is for creating the output artifacts
          cat > $(step.artifacts.path) << EOF
          {
            "inputs":[
              {
                "name":"source",
                "values":[
                  {
                    "uri":"pkg:github/package-url/purl-spec@244fd47e07d1004f0aed9c",
                    "digest":{
                      "sha256":"b35cacccfdb1e24dc497d15d553891345fd155713ffe647c281c583269eaaae0"
                    }
                  }
                ]
              }
            ],
            "outputs":[
              {
                "name":"image",
                "values":[
                  {
                    "uri":"pkg:oci/nginx:stable-alpine3.17-slim?repository_url=docker.io/library",
                    "digest":{
                      "sha256":"df85b9e3983fe2ce20ef76ad675ecf435cc99fc9350adc54fa230bae8c32ce48",
                      "sha1":"95588b8f34c31eb7d62c92aaa4e6506639b06ef2"
                    }
                  }
                ]
              }
            ]
          }
          EOF
      - name: artifacts-consumer
        image: bash:latest
        script: |
          echo $(steps.artifacts-producer.outputs)
```


The resolved value of `$(steps.<step-name>.outputs.<artifact-category-name>)` or `$(steps.<step-name>.outputs)` is the values of an artifact. For this example, 
`$(steps.artifacts-producer.outputs)` is resolved to 
```json
[
                  {
                    "uri":"pkg:oci/nginx:stable-alpine3.17-slim?repository_url=docker.io/library",
                    "digest":{
                      "sha256":"df85b9e3983fe2ce20ef76ad675ecf435cc99fc9350adc54fa230bae8c32ce48",
                      "sha1":"95588b8f34c31eb7d62c92aaa4e6506639b06ef2"
                    }
                  }
]
```


Upon resolution and execution of the `TaskRun`, the `Status` will look something like:
```yaml
"steps": [
      {
        "container": "step-artifacts-producer",
        "imageID": "docker.io/library/bash@sha256:5353512b79d2963e92a2b97d9cb52df72d32f94661aa825fcfa0aede73304743",
        "inputs": [
          {
            "name": "source",
            "values": [
              {
                "digest": {
                  "sha256": "b35cacccfdb1e24dc497d15d553891345fd155713ffe647c281c583269eaaae0"
                },
                "uri":"pkg:github/package-url/purl-spec@244fd47e07d1004f0aed9c",
              }
            ]
          }
        ],
        "name": "artifacts-producer",
        "outputs": [
          {
            "name": "image",
            "values": [
              {
                "digest": {
                  "sha1": "95588b8f34c31eb7d62c92aaa4e6506639b06ef2",
                  "sha256": "df85b9e3983fe2ce20ef76ad675ecf435cc99fc9350adc54fa230bae8c32ce48"
                },
                "uri":"pkg:oci/nginx:stable-alpine3.17-slim?repository_url=docker.io/library",
              }
            ]
          }
        ],
        "terminated": {
          "containerID": "containerd://010f02d103d1db48531327a1fe09797c87c1d50b6a216892319b3af93e0f56e7",
          "exitCode": 0,
          "finishedAt": "2024-03-18T17:05:06Z",
          "message": "...",
          "reason": "Completed",
          "startedAt": "2024-03-18T17:05:06Z"
        },
        "terminationReason": "Completed"
      },
      {
        "container": "step-artifacts-consumer",
        "imageID": "docker.io/library/bash@sha256:5353512b79d2963e92a2b97d9cb52df72d32f94661aa825fcfa0aede73304743",
        "name": "artifacts-consumer",
        "terminated": {
          "containerID": "containerd://42428aa7e5a507eba924239f213d185dd4bc0882b6f217a79e6792f7fec3586e",
          "exitCode": 0,
          "finishedAt": "2024-03-18T17:05:06Z",
          "reason": "Completed",
          "startedAt": "2024-03-18T17:05:06Z"
        },
        "terminationReason": "Completed"
      }
    ],

```