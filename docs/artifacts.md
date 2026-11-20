<!--
---
linkTitle: "Artifacts"
weight: 201
---
-->

# Artifacts

- [Overview](#overview)
- [Artifact Storage Configuration Hierarchy](#artifact-storage-configuration-hierarchy)
  - [Level 4: Cluster ConfigMap (lowest priority)](#level-4-cluster-configmap-lowest-priority)
  - [Level 3: Namespace ConfigMap](#level-3-namespace-configmap)
  - [Level 2: Pipeline Annotations](#level-2-pipeline-annotations)
  - [Level 1: PipelineRun Annotations (highest priority)](#level-1-pipelinerun-annotations-highest-priority)
  - [Annotation Reference](#annotation-reference)
  - [Validation](#validation)
- [Artifact Provenance Data](#artifact-provenance-data)
  - [Passing Artifacts between Steps](#passing-artifacts-between-steps)
  - [Passing Artifacts between Tasks](#passing-artifacts-between-tasks)



## Overview
> :seedling: **`Artifacts` is an [alpha](additional-configs.md#alpha-features) feature.**
> The `enable-artifacts` feature flag must be set to `"true"` to read or write artifacts in a step.

Artifacts provide a way to track the origin of data produced and consumed within your Tekton Tasks.

## Artifact Storage Configuration Hierarchy

Artifact storage is configured through a 4-level hierarchy. Each level inherits unset fields from the level below it, so you only need to specify the fields you want to override.

| Priority | Source | Scope |
|----------|--------|-------|
| 1 (highest) | PipelineRun annotations | Per-PipelineRun |
| 2 | Pipeline annotations | Per-Pipeline |
| 3 | Namespace ConfigMap `config-artifact-storage` | Per-namespace |
| 4 (lowest) | Cluster ConfigMap `config-artifact-storage` in `tekton-pipelines` | Cluster-wide |

### Level 4: Cluster ConfigMap (lowest priority)

The cluster-wide default. Install a `config-artifact-storage` ConfigMap in the `tekton-pipelines` namespace:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: config-artifact-storage
  namespace: tekton-pipelines
data:
  enabled: "true"
  backend: "oci"
  oci-repository: "registry.example.com/tekton-artifacts"
  inline-threshold: "512"
  insecure: "false"
  oci.credentialsSecret: "registry-creds"
  oci.attachReferrers: "true"
  oci.tagPattern: "{{namespace}}.{{taskrun}}.{{artifact}}"
  oci.groupByPipelineRun: "false"
```

### Level 3: Namespace ConfigMap

Override cluster defaults for all TaskRuns/PipelineRuns in a specific namespace. Create a `config-artifact-storage` ConfigMap in the target namespace with only the fields you want to override:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: config-artifact-storage
  namespace: team-a
data:
  oci-repository: "registry.example.com/team-a/artifacts"
  insecure: "true"
```

In this example, `team-a` gets its own OCI repository with insecure access, but inherits `enabled`, `backend`, `inline-threshold`, and all other fields from the cluster ConfigMap.

### Level 2: Pipeline Annotations

Override storage config for all runs of a specific Pipeline by adding `tekton.dev/artifact-storage-*` annotations to the Pipeline:

```yaml
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: release-pipeline
  annotations:
    tekton.dev/artifact-storage-oci-repository: "registry.example.com/releases"
    tekton.dev/artifact-storage-oci-attach-referrers: "true"
spec:
  tasks:
    - name: build
      taskSpec:
        # ...
```

### Level 1: PipelineRun Annotations (highest priority)

Override storage config for a single PipelineRun. These annotations take the highest priority:

```yaml
apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  generateName: release-
  annotations:
    tekton.dev/artifact-storage-oci-repository: "registry.example.com/staging"
    tekton.dev/artifact-storage-enabled: "true"
spec:
  pipelineRef:
    name: release-pipeline
```

The same annotations work on TaskRuns:

```yaml
apiVersion: tekton.dev/v1
kind: TaskRun
metadata:
  generateName: build-
  annotations:
    tekton.dev/artifact-storage-oci-repository: "registry.example.com/dev"
    tekton.dev/artifact-storage-enabled: "true"
spec:
  taskRef:
    name: build-task
```

### Annotation Reference

| Annotation | ConfigMap Key | Type | Description |
|------------|---------------|------|-------------|
| `tekton.dev/artifact-storage-enabled` | `enabled` | boolean | Enable/disable artifact storage |
| `tekton.dev/artifact-storage-backend` | `backend` | string | Storage backend (e.g. `oci`) |
| `tekton.dev/artifact-storage-oci-repository` | `oci-repository` | string | Base OCI repository URL |
| `tekton.dev/artifact-storage-inline-threshold` | `inline-threshold` | integer (0-1024) | Max bytes for inline storage |
| `tekton.dev/artifact-storage-insecure` | `insecure` | boolean | Allow plain HTTP / skip TLS verify |
| `tekton.dev/artifact-storage-oci-credentials-secret` | `oci.credentialsSecret` | string | Name of dockerconfigjson Secret |
| `tekton.dev/artifact-storage-oci-attach-referrers` | `oci.attachReferrers` | boolean | Attach artifacts as OCI referrers |
| `tekton.dev/artifact-storage-oci-tag-pattern` | `oci.tagPattern` | string | Tag pattern for artifact manifests |
| `tekton.dev/artifact-storage-oci-group-by-pipelinerun` | `oci.groupByPipelineRun` | boolean | Group artifacts per PipelineRun |

### Validation

Invalid configurations are rejected at admission time (when you create the resource), not at runtime. For example:

- Setting `oci-repository` without `enabled: true` is rejected
- Invalid boolean or integer annotation values are rejected
- The error message includes the full field path (e.g. `spec.pipelineSpec.tasks[0].taskSpec.artifacts`)

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

### Output Artifacts in SLSA Provenance

Artifacts are classified as either:

- Build Outputs - packages, images, etc. that are being published by the build.
- Build Byproducts - logs, caches, etc. that are incidental artifacts that are produced by the build.

By default, Tekton Chains will consider all output artifacts as `byProducts` when generating in the [SLSA provenance](https://slsa.dev/spec/v1.0/provenance). In order to treat an artifact as a [subject](https://slsa.dev/spec/v1.0/provenance#schema) of the build, you must set a boolean field `"buildOutput": true` for the output artifact.

e.g.  
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
          cat > $(artifacts.path) << EOF
          {
            "outputs":[
              {
                "name":"image",
                "buildOutput": true,
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

This informs Tekton Chains your desire to handle the artifact.

> [!TIP] 
> When authoring a `StepAction` or a `Task`, you can parametrize this field to allow users to indicate their desire depending on what they are uploading - this can be useful for actions that may produce either a build output or a byproduct depending on the context!

### Passing Artifacts between Steps
You can pass artifacts from one step to the next using:
- Specific Artifact: `$(steps.<step-name>.inputs.<artifact-category-name>)` or `$(steps.<step-name>.outputs.<artifact-category-name>)`

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
          echo $(steps.artifacts-producer.outputs.image)
```


The resolved value of `$(steps.<step-name>.outputs.<artifact-category-name>)` is the values of an artifact. For this example, 
`$(steps.artifacts-producer.outputs.image)` is resolved to 
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

```json
{
  "artifacts": {
    "inputs": [
      {
        "name": "source",
        "values": [
          {
            "digest": {
              "sha256": "b35cacccfdb1e24dc497d15d553891345fd155713ffe647c281c583269eaaae0"
            },
            "uri": "pkg:github/package-url/purl-spec@244fd47e07d1004f0aed9c"
          }
        ]
      }
    ],
    "outputs": [
      {
        "name": "image",
        "values": [
          {
            "digest": {
              "sha1": "95588b8f34c31eb7d62c92aaa4e6506639b06ef2",
              "sha256": "df85b9e3983fe2ce20ef76ad675ecf435cc99fc9350adc54fa230bae8c32ce48"
            },
            "uri": "pkg:oci/nginx:stable-alpine3.17-slim?repository_url=docker.io/library"
          }
        ]
      }
    ]
  },
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
              "uri": "pkg:github/package-url/purl-spec@244fd47e07d1004f0aed9c"
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
              "uri": "pkg:oci/nginx:stable-alpine3.17-slim?repository_url=docker.io/library"
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
  ]
}

```

### Passing Artifacts between Tasks
You can pass artifacts from one task to the another using:

- Specific Artifact: `$(tasks.<task-name>.inputs.<artifact-category-name>)` or `$(tasks.<task-name>.outputs.<artifact-category-name>)`

The example below shows how to access the previous' task artifacts from another task in a pipeline

```yaml
apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  generateName: pipelinerun-consume-tasks-artifacts
spec:
  pipelineSpec:
    tasks:
      - name: produce-artifacts-task
        taskSpec:
          description: |
            A simple task that produces artifacts
          steps:
            - name: produce-artifacts
              image: bash:latest
              script: |
                #!/usr/bin/env bash
                cat > $(artifacts.path) << EOF
                {
                  "inputs":[
                    {
                      "name":"input-artifacts",
                      "values":[
                        {
                          "uri":"pkg:example.github.com/inputs",
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
                          "uri":"pkg:github/package-url/purl-spec@244fd47e07d1004f0aed9c",
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
      - name: consume-artifacts
        runAfter:
          - produce-artifacts-task
        taskSpec:
          steps:
            - name: artifacts-consumer-python
              image: python:latest
              script: |
                #!/usr/bin/env python3
                import json
                data = json.loads('$(tasks.produce-artifacts-task.outputs.image)')
                if data[0]['uri'] != "pkg:github/package-url/purl-spec@244fd47e07d1004f0aed9c":
                  exit(1)
```


Similar to Step Artifacts. The resolved value of `$(tasks.<task-name>.outputs.<artifact-category-name>)` is the values of an artifact. For this example,
`$(tasks.produce-artifacts-task.outputs.image)` is resolved to
```json
[
  {
    "uri":"pkg:example.github.com/inputs",
    "digest":{
      "sha256":"b35cacccfdb1e24dc497d15d553891345fd155713ffe647c281c583269eaaae0"
    }
 }
]
```
Upon resolution and execution of the `TaskRun`, the `Status` will look something like:
```json
{
 "artifacts": {
      "inputs": [
        {
          "name": "input-artifacts",
          "values": [
            {
              "digest": {
                "sha256": "b35cacccfdb1e24dc497d15d553891345fd155713ffe647c281c583269eaaae0"
              },
              "uri": "pkg:example.github.com/inputs"
            }
          ]
        }
      ],
      "outputs": [
        {
          "name": "image",
          "values": [
            {
              "digest": {
                "sha1": "95588b8f34c31eb7d62c92aaa4e6506639b06ef2",
                "sha256": "df85b9e3983fe2ce20ef76ad675ecf435cc99fc9350adc54fa230bae8c32ce48"
              },
              "uri": "pkg:github/package-url/purl-spec@244fd47e07d1004f0aed9c"
            }
          ]
        }
      ]
    },
    "completionTime": "2024-05-28T14:10:58Z",
    "conditions": [
      {
        "lastTransitionTime": "2024-05-28T14:10:58Z",
        "message": "All Steps have completed executing",
        "reason": "Succeeded",
        "status": "True",
        "type": "Succeeded"
      }
    ],
    "podName": "pipelinerun-consume-tasks-a41ee44e4f964e95adfd3aea417d52f90-pod",
    "provenance": {
      "featureFlags": {
        "awaitSidecarReadiness": true,
        "coschedule": "workspaces",
        "disableCredsInit": false,
        "disableInlineSpec": "",
        "enableAPIFields": "beta",
        "enableArtifacts": true,
        "enableCELInWhenExpression": false,
        "enableConciseResolverSyntax": false,
        "enableKeepPodOnCancel": false,
        "enableParamEnum": false,
        "enableProvenanceInStatus": true,
        "enableStepActions": true,
        "enableTektonOCIBundles": false,
        "enforceNonfalsifiability": "none",
        "maxResultSize": 4096,
        "requireGitSSHSecretKnownHosts": false,
        "resultExtractionMethod": "termination-message",
        "runningInEnvWithInjectedSidecars": true,
        "scopeWhenExpressionsToTask": false,
        "sendCloudEventsForRuns": false,
        "setSecurityContext": false,
        "verificationNoMatchPolicy": "ignore"
      }
    },
    "startTime": "2024-05-28T14:10:48Z",
    "steps": [
      {
        "container": "step-produce-artifacts",
        "imageID": "docker.io/library/bash@sha256:23f90212fd89e4c292d7b41386ef1a6ac2b8a02bbc6947680bfe184cbc1a2899",
        "name": "produce-artifacts",
        "terminated": {
          "containerID": "containerd://1291ce07b175a7897beee6ba62eaa1528427bacb1f76b31435eeba68828c445a",
          "exitCode": 0,
          "finishedAt": "2024-05-28T14:10:57Z",
          "message": "...",
          "reason": "Completed",
          "startedAt": "2024-05-28T14:10:57Z"
        },
        "terminationReason": "Completed"
      }
    ],
    "taskSpec": {
      "description": "A simple task that produces artifacts\n",
      "steps": [
        {
          "computeResources": {},
          "image": "bash:latest",
          "name": "produce-artifacts",
          "script": "#!/usr/bin/env bash\ncat > /tekton/artifacts/provenance.json << EOF\n{\n  \"inputs\":[\n    {\n      \"name\":\"input-artifacts\",\n      \"values\":[\n        {\n          \"uri\":\"pkg:example.github.com/inputs\",\n          \"digest\":{\n            \"sha256\":\"b35cacccfdb1e24dc497d15d553891345fd155713ffe647c281c583269eaaae0\"\n          }\n        }\n      ]\n    }\n  ],\n  \"outputs\":[\n    {\n      \"name\":\"image\",\n      \"values\":[\n        {\n          \"uri\":\"pkg:github/package-url/purl-spec@244fd47e07d1004f0aed9c\",\n          \"digest\":{\n            \"sha256\":\"df85b9e3983fe2ce20ef76ad675ecf435cc99fc9350adc54fa230bae8c32ce48\",\n            \"sha1\":\"95588b8f34c31eb7d62c92aaa4e6506639b06ef2\"\n          }\n        }\n      ]\n    }\n  ]\n}\nEOF\n"
        }
      ]
    }
}
```
