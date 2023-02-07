<!--
---
linkTitle: "Windows"
weight: 306
---
-->

# Windows

- [Overview](#overview)
- [Scheduling Tasks on Windows Nodes](#scheduling-tasks-on-windows-nodes)
  - [Node Selectors](#node-selectors)
  - [Node Affinity](#node-affinity)
  
## Overview

If you need a Windows environment as part of a Tekton Task or Pipeline, you can include Windows container images in your Task steps. Because Windows containers can only run on a Windows host, you will need to have Windows nodes available in your Kubernetes cluster. You should read [Windows support in Kubernetes](https://kubernetes.io/docs/setup/production-environment/windows/intro-windows-in-kubernetes/) to understand the functionality and limitations of Kubernetes on Windows.

Some important things to note about **Windows containers and Kubernetes**:

- Windows containers cannot run on a Linux host.
- Kubernetes does not support *Windows only* clusters. The Kubernetes control plane components can only run on Linux. 
- Kubernetes currently only supports process isolated containers, which means a container's base image OS version **must** match that of the host OS. See [Windows container version compatibility](https://docs.microsoft.com/en-us/virtualization/windowscontainers/deploy-containers/version-compatibility?tabs=windows-server-20H2%2Cwindows-10-20H2) for more information.
- A Kubernetes Pod cannot contain both Windows and Linux containers.

Some important things to note about **Windows support in Tekton**: 

- A Task can only have Windows or Linux containers, but not both. 
- A Pipeline can contain both Windows and Linux Tasks. 
- In a mixed-OS cluster, TaskRuns and PipelineRuns will need to be scheduled to the correct node using one of the methods [described below](#scheduling-tasks-on-windows-nodes). 
- Tekton's controller components can only run on Linux nodes.

## Scheduling Tasks on Windows Nodes

In order to ensure that Tasks are scheduled to a node with the correct host OS, you will need to update the TaskRun or PipelineRun spec with rules to define this behaviour. This can be done in a couple of different ways, but the simplest option is to specify a node selector. 

### Node Selectors

Node selectors are the simplest way to schedule pods to a Windows or Linux node. By default, Kubernetes nodes include a label `kubernetes.io/os` to identify the host OS. The Kubelet populates this with `runtime.GOOS` as defined by Go. Use `spec.podTemplate.nodeSelector` (or `spec.taskRunSpecs[i].taskPodTemplate.nodeSelector` in a PipelineRun) to schedule Tasks to a node with a specific label and value.

For example:

``` yaml
apiVersion: tekton.dev/v1beta1
kind: TaskRun
metadata:
  name: windows-taskrun
spec:
  taskRef:
    name: windows-task
  podTemplate:
    nodeSelector:
      kubernetes.io/os: windows
---
apiVersion: tekton.dev/v1beta1
kind: TaskRun
metadata:
  name: linux-taskrun
spec:
  taskRef:
    name: linux-task
  podTemplate:
    nodeSelector:
      kubernetes.io/os: linux
```

### Node Affinity

Node affinity can be used as an alternative method of defining the OS requirement of a Task. These rules can be set under `spec.podTemplate.affinity.nodeAffinity` in a TaskRun definition. The example below produces the same result as the previous example which used node selectors.

For example:

```yaml
apiVersion: tekton.dev/v1beta1
kind: TaskRun
metadata:
  name: windows-taskrun
spec:
  taskRef:
    name: windows-task
  podTemplate:
    affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                - key: kubernetes.io/os
                  operator: In
                  values:
                  - windows
---
apiVersion: tekton.dev/v1beta1
kind: TaskRun
metadata:
  name: linux-taskrun
spec:
  taskRef:
    name: linux-task
  podTemplate:
    affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                - key: kubernetes.io/os
                  operator: In
                  values:
                  - linux
```
