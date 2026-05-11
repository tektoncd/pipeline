<!--
---
title: Pipeline API
linkTitle: Pipeline API
weight: 404
---
-->

# API Reference

## Packages
- [resolution.tekton.dev/v1alpha1](#resolutiontektondevv1alpha1)
- [resolution.tekton.dev/v1beta1](#resolutiontektondevv1beta1)
- [tekton.dev/unversioned](#tektondevunversioned)
- [tekton.dev/v1](#tektondevv1)
- [tekton.dev/v1alpha1](#tektondevv1alpha1)
- [tekton.dev/v1beta1](#tektondevv1beta1)


## resolution.tekton.dev/v1alpha1


### Resource Types
- [ResolutionRequest](#resolutionrequest)



#### ResolutionRequest



ResolutionRequest is an object for requesting the content of
a Tekton resource like a pipeline.yaml.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `resolution.tekton.dev/v1alpha1` | | |
| `kind` _string_ | `ResolutionRequest` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[ResolutionRequestSpec](#resolutionrequestspec)_ | Spec holds the information for the request part of the resource request. |  | Optional: \{\} <br /> |
| `status` _[ResolutionRequestStatus](#resolutionrequeststatus)_ | Status communicates the state of the request and, ultimately,<br />the content of the resolved resource. |  | Optional: \{\} <br /> |


#### ResolutionRequestSpec



ResolutionRequestSpec are all the fields in the spec of the
ResolutionRequest CRD.



_Appears in:_
- [ResolutionRequest](#resolutionrequest)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `params` _object (keys:string, values:string)_ | Parameters are the runtime attributes passed to<br />the resolver to help it figure out how to resolve the<br />resource being requested. For example: repo URL, commit SHA,<br />path to file, the kind of authentication to leverage, etc. |  | Optional: \{\} <br /> |


#### ResolutionRequestStatus



ResolutionRequestStatus are all the fields in a ResolutionRequest's
status subresource.



_Appears in:_
- [ResolutionRequest](#resolutionrequest)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the 'Generation' of the Service that<br />was last processed by the controller. |  | Optional: \{\} <br /> |
| `conditions` _[Conditions](#conditions)_ | Conditions the latest available observations of a resource's current state. |  | Optional: \{\} <br /> |
| `annotations` _object (keys:string, values:string)_ | Annotations is additional Status fields for the Resource to save some<br />additional State as well as convey more information to the user. This is<br />roughly akin to Annotations on any k8s resource, just the reconciler conveying<br />richer information outwards. |  |  |
| `data` _string_ | Data is a string representation of the resolved content<br />of the requested resource in-lined into the ResolutionRequest<br />object. |  |  |
| `refSource` _[RefSource](#refsource)_ | RefSource is the source reference of the remote data that records where the remote<br />file came from including the url, digest and the entrypoint. |  | Schemaless: \{\} <br /> |


#### ResolutionRequestStatusFields



ResolutionRequestStatusFields are the ResolutionRequest-specific fields
for the status subresource.



_Appears in:_
- [ResolutionRequestStatus](#resolutionrequeststatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `data` _string_ | Data is a string representation of the resolved content<br />of the requested resource in-lined into the ResolutionRequest<br />object. |  |  |
| `refSource` _[RefSource](#refsource)_ | RefSource is the source reference of the remote data that records where the remote<br />file came from including the url, digest and the entrypoint. |  | Schemaless: \{\} <br /> |



## resolution.tekton.dev/v1beta1


### Resource Types
- [ResolutionRequest](#resolutionrequest)



#### ResolutionRequest



ResolutionRequest is an object for requesting the content of
a Tekton resource like a pipeline.yaml.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `resolution.tekton.dev/v1beta1` | | |
| `kind` _string_ | `ResolutionRequest` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[ResolutionRequestSpec](#resolutionrequestspec)_ | Spec holds the information for the request part of the resource request. |  | Optional: \{\} <br /> |
| `status` _[ResolutionRequestStatus](#resolutionrequeststatus)_ | Status communicates the state of the request and, ultimately,<br />the content of the resolved resource. |  | Optional: \{\} <br /> |


#### ResolutionRequestSpec



ResolutionRequestSpec are all the fields in the spec of the
ResolutionRequest CRD.



_Appears in:_
- [ResolutionRequest](#resolutionrequest)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `params` _[Param](#param) array_ | Parameters are the runtime attributes passed to<br />the resolver to help it figure out how to resolve the<br />resource being requested. For example: repo URL, commit SHA,<br />path to file, the kind of authentication to leverage, etc. |  | Optional: \{\} <br /> |
| `url` _string_ | URL is the runtime url passed to the resolver<br />to help it figure out how to resolver the resource being<br />requested.<br />This is currently at an ALPHA stability level and subject to<br />alpha API compatibility policies. |  | Optional: \{\} <br /> |


#### ResolutionRequestStatus



ResolutionRequestStatus are all the fields in a ResolutionRequest's
status subresource.



_Appears in:_
- [ResolutionRequest](#resolutionrequest)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the 'Generation' of the Service that<br />was last processed by the controller. |  | Optional: \{\} <br /> |
| `conditions` _[Conditions](#conditions)_ | Conditions the latest available observations of a resource's current state. |  | Optional: \{\} <br /> |
| `annotations` _object (keys:string, values:string)_ | Annotations is additional Status fields for the Resource to save some<br />additional State as well as convey more information to the user. This is<br />roughly akin to Annotations on any k8s resource, just the reconciler conveying<br />richer information outwards. |  |  |
| `data` _string_ | Data is a string representation of the resolved content<br />of the requested resource in-lined into the ResolutionRequest<br />object. |  |  |
| `source` _[RefSource](#refsource)_ | Deprecated: Use RefSource instead |  | Schemaless: \{\} <br /> |
| `refSource` _[RefSource](#refsource)_ | RefSource is the source reference of the remote data that records the url, digest<br />and the entrypoint. |  | Schemaless: \{\} <br /> |


#### ResolutionRequestStatusFields



ResolutionRequestStatusFields are the ResolutionRequest-specific fields
for the status subresource.



_Appears in:_
- [ResolutionRequestStatus](#resolutionrequeststatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `data` _string_ | Data is a string representation of the resolved content<br />of the requested resource in-lined into the ResolutionRequest<br />object. |  |  |
| `source` _[RefSource](#refsource)_ | Deprecated: Use RefSource instead |  | Schemaless: \{\} <br /> |
| `refSource` _[RefSource](#refsource)_ | RefSource is the source reference of the remote data that records the url, digest<br />and the entrypoint. |  | Schemaless: \{\} <br /> |



## tekton.dev/unversioned

Package pod contains non-versioned pod configuration











#### Volumes

_Underlying type:_ _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volume-v1-core)_





_Appears in:_
- [Template](#template)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | name of the volume.<br />Must be a DNS_LABEL and unique within the pod.<br />More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names |  |  |
| `hostPath` _[HostPathVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#hostpathvolumesource-v1-core)_ | hostPath represents a pre-existing file or directory on the host<br />machine that is directly exposed to the container. This is generally<br />used for system agents or other privileged things that are allowed<br />to see the host machine. Most containers will NOT need this.<br />More info: https://kubernetes.io/docs/concepts/storage/volumes#hostpath |  | Optional: \{\} <br /> |
| `emptyDir` _[EmptyDirVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#emptydirvolumesource-v1-core)_ | emptyDir represents a temporary directory that shares a pod's lifetime.<br />More info: https://kubernetes.io/docs/concepts/storage/volumes#emptydir |  | Optional: \{\} <br /> |
| `gcePersistentDisk` _[GCEPersistentDiskVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#gcepersistentdiskvolumesource-v1-core)_ | gcePersistentDisk represents a GCE Disk resource that is attached to a<br />kubelet's host machine and then exposed to the pod.<br />Deprecated: GCEPersistentDisk is deprecated. All operations for the in-tree<br />gcePersistentDisk type are redirected to the pd.csi.storage.gke.io CSI driver.<br />More info: https://kubernetes.io/docs/concepts/storage/volumes#gcepersistentdisk |  | Optional: \{\} <br /> |
| `awsElasticBlockStore` _[AWSElasticBlockStoreVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#awselasticblockstorevolumesource-v1-core)_ | awsElasticBlockStore represents an AWS Disk resource that is attached to a<br />kubelet's host machine and then exposed to the pod.<br />Deprecated: AWSElasticBlockStore is deprecated. All operations for the in-tree<br />awsElasticBlockStore type are redirected to the ebs.csi.aws.com CSI driver.<br />More info: https://kubernetes.io/docs/concepts/storage/volumes#awselasticblockstore |  | Optional: \{\} <br /> |
| `gitRepo` _[GitRepoVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#gitrepovolumesource-v1-core)_ | gitRepo represents a git repository at a particular revision.<br />Deprecated: GitRepo is deprecated. To provision a container with a git repo, mount an<br />EmptyDir into an InitContainer that clones the repo using git, then mount the EmptyDir<br />into the Pod's container. |  | Optional: \{\} <br /> |
| `secret` _[SecretVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#secretvolumesource-v1-core)_ | secret represents a secret that should populate this volume.<br />More info: https://kubernetes.io/docs/concepts/storage/volumes#secret |  | Optional: \{\} <br /> |
| `nfs` _[NFSVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#nfsvolumesource-v1-core)_ | nfs represents an NFS mount on the host that shares a pod's lifetime<br />More info: https://kubernetes.io/docs/concepts/storage/volumes#nfs |  | Optional: \{\} <br /> |
| `iscsi` _[ISCSIVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#iscsivolumesource-v1-core)_ | iscsi represents an ISCSI Disk resource that is attached to a<br />kubelet's host machine and then exposed to the pod.<br />More info: https://kubernetes.io/docs/concepts/storage/volumes/#iscsi |  | Optional: \{\} <br /> |
| `glusterfs` _[GlusterfsVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#glusterfsvolumesource-v1-core)_ | glusterfs represents a Glusterfs mount on the host that shares a pod's lifetime.<br />Deprecated: Glusterfs is deprecated and the in-tree glusterfs type is no longer supported. |  | Optional: \{\} <br /> |
| `persistentVolumeClaim` _[PersistentVolumeClaimVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#persistentvolumeclaimvolumesource-v1-core)_ | persistentVolumeClaimVolumeSource represents a reference to a<br />PersistentVolumeClaim in the same namespace.<br />More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#persistentvolumeclaims |  | Optional: \{\} <br /> |
| `rbd` _[RBDVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#rbdvolumesource-v1-core)_ | rbd represents a Rados Block Device mount on the host that shares a pod's lifetime.<br />Deprecated: RBD is deprecated and the in-tree rbd type is no longer supported. |  | Optional: \{\} <br /> |
| `flexVolume` _[FlexVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#flexvolumesource-v1-core)_ | flexVolume represents a generic volume resource that is<br />provisioned/attached using an exec based plugin.<br />Deprecated: FlexVolume is deprecated. Consider using a CSIDriver instead. |  | Optional: \{\} <br /> |
| `cinder` _[CinderVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#cindervolumesource-v1-core)_ | cinder represents a cinder volume attached and mounted on kubelets host machine.<br />Deprecated: Cinder is deprecated. All operations for the in-tree cinder type<br />are redirected to the cinder.csi.openstack.org CSI driver.<br />More info: https://examples.k8s.io/mysql-cinder-pd/README.md |  | Optional: \{\} <br /> |
| `cephfs` _[CephFSVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#cephfsvolumesource-v1-core)_ | cephFS represents a Ceph FS mount on the host that shares a pod's lifetime.<br />Deprecated: CephFS is deprecated and the in-tree cephfs type is no longer supported. |  | Optional: \{\} <br /> |
| `flocker` _[FlockerVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#flockervolumesource-v1-core)_ | flocker represents a Flocker volume attached to a kubelet's host machine. This depends on the Flocker control service being running.<br />Deprecated: Flocker is deprecated and the in-tree flocker type is no longer supported. |  | Optional: \{\} <br /> |
| `downwardAPI` _[DownwardAPIVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#downwardapivolumesource-v1-core)_ | downwardAPI represents downward API about the pod that should populate this volume |  | Optional: \{\} <br /> |
| `fc` _[FCVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#fcvolumesource-v1-core)_ | fc represents a Fibre Channel resource that is attached to a kubelet's host machine and then exposed to the pod. |  | Optional: \{\} <br /> |
| `azureFile` _[AzureFileVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#azurefilevolumesource-v1-core)_ | azureFile represents an Azure File Service mount on the host and bind mount to the pod.<br />Deprecated: AzureFile is deprecated. All operations for the in-tree azureFile type<br />are redirected to the file.csi.azure.com CSI driver. |  | Optional: \{\} <br /> |
| `configMap` _[ConfigMapVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#configmapvolumesource-v1-core)_ | configMap represents a configMap that should populate this volume |  | Optional: \{\} <br /> |
| `vsphereVolume` _[VsphereVirtualDiskVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#vspherevirtualdiskvolumesource-v1-core)_ | vsphereVolume represents a vSphere volume attached and mounted on kubelets host machine.<br />Deprecated: VsphereVolume is deprecated. All operations for the in-tree vsphereVolume type<br />are redirected to the csi.vsphere.vmware.com CSI driver. |  | Optional: \{\} <br /> |
| `quobyte` _[QuobyteVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#quobytevolumesource-v1-core)_ | quobyte represents a Quobyte mount on the host that shares a pod's lifetime.<br />Deprecated: Quobyte is deprecated and the in-tree quobyte type is no longer supported. |  | Optional: \{\} <br /> |
| `azureDisk` _[AzureDiskVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#azurediskvolumesource-v1-core)_ | azureDisk represents an Azure Data Disk mount on the host and bind mount to the pod.<br />Deprecated: AzureDisk is deprecated. All operations for the in-tree azureDisk type<br />are redirected to the disk.csi.azure.com CSI driver. |  | Optional: \{\} <br /> |
| `photonPersistentDisk` _[PhotonPersistentDiskVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#photonpersistentdiskvolumesource-v1-core)_ | photonPersistentDisk represents a PhotonController persistent disk attached and mounted on kubelets host machine.<br />Deprecated: PhotonPersistentDisk is deprecated and the in-tree photonPersistentDisk type is no longer supported. |  |  |
| `projected` _[ProjectedVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#projectedvolumesource-v1-core)_ | projected items for all in one resources secrets, configmaps, and downward API |  |  |
| `portworxVolume` _[PortworxVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#portworxvolumesource-v1-core)_ | portworxVolume represents a portworx volume attached and mounted on kubelets host machine.<br />Deprecated: PortworxVolume is deprecated. All operations for the in-tree portworxVolume type<br />are redirected to the pxd.portworx.com CSI driver when the CSIMigrationPortworx feature-gate<br />is on. |  | Optional: \{\} <br /> |
| `scaleIO` _[ScaleIOVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#scaleiovolumesource-v1-core)_ | scaleIO represents a ScaleIO persistent volume attached and mounted on Kubernetes nodes.<br />Deprecated: ScaleIO is deprecated and the in-tree scaleIO type is no longer supported. |  | Optional: \{\} <br /> |
| `storageos` _[StorageOSVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#storageosvolumesource-v1-core)_ | storageOS represents a StorageOS volume attached and mounted on Kubernetes nodes.<br />Deprecated: StorageOS is deprecated and the in-tree storageos type is no longer supported. |  | Optional: \{\} <br /> |
| `csi` _[CSIVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#csivolumesource-v1-core)_ | csi (Container Storage Interface) represents ephemeral storage that is handled by certain external CSI drivers. |  | Optional: \{\} <br /> |
| `ephemeral` _[EphemeralVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#ephemeralvolumesource-v1-core)_ | ephemeral represents a volume that is handled by a cluster storage driver.<br />The volume's lifecycle is tied to the pod that defines it - it will be created before the pod starts,<br />and deleted when the pod is removed.<br />Use this if:<br />a) the volume is only needed while the pod runs,<br />b) features of normal volumes like restoring from snapshot or capacity<br />   tracking are needed,<br />c) the storage driver is specified through a storage class, and<br />d) the storage driver supports dynamic volume provisioning through<br />   a PersistentVolumeClaim (see EphemeralVolumeSource for more<br />   information on the connection between this volume type<br />   and PersistentVolumeClaim).<br />Use PersistentVolumeClaim or one of the vendor-specific<br />APIs for volumes that persist for longer than the lifecycle<br />of an individual pod.<br />Use CSI for light-weight local ephemeral volumes if the CSI driver is meant to<br />be used that way - see the documentation of the driver for<br />more information.<br />A pod can use both types of ephemeral volumes and<br />persistent volumes at the same time. |  | Optional: \{\} <br /> |
| `image` _[ImageVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#imagevolumesource-v1-core)_ | image represents an OCI object (a container image or artifact) pulled and mounted on the kubelet's host machine.<br />The volume is resolved at pod startup depending on which PullPolicy value is provided:<br />- Always: the kubelet always attempts to pull the reference. Container creation will fail If the pull fails.<br />- Never: the kubelet never pulls the reference and only uses a local image or artifact. Container creation will fail if the reference isn't present.<br />- IfNotPresent: the kubelet pulls if the reference isn't already present on disk. Container creation will fail if the reference isn't present and the pull fails.<br />The volume gets re-resolved if the pod gets deleted and recreated, which means that new remote content will become available on pod recreation.<br />A failure to resolve or pull the image during pod startup will block containers from starting and may add significant latency. Failures will be retried using normal volume backoff and will be reported on the pod reason and message.<br />The types of objects that may be mounted by this volume are defined by the container runtime implementation on a host machine and at minimum must include all valid types supported by the container image field.<br />The OCI object gets mounted in a single directory (spec.containers[*].volumeMounts.mountPath) by merging the manifest layers in the same way as for container images.<br />The volume will be mounted read-only (ro) and non-executable files (noexec).<br />Sub path mounts for containers are not supported (spec.containers[*].volumeMounts.subpath) before 1.33.<br />The field spec.securityContext.fsGroupChangePolicy has no effect on this volume type. |  | Optional: \{\} <br /> |



## tekton.dev/v1


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

Package v1 contains API Schema definitions for the pipeline v1 API group

### Resource Types
- [Pipeline](#pipeline)
- [PipelineRun](#pipelinerun)
- [Task](#task)
- [TaskRun](#taskrun)



#### Algorithm

_Underlying type:_ _string_

Algorithm Standard cryptographic hash algorithm



_Appears in:_
- [ArtifactValue](#artifactvalue)



#### Artifact



Artifact represents an artifact within a system, potentially containing multiple values
associated with it.



_Appears in:_
- [Artifacts](#artifacts)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | The artifact's identifying category name |  |  |
| `values` _[ArtifactValue](#artifactvalue) array_ | A collection of values related to the artifact |  |  |
| `buildOutput` _boolean_ | Indicate if the artifact is a build output or a by-product |  |  |


#### ArtifactValue



ArtifactValue represents a specific value or data element within an Artifact.



_Appears in:_
- [Artifact](#artifact)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `digest` _object (keys:[Algorithm](#algorithm), values:string)_ |  |  |  |
| `uri` _string_ |  |  |  |


#### Artifacts



Artifacts represents the collection of input and output artifacts associated with
a task run or a similar process. Artifacts in this context are units of data or resources
that the process either consumes as input or produces as output.



_Appears in:_
- [TaskRunStatus](#taskrunstatus)
- [TaskRunStatusFields](#taskrunstatusfields)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `inputs` _[Artifact](#artifact) array_ |  |  |  |
| `outputs` _[Artifact](#artifact) array_ |  |  |  |


#### ChildStatusReference



ChildStatusReference is used to point to the statuses of individual TaskRuns and Runs within this PipelineRun.



_Appears in:_
- [PipelineRunStatus](#pipelinerunstatus)
- [PipelineRunStatusFields](#pipelinerunstatusfields)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the TaskRun or Run this is referencing. |  |  |
| `displayName` _string_ | DisplayName is a user-facing name of the pipelineTask that may be<br />used to populate a UI. |  |  |
| `pipelineTaskName` _string_ | PipelineTaskName is the name of the PipelineTask this is referencing. |  |  |
| `whenExpressions` _[WhenExpression](#whenexpression) array_ | WhenExpressions is the list of checks guarding the execution of the PipelineTask |  | Optional: \{\} <br /> |


#### Combination

_Underlying type:_ _object_

Combination is a map, mainly defined to hold a single combination from a Matrix with key as param.Name and value as param.Value



_Appears in:_
- [Combinations](#combinations)





#### EmbeddedTask



EmbeddedTask is used to define a Task inline within a Pipeline's PipelineTasks.



_Appears in:_
- [PipelineTask](#pipelinetask)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `spec` _[RawExtension](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#rawextension-runtime-pkg)_ | Spec is a specification of a custom task |  | Optional: \{\} <br /> |
| `metadata` _[PipelineTaskMetadata](#pipelinetaskmetadata)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `params` _[ParamSpecs](#paramspecs)_ | Params is a list of input parameters required to run the task. Params<br />must be supplied as inputs in TaskRuns unless they declare a default<br />value. |  | Optional: \{\} <br /> |
| `displayName` _string_ | DisplayName is a user-facing name of the task that may be<br />used to populate a UI. |  | Optional: \{\} <br /> |
| `description` _string_ | Description is a user-facing description of the task that may be<br />used to populate a UI. |  | Optional: \{\} <br /> |
| `steps` _[Step](#step) array_ | Steps are the steps of the build; each step is run sequentially with the<br />source mounted into /workspace. |  |  |
| `volumes` _[Volumes](#volumes)_ | Volumes is a collection of volumes that are available to mount into the<br />steps of the build.<br />See Pod.spec.volumes (API version: v1) |  | Schemaless: \{\} <br /> |
| `stepTemplate` _[StepTemplate](#steptemplate)_ | StepTemplate can be used as the basis for all step containers within the<br />Task, so that the steps inherit settings on the base container. |  |  |
| `sidecars` _[Sidecar](#sidecar) array_ | Sidecars are run alongside the Task's step containers. They begin before<br />the steps start and end after the steps complete. |  |  |
| `workspaces` _[WorkspaceDeclaration](#workspacedeclaration) array_ | Workspaces are the volumes that this Task requires. |  |  |
| `results` _[TaskResult](#taskresult) array_ | Results are values that this Task can output |  |  |




#### Matrix



Matrix is used to fan out Tasks in a Pipeline



_Appears in:_
- [PipelineTask](#pipelinetask)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `params` _[Params](#params)_ | Params is a list of parameters used to fan out the pipelineTask<br />Params takes only `Parameters` of type `"array"`<br />Each array element is supplied to the `PipelineTask` by substituting `params` of type `"string"` in the underlying `Task`.<br />The names of the `params` in the `Matrix` must match the names of the `params` in the underlying `Task` that they will be substituting. |  |  |


#### OnErrorType

_Underlying type:_ _string_

OnErrorType defines a list of supported exiting behavior of a container on error



_Appears in:_
- [Step](#step)

| Field | Description |
| --- | --- |
| `stopAndFail` | StopAndFail indicates exit the taskRun if the container exits with non-zero exit code<br /> |
| `continue` | Continue indicates continue executing the rest of the steps irrespective of the container exit code<br /> |


#### Param



Param declares an ParamValues to use for the parameter called name.



_Appears in:_
- [Params](#params)
- [ResolutionRequestSpec](#resolutionrequestspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ |  |  |  |
| `value` _[ParamValue](#paramvalue)_ |  |  | Schemaless: \{\} <br /> |


#### ParamSpec



ParamSpec defines arbitrary parameters needed beyond typed inputs (such as
resources). Parameter values are provided by users as inputs on a TaskRun
or PipelineRun.



_Appears in:_
- [ParamSpecs](#paramspecs)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name declares the name by which a parameter is referenced. |  |  |
| `type` _[ParamType](#paramtype)_ | Type is the user-specified type of the parameter. The possible types<br />are currently "string", "array" and "object", and "string" is the default. |  | Optional: \{\} <br /> |
| `description` _string_ | Description is a user-facing description of the parameter that may be<br />used to populate a UI. |  | Optional: \{\} <br /> |
| `properties` _object (keys:string, values:[PropertySpec](#propertyspec))_ | Properties is the JSON Schema properties to support key-value pairs parameter. |  | Optional: \{\} <br /> |
| `default` _[ParamValue](#paramvalue)_ | Default is the value a parameter takes if no input value is supplied. If<br />default is set, a Task may be executed without a supplied value for the<br />parameter. |  | Schemaless: \{\} <br />Optional: \{\} <br /> |
| `enum` _string array_ | Enum declares a set of allowed param input values for tasks/pipelines that can be validated.<br />If Enum is not set, no input validation is performed for the param. |  | Optional: \{\} <br /> |


#### ParamSpecs

_Underlying type:_ _[ParamSpec](#paramspec)_

ParamSpecs is a list of ParamSpec



_Appears in:_
- [EmbeddedTask](#embeddedtask)
- [PipelineSpec](#pipelinespec)
- [StepActionSpec](#stepactionspec)
- [StepActionSpec](#stepactionspec)
- [TaskSpec](#taskspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name declares the name by which a parameter is referenced. |  |  |
| `type` _[ParamType](#paramtype)_ | Type is the user-specified type of the parameter. The possible types<br />are currently "string", "array" and "object", and "string" is the default. |  | Optional: \{\} <br /> |
| `description` _string_ | Description is a user-facing description of the parameter that may be<br />used to populate a UI. |  | Optional: \{\} <br /> |
| `properties` _object (keys:string, values:[PropertySpec](#propertyspec))_ | Properties is the JSON Schema properties to support key-value pairs parameter. |  | Optional: \{\} <br /> |
| `default` _[ParamValue](#paramvalue)_ | Default is the value a parameter takes if no input value is supplied. If<br />default is set, a Task may be executed without a supplied value for the<br />parameter. |  | Schemaless: \{\} <br />Optional: \{\} <br /> |
| `enum` _string array_ | Enum declares a set of allowed param input values for tasks/pipelines that can be validated.<br />If Enum is not set, no input validation is performed for the param. |  | Optional: \{\} <br /> |


#### ParamType

_Underlying type:_ _string_

ParamType indicates the type of an input parameter;
Used to distinguish between a single string and an array of strings.



_Appears in:_
- [ParamSpec](#paramspec)
- [ParamValue](#paramvalue)
- [PropertySpec](#propertyspec)

| Field | Description |
| --- | --- |
| `string` |  |
| `array` |  |
| `object` |  |


#### ParamValue



ParamValue is a type that can hold a single string, string array, or string map.
Used in JSON unmarshalling so that a single JSON field can accept
either an individual string or an array of strings.



_Appears in:_
- [Param](#param)
- [ParamSpec](#paramspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `Type` _[ParamType](#paramtype)_ |  |  |  |
| `StringVal` _string_ |  |  |  |
| `ArrayVal` _string array_ |  |  |  |
| `ObjectVal` _object (keys:string, values:string)_ |  |  |  |


#### Params

_Underlying type:_ _[Param](#param)_

Params is a list of Param



_Appears in:_
- [IncludeParams](#includeparams)
- [Matrix](#matrix)
- [PipelineRunSpec](#pipelinerunspec)
- [PipelineTask](#pipelinetask)
- [ResolverRef](#resolverref)
- [Step](#step)
- [TaskRunInputs](#taskruninputs)
- [TaskRunSpec](#taskrunspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ |  |  |  |
| `value` _[ParamValue](#paramvalue)_ |  |  | Schemaless: \{\} <br /> |


#### Pipeline



Pipeline describes a list of Tasks to execute. It expresses how outputs
of tasks feed into inputs of subsequent tasks.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `tekton.dev/v1` | | |
| `kind` _string_ | `Pipeline` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[PipelineSpec](#pipelinespec)_ | Spec holds the desired state of the Pipeline from the client |  | Optional: \{\} <br /> |


#### PipelineRef



PipelineRef can be used to refer to a specific instance of a Pipeline.



_Appears in:_
- [PipelineRunSpec](#pipelinerunspec)
- [PipelineTask](#pipelinetask)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the referent; More info: http://kubernetes.io/docs/user-guide/identifiers#names |  |  |
| `apiVersion` _string_ | API version of the referent |  | Optional: \{\} <br /> |
| `ResolverRef` _[ResolverRef](#resolverref)_ | ResolverRef allows referencing a Pipeline in a remote location<br />like a git repo. This field is only supported when the alpha<br />feature gate is enabled. |  | Optional: \{\} <br /> |


#### PipelineResult

_Underlying type:_ _[struct{Name string "json:\"name\""; Type ResultsType "json:\"type,omitempty\""; Description string "json:\"description\""; Value ResultValue "json:\"value\""}](#struct{name-string-"json:\"name\"";-type-resultstype-"json:\"type,omitempty\"";-description-string-"json:\"description\"";-value-resultvalue-"json:\"value\""})_

PipelineResult used to describe the results of a pipeline



_Appears in:_
- [PipelineSpec](#pipelinespec)



#### PipelineRun



PipelineRun represents a single execution of a Pipeline. PipelineRuns are how
the graph of Tasks declared in a Pipeline are executed; they specify inputs
to Pipelines such as parameter values and capture operational aspects of the
Tasks execution such as service account and tolerations. Creating a
PipelineRun creates TaskRuns for Tasks in the referenced Pipeline.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `tekton.dev/v1` | | |
| `kind` _string_ | `PipelineRun` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[PipelineRunSpec](#pipelinerunspec)_ |  |  | Optional: \{\} <br /> |
| `status` _[PipelineRunStatus](#pipelinerunstatus)_ |  |  | Optional: \{\} <br /> |




#### PipelineRunResult



PipelineRunResult used to describe the results of a pipeline



_Appears in:_
- [PipelineRunStatus](#pipelinerunstatus)
- [PipelineRunStatusFields](#pipelinerunstatusfields)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the result's name as declared by the Pipeline |  |  |
| `value` _[ResultValue](#resultvalue)_ | Value is the result returned from the execution of this PipelineRun |  | Schemaless: \{\} <br /> |




#### PipelineRunSpec



PipelineRunSpec defines the desired state of PipelineRun



_Appears in:_
- [PipelineRun](#pipelinerun)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `pipelineRef` _[PipelineRef](#pipelineref)_ |  |  | Optional: \{\} <br /> |
| `pipelineSpec` _[PipelineSpec](#pipelinespec)_ | Specifying PipelineSpec can be disabled by setting<br />`disable-inline-spec` feature flag.<br />See Pipeline.spec (API version: tekton.dev/v1) |  | Schemaless: \{\} <br />Optional: \{\} <br /> |
| `params` _[Params](#params)_ | Params is a list of parameter names and values. |  |  |
| `status` _[PipelineRunSpecStatus](#pipelinerunspecstatus)_ | Used for cancelling a pipelinerun (and maybe more later on) |  | Optional: \{\} <br /> |
| `timeouts` _[TimeoutFields](#timeoutfields)_ | Time after which the Pipeline times out.<br />Currently three keys are accepted in the map<br />pipeline, tasks and finally<br />with Timeouts.pipeline >= Timeouts.tasks + Timeouts.finally |  | Optional: \{\} <br /> |
| `taskRunTemplate` _[PipelineTaskRunTemplate](#pipelinetaskruntemplate)_ | TaskRunTemplate represent template of taskrun |  | Optional: \{\} <br /> |
| `workspaces` _[WorkspaceBinding](#workspacebinding) array_ | Workspaces holds a set of workspace bindings that must match names<br />with those declared in the pipeline. |  | Optional: \{\} <br /> |
| `taskRunSpecs` _[PipelineTaskRunSpec](#pipelinetaskrunspec) array_ | TaskRunSpecs holds a set of runtime specs |  | Optional: \{\} <br /> |
| `managedBy` _string_ | ManagedBy indicates which controller is responsible for reconciling<br />this resource. If unset or set to "tekton.dev/pipeline", the default<br />Tekton controller will manage this resource.<br />This field is immutable. |  | Optional: \{\} <br /> |


#### PipelineRunSpecStatus

_Underlying type:_ _string_

PipelineRunSpecStatus defines the pipelinerun spec status the user can provide



_Appears in:_
- [PipelineRunSpec](#pipelinerunspec)



#### PipelineRunStatus



PipelineRunStatus defines the observed state of PipelineRun



_Appears in:_
- [PipelineRun](#pipelinerun)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the 'Generation' of the Service that<br />was last processed by the controller. |  | Optional: \{\} <br /> |
| `conditions` _[Conditions](#conditions)_ | Conditions the latest available observations of a resource's current state. |  | Optional: \{\} <br /> |
| `annotations` _object (keys:string, values:string)_ | Annotations is additional Status fields for the Resource to save some<br />additional State as well as convey more information to the user. This is<br />roughly akin to Annotations on any k8s resource, just the reconciler conveying<br />richer information outwards. |  |  |
| `startTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#time-v1-meta)_ | StartTime is the time the PipelineRun is actually started. |  |  |
| `completionTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#time-v1-meta)_ | CompletionTime is the time the PipelineRun completed. |  |  |
| `results` _[PipelineRunResult](#pipelinerunresult) array_ | Results are the list of results written out by the pipeline task's containers |  | Optional: \{\} <br /> |
| `pipelineSpec` _[PipelineSpec](#pipelinespec)_ | PipelineSpec contains the exact spec used to instantiate the run.<br />See Pipeline.spec (API version: tekton.dev/v1) |  | Schemaless: \{\} <br /> |
| `skippedTasks` _[SkippedTask](#skippedtask) array_ | list of tasks that were skipped due to when expressions evaluating to false |  | Optional: \{\} <br /> |
| `childReferences` _[ChildStatusReference](#childstatusreference) array_ | list of TaskRun and Run names, PipelineTask names, and API versions/kinds for children of this PipelineRun. |  | Optional: \{\} <br /> |
| `finallyStartTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#time-v1-meta)_ | FinallyStartTime is when all non-finally tasks have been completed and only finally tasks are being executed. |  | Optional: \{\} <br /> |
| `provenance` _[Provenance](#provenance)_ | Provenance contains some key authenticated metadata about how a software artifact was built (what sources, what inputs/outputs, etc.). |  | Optional: \{\} <br /> |
| `spanContext` _object (keys:string, values:string)_ | SpanContext contains tracing span context fields |  |  |


#### PipelineRunStatusFields



PipelineRunStatusFields holds the fields of PipelineRunStatus' status.
This is defined separately and inlined so that other types can readily
consume these fields via duck typing.



_Appears in:_
- [PipelineRunStatus](#pipelinerunstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `startTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#time-v1-meta)_ | StartTime is the time the PipelineRun is actually started. |  |  |
| `completionTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#time-v1-meta)_ | CompletionTime is the time the PipelineRun completed. |  |  |
| `results` _[PipelineRunResult](#pipelinerunresult) array_ | Results are the list of results written out by the pipeline task's containers |  | Optional: \{\} <br /> |
| `pipelineSpec` _[PipelineSpec](#pipelinespec)_ | PipelineSpec contains the exact spec used to instantiate the run.<br />See Pipeline.spec (API version: tekton.dev/v1) |  | Schemaless: \{\} <br /> |
| `skippedTasks` _[SkippedTask](#skippedtask) array_ | list of tasks that were skipped due to when expressions evaluating to false |  | Optional: \{\} <br /> |
| `childReferences` _[ChildStatusReference](#childstatusreference) array_ | list of TaskRun and Run names, PipelineTask names, and API versions/kinds for children of this PipelineRun. |  | Optional: \{\} <br /> |
| `finallyStartTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#time-v1-meta)_ | FinallyStartTime is when all non-finally tasks have been completed and only finally tasks are being executed. |  | Optional: \{\} <br /> |
| `provenance` _[Provenance](#provenance)_ | Provenance contains some key authenticated metadata about how a software artifact was built (what sources, what inputs/outputs, etc.). |  | Optional: \{\} <br /> |
| `spanContext` _object (keys:string, values:string)_ | SpanContext contains tracing span context fields |  |  |




#### PipelineSpec



PipelineSpec defines the desired state of Pipeline.



_Appears in:_
- [Pipeline](#pipeline)
- [PipelineRunSpec](#pipelinerunspec)
- [PipelineRunStatus](#pipelinerunstatus)
- [PipelineRunStatusFields](#pipelinerunstatusfields)
- [PipelineTask](#pipelinetask)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `displayName` _string_ | DisplayName is a user-facing name of the pipeline that may be<br />used to populate a UI. |  | Optional: \{\} <br /> |
| `description` _string_ | Description is a user-facing description of the pipeline that may be<br />used to populate a UI. |  | Optional: \{\} <br /> |
| `tasks` _[PipelineTask](#pipelinetask) array_ | Tasks declares the graph of Tasks that execute when this Pipeline is run. |  |  |
| `params` _[ParamSpecs](#paramspecs)_ | Params declares a list of input parameters that must be supplied when<br />this Pipeline is run. |  |  |
| `workspaces` _[PipelineWorkspaceDeclaration](#pipelineworkspacedeclaration) array_ | Workspaces declares a set of named workspaces that are expected to be<br />provided by a PipelineRun. |  | Optional: \{\} <br /> |
| `results` _[PipelineResult](#pipelineresult) array_ | Results are values that this pipeline can output once run |  | Optional: \{\} <br /> |
| `finally` _[PipelineTask](#pipelinetask) array_ | Finally declares the list of Tasks that execute just before leaving the Pipeline<br />i.e. either after all Tasks are finished executing successfully<br />or after a failure which would result in ending the Pipeline |  |  |


#### PipelineTask



PipelineTask defines a task in a Pipeline, passing inputs from both
Params and from the output of previous tasks.



_Appears in:_
- [PipelineSpec](#pipelinespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of this task within the context of a Pipeline. Name is<br />used as a coordinate with the `from` and `runAfter` fields to establish<br />the execution order of tasks relative to one another. |  |  |
| `displayName` _string_ | DisplayName is the display name of this task within the context of a Pipeline.<br />This display name may be used to populate a UI. |  | Optional: \{\} <br /> |
| `description` _string_ | Description is the description of this task within the context of a Pipeline.<br />This description may be used to populate a UI. |  | Optional: \{\} <br /> |
| `taskRef` _[TaskRef](#taskref)_ | TaskRef is a reference to a task definition. |  | Optional: \{\} <br /> |
| `taskSpec` _[EmbeddedTask](#embeddedtask)_ | TaskSpec is a specification of a task<br />Specifying TaskSpec can be disabled by setting<br />`disable-inline-spec` feature flag.<br />See Task.spec (API version: tekton.dev/v1) |  | Schemaless: \{\} <br />Optional: \{\} <br /> |
| `when` _[WhenExpressions](#whenexpressions)_ | When is a list of when expressions that need to be true for the task to run |  | Optional: \{\} <br /> |
| `retries` _integer_ | Retries represents how many times this task should be retried in case of task failure: ConditionSucceeded set to False |  | Optional: \{\} <br /> |
| `runAfter` _string array_ | RunAfter is the list of PipelineTask names that should be executed before<br />this Task executes. (Used to force a specific ordering in graph execution.) |  | Optional: \{\} <br /> |
| `params` _[Params](#params)_ | Parameters declares parameters passed to this task. |  | Optional: \{\} <br /> |
| `matrix` _[Matrix](#matrix)_ | Matrix declares parameters used to fan out this task. |  | Optional: \{\} <br /> |
| `workspaces` _[WorkspacePipelineTaskBinding](#workspacepipelinetaskbinding) array_ | Workspaces maps workspaces from the pipeline spec to the workspaces<br />declared in the Task. |  | Optional: \{\} <br /> |
| `timeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | Duration after which the TaskRun times out. Defaults to 1 hour.<br />Refer Go's ParseDuration documentation for expected format: https://golang.org/pkg/time/#ParseDuration |  | Optional: \{\} <br /> |
| `pipelineRef` _[PipelineRef](#pipelineref)_ | PipelineRef is a reference to a pipeline definition<br />Note: PipelineRef is in preview mode and not yet supported |  | Optional: \{\} <br /> |
| `pipelineSpec` _[PipelineSpec](#pipelinespec)_ | PipelineSpec is a specification of a pipeline<br />Note: PipelineSpec is in preview mode and not yet supported<br />Specifying PipelineSpec can be disabled by setting<br />`disable-inline-spec` feature flag.<br />See Pipeline.spec (API version: tekton.dev/v1) |  | Schemaless: \{\} <br />Optional: \{\} <br /> |
| `onError` _[PipelineTaskOnErrorType](#pipelinetaskonerrortype)_ | OnError defines the exiting behavior of a PipelineRun on error<br />can be set to [ continue \| stopAndFail ] |  | Optional: \{\} <br /> |


#### PipelineTaskMetadata



PipelineTaskMetadata contains the labels or annotations for an EmbeddedTask



_Appears in:_
- [EmbeddedTask](#embeddedtask)
- [PipelineTaskRunSpec](#pipelinetaskrunspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `labels` _object (keys:string, values:string)_ |  |  | Optional: \{\} <br /> |
| `annotations` _object (keys:string, values:string)_ |  |  | Optional: \{\} <br /> |


#### PipelineTaskOnErrorType

_Underlying type:_ _string_

PipelineTaskOnErrorType defines a list of supported failure handling behaviors of a PipelineTask on error



_Appears in:_
- [PipelineTask](#pipelinetask)

| Field | Description |
| --- | --- |
| `stopAndFail` | PipelineTaskStopAndFail indicates to stop and fail the PipelineRun if the PipelineTask fails<br /> |
| `continue` | PipelineTaskContinue indicates to continue executing the rest of the DAG when the PipelineTask fails<br /> |






#### PipelineTaskRunSpec



PipelineTaskRunSpec  can be used to configure specific
specs for a concrete Task



_Appears in:_
- [PipelineRunSpec](#pipelinerunspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `pipelineTaskName` _string_ |  |  |  |
| `serviceAccountName` _string_ |  |  |  |
| `podTemplate` _[PodTemplate](#podtemplate)_ |  |  |  |
| `stepSpecs` _[TaskRunStepSpec](#taskrunstepspec) array_ |  |  |  |
| `sidecarSpecs` _[TaskRunSidecarSpec](#taskrunsidecarspec) array_ |  |  |  |
| `metadata` _[PipelineTaskMetadata](#pipelinetaskmetadata)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `computeResources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Compute resources to use for this TaskRun |  |  |
| `timeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | Duration after which the TaskRun times out. Overrides the timeout specified<br />on the Task's spec if specified. Takes lower precedence to PipelineRun's<br />`spec.timeouts.tasks`<br />Refer Go's ParseDuration documentation for expected format: https://golang.org/pkg/time/#ParseDuration |  | Optional: \{\} <br /> |


#### PipelineTaskRunTemplate



PipelineTaskRunTemplate is used to specify run specifications for all Task in pipelinerun.



_Appears in:_
- [PipelineRunSpec](#pipelinerunspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podTemplate` _[PodTemplate](#podtemplate)_ |  |  | Optional: \{\} <br /> |
| `serviceAccountName` _string_ |  |  | Optional: \{\} <br /> |


#### PipelineWorkspaceDeclaration

_Underlying type:_ _[struct{Name string "json:\"name\""; Description string "json:\"description,omitempty\""; Optional bool "json:\"optional,omitempty\""}](#struct{name-string-"json:\"name\"";-description-string-"json:\"description,omitempty\"";-optional-bool-"json:\"optional,omitempty\""})_

PipelineWorkspaceDeclaration creates a named slot in a Pipeline that a PipelineRun
is expected to populate with a workspace binding.



_Appears in:_
- [PipelineSpec](#pipelinespec)



#### PropertySpec



PropertySpec defines the struct for object keys



_Appears in:_
- [ParamSpec](#paramspec)
- [StepResult](#stepresult)
- [TaskResult](#taskresult)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[ParamType](#paramtype)_ |  |  |  |


#### Provenance



Provenance contains metadata about resources used in the TaskRun/PipelineRun
such as the source from where a remote build definition was fetched.
This field aims to carry minimum amoumt of metadata in *Run status so that
Tekton Chains can capture them in the provenance.



_Appears in:_
- [PipelineRunStatus](#pipelinerunstatus)
- [PipelineRunStatusFields](#pipelinerunstatusfields)
- [StepState](#stepstate)
- [TaskRunStatus](#taskrunstatus)
- [TaskRunStatusFields](#taskrunstatusfields)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `refSource` _[RefSource](#refsource)_ | RefSource identifies the source where a remote task/pipeline came from. |  |  |
| `featureFlags` _[FeatureFlags](#featureflags)_ | FeatureFlags identifies the feature flags that were used during the task/pipeline run |  |  |


#### Ref



Ref can be used to refer to a specific instance of a StepAction.



_Appears in:_
- [Step](#step)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the referenced step |  |  |
| `ResolverRef` _[ResolverRef](#resolverref)_ | ResolverRef allows referencing a StepAction in a remote location<br />like a git repo. |  | Optional: \{\} <br /> |


#### RefSource



RefSource contains the information that can uniquely identify where a remote
built definition came from i.e. Git repositories, Tekton Bundles in OCI registry
and hub.



_Appears in:_
- [Provenance](#provenance)
- [ResolutionRequestStatus](#resolutionrequeststatus)
- [ResolutionRequestStatus](#resolutionrequeststatus)
- [ResolutionRequestStatusFields](#resolutionrequeststatusfields)
- [ResolutionRequestStatusFields](#resolutionrequeststatusfields)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `uri` _string_ | URI indicates the identity of the source of the build definition.<br />Example: "https://github.com/tektoncd/catalog" |  |  |
| `digest` _object (keys:string, values:string)_ | Digest is a collection of cryptographic digests for the contents of the artifact specified by URI.<br />Example: \{"sha1": "f99d13e554ffcb696dee719fa85b695cb5b0f428"\} |  |  |
| `entryPoint` _string_ | EntryPoint identifies the entry point into the build. This is often a path to a<br />build definition file and/or a target label within that file.<br />Example: "task/git-clone/0.10/git-clone.yaml" |  |  |


#### ResolverName

_Underlying type:_ _string_

ResolverName is the name of a resolver from which a resource can be
requested.



_Appears in:_
- [ResolverRef](#resolverref)



#### ResolverRef



ResolverRef can be used to refer to a Pipeline or Task in a remote
location like a git repo. This feature is in beta and these fields
are only available when the beta feature gate is enabled.



_Appears in:_
- [PipelineRef](#pipelineref)
- [Ref](#ref)
- [TaskRef](#taskref)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `resolver` _[ResolverName](#resolvername)_ | Resolver is the name of the resolver that should perform<br />resolution of the referenced Tekton resource, such as "git". |  | Optional: \{\} <br /> |
| `params` _[Params](#params)_ | Params contains the parameters used to identify the<br />referenced Tekton resource. Example entries might include<br />"repo" or "path" but the set of params ultimately depends on<br />the chosen resolver. |  | Optional: \{\} <br /> |






#### ResultsType

_Underlying type:_ _string_

ResultsType indicates the type of a result;
Used to distinguish between a single string and an array of strings.
Note that there is ResultType used to find out whether a
RunResult is from a task result or not, which is different from
this ResultsType.



_Appears in:_
- [StepResult](#stepresult)
- [TaskResult](#taskresult)
- [TaskRunResult](#taskrunresult)

| Field | Description |
| --- | --- |
| `string` |  |
| `array` |  |
| `object` |  |


#### RetriesStatus

_Underlying type:_ _[TaskRunStatus](#taskrunstatus)_





_Appears in:_
- [TaskRunStatus](#taskrunstatus)
- [TaskRunStatusFields](#taskrunstatusfields)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `Status` _[Status](https://pkg.go.dev/knative.dev/pkg/apis/duck/v1#Status)_ |  |  |  |
| `TaskRunStatusFields` _[TaskRunStatusFields](#taskrunstatusfields)_ | TaskRunStatusFields inlines the status fields. |  |  |


#### Sidecar



Sidecar has nearly the same data structure as Step but does not have the ability to timeout.



_Appears in:_
- [EmbeddedTask](#embeddedtask)
- [TaskSpec](#taskspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the Sidecar specified as a DNS_LABEL.<br />Each Sidecar in a Task must have a unique name (DNS_LABEL).<br />Cannot be updated. |  |  |
| `image` _string_ | Image reference name.<br />More info: https://kubernetes.io/docs/concepts/containers/images |  | Optional: \{\} <br /> |
| `command` _string array_ | Entrypoint array. Not executed within a shell.<br />The image's ENTRYPOINT is used if this is not provided.<br />Variable references $(VAR_NAME) are expanded using the Sidecar's environment. If a variable<br />cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced<br />to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)" will<br />produce the string literal "$(VAR_NAME)". Escaped references will never be expanded, regardless<br />of whether the variable exists or not. Cannot be updated.<br />More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell |  | Optional: \{\} <br /> |
| `args` _string array_ | Arguments to the entrypoint.<br />The image's CMD is used if this is not provided.<br />Variable references $(VAR_NAME) are expanded using the Sidecar's environment. If a variable<br />cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced<br />to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)" will<br />produce the string literal "$(VAR_NAME)". Escaped references will never be expanded, regardless<br />of whether the variable exists or not. Cannot be updated.<br />More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell |  | Optional: \{\} <br /> |
| `workingDir` _string_ | Sidecar's working directory.<br />If not specified, the container runtime's default will be used, which<br />might be configured in the container image.<br />Cannot be updated. |  | Optional: \{\} <br /> |
| `ports` _[ContainerPort](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#containerport-v1-core) array_ | List of ports to expose from the Sidecar. Exposing a port here gives<br />the system additional information about the network connections a<br />container uses, but is primarily informational. Not specifying a port here<br />DOES NOT prevent that port from being exposed. Any port which is<br />listening on the default "0.0.0.0" address inside a container will be<br />accessible from the network.<br />Cannot be updated. |  | Optional: \{\} <br /> |
| `envFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envfromsource-v1-core) array_ | List of sources to populate environment variables in the Sidecar.<br />The keys defined within a source must be a C_IDENTIFIER. All invalid keys<br />will be reported as an event when the container is starting. When a key exists in multiple<br />sources, the value associated with the last source will take precedence.<br />Values defined by an Env with a duplicate key will take precedence.<br />Cannot be updated. |  | Optional: \{\} <br /> |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envvar-v1-core) array_ | List of environment variables to set in the Sidecar.<br />Cannot be updated. |  | Optional: \{\} <br /> |
| `computeResources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | ComputeResources required by this Sidecar.<br />Cannot be updated.<br />More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/ |  | Optional: \{\} <br /> |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ | Volumes to mount into the Sidecar's filesystem.<br />Cannot be updated. |  | Optional: \{\} <br /> |
| `volumeDevices` _[VolumeDevice](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumedevice-v1-core) array_ | volumeDevices is the list of block devices to be used by the Sidecar. |  | Optional: \{\} <br /> |
| `livenessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#probe-v1-core)_ | Periodic probe of Sidecar liveness.<br />Container will be restarted if the probe fails.<br />Cannot be updated.<br />More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes |  | Optional: \{\} <br /> |
| `readinessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#probe-v1-core)_ | Periodic probe of Sidecar service readiness.<br />Container will be removed from service endpoints if the probe fails.<br />Cannot be updated.<br />More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes |  | Optional: \{\} <br /> |
| `startupProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#probe-v1-core)_ | StartupProbe indicates that the Pod the Sidecar is running in has successfully initialized.<br />If specified, no other probes are executed until this completes successfully.<br />If this probe fails, the Pod will be restarted, just as if the livenessProbe failed.<br />This can be used to provide different probe parameters at the beginning of a Pod's lifecycle,<br />when it might take a long time to load data or warm a cache, than during steady-state operation.<br />This cannot be updated.<br />More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes |  | Optional: \{\} <br /> |
| `lifecycle` _[Lifecycle](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#lifecycle-v1-core)_ | Actions that the management system should take in response to Sidecar lifecycle events.<br />Cannot be updated. |  | Optional: \{\} <br /> |
| `terminationMessagePath` _string_ | Optional: Path at which the file to which the Sidecar's termination message<br />will be written is mounted into the Sidecar's filesystem.<br />Message written is intended to be brief final status, such as an assertion failure message.<br />Will be truncated by the node if greater than 4096 bytes. The total message length across<br />all containers will be limited to 12kb.<br />Defaults to /dev/termination-log.<br />Cannot be updated. |  | Optional: \{\} <br /> |
| `terminationMessagePolicy` _[TerminationMessagePolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#terminationmessagepolicy-v1-core)_ | Indicate how the termination message should be populated. File will use the contents of<br />terminationMessagePath to populate the Sidecar status message on both success and failure.<br />FallbackToLogsOnError will use the last chunk of Sidecar log output if the termination<br />message file is empty and the Sidecar exited with an error.<br />The log output is limited to 2048 bytes or 80 lines, whichever is smaller.<br />Defaults to File.<br />Cannot be updated. |  | Optional: \{\} <br /> |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#pullpolicy-v1-core)_ | Image pull policy.<br />One of Always, Never, IfNotPresent.<br />Defaults to Always if :latest tag is specified, or IfNotPresent otherwise.<br />Cannot be updated.<br />More info: https://kubernetes.io/docs/concepts/containers/images#updating-images |  | Optional: \{\} <br /> |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#securitycontext-v1-core)_ | SecurityContext defines the security options the Sidecar should be run with.<br />If set, the fields of SecurityContext override the equivalent fields of PodSecurityContext.<br />More info: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/ |  | Optional: \{\} <br /> |
| `stdin` _boolean_ | Whether this Sidecar should allocate a buffer for stdin in the container runtime. If this<br />is not set, reads from stdin in the Sidecar will always result in EOF.<br />Default is false. |  | Optional: \{\} <br /> |
| `stdinOnce` _boolean_ | Whether the container runtime should close the stdin channel after it has been opened by<br />a single attach. When stdin is true the stdin stream will remain open across multiple attach<br />sessions. If stdinOnce is set to true, stdin is opened on Sidecar start, is empty until the<br />first client attaches to stdin, and then remains open and accepts data until the client disconnects,<br />at which time stdin is closed and remains closed until the Sidecar is restarted. If this<br />flag is false, a container processes that reads from stdin will never receive an EOF.<br />Default is false |  | Optional: \{\} <br /> |
| `tty` _boolean_ | Whether this Sidecar should allocate a TTY for itself, also requires 'stdin' to be true.<br />Default is false. |  | Optional: \{\} <br /> |
| `script` _string_ | Script is the contents of an executable file to execute.<br />If Script is not empty, the Step cannot have an Command or Args. |  | Optional: \{\} <br /> |
| `workspaces` _[WorkspaceUsage](#workspaceusage) array_ | This is an alpha field. You must set the "enable-api-fields" feature flag to "alpha"<br />for this field to be supported.<br />Workspaces is a list of workspaces from the Task that this Sidecar wants<br />exclusive access to. Adding a workspace to this list means that any<br />other Step or Sidecar that does not also request this Workspace will<br />not have access to it. |  | Optional: \{\} <br /> |
| `restartPolicy` _[ContainerRestartPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#containerrestartpolicy-v1-core)_ | RestartPolicy refers to kubernetes RestartPolicy. It can only be set for an<br />initContainer and must have it's policy set to "Always". It is currently<br />left optional to help support Kubernetes versions prior to 1.29 when this feature<br />was introduced. |  | Optional: \{\} <br /> |


#### SidecarState



SidecarState reports the results of running a sidecar in a Task.



_Appears in:_
- [TaskRunStatus](#taskrunstatus)
- [TaskRunStatusFields](#taskrunstatusfields)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `waiting` _[ContainerStateWaiting](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#containerstatewaiting-v1-core)_ | Details about a waiting container |  | Optional: \{\} <br /> |
| `running` _[ContainerStateRunning](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#containerstaterunning-v1-core)_ | Details about a running container |  | Optional: \{\} <br /> |
| `terminated` _[ContainerStateTerminated](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#containerstateterminated-v1-core)_ | Details about a terminated container |  | Optional: \{\} <br /> |
| `name` _string_ |  |  |  |
| `container` _string_ |  |  |  |
| `imageID` _string_ |  |  |  |


#### SkippedTask



SkippedTask is used to describe the Tasks that were skipped due to their When Expressions
evaluating to False. This is a struct because we are looking into including more details
about the When Expressions that caused this Task to be skipped.



_Appears in:_
- [PipelineRunStatus](#pipelinerunstatus)
- [PipelineRunStatusFields](#pipelinerunstatusfields)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the Pipeline Task name |  |  |
| `reason` _[SkippingReason](#skippingreason)_ | Reason is the cause of the PipelineTask being skipped. |  |  |
| `whenExpressions` _[WhenExpression](#whenexpression) array_ | WhenExpressions is the list of checks guarding the execution of the PipelineTask |  | Optional: \{\} <br /> |


#### SkippingReason

_Underlying type:_ _string_

SkippingReason explains why a PipelineTask was skipped.



_Appears in:_
- [SkippedTask](#skippedtask)

| Field | Description |
| --- | --- |
| `When Expressions evaluated to false` | WhenExpressionsSkip means the task was skipped due to at least one of its when expressions evaluating to false<br /> |
| `Parent Tasks were skipped` | ParentTasksSkip means the task was skipped because its parent was skipped<br /> |
| `PipelineRun was stopping` | StoppingSkip means the task was skipped because the pipeline run is stopping<br /> |
| `PipelineRun was gracefully cancelled` | GracefullyCancelledSkip means the task was skipped because the pipeline run has been gracefully cancelled<br /> |
| `PipelineRun was gracefully stopped` | GracefullyStoppedSkip means the task was skipped because the pipeline run has been gracefully stopped<br /> |
| `Results were missing` | MissingResultsSkip means the task was skipped because it's missing necessary results<br /> |
| `PipelineRun timeout has been reached` | PipelineTimedOutSkip means the task was skipped because the PipelineRun has passed its overall timeout.<br /> |
| `PipelineRun Tasks timeout has been reached` | TasksTimedOutSkip means the task was skipped because the PipelineRun has passed its Timeouts.Tasks.<br /> |
| `PipelineRun Finally timeout has been reached` | FinallyTimedOutSkip means the task was skipped because the PipelineRun has passed its Timeouts.Finally.<br /> |
| `Matrix Parameters have an empty array` | EmptyArrayInMatrixParams means the task was skipped because Matrix parameters contain empty array.<br /> |
| `None` | None means the task was not skipped<br /> |


#### Step



Step runs a subcomponent of a Task



_Appears in:_
- [EmbeddedTask](#embeddedtask)
- [TaskSpec](#taskspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the Step specified as a DNS_LABEL.<br />Each Step in a Task must have a unique name. |  |  |
| `displayName` _string_ | DisplayName is a user-facing name of the step that may be<br />used to populate a UI. |  | Optional: \{\} <br /> |
| `image` _string_ | Docker image name.<br />More info: https://kubernetes.io/docs/concepts/containers/images |  | Optional: \{\} <br /> |
| `command` _string array_ | Entrypoint array. Not executed within a shell.<br />The image's ENTRYPOINT is used if this is not provided.<br />Variable references $(VAR_NAME) are expanded using the container's environment. If a variable<br />cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced<br />to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)" will<br />produce the string literal "$(VAR_NAME)". Escaped references will never be expanded, regardless<br />of whether the variable exists or not. Cannot be updated.<br />More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell |  | Optional: \{\} <br /> |
| `args` _string array_ | Arguments to the entrypoint.<br />The image's CMD is used if this is not provided.<br />Variable references $(VAR_NAME) are expanded using the container's environment. If a variable<br />cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced<br />to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)" will<br />produce the string literal "$(VAR_NAME)". Escaped references will never be expanded, regardless<br />of whether the variable exists or not. Cannot be updated.<br />More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell |  | Optional: \{\} <br /> |
| `workingDir` _string_ | Step's working directory.<br />If not specified, the container runtime's default will be used, which<br />might be configured in the container image.<br />Cannot be updated. |  | Optional: \{\} <br /> |
| `envFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envfromsource-v1-core) array_ | List of sources to populate environment variables in the Step.<br />The keys defined within a source must be a C_IDENTIFIER. All invalid keys<br />will be reported as an event when the Step is starting. When a key exists in multiple<br />sources, the value associated with the last source will take precedence.<br />Values defined by an Env with a duplicate key will take precedence.<br />Cannot be updated. |  | Optional: \{\} <br /> |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envvar-v1-core) array_ | List of environment variables to set in the Step.<br />Cannot be updated. |  | Optional: \{\} <br /> |
| `computeResources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | ComputeResources required by this Step.<br />Cannot be updated.<br />More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/ |  | Optional: \{\} <br /> |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ | Volumes to mount into the Step's filesystem.<br />Cannot be updated. |  | Optional: \{\} <br /> |
| `volumeDevices` _[VolumeDevice](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumedevice-v1-core) array_ | volumeDevices is the list of block devices to be used by the Step. |  | Optional: \{\} <br /> |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#pullpolicy-v1-core)_ | Image pull policy.<br />One of Always, Never, IfNotPresent.<br />Defaults to Always if :latest tag is specified, or IfNotPresent otherwise.<br />Cannot be updated.<br />More info: https://kubernetes.io/docs/concepts/containers/images#updating-images |  | Optional: \{\} <br /> |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#securitycontext-v1-core)_ | SecurityContext defines the security options the Step should be run with.<br />If set, the fields of SecurityContext override the equivalent fields of PodSecurityContext.<br />More info: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/ |  | Optional: \{\} <br /> |
| `script` _string_ | Script is the contents of an executable file to execute.<br />If Script is not empty, the Step cannot have an Command and the Args will be passed to the Script. |  | Optional: \{\} <br /> |
| `timeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | Timeout is the time after which the step times out. Defaults to never.<br />Refer to Go's ParseDuration documentation for expected format: https://golang.org/pkg/time/#ParseDuration |  | Optional: \{\} <br /> |
| `workspaces` _[WorkspaceUsage](#workspaceusage) array_ | This is an alpha field. You must set the "enable-api-fields" feature flag to "alpha"<br />for this field to be supported.<br />Workspaces is a list of workspaces from the Task that this Step wants<br />exclusive access to. Adding a workspace to this list means that any<br />other Step or Sidecar that does not also request this Workspace will<br />not have access to it. |  | Optional: \{\} <br /> |
| `onError` _[OnErrorType](#onerrortype)_ | OnError defines the exiting behavior of a container on error<br />can be set to [ continue \| stopAndFail ] |  |  |
| `stdoutConfig` _[StepOutputConfig](#stepoutputconfig)_ | Stores configuration for the stdout stream of the step. |  | Optional: \{\} <br /> |
| `stderrConfig` _[StepOutputConfig](#stepoutputconfig)_ | Stores configuration for the stderr stream of the step. |  | Optional: \{\} <br /> |
| `ref` _[Ref](#ref)_ | Contains the reference to an existing StepAction. |  | Optional: \{\} <br /> |
| `params` _[Params](#params)_ | Params declares parameters passed to this step action. |  | Optional: \{\} <br /> |
| `results` _[StepResult](#stepresult) array_ | Results declares StepResults produced by the Step.<br />It can be used in an inlined Step when used to store Results to $(step.results.resultName.path).<br />It cannot be used when referencing StepActions using [v1.Step.Ref].<br />The Results declared by the StepActions will be stored here instead. |  | Optional: \{\} <br /> |
| `when` _[StepWhenExpressions](#stepwhenexpressions)_ | When is a list of when expressions that need to be true for the task to run |  | Optional: \{\} <br /> |


#### StepOutputConfig



StepOutputConfig stores configuration for a step output stream.



_Appears in:_
- [Step](#step)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `path` _string_ | Path to duplicate stdout stream to on container's local filesystem. |  | Optional: \{\} <br /> |


#### StepResult



StepResult used to describe the Results of a Step.



_Appears in:_
- [Step](#step)
- [Step](#step)
- [StepActionSpec](#stepactionspec)
- [StepActionSpec](#stepactionspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name the given name |  |  |
| `type` _[ResultsType](#resultstype)_ | The possible types are 'string', 'array', and 'object', with 'string' as the default. |  | Optional: \{\} <br /> |
| `properties` _object (keys:string, values:[PropertySpec](#propertyspec))_ | Properties is the JSON Schema properties to support key-value pairs results. |  | Optional: \{\} <br /> |
| `description` _string_ | Description is a human-readable description of the result |  | Optional: \{\} <br /> |


#### StepState



StepState reports the results of running a step in a Task.



_Appears in:_
- [TaskRunStatus](#taskrunstatus)
- [TaskRunStatusFields](#taskrunstatusfields)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `waiting` _[ContainerStateWaiting](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#containerstatewaiting-v1-core)_ | Details about a waiting container |  | Optional: \{\} <br /> |
| `running` _[ContainerStateRunning](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#containerstaterunning-v1-core)_ | Details about a running container |  | Optional: \{\} <br /> |
| `terminated` _[ContainerStateTerminated](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#containerstateterminated-v1-core)_ | Details about a terminated container |  | Optional: \{\} <br /> |
| `name` _string_ |  |  |  |
| `container` _string_ |  |  |  |
| `imageID` _string_ |  |  |  |
| `results` _[TaskRunStepResult](#taskrunstepresult) array_ |  |  |  |
| `provenance` _[Provenance](#provenance)_ |  |  |  |
| `terminationReason` _string_ |  |  |  |
| `inputs` _[TaskRunStepArtifact](#taskrunstepartifact) array_ |  |  |  |
| `outputs` _[TaskRunStepArtifact](#taskrunstepartifact) array_ |  |  |  |


#### StepTemplate



StepTemplate is a template for a Step



_Appears in:_
- [EmbeddedTask](#embeddedtask)
- [TaskSpec](#taskspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ | Image reference name.<br />More info: https://kubernetes.io/docs/concepts/containers/images |  | Optional: \{\} <br /> |
| `command` _string array_ | Entrypoint array. Not executed within a shell.<br />The image's ENTRYPOINT is used if this is not provided.<br />Variable references $(VAR_NAME) are expanded using the Step's environment. If a variable<br />cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced<br />to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)" will<br />produce the string literal "$(VAR_NAME)". Escaped references will never be expanded, regardless<br />of whether the variable exists or not. Cannot be updated.<br />More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell |  | Optional: \{\} <br /> |
| `args` _string array_ | Arguments to the entrypoint.<br />The image's CMD is used if this is not provided.<br />Variable references $(VAR_NAME) are expanded using the Step's environment. If a variable<br />cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced<br />to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)" will<br />produce the string literal "$(VAR_NAME)". Escaped references will never be expanded, regardless<br />of whether the variable exists or not. Cannot be updated.<br />More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell |  | Optional: \{\} <br /> |
| `workingDir` _string_ | Step's working directory.<br />If not specified, the container runtime's default will be used, which<br />might be configured in the container image.<br />Cannot be updated. |  | Optional: \{\} <br /> |
| `envFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envfromsource-v1-core) array_ | List of sources to populate environment variables in the Step.<br />The keys defined within a source must be a C_IDENTIFIER. All invalid keys<br />will be reported as an event when the Step is starting. When a key exists in multiple<br />sources, the value associated with the last source will take precedence.<br />Values defined by an Env with a duplicate key will take precedence.<br />Cannot be updated. |  | Optional: \{\} <br /> |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envvar-v1-core) array_ | List of environment variables to set in the Step.<br />Cannot be updated. |  | Optional: \{\} <br /> |
| `computeResources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | ComputeResources required by this Step.<br />Cannot be updated.<br />More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/ |  | Optional: \{\} <br /> |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ | Volumes to mount into the Step's filesystem.<br />Cannot be updated. |  | Optional: \{\} <br /> |
| `volumeDevices` _[VolumeDevice](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumedevice-v1-core) array_ | volumeDevices is the list of block devices to be used by the Step. |  | Optional: \{\} <br /> |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#pullpolicy-v1-core)_ | Image pull policy.<br />One of Always, Never, IfNotPresent.<br />Defaults to Always if :latest tag is specified, or IfNotPresent otherwise.<br />Cannot be updated.<br />More info: https://kubernetes.io/docs/concepts/containers/images#updating-images |  | Optional: \{\} <br /> |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#securitycontext-v1-core)_ | SecurityContext defines the security options the Step should be run with.<br />If set, the fields of SecurityContext override the equivalent fields of PodSecurityContext.<br />More info: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/ |  | Optional: \{\} <br /> |




#### Task



Task represents a collection of sequential steps that are run as part of a
Pipeline using a set of inputs and producing a set of outputs. Tasks execute
when TaskRuns are created that provide the input parameters and resources and
output resources the Task requires.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `tekton.dev/v1` | | |
| `kind` _string_ | `Task` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[TaskSpec](#taskspec)_ | Spec holds the desired state of the Task from the client |  | Optional: \{\} <br /> |


#### TaskBreakpoints



TaskBreakpoints defines the breakpoint config for a particular Task



_Appears in:_
- [TaskRunDebug](#taskrundebug)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `onFailure` _string_ | if enabled, pause TaskRun on failure of a step<br />failed step will not exit |  | Optional: \{\} <br /> |
| `beforeSteps` _string array_ |  |  | Optional: \{\} <br /> |


#### TaskKind

_Underlying type:_ _string_

TaskKind defines the type of Task used by the pipeline.



_Appears in:_
- [TaskRef](#taskref)

| Field | Description |
| --- | --- |
| `Task` | NamespacedTaskKind indicates that the task type has a namespaced scope.<br /> |


#### TaskRef



TaskRef can be used to refer to a specific instance of a task.



_Appears in:_
- [PipelineTask](#pipelinetask)
- [TaskRunSpec](#taskrunspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the referent; More info: http://kubernetes.io/docs/user-guide/identifiers#names |  |  |
| `kind` _[TaskKind](#taskkind)_ | TaskKind indicates the Kind of the Task:<br />1. Namespaced Task when Kind is set to "Task". If Kind is "", it defaults to "Task".<br />2. Custom Task when Kind is non-empty and APIVersion is non-empty |  |  |
| `apiVersion` _string_ | API version of the referent<br />Note: A Task with non-empty APIVersion and Kind is considered a Custom Task |  | Optional: \{\} <br /> |
| `ResolverRef` _[ResolverRef](#resolverref)_ | ResolverRef allows referencing a Task in a remote location<br />like a git repo. This field is only supported when the alpha<br />feature gate is enabled. |  | Optional: \{\} <br /> |


#### TaskResult



TaskResult used to describe the results of a task



_Appears in:_
- [EmbeddedTask](#embeddedtask)
- [TaskSpec](#taskspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name the given name |  |  |
| `type` _[ResultsType](#resultstype)_ | Type is the user-specified type of the result. The possible type<br />is currently "string" and will support "array" in following work. |  | Optional: \{\} <br /> |
| `properties` _object (keys:string, values:[PropertySpec](#propertyspec))_ | Properties is the JSON Schema properties to support key-value pairs results. |  | Optional: \{\} <br /> |
| `description` _string_ | Description is a human-readable description of the result |  | Optional: \{\} <br /> |
| `value` _[ResultValue](#resultvalue)_ | Value the expression used to retrieve the value of the result from an underlying Step. |  | Schemaless: \{\} <br />Optional: \{\} <br /> |


#### TaskRun



TaskRun represents a single execution of a Task. TaskRuns are how the steps
specified in a Task are executed; they specify the parameters and resources
used to run the steps in a Task.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `tekton.dev/v1` | | |
| `kind` _string_ | `TaskRun` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[TaskRunSpec](#taskrunspec)_ |  |  | Optional: \{\} <br /> |
| `status` _[TaskRunStatus](#taskrunstatus)_ |  |  | Optional: \{\} <br /> |


#### TaskRunDebug



TaskRunDebug defines the breakpoint config for a particular TaskRun



_Appears in:_
- [TaskRunSpec](#taskrunspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `breakpoints` _[TaskBreakpoints](#taskbreakpoints)_ |  |  | Optional: \{\} <br /> |






#### TaskRunResult



TaskRunResult used to describe the results of a task



_Appears in:_
- [TaskRunStatus](#taskrunstatus)
- [TaskRunStatusFields](#taskrunstatusfields)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name the given name |  |  |
| `type` _[ResultsType](#resultstype)_ | Type is the user-specified type of the result. The possible type<br />is currently "string" and will support "array" in following work. |  | Optional: \{\} <br /> |
| `value` _[ResultValue](#resultvalue)_ | Value the given value of the result |  | Schemaless: \{\} <br /> |


#### TaskRunSidecarSpec



TaskRunSidecarSpec is used to override the values of a Sidecar in the corresponding Task.



_Appears in:_
- [PipelineTaskRunSpec](#pipelinetaskrunspec)
- [TaskRunSpec](#taskrunspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | The name of the Sidecar to override. |  |  |
| `computeResources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | The resource requirements to apply to the Sidecar. |  |  |


#### TaskRunSpec



TaskRunSpec defines the desired state of TaskRun



_Appears in:_
- [TaskRun](#taskrun)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `debug` _[TaskRunDebug](#taskrundebug)_ |  |  | Optional: \{\} <br /> |
| `params` _[Params](#params)_ |  |  | Optional: \{\} <br /> |
| `serviceAccountName` _string_ |  |  | Optional: \{\} <br /> |
| `taskRef` _[TaskRef](#taskref)_ | no more than one of the TaskRef and TaskSpec may be specified. |  | Optional: \{\} <br /> |
| `taskSpec` _[TaskSpec](#taskspec)_ | Specifying TaskSpec can be disabled by setting<br />`disable-inline-spec` feature flag.<br />See Task.spec (API version: tekton.dev/v1) |  | Schemaless: \{\} <br />Optional: \{\} <br /> |
| `status` _[TaskRunSpecStatus](#taskrunspecstatus)_ | Used for cancelling a TaskRun (and maybe more later on) |  | Optional: \{\} <br /> |
| `statusMessage` _[TaskRunSpecStatusMessage](#taskrunspecstatusmessage)_ | Status message for cancellation. |  | Optional: \{\} <br /> |
| `retries` _integer_ | Retries represents how many times this TaskRun should be retried in the event of task failure. |  | Optional: \{\} <br /> |
| `timeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | Time after which one retry attempt times out. Defaults to 1 hour.<br />Refer Go's ParseDuration documentation for expected format: https://golang.org/pkg/time/#ParseDuration |  | Optional: \{\} <br /> |
| `podTemplate` _[PodTemplate](#podtemplate)_ | PodTemplate holds pod specific configuration |  |  |
| `workspaces` _[WorkspaceBinding](#workspacebinding) array_ | Workspaces is a list of WorkspaceBindings from volumes to workspaces. |  | Optional: \{\} <br /> |
| `stepSpecs` _[TaskRunStepSpec](#taskrunstepspec) array_ | Specs to apply to Steps in this TaskRun.<br />If a field is specified in both a Step and a StepSpec,<br />the value from the StepSpec will be used.<br />This field is only supported when the alpha feature gate is enabled. |  | Optional: \{\} <br /> |
| `sidecarSpecs` _[TaskRunSidecarSpec](#taskrunsidecarspec) array_ | Specs to apply to Sidecars in this TaskRun.<br />If a field is specified in both a Sidecar and a SidecarSpec,<br />the value from the SidecarSpec will be used.<br />This field is only supported when the alpha feature gate is enabled. |  | Optional: \{\} <br /> |
| `computeResources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Compute resources to use for this TaskRun |  |  |
| `managedBy` _string_ | ManagedBy indicates which controller is responsible for reconciling<br />this resource. If unset or set to "tekton.dev/pipeline", the default<br />Tekton controller will manage this resource.<br />This field is immutable. |  | Optional: \{\} <br /> |


#### TaskRunSpecStatus

_Underlying type:_ _string_

TaskRunSpecStatus defines the TaskRun spec status the user can provide



_Appears in:_
- [TaskRunSpec](#taskrunspec)



#### TaskRunSpecStatusMessage

_Underlying type:_ _string_

TaskRunSpecStatusMessage defines human readable status messages for the TaskRun.



_Appears in:_
- [TaskRunSpec](#taskrunspec)

| Field | Description |
| --- | --- |
| `TaskRun cancelled as the PipelineRun it belongs to has been cancelled.` | TaskRunCancelledByPipelineMsg indicates that the PipelineRun of which this<br />TaskRun was a part of has been cancelled.<br /> |
| `TaskRun cancelled as the PipelineRun it belongs to has timed out.` | TaskRunCancelledByPipelineTimeoutMsg indicates that the TaskRun was cancelled because the PipelineRun running it timed out.<br /> |


#### TaskRunStatus



TaskRunStatus defines the observed state of TaskRun



_Appears in:_
- [PipelineRunTaskRunStatus](#pipelineruntaskrunstatus)
- [RetriesStatus](#retriesstatus)
- [TaskRun](#taskrun)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the 'Generation' of the Service that<br />was last processed by the controller. |  | Optional: \{\} <br /> |
| `conditions` _[Conditions](#conditions)_ | Conditions the latest available observations of a resource's current state. |  | Optional: \{\} <br /> |
| `annotations` _object (keys:string, values:string)_ | Annotations is additional Status fields for the Resource to save some<br />additional State as well as convey more information to the user. This is<br />roughly akin to Annotations on any k8s resource, just the reconciler conveying<br />richer information outwards. |  |  |
| `podName` _string_ | PodName is the name of the pod responsible for executing this task's steps. |  |  |
| `startTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#time-v1-meta)_ | StartTime is the time the build is actually started. |  |  |
| `completionTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#time-v1-meta)_ | CompletionTime is the time the build completed. |  |  |
| `steps` _[StepState](#stepstate) array_ | Steps describes the state of each build step container. |  | Optional: \{\} <br /> |
| `retriesStatus` _[RetriesStatus](#retriesstatus)_ | RetriesStatus contains the history of TaskRunStatus in case of a retry in order to keep record of failures.<br />All TaskRunStatus stored in RetriesStatus will have no date within the RetriesStatus as is redundant. |  | Schemaless: \{\} <br />Optional: \{\} <br /> |
| `results` _[TaskRunResult](#taskrunresult) array_ | Results are the list of results written out by the task's containers |  | Optional: \{\} <br /> |
| `artifacts` _[Artifacts](#artifacts)_ | Artifacts are the list of artifacts written out by the task's containers |  | Optional: \{\} <br /> |
| `sidecars` _[SidecarState](#sidecarstate) array_ | The list has one entry per sidecar in the manifest. Each entry is<br />represents the imageid of the corresponding sidecar. |  |  |
| `taskSpec` _[TaskSpec](#taskspec)_ | TaskSpec contains the Spec from the dereferenced Task definition used to instantiate this TaskRun. |  |  |
| `provenance` _[Provenance](#provenance)_ | Provenance contains some key authenticated metadata about how a software artifact was built (what sources, what inputs/outputs, etc.). |  | Optional: \{\} <br /> |
| `spanContext` _object (keys:string, values:string)_ | SpanContext contains tracing span context fields |  |  |


#### TaskRunStatusFields



TaskRunStatusFields holds the fields of TaskRun's status.  This is defined
separately and inlined so that other types can readily consume these fields
via duck typing.



_Appears in:_
- [TaskRunStatus](#taskrunstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podName` _string_ | PodName is the name of the pod responsible for executing this task's steps. |  |  |
| `startTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#time-v1-meta)_ | StartTime is the time the build is actually started. |  |  |
| `completionTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#time-v1-meta)_ | CompletionTime is the time the build completed. |  |  |
| `steps` _[StepState](#stepstate) array_ | Steps describes the state of each build step container. |  | Optional: \{\} <br /> |
| `retriesStatus` _[RetriesStatus](#retriesstatus)_ | RetriesStatus contains the history of TaskRunStatus in case of a retry in order to keep record of failures.<br />All TaskRunStatus stored in RetriesStatus will have no date within the RetriesStatus as is redundant. |  | Schemaless: \{\} <br />Optional: \{\} <br /> |
| `results` _[TaskRunResult](#taskrunresult) array_ | Results are the list of results written out by the task's containers |  | Optional: \{\} <br /> |
| `artifacts` _[Artifacts](#artifacts)_ | Artifacts are the list of artifacts written out by the task's containers |  | Optional: \{\} <br /> |
| `sidecars` _[SidecarState](#sidecarstate) array_ | The list has one entry per sidecar in the manifest. Each entry is<br />represents the imageid of the corresponding sidecar. |  |  |
| `taskSpec` _[TaskSpec](#taskspec)_ | TaskSpec contains the Spec from the dereferenced Task definition used to instantiate this TaskRun. |  |  |
| `provenance` _[Provenance](#provenance)_ | Provenance contains some key authenticated metadata about how a software artifact was built (what sources, what inputs/outputs, etc.). |  | Optional: \{\} <br /> |
| `spanContext` _object (keys:string, values:string)_ | SpanContext contains tracing span context fields |  |  |






#### TaskRunStepSpec



TaskRunStepSpec is used to override the values of a Step in the corresponding Task.



_Appears in:_
- [PipelineTaskRunSpec](#pipelinetaskrunspec)
- [TaskRunSpec](#taskrunspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | The name of the Step to override. |  |  |
| `computeResources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | The resource requirements to apply to the Step. |  |  |


#### TaskSpec



TaskSpec defines the desired state of Task.



_Appears in:_
- [EmbeddedTask](#embeddedtask)
- [Task](#task)
- [TaskRunSpec](#taskrunspec)
- [TaskRunStatus](#taskrunstatus)
- [TaskRunStatusFields](#taskrunstatusfields)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `params` _[ParamSpecs](#paramspecs)_ | Params is a list of input parameters required to run the task. Params<br />must be supplied as inputs in TaskRuns unless they declare a default<br />value. |  | Optional: \{\} <br /> |
| `displayName` _string_ | DisplayName is a user-facing name of the task that may be<br />used to populate a UI. |  | Optional: \{\} <br /> |
| `description` _string_ | Description is a user-facing description of the task that may be<br />used to populate a UI. |  | Optional: \{\} <br /> |
| `steps` _[Step](#step) array_ | Steps are the steps of the build; each step is run sequentially with the<br />source mounted into /workspace. |  |  |
| `volumes` _[Volumes](#volumes)_ | Volumes is a collection of volumes that are available to mount into the<br />steps of the build.<br />See Pod.spec.volumes (API version: v1) |  | Schemaless: \{\} <br /> |
| `stepTemplate` _[StepTemplate](#steptemplate)_ | StepTemplate can be used as the basis for all step containers within the<br />Task, so that the steps inherit settings on the base container. |  |  |
| `sidecars` _[Sidecar](#sidecar) array_ | Sidecars are run alongside the Task's step containers. They begin before<br />the steps start and end after the steps complete. |  |  |
| `workspaces` _[WorkspaceDeclaration](#workspacedeclaration) array_ | Workspaces are the volumes that this Task requires. |  |  |
| `results` _[TaskResult](#taskresult) array_ | Results are values that this Task can output |  |  |


#### TimeoutFields



TimeoutFields allows granular specification of pipeline, task, and finally timeouts



_Appears in:_
- [PipelineRunSpec](#pipelinerunspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `pipeline` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | Pipeline sets the maximum allowed duration for execution of the entire pipeline. The sum of individual timeouts for tasks and finally must not exceed this value. |  |  |
| `tasks` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | Tasks sets the maximum allowed duration of this pipeline's tasks |  |  |
| `finally` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | Finally sets the maximum allowed duration of this pipeline's finally |  |  |


#### Volumes

_Underlying type:_ _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volume-v1-core)_





_Appears in:_
- [EmbeddedTask](#embeddedtask)
- [TaskSpec](#taskspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | name of the volume.<br />Must be a DNS_LABEL and unique within the pod.<br />More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names |  |  |
| `hostPath` _[HostPathVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#hostpathvolumesource-v1-core)_ | hostPath represents a pre-existing file or directory on the host<br />machine that is directly exposed to the container. This is generally<br />used for system agents or other privileged things that are allowed<br />to see the host machine. Most containers will NOT need this.<br />More info: https://kubernetes.io/docs/concepts/storage/volumes#hostpath |  | Optional: \{\} <br /> |
| `emptyDir` _[EmptyDirVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#emptydirvolumesource-v1-core)_ | emptyDir represents a temporary directory that shares a pod's lifetime.<br />More info: https://kubernetes.io/docs/concepts/storage/volumes#emptydir |  | Optional: \{\} <br /> |
| `gcePersistentDisk` _[GCEPersistentDiskVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#gcepersistentdiskvolumesource-v1-core)_ | gcePersistentDisk represents a GCE Disk resource that is attached to a<br />kubelet's host machine and then exposed to the pod.<br />Deprecated: GCEPersistentDisk is deprecated. All operations for the in-tree<br />gcePersistentDisk type are redirected to the pd.csi.storage.gke.io CSI driver.<br />More info: https://kubernetes.io/docs/concepts/storage/volumes#gcepersistentdisk |  | Optional: \{\} <br /> |
| `awsElasticBlockStore` _[AWSElasticBlockStoreVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#awselasticblockstorevolumesource-v1-core)_ | awsElasticBlockStore represents an AWS Disk resource that is attached to a<br />kubelet's host machine and then exposed to the pod.<br />Deprecated: AWSElasticBlockStore is deprecated. All operations for the in-tree<br />awsElasticBlockStore type are redirected to the ebs.csi.aws.com CSI driver.<br />More info: https://kubernetes.io/docs/concepts/storage/volumes#awselasticblockstore |  | Optional: \{\} <br /> |
| `gitRepo` _[GitRepoVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#gitrepovolumesource-v1-core)_ | gitRepo represents a git repository at a particular revision.<br />Deprecated: GitRepo is deprecated. To provision a container with a git repo, mount an<br />EmptyDir into an InitContainer that clones the repo using git, then mount the EmptyDir<br />into the Pod's container. |  | Optional: \{\} <br /> |
| `secret` _[SecretVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#secretvolumesource-v1-core)_ | secret represents a secret that should populate this volume.<br />More info: https://kubernetes.io/docs/concepts/storage/volumes#secret |  | Optional: \{\} <br /> |
| `nfs` _[NFSVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#nfsvolumesource-v1-core)_ | nfs represents an NFS mount on the host that shares a pod's lifetime<br />More info: https://kubernetes.io/docs/concepts/storage/volumes#nfs |  | Optional: \{\} <br /> |
| `iscsi` _[ISCSIVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#iscsivolumesource-v1-core)_ | iscsi represents an ISCSI Disk resource that is attached to a<br />kubelet's host machine and then exposed to the pod.<br />More info: https://kubernetes.io/docs/concepts/storage/volumes/#iscsi |  | Optional: \{\} <br /> |
| `glusterfs` _[GlusterfsVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#glusterfsvolumesource-v1-core)_ | glusterfs represents a Glusterfs mount on the host that shares a pod's lifetime.<br />Deprecated: Glusterfs is deprecated and the in-tree glusterfs type is no longer supported. |  | Optional: \{\} <br /> |
| `persistentVolumeClaim` _[PersistentVolumeClaimVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#persistentvolumeclaimvolumesource-v1-core)_ | persistentVolumeClaimVolumeSource represents a reference to a<br />PersistentVolumeClaim in the same namespace.<br />More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#persistentvolumeclaims |  | Optional: \{\} <br /> |
| `rbd` _[RBDVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#rbdvolumesource-v1-core)_ | rbd represents a Rados Block Device mount on the host that shares a pod's lifetime.<br />Deprecated: RBD is deprecated and the in-tree rbd type is no longer supported. |  | Optional: \{\} <br /> |
| `flexVolume` _[FlexVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#flexvolumesource-v1-core)_ | flexVolume represents a generic volume resource that is<br />provisioned/attached using an exec based plugin.<br />Deprecated: FlexVolume is deprecated. Consider using a CSIDriver instead. |  | Optional: \{\} <br /> |
| `cinder` _[CinderVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#cindervolumesource-v1-core)_ | cinder represents a cinder volume attached and mounted on kubelets host machine.<br />Deprecated: Cinder is deprecated. All operations for the in-tree cinder type<br />are redirected to the cinder.csi.openstack.org CSI driver.<br />More info: https://examples.k8s.io/mysql-cinder-pd/README.md |  | Optional: \{\} <br /> |
| `cephfs` _[CephFSVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#cephfsvolumesource-v1-core)_ | cephFS represents a Ceph FS mount on the host that shares a pod's lifetime.<br />Deprecated: CephFS is deprecated and the in-tree cephfs type is no longer supported. |  | Optional: \{\} <br /> |
| `flocker` _[FlockerVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#flockervolumesource-v1-core)_ | flocker represents a Flocker volume attached to a kubelet's host machine. This depends on the Flocker control service being running.<br />Deprecated: Flocker is deprecated and the in-tree flocker type is no longer supported. |  | Optional: \{\} <br /> |
| `downwardAPI` _[DownwardAPIVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#downwardapivolumesource-v1-core)_ | downwardAPI represents downward API about the pod that should populate this volume |  | Optional: \{\} <br /> |
| `fc` _[FCVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#fcvolumesource-v1-core)_ | fc represents a Fibre Channel resource that is attached to a kubelet's host machine and then exposed to the pod. |  | Optional: \{\} <br /> |
| `azureFile` _[AzureFileVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#azurefilevolumesource-v1-core)_ | azureFile represents an Azure File Service mount on the host and bind mount to the pod.<br />Deprecated: AzureFile is deprecated. All operations for the in-tree azureFile type<br />are redirected to the file.csi.azure.com CSI driver. |  | Optional: \{\} <br /> |
| `configMap` _[ConfigMapVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#configmapvolumesource-v1-core)_ | configMap represents a configMap that should populate this volume |  | Optional: \{\} <br /> |
| `vsphereVolume` _[VsphereVirtualDiskVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#vspherevirtualdiskvolumesource-v1-core)_ | vsphereVolume represents a vSphere volume attached and mounted on kubelets host machine.<br />Deprecated: VsphereVolume is deprecated. All operations for the in-tree vsphereVolume type<br />are redirected to the csi.vsphere.vmware.com CSI driver. |  | Optional: \{\} <br /> |
| `quobyte` _[QuobyteVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#quobytevolumesource-v1-core)_ | quobyte represents a Quobyte mount on the host that shares a pod's lifetime.<br />Deprecated: Quobyte is deprecated and the in-tree quobyte type is no longer supported. |  | Optional: \{\} <br /> |
| `azureDisk` _[AzureDiskVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#azurediskvolumesource-v1-core)_ | azureDisk represents an Azure Data Disk mount on the host and bind mount to the pod.<br />Deprecated: AzureDisk is deprecated. All operations for the in-tree azureDisk type<br />are redirected to the disk.csi.azure.com CSI driver. |  | Optional: \{\} <br /> |
| `photonPersistentDisk` _[PhotonPersistentDiskVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#photonpersistentdiskvolumesource-v1-core)_ | photonPersistentDisk represents a PhotonController persistent disk attached and mounted on kubelets host machine.<br />Deprecated: PhotonPersistentDisk is deprecated and the in-tree photonPersistentDisk type is no longer supported. |  |  |
| `projected` _[ProjectedVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#projectedvolumesource-v1-core)_ | projected items for all in one resources secrets, configmaps, and downward API |  |  |
| `portworxVolume` _[PortworxVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#portworxvolumesource-v1-core)_ | portworxVolume represents a portworx volume attached and mounted on kubelets host machine.<br />Deprecated: PortworxVolume is deprecated. All operations for the in-tree portworxVolume type<br />are redirected to the pxd.portworx.com CSI driver when the CSIMigrationPortworx feature-gate<br />is on. |  | Optional: \{\} <br /> |
| `scaleIO` _[ScaleIOVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#scaleiovolumesource-v1-core)_ | scaleIO represents a ScaleIO persistent volume attached and mounted on Kubernetes nodes.<br />Deprecated: ScaleIO is deprecated and the in-tree scaleIO type is no longer supported. |  | Optional: \{\} <br /> |
| `storageos` _[StorageOSVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#storageosvolumesource-v1-core)_ | storageOS represents a StorageOS volume attached and mounted on Kubernetes nodes.<br />Deprecated: StorageOS is deprecated and the in-tree storageos type is no longer supported. |  | Optional: \{\} <br /> |
| `csi` _[CSIVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#csivolumesource-v1-core)_ | csi (Container Storage Interface) represents ephemeral storage that is handled by certain external CSI drivers. |  | Optional: \{\} <br /> |
| `ephemeral` _[EphemeralVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#ephemeralvolumesource-v1-core)_ | ephemeral represents a volume that is handled by a cluster storage driver.<br />The volume's lifecycle is tied to the pod that defines it - it will be created before the pod starts,<br />and deleted when the pod is removed.<br />Use this if:<br />a) the volume is only needed while the pod runs,<br />b) features of normal volumes like restoring from snapshot or capacity<br />   tracking are needed,<br />c) the storage driver is specified through a storage class, and<br />d) the storage driver supports dynamic volume provisioning through<br />   a PersistentVolumeClaim (see EphemeralVolumeSource for more<br />   information on the connection between this volume type<br />   and PersistentVolumeClaim).<br />Use PersistentVolumeClaim or one of the vendor-specific<br />APIs for volumes that persist for longer than the lifecycle<br />of an individual pod.<br />Use CSI for light-weight local ephemeral volumes if the CSI driver is meant to<br />be used that way - see the documentation of the driver for<br />more information.<br />A pod can use both types of ephemeral volumes and<br />persistent volumes at the same time. |  | Optional: \{\} <br /> |
| `image` _[ImageVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#imagevolumesource-v1-core)_ | image represents an OCI object (a container image or artifact) pulled and mounted on the kubelet's host machine.<br />The volume is resolved at pod startup depending on which PullPolicy value is provided:<br />- Always: the kubelet always attempts to pull the reference. Container creation will fail If the pull fails.<br />- Never: the kubelet never pulls the reference and only uses a local image or artifact. Container creation will fail if the reference isn't present.<br />- IfNotPresent: the kubelet pulls if the reference isn't already present on disk. Container creation will fail if the reference isn't present and the pull fails.<br />The volume gets re-resolved if the pod gets deleted and recreated, which means that new remote content will become available on pod recreation.<br />A failure to resolve or pull the image during pod startup will block containers from starting and may add significant latency. Failures will be retried using normal volume backoff and will be reported on the pod reason and message.<br />The types of objects that may be mounted by this volume are defined by the container runtime implementation on a host machine and at minimum must include all valid types supported by the container image field.<br />The OCI object gets mounted in a single directory (spec.containers[*].volumeMounts.mountPath) by merging the manifest layers in the same way as for container images.<br />The volume will be mounted read-only (ro) and non-executable files (noexec).<br />Sub path mounts for containers are not supported (spec.containers[*].volumeMounts.subpath) before 1.33.<br />The field spec.securityContext.fsGroupChangePolicy has no effect on this volume type. |  | Optional: \{\} <br /> |


#### WhenExpression



WhenExpression allows a PipelineTask to declare expressions to be evaluated before the Task is run
to determine whether the Task should be executed or skipped



_Appears in:_
- [ChildStatusReference](#childstatusreference)
- [PipelineRunRunStatus](#pipelinerunrunstatus)
- [PipelineRunTaskRunStatus](#pipelineruntaskrunstatus)
- [SkippedTask](#skippedtask)
- [WhenExpressions](#whenexpressions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `input` _string_ | Input is the string for guard checking which can be a static input or an output from a parent Task |  |  |
| `operator` _[Operator](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#operator-selection-pkg)_ | Operator that represents an Input's relationship to the values |  |  |
| `values` _string array_ | Values is an array of strings, which is compared against the input, for guard checking<br />It must be non-empty |  |  |
| `cel` _string_ | CEL is a string of Common Language Expression, which can be used to conditionally execute<br />the task based on the result of the expression evaluation<br />More info about CEL syntax: https://github.com/google/cel-spec/blob/master/doc/langdef.md |  | Optional: \{\} <br /> |


#### WhenExpressions

_Underlying type:_ _[WhenExpression](#whenexpression)_

WhenExpressions are used to specify whether a Task should be executed or skipped
All of them need to evaluate to True for a guarded Task to be executed.



_Appears in:_
- [PipelineTask](#pipelinetask)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `input` _string_ | Input is the string for guard checking which can be a static input or an output from a parent Task |  |  |
| `operator` _[Operator](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#operator-selection-pkg)_ | Operator that represents an Input's relationship to the values |  |  |
| `values` _string array_ | Values is an array of strings, which is compared against the input, for guard checking<br />It must be non-empty |  |  |
| `cel` _string_ | CEL is a string of Common Language Expression, which can be used to conditionally execute<br />the task based on the result of the expression evaluation<br />More info about CEL syntax: https://github.com/google/cel-spec/blob/master/doc/langdef.md |  | Optional: \{\} <br /> |


#### WorkspaceBinding



WorkspaceBinding maps a Task's declared workspace to a Volume.



_Appears in:_
- [PipelineRunSpec](#pipelinerunspec)
- [TaskRunSpec](#taskrunspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the workspace populated by the volume. |  |  |
| `subPath` _string_ | SubPath is optionally a directory on the volume which should be used<br />for this binding (i.e. the volume will be mounted at this sub directory). |  | Optional: \{\} <br /> |
| `volumeClaimTemplate` _[PersistentVolumeClaim](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#persistentvolumeclaim-v1-core)_ | VolumeClaimTemplate is a template for a claim that will be created in the same namespace.<br />The PipelineRun controller is responsible for creating a unique claim for each instance of PipelineRun.<br />See PersistentVolumeClaim (API version: v1) |  | Schemaless: \{\} <br />Optional: \{\} <br /> |
| `persistentVolumeClaim` _[PersistentVolumeClaimVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#persistentvolumeclaimvolumesource-v1-core)_ | PersistentVolumeClaimVolumeSource represents a reference to a<br />PersistentVolumeClaim in the same namespace. Either this OR EmptyDir can be used. |  | Optional: \{\} <br /> |
| `emptyDir` _[EmptyDirVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#emptydirvolumesource-v1-core)_ | EmptyDir represents a temporary directory that shares a Task's lifetime.<br />More info: https://kubernetes.io/docs/concepts/storage/volumes#emptydir<br />Either this OR PersistentVolumeClaim can be used. |  | Optional: \{\} <br /> |
| `configMap` _[ConfigMapVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#configmapvolumesource-v1-core)_ | ConfigMap represents a configMap that should populate this workspace. |  | Optional: \{\} <br /> |
| `secret` _[SecretVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#secretvolumesource-v1-core)_ | Secret represents a secret that should populate this workspace. |  | Optional: \{\} <br /> |
| `projected` _[ProjectedVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#projectedvolumesource-v1-core)_ | Projected represents a projected volume that should populate this workspace. |  | Optional: \{\} <br /> |
| `csi` _[CSIVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#csivolumesource-v1-core)_ | CSI (Container Storage Interface) represents ephemeral storage that is handled by certain external CSI drivers. |  | Optional: \{\} <br /> |


#### WorkspaceDeclaration



WorkspaceDeclaration is a declaration of a volume that a Task requires.



_Appears in:_
- [EmbeddedTask](#embeddedtask)
- [TaskSpec](#taskspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name by which you can bind the volume at runtime. |  |  |
| `description` _string_ | Description is an optional human readable description of this volume. |  | Optional: \{\} <br /> |
| `mountPath` _string_ | MountPath overrides the directory that the volume will be made available at. |  | Optional: \{\} <br /> |
| `readOnly` _boolean_ | ReadOnly dictates whether a mounted volume is writable. By default this<br />field is false and so mounted volumes are writable. |  |  |
| `optional` _boolean_ | Optional marks a Workspace as not being required in TaskRuns. By default<br />this field is false and so declared workspaces are required. |  |  |




#### WorkspacePipelineTaskBinding



WorkspacePipelineTaskBinding describes how a workspace passed into the pipeline should be
mapped to a task's declared workspace.



_Appears in:_
- [PipelineTask](#pipelinetask)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the workspace as declared by the task |  |  |
| `workspace` _string_ | Workspace is the name of the workspace declared by the pipeline |  | Optional: \{\} <br /> |
| `subPath` _string_ | SubPath is optionally a directory on the volume which should be used<br />for this binding (i.e. the volume will be mounted at this sub directory). |  | Optional: \{\} <br /> |


#### WorkspaceUsage



WorkspaceUsage is used by a Step or Sidecar to declare that it wants isolated access
to a Workspace defined in a Task.



_Appears in:_
- [Sidecar](#sidecar)
- [Step](#step)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the workspace this Step or Sidecar wants access to. |  |  |
| `mountPath` _string_ | MountPath is the path that the workspace should be mounted to inside the Step or Sidecar,<br />overriding any MountPath specified in the Task's WorkspaceDeclaration. |  |  |



## tekton.dev/v1alpha1

Package v1alpha1 contains API Schema definitions for the run v1alpha1 API group

### Resource Types
- [PipelineResource](#pipelineresource)
- [Run](#run)
- [StepAction](#stepaction)
- [VerificationPolicy](#verificationpolicy)



#### Authority



The Authority block defines the keys for validating signatures.



_Appears in:_
- [VerificationPolicySpec](#verificationpolicyspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name for this authority. |  |  |
| `key` _[KeyRef](#keyref)_ | Key contains the public key to validate the resource. |  |  |


#### EmbeddedRunSpec



EmbeddedRunSpec allows custom task definitions to be embedded



_Appears in:_
- [RunSpec](#runspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `metadata` _[PipelineTaskMetadata](#pipelinetaskmetadata)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[RawExtension](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#rawextension-runtime-pkg)_ | Spec is a specification of a custom task |  | Optional: \{\} <br /> |


#### HashAlgorithm

_Underlying type:_ _string_

HashAlgorithm defines the hash algorithm used for the public key



_Appears in:_
- [KeyRef](#keyref)

| Field | Description |
| --- | --- |
| `sha224` |  |
| `sha256` |  |
| `sha384` |  |
| `sha512` |  |
| `` |  |


#### KeyRef



KeyRef defines the reference to a public key



_Appears in:_
- [Authority](#authority)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `secretRef` _[SecretReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#secretreference-v1-core)_ | SecretRef sets a reference to a secret with the key. |  | Optional: \{\} <br /> |
| `data` _string_ | Data contains the inline public key. |  | Optional: \{\} <br /> |
| `kms` _string_ | KMS contains the KMS url of the public key<br />Supported formats differ based on the KMS system used.<br />One example of a KMS url could be:<br />gcpkms://projects/[PROJECT]/locations/[LOCATION]>/keyRings/[KEYRING]/cryptoKeys/[KEY]/cryptoKeyVersions/[KEY_VERSION]<br />For more examples please refer https://docs.sigstore.dev/cosign/kms_support.<br />Note that the KMS is not supported yet. |  | Optional: \{\} <br /> |
| `hashAlgorithm` _[HashAlgorithm](#hashalgorithm)_ | HashAlgorithm always defaults to sha256 if the algorithm hasn't been explicitly set |  | Optional: \{\} <br /> |


#### ModeType

_Underlying type:_ _string_

ModeType indicates the type of a mode for VerificationPolicy



_Appears in:_
- [VerificationPolicySpec](#verificationpolicyspec)

| Field | Description |
| --- | --- |
| `warn` |  |
| `enforce` |  |


#### PipelineResource



PipelineResource describes a resource that is an input to or output from a
Task.

Deprecated: Unused, preserved only for backwards compatibility





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `tekton.dev/v1alpha1` | | |
| `kind` _string_ | `PipelineResource` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[PipelineResourceSpec](#pipelineresourcespec)_ | Spec holds the desired state of the PipelineResource from the client |  |  |
| `status` _[PipelineResourceStatus](#pipelineresourcestatus)_ | Status is used to communicate the observed state of the PipelineResource from<br />the controller, but was unused as there is no controller for PipelineResource. |  | Optional: \{\} <br /> |


#### PipelineResourceSpec



PipelineResourceSpec defines an individual resources used in the pipeline.

Deprecated: Unused, preserved only for backwards compatibility



_Appears in:_
- [PipelineResource](#pipelineresource)
- [PipelineResourceBinding](#pipelineresourcebinding)
- [TaskResourceBinding](#taskresourcebinding)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `description` _string_ | Description is a user-facing description of the resource that may be<br />used to populate a UI. |  | Optional: \{\} <br /> |
| `type` _[PipelineResourceType](#pipelineresourcetype)_ |  |  |  |
| `params` _[ResourceParam](#resourceparam) array_ |  |  |  |
| `secrets` _[SecretParam](#secretparam) array_ | Secrets to fetch to populate some of resource fields |  | Optional: \{\} <br /> |


#### PipelineResourceStatus



PipelineResourceStatus does not contain anything because PipelineResources on their own
do not have a status

Deprecated: Unused, preserved only for backwards compatibility



_Appears in:_
- [PipelineResource](#pipelineresource)







#### ResourceParam



ResourceParam declares a string value to use for the parameter called Name, and is used in
the specific context of PipelineResources.

Deprecated: Unused, preserved only for backwards compatibility



_Appears in:_
- [PipelineResourceSpec](#pipelineresourcespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ |  |  |  |
| `value` _string_ |  |  |  |


#### ResourcePattern



ResourcePattern defines the pattern of the resource source



_Appears in:_
- [VerificationPolicySpec](#verificationpolicyspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `pattern` _string_ | Pattern defines a resource pattern. Regex is created to filter resources based on `Pattern`<br />Example patterns:<br />GitHub resource: https://github.com/tektoncd/catalog.git, https://github.com/tektoncd/*<br />Bundle resource: gcr.io/tekton-releases/catalog/upstream/git-clone, gcr.io/tekton-releases/catalog/upstream/*<br />Hub resource: https://artifacthub.io/*, |  |  |


#### Run



Run represents a single execution of a Custom Task.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `tekton.dev/v1alpha1` | | |
| `kind` _string_ | `Run` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[RunSpec](#runspec)_ |  |  | Optional: \{\} <br /> |
| `status` _[RunStatus](#runstatus)_ |  |  | Optional: \{\} <br /> |






#### RunSpec



RunSpec defines the desired state of Run



_Appears in:_
- [Run](#run)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ref` _[TaskRef](#taskref)_ |  |  | Optional: \{\} <br /> |
| `spec` _[EmbeddedRunSpec](#embeddedrunspec)_ | Spec is a specification of a custom task |  | Optional: \{\} <br /> |
| `params` _[Params](#params)_ |  |  | Optional: \{\} <br /> |
| `status` _[RunSpecStatus](#runspecstatus)_ | Used for cancelling a run (and maybe more later on) |  | Optional: \{\} <br /> |
| `statusMessage` _[RunSpecStatusMessage](#runspecstatusmessage)_ | Status message for cancellation. |  | Optional: \{\} <br /> |
| `retries` _integer_ | Used for propagating retries count to custom tasks |  | Optional: \{\} <br /> |
| `serviceAccountName` _string_ |  |  | Optional: \{\} <br /> |
| `podTemplate` _[PodTemplate](#podtemplate)_ | PodTemplate holds pod specific configuration |  | Optional: \{\} <br /> |
| `timeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | Time after which the custom-task times out.<br />Refer Go's ParseDuration documentation for expected format: https://golang.org/pkg/time/#ParseDuration |  | Optional: \{\} <br /> |
| `workspaces` _[WorkspaceBinding](#workspacebinding) array_ | Workspaces is a list of WorkspaceBindings from volumes to workspaces. |  | Optional: \{\} <br /> |


#### RunSpecStatus

_Underlying type:_ _string_

RunSpecStatus defines the taskrun spec status the user can provide



_Appears in:_
- [RunSpec](#runspec)

| Field | Description |
| --- | --- |
| `RunCancelled` | RunSpecStatusCancelled indicates that the user wants to cancel the run,<br />if not already cancelled or terminated<br /> |


#### RunSpecStatusMessage

_Underlying type:_ _string_

RunSpecStatusMessage defines human readable status messages for the TaskRun.



_Appears in:_
- [RunSpec](#runspec)

| Field | Description |
| --- | --- |
| `Run cancelled as the PipelineRun it belongs to has been cancelled.` | RunCancelledByPipelineMsg indicates that the PipelineRun of which part this Run was<br />has been cancelled.<br /> |
| `Run cancelled as the PipelineRun it belongs to has timed out.` | RunCancelledByPipelineTimeoutMsg indicates that the Run was cancelled because the PipelineRun running it timed out.<br /> |






#### SecretParam



SecretParam indicates which secret can be used to populate a field of the resource

Deprecated: Unused, preserved only for backwards compatibility



_Appears in:_
- [PipelineResourceSpec](#pipelineresourcespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `fieldName` _string_ |  |  |  |
| `secretKey` _string_ |  |  |  |
| `secretName` _string_ |  |  |  |


#### StepAction



StepAction represents the actionable components of Step.
The Step can only reference it from the cluster or using remote resolution.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `tekton.dev/v1alpha1` | | |
| `kind` _string_ | `StepAction` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[StepActionSpec](#stepactionspec)_ | Spec holds the desired state of the Step from the client |  | Optional: \{\} <br /> |




#### StepActionSpec



StepActionSpec contains the actionable components of a step.



_Appears in:_
- [StepAction](#stepaction)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `description` _string_ | Description is a user-facing description of the stepaction that may be<br />used to populate a UI. |  | Optional: \{\} <br /> |
| `image` _string_ | Image reference name to run for this StepAction.<br />More info: https://kubernetes.io/docs/concepts/containers/images |  | Optional: \{\} <br /> |
| `command` _string array_ | Entrypoint array. Not executed within a shell.<br />The image's ENTRYPOINT is used if this is not provided.<br />Variable references $(VAR_NAME) are expanded using the container's environment. If a variable<br />cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced<br />to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)" will<br />produce the string literal "$(VAR_NAME)". Escaped references will never be expanded, regardless<br />of whether the variable exists or not. Cannot be updated.<br />More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell |  | Optional: \{\} <br /> |
| `args` _string array_ | Arguments to the entrypoint.<br />The image's CMD is used if this is not provided.<br />Variable references $(VAR_NAME) are expanded using the container's environment. If a variable<br />cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced<br />to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)" will<br />produce the string literal "$(VAR_NAME)". Escaped references will never be expanded, regardless<br />of whether the variable exists or not. Cannot be updated.<br />More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell |  | Optional: \{\} <br /> |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envvar-v1-core) array_ | List of environment variables to set in the container.<br />Cannot be updated. |  | Optional: \{\} <br /> |
| `script` _string_ | Script is the contents of an executable file to execute.<br />If Script is not empty, the Step cannot have an Command and the Args will be passed to the Script. |  | Optional: \{\} <br /> |
| `workingDir` _string_ | Step's working directory.<br />If not specified, the container runtime's default will be used, which<br />might be configured in the container image.<br />Cannot be updated. |  | Optional: \{\} <br /> |
| `params` _[ParamSpecs](#paramspecs)_ | Params is a list of input parameters required to run the stepAction.<br />Params must be supplied as inputs in Steps unless they declare a defaultvalue. |  | Optional: \{\} <br /> |
| `results` _[StepResult](#stepresult) array_ | Results are values that this StepAction can output |  | Optional: \{\} <br /> |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#securitycontext-v1-core)_ | SecurityContext defines the security options the Step should be run with.<br />If set, the fields of SecurityContext override the equivalent fields of PodSecurityContext.<br />More info: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/<br />The value set in StepAction will take precedence over the value from Task. |  | Optional: \{\} <br /> |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ | Volumes to mount into the Step's filesystem.<br />Cannot be updated. |  | Optional: \{\} <br /> |


#### VerificationPolicy



VerificationPolicy defines the rules to verify Tekton resources.
VerificationPolicy can config the mapping from resources to a list of public
keys, so when verifying the resources we can use the corresponding public keys.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `tekton.dev/v1alpha1` | | |
| `kind` _string_ | `VerificationPolicy` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[VerificationPolicySpec](#verificationpolicyspec)_ | Spec holds the desired state of the VerificationPolicy. |  |  |


#### VerificationPolicySpec



VerificationPolicySpec defines the patterns and authorities.



_Appears in:_
- [VerificationPolicy](#verificationpolicy)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `resources` _[ResourcePattern](#resourcepattern) array_ | Resources defines the patterns of resources sources that should be subject to this policy.<br />For example, we may want to apply this Policy from a certain GitHub repo.<br />Then the ResourcesPattern should be valid regex. E.g. If using gitresolver, and we want to config keys from a certain git repo.<br />`ResourcesPattern` can be `https://github.com/tektoncd/catalog.git`, we will use regex to filter out those resources. |  |  |
| `authorities` _[Authority](#authority) array_ | Authorities defines the rules for validating signatures. |  |  |
| `mode` _[ModeType](#modetype)_ | Mode controls whether a failing policy will fail the taskrun/pipelinerun, or only log the warnings<br />enforce - fail the taskrun/pipelinerun if verification fails (default)<br />warn - don't fail the taskrun/pipelinerun if verification fails but log warnings |  | Optional: \{\} <br /> |



## tekton.dev/v1beta1

Package v1beta1 contains API Schema definitions for the customrun v1beta1 API group

### Resource Types
- [CustomRun](#customrun)
- [Pipeline](#pipeline)
- [PipelineRun](#pipelinerun)
- [StepAction](#stepaction)
- [Task](#task)
- [TaskRun](#taskrun)



#### Algorithm

_Underlying type:_ _string_

Algorithm Standard cryptographic hash algorithm



_Appears in:_
- [ArtifactValue](#artifactvalue)



#### Args

_Underlying type:_ _string array_





_Appears in:_
- [StepActionSpec](#stepactionspec)





#### Artifact



Artifact represents an artifact within a system, potentially containing multiple values
associated with it.



_Appears in:_
- [Artifacts](#artifacts)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | The artifact's identifying category name |  |  |
| `values` _[ArtifactValue](#artifactvalue) array_ | A collection of values related to the artifact |  |  |
| `buildOutput` _boolean_ | Indicate if the artifact is a build output or a by-product |  |  |


#### ArtifactValue



ArtifactValue represents a specific value or data element within an Artifact.



_Appears in:_
- [Artifact](#artifact)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `digest` _object (keys:[Algorithm](#algorithm), values:string)_ |  |  |  |
| `uri` _string_ |  |  |  |




#### ChildStatusReference



ChildStatusReference is used to point to the statuses of individual TaskRuns and Runs within this PipelineRun.



_Appears in:_
- [PipelineRunStatus](#pipelinerunstatus)
- [PipelineRunStatusFields](#pipelinerunstatusfields)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the TaskRun or Run this is referencing. |  |  |
| `displayName` _string_ | DisplayName is a user-facing name of the pipelineTask that may be<br />used to populate a UI. |  |  |
| `pipelineTaskName` _string_ | PipelineTaskName is the name of the PipelineTask this is referencing. |  |  |
| `whenExpressions` _[WhenExpression](#whenexpression) array_ | WhenExpressions is the list of checks guarding the execution of the PipelineTask |  | Optional: \{\} <br /> |


#### CloudEventCondition

_Underlying type:_ _string_

CloudEventCondition is a string that represents the condition of the event.



_Appears in:_
- [CloudEventDeliveryState](#cloudeventdeliverystate)

| Field | Description |
| --- | --- |
| `Unknown` | CloudEventConditionUnknown means that the condition for the event to be<br />triggered was not met yet, or we don't know the state yet.<br /> |
| `Sent` | CloudEventConditionSent means that the event was sent successfully<br /> |
| `Failed` | CloudEventConditionFailed means that there was one or more attempts to<br />send the event, and none was successful so far.<br /> |


#### CloudEventDelivery



CloudEventDelivery is the target of a cloud event along with the state of
delivery.



_Appears in:_
- [TaskRunStatus](#taskrunstatus)
- [TaskRunStatusFields](#taskrunstatusfields)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `target` _string_ | Target points to an addressable |  |  |
| `status` _[CloudEventDeliveryState](#cloudeventdeliverystate)_ |  |  |  |


#### CloudEventDeliveryState



CloudEventDeliveryState reports the state of a cloud event to be sent.



_Appears in:_
- [CloudEventDelivery](#cloudeventdelivery)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `condition` _[CloudEventCondition](#cloudeventcondition)_ | Current status |  |  |
| `sentAt` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#time-v1-meta)_ | SentAt is the time at which the last attempt to send the event was made |  | Optional: \{\} <br /> |
| `message` _string_ | Error is the text of error (if any) |  |  |
| `retryCount` _integer_ | RetryCount is the number of attempts of sending the cloud event |  |  |


#### Combination

_Underlying type:_ _object_

Combination is a map, mainly defined to hold a single combination from a Matrix with key as param.Name and value as param.Value



_Appears in:_
- [Combinations](#combinations)





#### ConfigSource



ConfigSource contains the information that can uniquely identify where a remote
built definition came from i.e. Git repositories, Tekton Bundles in OCI registry
and hub.



_Appears in:_
- [Provenance](#provenance)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `uri` _string_ | URI indicates the identity of the source of the build definition.<br />Example: "https://github.com/tektoncd/catalog" |  |  |
| `digest` _object (keys:string, values:string)_ | Digest is a collection of cryptographic digests for the contents of the artifact specified by URI.<br />Example: \{"sha1": "f99d13e554ffcb696dee719fa85b695cb5b0f428"\} |  |  |
| `entryPoint` _string_ | EntryPoint identifies the entry point into the build. This is often a path to a<br />build definition file and/or a target label within that file.<br />Example: "task/git-clone/0.10/git-clone.yaml" |  |  |


#### CustomRun



CustomRun represents a single execution of a Custom Task.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `tekton.dev/v1beta1` | | |
| `kind` _string_ | `CustomRun` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[CustomRunSpec](#customrunspec)_ |  |  | Optional: \{\} <br /> |
| `status` _[CustomRunStatus](#customrunstatus)_ |  |  | Optional: \{\} <br /> |






#### CustomRunSpec



CustomRunSpec defines the desired state of CustomRun



_Appears in:_
- [CustomRun](#customrun)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `customRef` _[TaskRef](#taskref)_ |  |  | Optional: \{\} <br /> |
| `customSpec` _[EmbeddedCustomRunSpec](#embeddedcustomrunspec)_ | Spec is a specification of a custom task |  | Optional: \{\} <br /> |
| `params` _[Params](#params)_ |  |  | Optional: \{\} <br /> |
| `status` _[CustomRunSpecStatus](#customrunspecstatus)_ | Used for cancelling a customrun (and maybe more later on) |  | Optional: \{\} <br /> |
| `statusMessage` _[CustomRunSpecStatusMessage](#customrunspecstatusmessage)_ | Status message for cancellation. |  | Optional: \{\} <br /> |
| `retries` _integer_ | Used for propagating retries count to custom tasks |  | Optional: \{\} <br /> |
| `serviceAccountName` _string_ |  |  | Optional: \{\} <br /> |
| `timeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | Time after which the custom-task times out.<br />Refer Go's ParseDuration documentation for expected format: https://golang.org/pkg/time/#ParseDuration |  | Optional: \{\} <br /> |
| `workspaces` _[WorkspaceBinding](#workspacebinding) array_ | Workspaces is a list of WorkspaceBindings from volumes to workspaces. |  | Optional: \{\} <br /> |


#### CustomRunSpecStatus

_Underlying type:_ _string_

CustomRunSpecStatus defines the taskrun spec status the user can provide



_Appears in:_
- [CustomRunSpec](#customrunspec)

| Field | Description |
| --- | --- |
| `RunCancelled` | CustomRunSpecStatusCancelled indicates that the user wants to cancel the run,<br />if not already cancelled or terminated<br /> |


#### CustomRunSpecStatusMessage

_Underlying type:_ _string_

CustomRunSpecStatusMessage defines human readable status messages for the TaskRun.



_Appears in:_
- [CustomRunSpec](#customrunspec)

| Field | Description |
| --- | --- |
| `CustomRun cancelled as the PipelineRun it belongs to has been cancelled.` | CustomRunCancelledByPipelineMsg indicates that the PipelineRun of which part this CustomRun was<br />has been cancelled.<br /> |
| `CustomRun cancelled as the PipelineRun it belongs to has timed out.` | CustomRunCancelledByPipelineTimeoutMsg indicates that the Run was cancelled because the PipelineRun running it timed out.<br /> |






#### EmbeddedCustomRunSpec



EmbeddedCustomRunSpec allows custom task definitions to be embedded



_Appears in:_
- [CustomRunSpec](#customrunspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `metadata` _[PipelineTaskMetadata](#pipelinetaskmetadata)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[RawExtension](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#rawextension-runtime-pkg)_ | Spec is a specification of a custom task |  | Optional: \{\} <br /> |


#### EmbeddedTask



EmbeddedTask is used to define a Task inline within a Pipeline's PipelineTasks.



_Appears in:_
- [PipelineTask](#pipelinetask)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `spec` _[RawExtension](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#rawextension-runtime-pkg)_ | Spec is a specification of a custom task |  | Optional: \{\} <br /> |
| `metadata` _[PipelineTaskMetadata](#pipelinetaskmetadata)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `resources` _[TaskResources](#taskresources)_ | Resources is a list input and output resource to run the task<br />Resources are represented in TaskRuns as bindings to instances of<br />PipelineResources.<br />Deprecated: Unused, preserved only for backwards compatibility |  | Optional: \{\} <br /> |
| `params` _[ParamSpecs](#paramspecs)_ | Params is a list of input parameters required to run the task. Params<br />must be supplied as inputs in TaskRuns unless they declare a default<br />value. |  | Optional: \{\} <br /> |
| `displayName` _string_ | DisplayName is a user-facing name of the task that may be<br />used to populate a UI. |  | Optional: \{\} <br /> |
| `description` _string_ | Description is a user-facing description of the task that may be<br />used to populate a UI. |  | Optional: \{\} <br /> |
| `steps` _[Step](#step) array_ | Steps are the steps of the build; each step is run sequentially with the<br />source mounted into /workspace. |  |  |
| `volumes` _[Volumes](#volumes)_ | Volumes is a collection of volumes that are available to mount into the<br />steps of the build.<br />See Pod.spec.volumes (API version: v1) |  | Schemaless: \{\} <br /> |
| `stepTemplate` _[StepTemplate](#steptemplate)_ | StepTemplate can be used as the basis for all step containers within the<br />Task, so that the steps inherit settings on the base container. |  |  |
| `sidecars` _[Sidecar](#sidecar) array_ | Sidecars are run alongside the Task's step containers. They begin before<br />the steps start and end after the steps complete. |  |  |
| `workspaces` _[WorkspaceDeclaration](#workspacedeclaration) array_ | Workspaces are the volumes that this Task requires. |  |  |
| `results` _[TaskResult](#taskresult) array_ | Results are values that this Task can output |  |  |






#### Matrix



Matrix is used to fan out Tasks in a Pipeline



_Appears in:_
- [PipelineTask](#pipelinetask)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `params` _[Params](#params)_ | Params is a list of parameters used to fan out the pipelineTask<br />Params takes only `Parameters` of type `"array"`<br />Each array element is supplied to the `PipelineTask` by substituting `params` of type `"string"` in the underlying `Task`.<br />The names of the `params` in the `Matrix` must match the names of the `params` in the underlying `Task` that they will be substituting. |  |  |


#### OnErrorType

_Underlying type:_ _string_

OnErrorType defines a list of supported exiting behavior of a container on error



_Appears in:_
- [Step](#step)

| Field | Description |
| --- | --- |
| `stopAndFail` | StopAndFail indicates exit the taskRun if the container exits with non-zero exit code<br /> |
| `continue` | Continue indicates continue executing the rest of the steps irrespective of the container exit code<br /> |


#### Param



Param declares an ParamValues to use for the parameter called name.



_Appears in:_
- [Params](#params)
- [TaskRunInputs](#taskruninputs)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ |  |  |  |
| `value` _[ParamValue](#paramvalue)_ |  |  | Schemaless: \{\} <br /> |


#### ParamSpec



ParamSpec defines arbitrary parameters needed beyond typed inputs (such as
resources). Parameter values are provided by users as inputs on a TaskRun
or PipelineRun.



_Appears in:_
- [ParamSpecs](#paramspecs)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name declares the name by which a parameter is referenced. |  |  |
| `type` _[ParamType](#paramtype)_ | Type is the user-specified type of the parameter. The possible types<br />are currently "string", "array" and "object", and "string" is the default. |  | Optional: \{\} <br /> |
| `description` _string_ | Description is a user-facing description of the parameter that may be<br />used to populate a UI. |  | Optional: \{\} <br /> |
| `properties` _object (keys:string, values:[PropertySpec](#propertyspec))_ | Properties is the JSON Schema properties to support key-value pairs parameter. |  | Optional: \{\} <br /> |
| `default` _[ParamValue](#paramvalue)_ | Default is the value a parameter takes if no input value is supplied. If<br />default is set, a Task may be executed without a supplied value for the<br />parameter. |  | Schemaless: \{\} <br />Optional: \{\} <br /> |
| `enum` _string array_ | Enum declares a set of allowed param input values for tasks/pipelines that can be validated.<br />If Enum is not set, no input validation is performed for the param. |  | Optional: \{\} <br /> |


#### ParamSpecs

_Underlying type:_ _[ParamSpec](#paramspec)_

ParamSpecs is a list of ParamSpec



_Appears in:_
- [EmbeddedTask](#embeddedtask)
- [PipelineSpec](#pipelinespec)
- [TaskSpec](#taskspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name declares the name by which a parameter is referenced. |  |  |
| `type` _[ParamType](#paramtype)_ | Type is the user-specified type of the parameter. The possible types<br />are currently "string", "array" and "object", and "string" is the default. |  | Optional: \{\} <br /> |
| `description` _string_ | Description is a user-facing description of the parameter that may be<br />used to populate a UI. |  | Optional: \{\} <br /> |
| `properties` _object (keys:string, values:[PropertySpec](#propertyspec))_ | Properties is the JSON Schema properties to support key-value pairs parameter. |  | Optional: \{\} <br /> |
| `default` _[ParamValue](#paramvalue)_ | Default is the value a parameter takes if no input value is supplied. If<br />default is set, a Task may be executed without a supplied value for the<br />parameter. |  | Schemaless: \{\} <br />Optional: \{\} <br /> |
| `enum` _string array_ | Enum declares a set of allowed param input values for tasks/pipelines that can be validated.<br />If Enum is not set, no input validation is performed for the param. |  | Optional: \{\} <br /> |


#### ParamType

_Underlying type:_ _string_

ParamType indicates the type of an input parameter;
Used to distinguish between a single string and an array of strings.



_Appears in:_
- [ParamSpec](#paramspec)
- [ParamValue](#paramvalue)
- [PropertySpec](#propertyspec)

| Field | Description |
| --- | --- |
| `string` |  |
| `array` |  |
| `object` |  |


#### ParamValue



ParamValue is a type that can hold a single string or string array.
Used in JSON unmarshalling so that a single JSON field can accept
either an individual string or an array of strings.



_Appears in:_
- [Param](#param)
- [ParamSpec](#paramspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `Type` _[ParamType](#paramtype)_ |  |  |  |
| `StringVal` _string_ |  |  |  |
| `ArrayVal` _string array_ |  |  |  |
| `ObjectVal` _object (keys:string, values:string)_ |  |  |  |


#### Params

_Underlying type:_ _[Param](#param)_

Params is a list of Param



_Appears in:_
- [CustomRunSpec](#customrunspec)
- [IncludeParams](#includeparams)
- [Matrix](#matrix)
- [PipelineRunSpec](#pipelinerunspec)
- [PipelineTask](#pipelinetask)
- [ResolverRef](#resolverref)
- [RunSpec](#runspec)
- [Step](#step)
- [TaskRunSpec](#taskrunspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ |  |  |  |
| `value` _[ParamValue](#paramvalue)_ |  |  | Schemaless: \{\} <br /> |


#### Pipeline



Pipeline describes a list of Tasks to execute. It expresses how outputs
of tasks feed into inputs of subsequent tasks.

Deprecated: Please use v1.Pipeline instead.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `tekton.dev/v1beta1` | | |
| `kind` _string_ | `Pipeline` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[PipelineSpec](#pipelinespec)_ | Spec holds the desired state of the Pipeline from the client |  | Optional: \{\} <br /> |


#### PipelineDeclaredResource



PipelineDeclaredResource is used by a Pipeline to declare the types of the
PipelineResources that it will required to run and names which can be used to
refer to these PipelineResources in PipelineTaskResourceBindings.

Deprecated: Unused, preserved only for backwards compatibility



_Appears in:_
- [PipelineSpec](#pipelinespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name that will be used by the Pipeline to refer to this resource.<br />It does not directly correspond to the name of any PipelineResources Task<br />inputs or outputs, and it does not correspond to the actual names of the<br />PipelineResources that will be bound in the PipelineRun. |  |  |
| `type` _[PipelineResourceType](#pipelineresourcetype)_ | Type is the type of the PipelineResource. |  |  |
| `optional` _boolean_ | Optional declares the resource as optional.<br />optional: true - the resource is considered optional<br />optional: false - the resource is considered required (default/equivalent of not specifying it) |  |  |




#### PipelineRef



PipelineRef can be used to refer to a specific instance of a Pipeline.



_Appears in:_
- [PipelineRunSpec](#pipelinerunspec)
- [PipelineTask](#pipelinetask)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the referent; More info: http://kubernetes.io/docs/user-guide/identifiers#names |  |  |
| `apiVersion` _string_ | API version of the referent |  | Optional: \{\} <br /> |
| `bundle` _string_ | Bundle url reference to a Tekton Bundle.<br />Deprecated: Please use ResolverRef with the bundles resolver instead.<br />The field is staying there for go client backward compatibility, but is not used/allowed anymore. |  | Optional: \{\} <br /> |
| `ResolverRef` _[ResolverRef](#resolverref)_ | ResolverRef allows referencing a Pipeline in a remote location<br />like a git repo. This field is only supported when the alpha<br />feature gate is enabled. |  | Optional: \{\} <br /> |


#### PipelineResourceBinding



PipelineResourceBinding connects a reference to an instance of a PipelineResource
with a PipelineResource dependency that the Pipeline has declared

Deprecated: Unused, preserved only for backwards compatibility



_Appears in:_
- [PipelineRunSpec](#pipelinerunspec)
- [TaskResourceBinding](#taskresourcebinding)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the PipelineResource in the Pipeline's declaration |  |  |
| `resourceRef` _[PipelineResourceRef](#pipelineresourceref)_ | ResourceRef is a reference to the instance of the actual PipelineResource<br />that should be used |  | Optional: \{\} <br /> |
| `resourceSpec` _[PipelineResourceSpec](#pipelineresourcespec)_ | ResourceSpec is specification of a resource that should be created and<br />consumed by the task |  | Optional: \{\} <br /> |




#### PipelineResourceRef



PipelineResourceRef can be used to refer to a specific instance of a Resource

Deprecated: Unused, preserved only for backwards compatibility



_Appears in:_
- [PipelineResourceBinding](#pipelineresourcebinding)
- [TaskResourceBinding](#taskresourcebinding)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the referent; More info: http://kubernetes.io/docs/user-guide/identifiers#names |  |  |
| `apiVersion` _string_ | API version of the referent |  | Optional: \{\} <br /> |






#### PipelineResult

_Underlying type:_ _[struct{Name string "json:\"name\""; Type ResultsType "json:\"type,omitempty\""; Description string "json:\"description\""; Value ResultValue "json:\"value\""}](#struct{name-string-"json:\"name\"";-type-resultstype-"json:\"type,omitempty\"";-description-string-"json:\"description\"";-value-resultvalue-"json:\"value\""})_

PipelineResult used to describe the results of a pipeline



_Appears in:_
- [PipelineSpec](#pipelinespec)



#### PipelineRun



PipelineRun represents a single execution of a Pipeline. PipelineRuns are how
the graph of Tasks declared in a Pipeline are executed; they specify inputs
to Pipelines such as parameter values and capture operational aspects of the
Tasks execution such as service account and tolerations. Creating a
PipelineRun creates TaskRuns for Tasks in the referenced Pipeline.

Deprecated: Please use v1.PipelineRun instead.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `tekton.dev/v1beta1` | | |
| `kind` _string_ | `PipelineRun` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[PipelineRunSpec](#pipelinerunspec)_ |  |  | Optional: \{\} <br /> |
| `status` _[PipelineRunStatus](#pipelinerunstatus)_ |  |  | Optional: \{\} <br /> |




#### PipelineRunResult



PipelineRunResult used to describe the results of a pipeline



_Appears in:_
- [PipelineRunStatus](#pipelinerunstatus)
- [PipelineRunStatusFields](#pipelinerunstatusfields)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the result's name as declared by the Pipeline |  |  |
| `value` _[ResultValue](#resultvalue)_ | Value is the result returned from the execution of this PipelineRun |  | Schemaless: \{\} <br /> |


#### PipelineRunRunStatus



PipelineRunRunStatus contains the name of the PipelineTask for this CustomRun or Run and the CustomRun or Run's Status



_Appears in:_
- [PipelineRunStatus](#pipelinerunstatus)
- [PipelineRunStatusFields](#pipelinerunstatusfields)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `pipelineTaskName` _string_ | PipelineTaskName is the name of the PipelineTask. |  |  |
| `status` _[CustomRunStatus](#customrunstatus)_ | Status is the CustomRunStatus for the corresponding CustomRun or Run |  | Optional: \{\} <br /> |
| `whenExpressions` _[WhenExpression](#whenexpression) array_ | WhenExpressions is the list of checks guarding the execution of the PipelineTask |  | Optional: \{\} <br /> |


#### PipelineRunSpec



PipelineRunSpec defines the desired state of PipelineRun



_Appears in:_
- [PipelineRun](#pipelinerun)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `pipelineRef` _[PipelineRef](#pipelineref)_ |  |  | Optional: \{\} <br /> |
| `pipelineSpec` _[PipelineSpec](#pipelinespec)_ | Specifying PipelineSpec can be disabled by setting<br />`disable-inline-spec` feature flag.<br />See Pipeline.spec (API version: tekton.dev/v1beta1) |  | Schemaless: \{\} <br />Optional: \{\} <br /> |
| `resources` _[PipelineResourceBinding](#pipelineresourcebinding) array_ | Resources is a list of bindings specifying which actual instances of<br />PipelineResources to use for the resources the Pipeline has declared<br />it needs.<br />Deprecated: Unused, preserved only for backwards compatibility |  |  |
| `params` _[Params](#params)_ | Params is a list of parameter names and values. |  |  |
| `serviceAccountName` _string_ |  |  | Optional: \{\} <br /> |
| `status` _[PipelineRunSpecStatus](#pipelinerunspecstatus)_ | Used for cancelling a pipelinerun (and maybe more later on) |  | Optional: \{\} <br /> |
| `timeouts` _[TimeoutFields](#timeoutfields)_ | Time after which the Pipeline times out.<br />Currently three keys are accepted in the map<br />pipeline, tasks and finally<br />with Timeouts.pipeline >= Timeouts.tasks + Timeouts.finally |  | Optional: \{\} <br /> |
| `timeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | Timeout is the Time after which the Pipeline times out.<br />Defaults to never.<br />Refer to Go's ParseDuration documentation for expected format: https://golang.org/pkg/time/#ParseDuration<br />Deprecated: use pipelineRunSpec.Timeouts.Pipeline instead |  | Optional: \{\} <br /> |
| `podTemplate` _[PodTemplate](#podtemplate)_ | PodTemplate holds pod specific configuration |  |  |
| `workspaces` _[WorkspaceBinding](#workspacebinding) array_ | Workspaces holds a set of workspace bindings that must match names<br />with those declared in the pipeline. |  | Optional: \{\} <br /> |
| `taskRunSpecs` _[PipelineTaskRunSpec](#pipelinetaskrunspec) array_ | TaskRunSpecs holds a set of runtime specs |  | Optional: \{\} <br /> |
| `managedBy` _string_ | ManagedBy indicates which controller is responsible for reconciling<br />this resource. If unset or set to "tekton.dev/pipeline", the default<br />Tekton controller will manage this resource.<br />This field is immutable. |  | Optional: \{\} <br /> |


#### PipelineRunSpecStatus

_Underlying type:_ _string_

PipelineRunSpecStatus defines the pipelinerun spec status the user can provide



_Appears in:_
- [PipelineRunSpec](#pipelinerunspec)



#### PipelineRunStatus



PipelineRunStatus defines the observed state of PipelineRun



_Appears in:_
- [PipelineRun](#pipelinerun)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the 'Generation' of the Service that<br />was last processed by the controller. |  | Optional: \{\} <br /> |
| `conditions` _[Conditions](#conditions)_ | Conditions the latest available observations of a resource's current state. |  | Optional: \{\} <br /> |
| `annotations` _object (keys:string, values:string)_ | Annotations is additional Status fields for the Resource to save some<br />additional State as well as convey more information to the user. This is<br />roughly akin to Annotations on any k8s resource, just the reconciler conveying<br />richer information outwards. |  |  |
| `startTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#time-v1-meta)_ | StartTime is the time the PipelineRun is actually started. |  |  |
| `completionTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#time-v1-meta)_ | CompletionTime is the time the PipelineRun completed. |  |  |
| `taskRuns` _object (keys:string, values:[PipelineRunTaskRunStatus](#pipelineruntaskrunstatus))_ | TaskRuns is a map of PipelineRunTaskRunStatus with the taskRun name as the key.<br />Deprecated: use ChildReferences instead. As of v0.45.0, this field is no<br />longer populated and is only included for backwards compatibility with<br />older server versions. |  | Optional: \{\} <br /> |
| `runs` _object (keys:string, values:[PipelineRunRunStatus](#pipelinerunrunstatus))_ | Runs is a map of PipelineRunRunStatus with the run name as the key<br />Deprecated: use ChildReferences instead. As of v0.45.0, this field is no<br />longer populated and is only included for backwards compatibility with<br />older server versions. |  | Optional: \{\} <br /> |
| `pipelineResults` _[PipelineRunResult](#pipelinerunresult) array_ | PipelineResults are the list of results written out by the pipeline task's containers |  | Optional: \{\} <br /> |
| `pipelineSpec` _[PipelineSpec](#pipelinespec)_ | PipelineSpec contains the exact spec used to instantiate the run.<br />See Pipeline.spec (API version: tekton.dev/v1beta1) |  | Schemaless: \{\} <br /> |
| `skippedTasks` _[SkippedTask](#skippedtask) array_ | list of tasks that were skipped due to when expressions evaluating to false |  | Optional: \{\} <br /> |
| `childReferences` _[ChildStatusReference](#childstatusreference) array_ | list of TaskRun and Run names, PipelineTask names, and API versions/kinds for children of this PipelineRun. |  | Optional: \{\} <br /> |
| `finallyStartTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#time-v1-meta)_ | FinallyStartTime is when all non-finally tasks have been completed and only finally tasks are being executed. |  | Optional: \{\} <br /> |
| `provenance` _[Provenance](#provenance)_ | Provenance contains some key authenticated metadata about how a software artifact was built (what sources, what inputs/outputs, etc.). |  | Optional: \{\} <br /> |
| `spanContext` _object (keys:string, values:string)_ | SpanContext contains tracing span context fields |  |  |


#### PipelineRunStatusFields



PipelineRunStatusFields holds the fields of PipelineRunStatus' status.
This is defined separately and inlined so that other types can readily
consume these fields via duck typing.



_Appears in:_
- [PipelineRunStatus](#pipelinerunstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `startTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#time-v1-meta)_ | StartTime is the time the PipelineRun is actually started. |  |  |
| `completionTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#time-v1-meta)_ | CompletionTime is the time the PipelineRun completed. |  |  |
| `taskRuns` _object (keys:string, values:[PipelineRunTaskRunStatus](#pipelineruntaskrunstatus))_ | TaskRuns is a map of PipelineRunTaskRunStatus with the taskRun name as the key.<br />Deprecated: use ChildReferences instead. As of v0.45.0, this field is no<br />longer populated and is only included for backwards compatibility with<br />older server versions. |  | Optional: \{\} <br /> |
| `runs` _object (keys:string, values:[PipelineRunRunStatus](#pipelinerunrunstatus))_ | Runs is a map of PipelineRunRunStatus with the run name as the key<br />Deprecated: use ChildReferences instead. As of v0.45.0, this field is no<br />longer populated and is only included for backwards compatibility with<br />older server versions. |  | Optional: \{\} <br /> |
| `pipelineResults` _[PipelineRunResult](#pipelinerunresult) array_ | PipelineResults are the list of results written out by the pipeline task's containers |  | Optional: \{\} <br /> |
| `pipelineSpec` _[PipelineSpec](#pipelinespec)_ | PipelineSpec contains the exact spec used to instantiate the run.<br />See Pipeline.spec (API version: tekton.dev/v1beta1) |  | Schemaless: \{\} <br /> |
| `skippedTasks` _[SkippedTask](#skippedtask) array_ | list of tasks that were skipped due to when expressions evaluating to false |  | Optional: \{\} <br /> |
| `childReferences` _[ChildStatusReference](#childstatusreference) array_ | list of TaskRun and Run names, PipelineTask names, and API versions/kinds for children of this PipelineRun. |  | Optional: \{\} <br /> |
| `finallyStartTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#time-v1-meta)_ | FinallyStartTime is when all non-finally tasks have been completed and only finally tasks are being executed. |  | Optional: \{\} <br /> |
| `provenance` _[Provenance](#provenance)_ | Provenance contains some key authenticated metadata about how a software artifact was built (what sources, what inputs/outputs, etc.). |  | Optional: \{\} <br /> |
| `spanContext` _object (keys:string, values:string)_ | SpanContext contains tracing span context fields |  |  |


#### PipelineRunTaskRunStatus



PipelineRunTaskRunStatus contains the name of the PipelineTask for this TaskRun and the TaskRun's Status



_Appears in:_
- [PipelineRunStatus](#pipelinerunstatus)
- [PipelineRunStatusFields](#pipelinerunstatusfields)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `pipelineTaskName` _string_ | PipelineTaskName is the name of the PipelineTask. |  |  |
| `status` _[TaskRunStatus](#taskrunstatus)_ | Status is the TaskRunStatus for the corresponding TaskRun |  | Optional: \{\} <br /> |
| `whenExpressions` _[WhenExpression](#whenexpression) array_ | WhenExpressions is the list of checks guarding the execution of the PipelineTask |  | Optional: \{\} <br /> |


#### PipelineSpec



PipelineSpec defines the desired state of Pipeline.



_Appears in:_
- [Pipeline](#pipeline)
- [PipelineRunSpec](#pipelinerunspec)
- [PipelineRunStatus](#pipelinerunstatus)
- [PipelineRunStatusFields](#pipelinerunstatusfields)
- [PipelineTask](#pipelinetask)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `displayName` _string_ | DisplayName is a user-facing name of the pipeline that may be<br />used to populate a UI. |  | Optional: \{\} <br /> |
| `description` _string_ | Description is a user-facing description of the pipeline that may be<br />used to populate a UI. |  | Optional: \{\} <br /> |
| `resources` _[PipelineDeclaredResource](#pipelinedeclaredresource) array_ | Deprecated: Unused, preserved only for backwards compatibility |  |  |
| `tasks` _[PipelineTask](#pipelinetask) array_ | Tasks declares the graph of Tasks that execute when this Pipeline is run. |  |  |
| `params` _[ParamSpecs](#paramspecs)_ | Params declares a list of input parameters that must be supplied when<br />this Pipeline is run. |  |  |
| `workspaces` _[PipelineWorkspaceDeclaration](#pipelineworkspacedeclaration) array_ | Workspaces declares a set of named workspaces that are expected to be<br />provided by a PipelineRun. |  | Optional: \{\} <br /> |
| `results` _[PipelineResult](#pipelineresult) array_ | Results are values that this pipeline can output once run |  | Optional: \{\} <br /> |
| `finally` _[PipelineTask](#pipelinetask) array_ | Finally declares the list of Tasks that execute just before leaving the Pipeline<br />i.e. either after all Tasks are finished executing successfully<br />or after a failure which would result in ending the Pipeline |  |  |


#### PipelineTask



PipelineTask defines a task in a Pipeline, passing inputs from both
Params and from the output of previous tasks.



_Appears in:_
- [PipelineSpec](#pipelinespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of this task within the context of a Pipeline. Name is<br />used as a coordinate with the `from` and `runAfter` fields to establish<br />the execution order of tasks relative to one another. |  |  |
| `displayName` _string_ | DisplayName is the display name of this task within the context of a Pipeline.<br />This display name may be used to populate a UI. |  | Optional: \{\} <br /> |
| `description` _string_ | Description is the description of this task within the context of a Pipeline.<br />This description may be used to populate a UI. |  | Optional: \{\} <br /> |
| `taskRef` _[TaskRef](#taskref)_ | TaskRef is a reference to a task definition. |  | Optional: \{\} <br /> |
| `taskSpec` _[EmbeddedTask](#embeddedtask)_ | TaskSpec is a specification of a task<br />Specifying TaskSpec can be disabled by setting<br />`disable-inline-spec` feature flag.<br />See Task.spec (API version: tekton.dev/v1beta1) |  | Schemaless: \{\} <br />Optional: \{\} <br /> |
| `when` _[WhenExpressions](#whenexpressions)_ | WhenExpressions is a list of when expressions that need to be true for the task to run |  | Optional: \{\} <br /> |
| `retries` _integer_ | Retries represents how many times this task should be retried in case of task failure: ConditionSucceeded set to False |  | Optional: \{\} <br /> |
| `runAfter` _string array_ | RunAfter is the list of PipelineTask names that should be executed before<br />this Task executes. (Used to force a specific ordering in graph execution.) |  | Optional: \{\} <br /> |
| `resources` _[PipelineTaskResources](#pipelinetaskresources)_ | Deprecated: Unused, preserved only for backwards compatibility |  | Optional: \{\} <br /> |
| `params` _[Params](#params)_ | Parameters declares parameters passed to this task. |  | Optional: \{\} <br /> |
| `matrix` _[Matrix](#matrix)_ | Matrix declares parameters used to fan out this task. |  | Optional: \{\} <br /> |
| `workspaces` _[WorkspacePipelineTaskBinding](#workspacepipelinetaskbinding) array_ | Workspaces maps workspaces from the pipeline spec to the workspaces<br />declared in the Task. |  | Optional: \{\} <br /> |
| `timeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | Duration after which the TaskRun times out. Defaults to 1 hour.<br />Refer Go's ParseDuration documentation for expected format: https://golang.org/pkg/time/#ParseDuration |  | Optional: \{\} <br /> |
| `pipelineRef` _[PipelineRef](#pipelineref)_ | PipelineRef is a reference to a pipeline definition<br />Note: PipelineRef is in preview mode and not yet supported |  | Optional: \{\} <br /> |
| `pipelineSpec` _[PipelineSpec](#pipelinespec)_ | PipelineSpec is a specification of a pipeline<br />Note: PipelineSpec is in preview mode and not yet supported<br />Specifying PipelineSpec can be disabled by setting<br />`disable-inline-spec` feature flag.<br />See Pipeline.spec (API version: tekton.dev/v1beta1) |  | Schemaless: \{\} <br />Optional: \{\} <br /> |
| `onError` _[PipelineTaskOnErrorType](#pipelinetaskonerrortype)_ | OnError defines the exiting behavior of a PipelineRun on error<br />can be set to [ continue \| stopAndFail ] |  | Optional: \{\} <br /> |


#### PipelineTaskInputResource



PipelineTaskInputResource maps the name of a declared PipelineResource input
dependency in a Task to the resource in the Pipeline's DeclaredPipelineResources
that should be used. This input may come from a previous task.

Deprecated: Unused, preserved only for backwards compatibility



_Appears in:_
- [PipelineTaskResources](#pipelinetaskresources)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the PipelineResource as declared by the Task. |  |  |
| `resource` _string_ | Resource is the name of the DeclaredPipelineResource to use. |  |  |
| `from` _string array_ | From is the list of PipelineTask names that the resource has to come from.<br />(Implies an ordering in the execution graph.) |  | Optional: \{\} <br /> |


#### PipelineTaskMetadata



PipelineTaskMetadata contains the labels or annotations for an EmbeddedTask



_Appears in:_
- [EmbeddedCustomRunSpec](#embeddedcustomrunspec)
- [EmbeddedRunSpec](#embeddedrunspec)
- [EmbeddedTask](#embeddedtask)
- [PipelineTaskRunSpec](#pipelinetaskrunspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `labels` _object (keys:string, values:string)_ |  |  | Optional: \{\} <br /> |
| `annotations` _object (keys:string, values:string)_ |  |  | Optional: \{\} <br /> |


#### PipelineTaskOnErrorType

_Underlying type:_ _string_

PipelineTaskOnErrorType defines a list of supported failure handling behaviors of a PipelineTask on error



_Appears in:_
- [PipelineTask](#pipelinetask)

| Field | Description |
| --- | --- |
| `stopAndFail` | PipelineTaskStopAndFail indicates to stop and fail the PipelineRun if the PipelineTask fails<br /> |
| `continue` | PipelineTaskContinue indicates to continue executing the rest of the DAG when the PipelineTask fails<br /> |


#### PipelineTaskOutputResource



PipelineTaskOutputResource maps the name of a declared PipelineResource output
dependency in a Task to the resource in the Pipeline's DeclaredPipelineResources
that should be used.

Deprecated: Unused, preserved only for backwards compatibility



_Appears in:_
- [PipelineTaskResources](#pipelinetaskresources)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the PipelineResource as declared by the Task. |  |  |
| `resource` _string_ | Resource is the name of the DeclaredPipelineResource to use. |  |  |




#### PipelineTaskResources



PipelineTaskResources allows a Pipeline to declare how its DeclaredPipelineResources
should be provided to a Task as its inputs and outputs.

Deprecated: Unused, preserved only for backwards compatibility



_Appears in:_
- [PipelineTask](#pipelinetask)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `inputs` _[PipelineTaskInputResource](#pipelinetaskinputresource) array_ | Inputs holds the mapping from the PipelineResources declared in<br />DeclaredPipelineResources to the input PipelineResources required by the Task. |  |  |
| `outputs` _[PipelineTaskOutputResource](#pipelinetaskoutputresource) array_ | Outputs holds the mapping from the PipelineResources declared in<br />DeclaredPipelineResources to the input PipelineResources required by the Task. |  |  |




#### PipelineTaskRunSpec



PipelineTaskRunSpec  can be used to configure specific
specs for a concrete Task



_Appears in:_
- [PipelineRunSpec](#pipelinerunspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `pipelineTaskName` _string_ |  |  |  |
| `taskServiceAccountName` _string_ |  |  |  |
| `taskPodTemplate` _[PodTemplate](#podtemplate)_ |  |  |  |
| `stepOverrides` _[TaskRunStepOverride](#taskrunstepoverride) array_ |  |  |  |
| `sidecarOverrides` _[TaskRunSidecarOverride](#taskrunsidecaroverride) array_ |  |  |  |
| `metadata` _[PipelineTaskMetadata](#pipelinetaskmetadata)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `computeResources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Compute resources to use for this TaskRun |  |  |
| `timeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | Duration after which the TaskRun times out.<br />Refer Go's ParseDuration documentation for expected format: https://golang.org/pkg/time/#ParseDuration |  | Optional: \{\} <br /> |


#### PipelineWorkspaceDeclaration

_Underlying type:_ _[struct{Name string "json:\"name\""; Description string "json:\"description,omitempty\""; Optional bool "json:\"optional,omitempty\""}](#struct{name-string-"json:\"name\"";-description-string-"json:\"description,omitempty\"";-optional-bool-"json:\"optional,omitempty\""})_

PipelineWorkspaceDeclaration creates a named slot in a Pipeline that a PipelineRun
is expected to populate with a workspace binding.



_Appears in:_
- [PipelineSpec](#pipelinespec)



#### PropertySpec



PropertySpec defines the struct for object keys



_Appears in:_
- [ParamSpec](#paramspec)
- [TaskResult](#taskresult)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[ParamType](#paramtype)_ |  |  |  |


#### Provenance



Provenance contains metadata about resources used in the TaskRun/PipelineRun
such as the source from where a remote build definition was fetched.
This field aims to carry minimum amoumt of metadata in *Run status so that
Tekton Chains can capture them in the provenance.



_Appears in:_
- [PipelineRunStatus](#pipelinerunstatus)
- [PipelineRunStatusFields](#pipelinerunstatusfields)
- [StepState](#stepstate)
- [TaskRunStatus](#taskrunstatus)
- [TaskRunStatusFields](#taskrunstatusfields)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `configSource` _[ConfigSource](#configsource)_ | Deprecated: Use RefSource instead |  |  |
| `refSource` _[RefSource](#refsource)_ | RefSource identifies the source where a remote task/pipeline came from. |  |  |
| `featureFlags` _[FeatureFlags](#featureflags)_ | FeatureFlags identifies the feature flags that were used during the task/pipeline run |  |  |


#### Ref



Ref can be used to refer to a specific instance of a StepAction.



_Appears in:_
- [Step](#step)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the referenced step |  |  |
| `ResolverRef` _[ResolverRef](#resolverref)_ | ResolverRef allows referencing a StepAction in a remote location<br />like a git repo. |  | Optional: \{\} <br /> |


#### RefSource



RefSource contains the information that can uniquely identify where a remote
built definition came from i.e. Git repositories, Tekton Bundles in OCI registry
and hub.



_Appears in:_
- [Provenance](#provenance)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `uri` _string_ | URI indicates the identity of the source of the build definition.<br />Example: "https://github.com/tektoncd/catalog" |  |  |
| `digest` _object (keys:string, values:string)_ | Digest is a collection of cryptographic digests for the contents of the artifact specified by URI.<br />Example: \{"sha1": "f99d13e554ffcb696dee719fa85b695cb5b0f428"\} |  |  |
| `entryPoint` _string_ | EntryPoint identifies the entry point into the build. This is often a path to a<br />build definition file and/or a target label within that file.<br />Example: "task/git-clone/0.10/git-clone.yaml" |  |  |


#### ResolverName

_Underlying type:_ _string_

ResolverName is the name of a resolver from which a resource can be
requested.



_Appears in:_
- [ResolverRef](#resolverref)



#### ResolverRef



ResolverRef can be used to refer to a Pipeline or Task in a remote
location like a git repo.



_Appears in:_
- [PipelineRef](#pipelineref)
- [Ref](#ref)
- [TaskRef](#taskref)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `resolver` _[ResolverName](#resolvername)_ | Resolver is the name of the resolver that should perform<br />resolution of the referenced Tekton resource, such as "git". |  | Optional: \{\} <br /> |
| `params` _[Params](#params)_ | Params contains the parameters used to identify the<br />referenced Tekton resource. Example entries might include<br />"repo" or "path" but the set of params ultimately depends on<br />the chosen resolver. |  | Optional: \{\} <br /> |












#### ResultsType

_Underlying type:_ _string_

ResultsType indicates the type of a result;
Used to distinguish between a single string and an array of strings.
Note that there is ResultType used to find out whether a
RunResult is from a task result or not, which is different from
this ResultsType.



_Appears in:_
- [TaskResult](#taskresult)
- [TaskRunResult](#taskrunresult)

| Field | Description |
| --- | --- |
| `string` |  |
| `array` |  |
| `object` |  |


#### RetriesStatus

_Underlying type:_ _[TaskRunStatus](#taskrunstatus)_





_Appears in:_
- [TaskRunStatus](#taskrunstatus)
- [TaskRunStatusFields](#taskrunstatusfields)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `Status` _[Status](https://pkg.go.dev/knative.dev/pkg/apis/duck/v1#Status)_ |  |  |  |
| `TaskRunStatusFields` _[TaskRunStatusFields](#taskrunstatusfields)_ | TaskRunStatusFields inlines the status fields. |  |  |








#### Sidecar



Sidecar has nearly the same data structure as Step but does not have the ability to timeout.



_Appears in:_
- [EmbeddedTask](#embeddedtask)
- [TaskSpec](#taskspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the Sidecar specified as a DNS_LABEL.<br />Each Sidecar in a Task must have a unique name (DNS_LABEL).<br />Cannot be updated. |  |  |
| `image` _string_ | Image name to be used by the Sidecar.<br />More info: https://kubernetes.io/docs/concepts/containers/images |  | Optional: \{\} <br /> |
| `command` _string array_ | Entrypoint array. Not executed within a shell.<br />The image's ENTRYPOINT is used if this is not provided.<br />Variable references $(VAR_NAME) are expanded using the Sidecar's environment. If a variable<br />cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced<br />to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)" will<br />produce the string literal "$(VAR_NAME)". Escaped references will never be expanded, regardless<br />of whether the variable exists or not. Cannot be updated.<br />More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell |  | Optional: \{\} <br /> |
| `args` _string array_ | Arguments to the entrypoint.<br />The image's CMD is used if this is not provided.<br />Variable references $(VAR_NAME) are expanded using the container's environment. If a variable<br />cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced<br />to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)" will<br />produce the string literal "$(VAR_NAME)". Escaped references will never be expanded, regardless<br />of whether the variable exists or not. Cannot be updated.<br />More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell |  | Optional: \{\} <br /> |
| `workingDir` _string_ | Sidecar's working directory.<br />If not specified, the container runtime's default will be used, which<br />might be configured in the container image.<br />Cannot be updated. |  | Optional: \{\} <br /> |
| `ports` _[ContainerPort](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#containerport-v1-core) array_ | List of ports to expose from the Sidecar. Exposing a port here gives<br />the system additional information about the network connections a<br />container uses, but is primarily informational. Not specifying a port here<br />DOES NOT prevent that port from being exposed. Any port which is<br />listening on the default "0.0.0.0" address inside a container will be<br />accessible from the network.<br />Cannot be updated. |  | Optional: \{\} <br /> |
| `envFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envfromsource-v1-core) array_ | List of sources to populate environment variables in the Sidecar.<br />The keys defined within a source must be a C_IDENTIFIER. All invalid keys<br />will be reported as an event when the Sidecar is starting. When a key exists in multiple<br />sources, the value associated with the last source will take precedence.<br />Values defined by an Env with a duplicate key will take precedence.<br />Cannot be updated. |  | Optional: \{\} <br /> |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envvar-v1-core) array_ | List of environment variables to set in the Sidecar.<br />Cannot be updated. |  | Optional: \{\} <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Compute Resources required by this Sidecar.<br />Cannot be updated.<br />More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/ |  | Optional: \{\} <br /> |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ | Volumes to mount into the Sidecar's filesystem.<br />Cannot be updated. |  | Optional: \{\} <br /> |
| `volumeDevices` _[VolumeDevice](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumedevice-v1-core) array_ | volumeDevices is the list of block devices to be used by the Sidecar. |  | Optional: \{\} <br /> |
| `livenessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#probe-v1-core)_ | Periodic probe of Sidecar liveness.<br />Container will be restarted if the probe fails.<br />Cannot be updated.<br />More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes |  | Optional: \{\} <br /> |
| `readinessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#probe-v1-core)_ | Periodic probe of Sidecar service readiness.<br />Container will be removed from service endpoints if the probe fails.<br />Cannot be updated.<br />More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes |  | Optional: \{\} <br /> |
| `startupProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#probe-v1-core)_ | StartupProbe indicates that the Pod the Sidecar is running in has successfully initialized.<br />If specified, no other probes are executed until this completes successfully.<br />If this probe fails, the Pod will be restarted, just as if the livenessProbe failed.<br />This can be used to provide different probe parameters at the beginning of a Pod's lifecycle,<br />when it might take a long time to load data or warm a cache, than during steady-state operation.<br />This cannot be updated.<br />More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes |  | Optional: \{\} <br /> |
| `lifecycle` _[Lifecycle](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#lifecycle-v1-core)_ | Actions that the management system should take in response to Sidecar lifecycle events.<br />Cannot be updated. |  | Optional: \{\} <br /> |
| `terminationMessagePath` _string_ | Optional: Path at which the file to which the Sidecar's termination message<br />will be written is mounted into the Sidecar's filesystem.<br />Message written is intended to be brief final status, such as an assertion failure message.<br />Will be truncated by the node if greater than 4096 bytes. The total message length across<br />all containers will be limited to 12kb.<br />Defaults to /dev/termination-log.<br />Cannot be updated. |  | Optional: \{\} <br /> |
| `terminationMessagePolicy` _[TerminationMessagePolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#terminationmessagepolicy-v1-core)_ | Indicate how the termination message should be populated. File will use the contents of<br />terminationMessagePath to populate the Sidecar status message on both success and failure.<br />FallbackToLogsOnError will use the last chunk of Sidecar log output if the termination<br />message file is empty and the Sidecar exited with an error.<br />The log output is limited to 2048 bytes or 80 lines, whichever is smaller.<br />Defaults to File.<br />Cannot be updated. |  | Optional: \{\} <br /> |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#pullpolicy-v1-core)_ | Image pull policy.<br />One of Always, Never, IfNotPresent.<br />Defaults to Always if :latest tag is specified, or IfNotPresent otherwise.<br />Cannot be updated.<br />More info: https://kubernetes.io/docs/concepts/containers/images#updating-images |  | Optional: \{\} <br /> |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#securitycontext-v1-core)_ | SecurityContext defines the security options the Sidecar should be run with.<br />If set, the fields of SecurityContext override the equivalent fields of PodSecurityContext.<br />More info: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/ |  | Optional: \{\} <br /> |
| `stdin` _boolean_ | Whether this Sidecar should allocate a buffer for stdin in the container runtime. If this<br />is not set, reads from stdin in the Sidecar will always result in EOF.<br />Default is false. |  | Optional: \{\} <br /> |
| `stdinOnce` _boolean_ | Whether the container runtime should close the stdin channel after it has been opened by<br />a single attach. When stdin is true the stdin stream will remain open across multiple attach<br />sessions. If stdinOnce is set to true, stdin is opened on Sidecar start, is empty until the<br />first client attaches to stdin, and then remains open and accepts data until the client disconnects,<br />at which time stdin is closed and remains closed until the Sidecar is restarted. If this<br />flag is false, a container processes that reads from stdin will never receive an EOF.<br />Default is false |  | Optional: \{\} <br /> |
| `tty` _boolean_ | Whether this Sidecar should allocate a TTY for itself, also requires 'stdin' to be true.<br />Default is false. |  | Optional: \{\} <br /> |
| `script` _string_ | Script is the contents of an executable file to execute.<br />If Script is not empty, the Step cannot have an Command or Args. |  | Optional: \{\} <br /> |
| `workspaces` _[WorkspaceUsage](#workspaceusage) array_ | This is an alpha field. You must set the "enable-api-fields" feature flag to "alpha"<br />for this field to be supported.<br />Workspaces is a list of workspaces from the Task that this Sidecar wants<br />exclusive access to. Adding a workspace to this list means that any<br />other Step or Sidecar that does not also request this Workspace will<br />not have access to it. |  | Optional: \{\} <br /> |
| `restartPolicy` _[ContainerRestartPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#containerrestartpolicy-v1-core)_ | RestartPolicy refers to kubernetes RestartPolicy. It can only be set for an<br />initContainer and must have it's policy set to "Always". It is currently<br />left optional to help support Kubernetes versions prior to 1.29 when this feature<br />was introduced. |  | Optional: \{\} <br /> |


#### SidecarState



SidecarState reports the results of running a sidecar in a Task.



_Appears in:_
- [TaskRunStatus](#taskrunstatus)
- [TaskRunStatusFields](#taskrunstatusfields)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `waiting` _[ContainerStateWaiting](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#containerstatewaiting-v1-core)_ | Details about a waiting container |  | Optional: \{\} <br /> |
| `running` _[ContainerStateRunning](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#containerstaterunning-v1-core)_ | Details about a running container |  | Optional: \{\} <br /> |
| `terminated` _[ContainerStateTerminated](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#containerstateterminated-v1-core)_ | Details about a terminated container |  | Optional: \{\} <br /> |
| `name` _string_ |  |  |  |
| `container` _string_ |  |  |  |
| `imageID` _string_ |  |  |  |


#### SkippedTask



SkippedTask is used to describe the Tasks that were skipped due to their When Expressions
evaluating to False. This is a struct because we are looking into including more details
about the When Expressions that caused this Task to be skipped.



_Appears in:_
- [PipelineRunStatus](#pipelinerunstatus)
- [PipelineRunStatusFields](#pipelinerunstatusfields)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the Pipeline Task name |  |  |
| `reason` _[SkippingReason](#skippingreason)_ | Reason is the cause of the PipelineTask being skipped. |  |  |
| `whenExpressions` _[WhenExpression](#whenexpression) array_ | WhenExpressions is the list of checks guarding the execution of the PipelineTask |  | Optional: \{\} <br /> |


#### SkippingReason

_Underlying type:_ _string_

SkippingReason explains why a PipelineTask was skipped.



_Appears in:_
- [SkippedTask](#skippedtask)

| Field | Description |
| --- | --- |
| `When Expressions evaluated to false` | WhenExpressionsSkip means the task was skipped due to at least one of its when expressions evaluating to false<br /> |
| `Parent Tasks were skipped` | ParentTasksSkip means the task was skipped because its parent was skipped<br /> |
| `PipelineRun was stopping` | StoppingSkip means the task was skipped because the pipeline run is stopping<br /> |
| `PipelineRun was gracefully cancelled` | GracefullyCancelledSkip means the task was skipped because the pipeline run has been gracefully cancelled<br /> |
| `PipelineRun was gracefully stopped` | GracefullyStoppedSkip means the task was skipped because the pipeline run has been gracefully stopped<br /> |
| `Results were missing` | MissingResultsSkip means the task was skipped because it's missing necessary results<br /> |
| `PipelineRun timeout has been reached` | PipelineTimedOutSkip means the task was skipped because the PipelineRun has passed its overall timeout.<br /> |
| `PipelineRun Tasks timeout has been reached` | TasksTimedOutSkip means the task was skipped because the PipelineRun has passed its Timeouts.Tasks.<br /> |
| `PipelineRun Finally timeout has been reached` | FinallyTimedOutSkip means the task was skipped because the PipelineRun has passed its Timeouts.Finally.<br /> |
| `Matrix Parameters have an empty array` | EmptyArrayInMatrixParams means the task was skipped because Matrix parameters contain empty array.<br /> |
| `None` | None means the task was not skipped<br /> |


#### Step



Step runs a subcomponent of a Task



_Appears in:_
- [EmbeddedTask](#embeddedtask)
- [InternalTaskModifier](#internaltaskmodifier)
- [TaskSpec](#taskspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the Step specified as a DNS_LABEL.<br />Each Step in a Task must have a unique name. |  |  |
| `displayName` _string_ | DisplayName is a user-facing name of the step that may be<br />used to populate a UI. |  | Optional: \{\} <br /> |
| `image` _string_ | Image reference name to run for this Step.<br />More info: https://kubernetes.io/docs/concepts/containers/images |  | Optional: \{\} <br /> |
| `command` _string array_ | Entrypoint array. Not executed within a shell.<br />The image's ENTRYPOINT is used if this is not provided.<br />Variable references $(VAR_NAME) are expanded using the container's environment. If a variable<br />cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced<br />to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)" will<br />produce the string literal "$(VAR_NAME)". Escaped references will never be expanded, regardless<br />of whether the variable exists or not. Cannot be updated.<br />More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell |  | Optional: \{\} <br /> |
| `args` _string array_ | Arguments to the entrypoint.<br />The image's CMD is used if this is not provided.<br />Variable references $(VAR_NAME) are expanded using the container's environment. If a variable<br />cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced<br />to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)" will<br />produce the string literal "$(VAR_NAME)". Escaped references will never be expanded, regardless<br />of whether the variable exists or not. Cannot be updated.<br />More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell |  | Optional: \{\} <br /> |
| `workingDir` _string_ | Step's working directory.<br />If not specified, the container runtime's default will be used, which<br />might be configured in the container image.<br />Cannot be updated. |  | Optional: \{\} <br /> |
| `ports` _[ContainerPort](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#containerport-v1-core) array_ | List of ports to expose from the Step's container. Exposing a port here gives<br />the system additional information about the network connections a<br />container uses, but is primarily informational. Not specifying a port here<br />DOES NOT prevent that port from being exposed. Any port which is<br />listening on the default "0.0.0.0" address inside a container will be<br />accessible from the network.<br />Cannot be updated.<br />Deprecated: This field will be removed in a future release. |  | Optional: \{\} <br /> |
| `envFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envfromsource-v1-core) array_ | List of sources to populate environment variables in the container.<br />The keys defined within a source must be a C_IDENTIFIER. All invalid keys<br />will be reported as an event when the container is starting. When a key exists in multiple<br />sources, the value associated with the last source will take precedence.<br />Values defined by an Env with a duplicate key will take precedence.<br />Cannot be updated. |  | Optional: \{\} <br /> |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envvar-v1-core) array_ | List of environment variables to set in the container.<br />Cannot be updated. |  | Optional: \{\} <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Compute Resources required by this Step.<br />Cannot be updated.<br />More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/ |  | Optional: \{\} <br /> |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ | Volumes to mount into the Step's filesystem.<br />Cannot be updated. |  | Optional: \{\} <br /> |
| `volumeDevices` _[VolumeDevice](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumedevice-v1-core) array_ | volumeDevices is the list of block devices to be used by the Step. |  | Optional: \{\} <br /> |
| `livenessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#probe-v1-core)_ | Periodic probe of container liveness.<br />Step will be restarted if the probe fails.<br />Cannot be updated.<br />More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes<br />Deprecated: This field will be removed in a future release. |  | Optional: \{\} <br /> |
| `readinessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#probe-v1-core)_ | Periodic probe of container service readiness.<br />Step will be removed from service endpoints if the probe fails.<br />Cannot be updated.<br />More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes<br />Deprecated: This field will be removed in a future release. |  | Optional: \{\} <br /> |
| `startupProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#probe-v1-core)_ | DeprecatedStartupProbe indicates that the Pod this Step runs in has successfully initialized.<br />If specified, no other probes are executed until this completes successfully.<br />If this probe fails, the Pod will be restarted, just as if the livenessProbe failed.<br />This can be used to provide different probe parameters at the beginning of a Pod's lifecycle,<br />when it might take a long time to load data or warm a cache, than during steady-state operation.<br />This cannot be updated.<br />More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes<br />Deprecated: This field will be removed in a future release. |  | Optional: \{\} <br /> |
| `lifecycle` _[Lifecycle](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#lifecycle-v1-core)_ | Actions that the management system should take in response to container lifecycle events.<br />Cannot be updated.<br />Deprecated: This field will be removed in a future release. |  | Optional: \{\} <br /> |
| `terminationMessagePath` _string_ | Deprecated: This field will be removed in a future release and can't be meaningfully used. |  | Optional: \{\} <br /> |
| `terminationMessagePolicy` _[TerminationMessagePolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#terminationmessagepolicy-v1-core)_ | Deprecated: This field will be removed in a future release and can't be meaningfully used. |  | Optional: \{\} <br /> |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#pullpolicy-v1-core)_ | Image pull policy.<br />One of Always, Never, IfNotPresent.<br />Defaults to Always if :latest tag is specified, or IfNotPresent otherwise.<br />Cannot be updated.<br />More info: https://kubernetes.io/docs/concepts/containers/images#updating-images |  | Optional: \{\} <br /> |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#securitycontext-v1-core)_ | SecurityContext defines the security options the Step should be run with.<br />If set, the fields of SecurityContext override the equivalent fields of PodSecurityContext.<br />More info: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/ |  | Optional: \{\} <br /> |
| `stdin` _boolean_ | Whether this container should allocate a buffer for stdin in the container runtime. If this<br />is not set, reads from stdin in the container will always result in EOF.<br />Default is false.<br />Deprecated: This field will be removed in a future release. |  | Optional: \{\} <br /> |
| `stdinOnce` _boolean_ | Whether the container runtime should close the stdin channel after it has been opened by<br />a single attach. When stdin is true the stdin stream will remain open across multiple attach<br />sessions. If stdinOnce is set to true, stdin is opened on container start, is empty until the<br />first client attaches to stdin, and then remains open and accepts data until the client disconnects,<br />at which time stdin is closed and remains closed until the container is restarted. If this<br />flag is false, a container processes that reads from stdin will never receive an EOF.<br />Default is false<br />Deprecated: This field will be removed in a future release. |  | Optional: \{\} <br /> |
| `tty` _boolean_ | Whether this container should allocate a DeprecatedTTY for itself, also requires 'stdin' to be true.<br />Default is false.<br />Deprecated: This field will be removed in a future release. |  | Optional: \{\} <br /> |
| `script` _string_ | Script is the contents of an executable file to execute.<br />If Script is not empty, the Step cannot have an Command and the Args will be passed to the Script. |  | Optional: \{\} <br /> |
| `timeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | Timeout is the time after which the step times out. Defaults to never.<br />Refer to Go's ParseDuration documentation for expected format: https://golang.org/pkg/time/#ParseDuration |  | Optional: \{\} <br /> |
| `workspaces` _[WorkspaceUsage](#workspaceusage) array_ | This is an alpha field. You must set the "enable-api-fields" feature flag to "alpha"<br />for this field to be supported.<br />Workspaces is a list of workspaces from the Task that this Step wants<br />exclusive access to. Adding a workspace to this list means that any<br />other Step or Sidecar that does not also request this Workspace will<br />not have access to it. |  | Optional: \{\} <br /> |
| `onError` _[OnErrorType](#onerrortype)_ | OnError defines the exiting behavior of a container on error<br />can be set to [ continue \| stopAndFail ] |  |  |
| `stdoutConfig` _[StepOutputConfig](#stepoutputconfig)_ | Stores configuration for the stdout stream of the step. |  | Optional: \{\} <br /> |
| `stderrConfig` _[StepOutputConfig](#stepoutputconfig)_ | Stores configuration for the stderr stream of the step. |  | Optional: \{\} <br /> |
| `ref` _[Ref](#ref)_ | Contains the reference to an existing StepAction. |  | Optional: \{\} <br /> |
| `params` _[Params](#params)_ | Params declares parameters passed to this step action. |  | Optional: \{\} <br /> |
| `results` _[StepResult](#stepresult) array_ | Results declares StepResults produced by the Step.<br />It can be used in an inlined Step when used to store Results to $(step.results.resultName.path).<br />It cannot be used when referencing StepActions using [v1beta1.Step.Ref].<br />The Results declared by the StepActions will be stored here instead. |  | Optional: \{\} <br /> |
| `when` _[StepWhenExpressions](#stepwhenexpressions)_ |  |  |  |


#### StepAction



StepAction represents the actionable components of Step.
The Step can only reference it from the cluster or using remote resolution.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `tekton.dev/v1beta1` | | |
| `kind` _string_ | `StepAction` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[StepActionSpec](#stepactionspec)_ | Spec holds the desired state of the Step from the client |  | Optional: \{\} <br /> |




#### StepActionSpec



StepActionSpec contains the actionable components of a step.



_Appears in:_
- [StepAction](#stepaction)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `description` _string_ | Description is a user-facing description of the stepaction that may be<br />used to populate a UI. |  | Optional: \{\} <br /> |
| `image` _string_ | Image reference name to run for this StepAction.<br />More info: https://kubernetes.io/docs/concepts/containers/images |  | Optional: \{\} <br /> |
| `command` _string array_ | Entrypoint array. Not executed within a shell.<br />The image's ENTRYPOINT is used if this is not provided.<br />Variable references $(VAR_NAME) are expanded using the container's environment. If a variable<br />cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced<br />to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)" will<br />produce the string literal "$(VAR_NAME)". Escaped references will never be expanded, regardless<br />of whether the variable exists or not. Cannot be updated.<br />More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell |  | Optional: \{\} <br /> |
| `args` _[Args](#args)_ | Arguments to the entrypoint.<br />The image's CMD is used if this is not provided.<br />Variable references $(VAR_NAME) are expanded using the container's environment. If a variable<br />cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced<br />to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)" will<br />produce the string literal "$(VAR_NAME)". Escaped references will never be expanded, regardless<br />of whether the variable exists or not. Cannot be updated.<br />More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell |  | Optional: \{\} <br /> |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envvar-v1-core) array_ | List of environment variables to set in the container.<br />Cannot be updated. |  | Optional: \{\} <br /> |
| `script` _string_ | Script is the contents of an executable file to execute.<br />If Script is not empty, the Step cannot have an Command and the Args will be passed to the Script. |  | Optional: \{\} <br /> |
| `workingDir` _string_ | Step's working directory.<br />If not specified, the container runtime's default will be used, which<br />might be configured in the container image.<br />Cannot be updated. |  | Optional: \{\} <br /> |
| `params` _[ParamSpecs](#paramspecs)_ | Params is a list of input parameters required to run the stepAction.<br />Params must be supplied as inputs in Steps unless they declare a defaultvalue. |  | Optional: \{\} <br /> |
| `results` _[StepResult](#stepresult) array_ | Results are values that this StepAction can output |  | Optional: \{\} <br /> |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#securitycontext-v1-core)_ | SecurityContext defines the security options the Step should be run with.<br />If set, the fields of SecurityContext override the equivalent fields of PodSecurityContext.<br />More info: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/<br />The value set in StepAction will take precedence over the value from Task. |  | Optional: \{\} <br /> |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ | Volumes to mount into the Step's filesystem.<br />Cannot be updated. |  | Optional: \{\} <br /> |


#### StepOutputConfig



StepOutputConfig stores configuration for a step output stream.



_Appears in:_
- [Step](#step)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `path` _string_ | Path to duplicate stdout stream to on container's local filesystem. |  | Optional: \{\} <br /> |


#### StepState



StepState reports the results of running a step in a Task.



_Appears in:_
- [TaskRunStatus](#taskrunstatus)
- [TaskRunStatusFields](#taskrunstatusfields)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `waiting` _[ContainerStateWaiting](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#containerstatewaiting-v1-core)_ | Details about a waiting container |  | Optional: \{\} <br /> |
| `running` _[ContainerStateRunning](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#containerstaterunning-v1-core)_ | Details about a running container |  | Optional: \{\} <br /> |
| `terminated` _[ContainerStateTerminated](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#containerstateterminated-v1-core)_ | Details about a terminated container |  | Optional: \{\} <br /> |
| `name` _string_ |  |  |  |
| `container` _string_ |  |  |  |
| `imageID` _string_ |  |  |  |
| `results` _[TaskRunStepResult](#taskrunstepresult) array_ |  |  |  |
| `provenance` _[Provenance](#provenance)_ |  |  |  |
| `inputs` _[TaskRunStepArtifact](#taskrunstepartifact) array_ |  |  |  |
| `outputs` _[TaskRunStepArtifact](#taskrunstepartifact) array_ |  |  |  |


#### StepTemplate



StepTemplate is a template for a Step



_Appears in:_
- [EmbeddedTask](#embeddedtask)
- [TaskSpec](#taskspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Default name for each Step specified as a DNS_LABEL.<br />Each Step in a Task must have a unique name.<br />Cannot be updated.<br />Deprecated: This field will be removed in a future release. |  |  |
| `image` _string_ | Default image name to use for each Step.<br />More info: https://kubernetes.io/docs/concepts/containers/images<br />This field is optional to allow higher level config management to default or override<br />container images in workload controllers like Deployments and StatefulSets. |  | Optional: \{\} <br /> |
| `command` _string array_ | Entrypoint array. Not executed within a shell.<br />The docker image's ENTRYPOINT is used if this is not provided.<br />Variable references $(VAR_NAME) are expanded using the Step's environment. If a variable<br />cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced<br />to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)" will<br />produce the string literal "$(VAR_NAME)". Escaped references will never be expanded, regardless<br />of whether the variable exists or not. Cannot be updated.<br />More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell |  | Optional: \{\} <br /> |
| `args` _string array_ | Arguments to the entrypoint.<br />The image's CMD is used if this is not provided.<br />Variable references $(VAR_NAME) are expanded using the Step's environment. If a variable<br />cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced<br />to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)" will<br />produce the string literal "$(VAR_NAME)". Escaped references will never be expanded, regardless<br />of whether the variable exists or not. Cannot be updated.<br />More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell |  | Optional: \{\} <br /> |
| `workingDir` _string_ | Step's working directory.<br />If not specified, the container runtime's default will be used, which<br />might be configured in the container image.<br />Cannot be updated. |  | Optional: \{\} <br /> |
| `ports` _[ContainerPort](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#containerport-v1-core) array_ | List of ports to expose from the Step's container. Exposing a port here gives<br />the system additional information about the network connections a<br />container uses, but is primarily informational. Not specifying a port here<br />DOES NOT prevent that port from being exposed. Any port which is<br />listening on the default "0.0.0.0" address inside a container will be<br />accessible from the network.<br />Cannot be updated.<br />Deprecated: This field will be removed in a future release. |  | Optional: \{\} <br /> |
| `envFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envfromsource-v1-core) array_ | List of sources to populate environment variables in the Step.<br />The keys defined within a source must be a C_IDENTIFIER. All invalid keys<br />will be reported as an event when the container is starting. When a key exists in multiple<br />sources, the value associated with the last source will take precedence.<br />Values defined by an Env with a duplicate key will take precedence.<br />Cannot be updated. |  | Optional: \{\} <br /> |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envvar-v1-core) array_ | List of environment variables to set in the container.<br />Cannot be updated. |  | Optional: \{\} <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Compute Resources required by this Step.<br />Cannot be updated.<br />More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/ |  | Optional: \{\} <br /> |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ | Volumes to mount into the Step's filesystem.<br />Cannot be updated. |  | Optional: \{\} <br /> |
| `volumeDevices` _[VolumeDevice](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumedevice-v1-core) array_ | volumeDevices is the list of block devices to be used by the Step. |  | Optional: \{\} <br /> |
| `livenessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#probe-v1-core)_ | Periodic probe of container liveness.<br />Container will be restarted if the probe fails.<br />Cannot be updated.<br />More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes<br />Deprecated: This field will be removed in a future release. |  | Optional: \{\} <br /> |
| `readinessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#probe-v1-core)_ | Periodic probe of container service readiness.<br />Container will be removed from service endpoints if the probe fails.<br />Cannot be updated.<br />More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes<br />Deprecated: This field will be removed in a future release. |  | Optional: \{\} <br /> |
| `startupProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#probe-v1-core)_ | DeprecatedStartupProbe indicates that the Pod has successfully initialized.<br />If specified, no other probes are executed until this completes successfully.<br />If this probe fails, the Pod will be restarted, just as if the livenessProbe failed.<br />This can be used to provide different probe parameters at the beginning of a Pod's lifecycle,<br />when it might take a long time to load data or warm a cache, than during steady-state operation.<br />This cannot be updated.<br />More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes<br />Deprecated: This field will be removed in a future release. |  | Optional: \{\} <br /> |
| `lifecycle` _[Lifecycle](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#lifecycle-v1-core)_ | Actions that the management system should take in response to container lifecycle events.<br />Cannot be updated.<br />Deprecated: This field will be removed in a future release. |  | Optional: \{\} <br /> |
| `terminationMessagePath` _string_ | Deprecated: This field will be removed in a future release and cannot be meaningfully used. |  | Optional: \{\} <br /> |
| `terminationMessagePolicy` _[TerminationMessagePolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#terminationmessagepolicy-v1-core)_ | Deprecated: This field will be removed in a future release and cannot be meaningfully used. |  | Optional: \{\} <br /> |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#pullpolicy-v1-core)_ | Image pull policy.<br />One of Always, Never, IfNotPresent.<br />Defaults to Always if :latest tag is specified, or IfNotPresent otherwise.<br />Cannot be updated.<br />More info: https://kubernetes.io/docs/concepts/containers/images#updating-images |  | Optional: \{\} <br /> |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#securitycontext-v1-core)_ | SecurityContext defines the security options the Step should be run with.<br />If set, the fields of SecurityContext override the equivalent fields of PodSecurityContext.<br />More info: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/ |  | Optional: \{\} <br /> |
| `stdin` _boolean_ | Whether this Step should allocate a buffer for stdin in the container runtime. If this<br />is not set, reads from stdin in the Step will always result in EOF.<br />Default is false.<br />Deprecated: This field will be removed in a future release. |  | Optional: \{\} <br /> |
| `stdinOnce` _boolean_ | Whether the container runtime should close the stdin channel after it has been opened by<br />a single attach. When stdin is true the stdin stream will remain open across multiple attach<br />sessions. If stdinOnce is set to true, stdin is opened on container start, is empty until the<br />first client attaches to stdin, and then remains open and accepts data until the client disconnects,<br />at which time stdin is closed and remains closed until the container is restarted. If this<br />flag is false, a container processes that reads from stdin will never receive an EOF.<br />Default is false<br />Deprecated: This field will be removed in a future release. |  | Optional: \{\} <br /> |
| `tty` _boolean_ | Whether this Step should allocate a DeprecatedTTY for itself, also requires 'stdin' to be true.<br />Default is false.<br />Deprecated: This field will be removed in a future release. |  | Optional: \{\} <br /> |




#### Task



Task represents a collection of sequential steps that are run as part of a
Pipeline using a set of inputs and producing a set of outputs. Tasks execute
when TaskRuns are created that provide the input parameters and resources and
output resources the Task requires.

Deprecated: Please use v1.Task instead.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `tekton.dev/v1beta1` | | |
| `kind` _string_ | `Task` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[TaskSpec](#taskspec)_ | Spec holds the desired state of the Task from the client |  | Optional: \{\} <br /> |


#### TaskBreakpoints



TaskBreakpoints defines the breakpoint config for a particular Task



_Appears in:_
- [TaskRunDebug](#taskrundebug)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `onFailure` _string_ | if enabled, pause TaskRun on failure of a step<br />failed step will not exit |  | Optional: \{\} <br /> |
| `beforeSteps` _string array_ |  |  | Optional: \{\} <br /> |


#### TaskKind

_Underlying type:_ _string_

TaskKind defines the type of Task used by the pipeline.



_Appears in:_
- [TaskRef](#taskref)

| Field | Description |
| --- | --- |
| `Task` | NamespacedTaskKind indicates that the task type has a namespaced scope.<br /> |






#### TaskRef



TaskRef can be used to refer to a specific instance of a task.



_Appears in:_
- [CustomRunSpec](#customrunspec)
- [PipelineTask](#pipelinetask)
- [RunSpec](#runspec)
- [TaskRunSpec](#taskrunspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the referent; More info: http://kubernetes.io/docs/user-guide/identifiers#names |  |  |
| `kind` _[TaskKind](#taskkind)_ | TaskKind indicates the Kind of the Task:<br />1. Namespaced Task when Kind is set to "Task". If Kind is "", it defaults to "Task".<br />2. Custom Task when Kind is non-empty and APIVersion is non-empty |  |  |
| `apiVersion` _string_ | API version of the referent<br />Note: A Task with non-empty APIVersion and Kind is considered a Custom Task |  | Optional: \{\} <br /> |
| `bundle` _string_ | Bundle url reference to a Tekton Bundle.<br />Deprecated: Please use ResolverRef with the bundles resolver instead.<br />The field is staying there for go client backward compatibility, but is not used/allowed anymore. |  | Optional: \{\} <br /> |
| `ResolverRef` _[ResolverRef](#resolverref)_ | ResolverRef allows referencing a Task in a remote location<br />like a git repo. This field is only supported when the alpha<br />feature gate is enabled. |  | Optional: \{\} <br /> |


#### TaskResource



TaskResource defines an input or output Resource declared as a requirement
by a Task. The Name field will be used to refer to these Resources within
the Task definition, and when provided as an Input, the Name will be the
path to the volume mounted containing this Resource as an input (e.g.
an input Resource named `workspace` will be mounted at `/workspace`).

Deprecated: Unused, preserved only for backwards compatibility



_Appears in:_
- [TaskResources](#taskresources)



#### TaskResourceBinding



TaskResourceBinding points to the PipelineResource that
will be used for the Task input or output called Name.

Deprecated: Unused, preserved only for backwards compatibility



_Appears in:_
- [TaskRunInputs](#taskruninputs)
- [TaskRunOutputs](#taskrunoutputs)
- [TaskRunResources](#taskrunresources)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the PipelineResource in the Pipeline's declaration |  |  |
| `resourceRef` _[PipelineResourceRef](#pipelineresourceref)_ | ResourceRef is a reference to the instance of the actual PipelineResource<br />that should be used |  | Optional: \{\} <br /> |
| `resourceSpec` _[PipelineResourceSpec](#pipelineresourcespec)_ | ResourceSpec is specification of a resource that should be created and<br />consumed by the task |  | Optional: \{\} <br /> |
| `paths` _string array_ | Paths will probably be removed in #1284, and then PipelineResourceBinding can be used instead.<br />The optional Path field corresponds to a path on disk at which the Resource can be found<br />(used when providing the resource via mounted volume, overriding the default logic to fetch the Resource). |  | Optional: \{\} <br /> |


#### TaskResources



TaskResources allows a Pipeline to declare how its DeclaredPipelineResources
should be provided to a Task as its inputs and outputs.

Deprecated: Unused, preserved only for backwards compatibility



_Appears in:_
- [EmbeddedTask](#embeddedtask)
- [TaskSpec](#taskspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `inputs` _[TaskResource](#taskresource) array_ | Inputs holds the mapping from the PipelineResources declared in<br />DeclaredPipelineResources to the input PipelineResources required by the Task. |  |  |
| `outputs` _[TaskResource](#taskresource) array_ | Outputs holds the mapping from the PipelineResources declared in<br />DeclaredPipelineResources to the input PipelineResources required by the Task. |  |  |


#### TaskResult



TaskResult used to describe the results of a task



_Appears in:_
- [EmbeddedTask](#embeddedtask)
- [TaskSpec](#taskspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name the given name |  |  |
| `type` _[ResultsType](#resultstype)_ | Type is the user-specified type of the result. The possible type<br />is currently "string" and will support "array" in following work. |  | Optional: \{\} <br /> |
| `properties` _object (keys:string, values:[PropertySpec](#propertyspec))_ | Properties is the JSON Schema properties to support key-value pairs results. |  | Optional: \{\} <br /> |
| `description` _string_ | Description is a human-readable description of the result |  | Optional: \{\} <br /> |
| `value` _[ResultValue](#resultvalue)_ | Value the expression used to retrieve the value of the result from an underlying Step. |  | Schemaless: \{\} <br />Optional: \{\} <br /> |


#### TaskRun



TaskRun represents a single execution of a Task. TaskRuns are how the steps
specified in a Task are executed; they specify the parameters and resources
used to run the steps in a Task.

Deprecated: Please use v1.TaskRun instead.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `tekton.dev/v1beta1` | | |
| `kind` _string_ | `TaskRun` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[TaskRunSpec](#taskrunspec)_ |  |  | Optional: \{\} <br /> |
| `status` _[TaskRunStatus](#taskrunstatus)_ |  |  | Optional: \{\} <br /> |




#### TaskRunDebug



TaskRunDebug defines the breakpoint config for a particular TaskRun



_Appears in:_
- [TaskRunSpec](#taskrunspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `breakpoints` _[TaskBreakpoints](#taskbreakpoints)_ |  |  | Optional: \{\} <br /> |








#### TaskRunResources



TaskRunResources allows a TaskRun to declare inputs and outputs TaskResourceBinding

Deprecated: Unused, preserved only for backwards compatibility



_Appears in:_
- [TaskRunSpec](#taskrunspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `inputs` _[TaskResourceBinding](#taskresourcebinding) array_ | Inputs holds the inputs resources this task was invoked with |  |  |
| `outputs` _[TaskResourceBinding](#taskresourcebinding) array_ | Outputs holds the inputs resources this task was invoked with |  |  |


#### TaskRunResult



TaskRunResult used to describe the results of a task



_Appears in:_
- [TaskRunStatus](#taskrunstatus)
- [TaskRunStatusFields](#taskrunstatusfields)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name the given name |  |  |
| `type` _[ResultsType](#resultstype)_ | Type is the user-specified type of the result. The possible type<br />is currently "string" and will support "array" in following work. |  | Optional: \{\} <br /> |
| `value` _[ResultValue](#resultvalue)_ | Value the given value of the result |  | Schemaless: \{\} <br /> |


#### TaskRunSidecarOverride



TaskRunSidecarOverride is used to override the values of a Sidecar in the corresponding Task.



_Appears in:_
- [PipelineTaskRunSpec](#pipelinetaskrunspec)
- [TaskRunSpec](#taskrunspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | The name of the Sidecar to override. |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | The resource requirements to apply to the Sidecar. |  |  |


#### TaskRunSpec



TaskRunSpec defines the desired state of TaskRun



_Appears in:_
- [TaskRun](#taskrun)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `debug` _[TaskRunDebug](#taskrundebug)_ |  |  | Optional: \{\} <br /> |
| `params` _[Params](#params)_ |  |  | Optional: \{\} <br /> |
| `resources` _[TaskRunResources](#taskrunresources)_ | Deprecated: Unused, preserved only for backwards compatibility |  | Optional: \{\} <br /> |
| `serviceAccountName` _string_ |  |  | Optional: \{\} <br /> |
| `taskRef` _[TaskRef](#taskref)_ | no more than one of the TaskRef and TaskSpec may be specified. |  | Optional: \{\} <br /> |
| `taskSpec` _[TaskSpec](#taskspec)_ | Specifying TaskSpec can be disabled by setting<br />`disable-inline-spec` feature flag.<br />See Task.spec (API version: tekton.dev/v1beta1) |  | Schemaless: \{\} <br />Optional: \{\} <br /> |
| `status` _[TaskRunSpecStatus](#taskrunspecstatus)_ | Used for cancelling a TaskRun (and maybe more later on) |  | Optional: \{\} <br /> |
| `statusMessage` _[TaskRunSpecStatusMessage](#taskrunspecstatusmessage)_ | Status message for cancellation. |  | Optional: \{\} <br /> |
| `retries` _integer_ | Retries represents how many times this TaskRun should be retried in the event of Task failure. |  | Optional: \{\} <br /> |
| `timeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | Time after which one retry attempt times out. Defaults to 1 hour.<br />Refer Go's ParseDuration documentation for expected format: https://golang.org/pkg/time/#ParseDuration |  | Optional: \{\} <br /> |
| `podTemplate` _[PodTemplate](#podtemplate)_ | PodTemplate holds pod specific configuration |  |  |
| `workspaces` _[WorkspaceBinding](#workspacebinding) array_ | Workspaces is a list of WorkspaceBindings from volumes to workspaces. |  | Optional: \{\} <br /> |
| `stepOverrides` _[TaskRunStepOverride](#taskrunstepoverride) array_ | Overrides to apply to Steps in this TaskRun.<br />If a field is specified in both a Step and a StepOverride,<br />the value from the StepOverride will be used.<br />This field is only supported when the alpha feature gate is enabled. |  | Optional: \{\} <br /> |
| `sidecarOverrides` _[TaskRunSidecarOverride](#taskrunsidecaroverride) array_ | Overrides to apply to Sidecars in this TaskRun.<br />If a field is specified in both a Sidecar and a SidecarOverride,<br />the value from the SidecarOverride will be used.<br />This field is only supported when the alpha feature gate is enabled. |  | Optional: \{\} <br /> |
| `computeResources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Compute resources to use for this TaskRun |  |  |
| `managedBy` _string_ | ManagedBy indicates which controller is responsible for reconciling<br />this resource. If unset or set to "tekton.dev/pipeline", the default<br />Tekton controller will manage this resource.<br />This field is immutable. |  | Optional: \{\} <br /> |


#### TaskRunSpecStatus

_Underlying type:_ _string_

TaskRunSpecStatus defines the TaskRun spec status the user can provide



_Appears in:_
- [TaskRunSpec](#taskrunspec)



#### TaskRunSpecStatusMessage

_Underlying type:_ _string_

TaskRunSpecStatusMessage defines human readable status messages for the TaskRun.



_Appears in:_
- [TaskRunSpec](#taskrunspec)

| Field | Description |
| --- | --- |
| `TaskRun cancelled as the PipelineRun it belongs to has been cancelled.` | TaskRunCancelledByPipelineMsg indicates that the PipelineRun of which this<br />TaskRun was a part of has been cancelled.<br /> |
| `TaskRun cancelled as the PipelineRun it belongs to has timed out.` | TaskRunCancelledByPipelineTimeoutMsg indicates that the TaskRun was cancelled because the PipelineRun running it timed out.<br /> |


#### TaskRunStatus



TaskRunStatus defines the observed state of TaskRun



_Appears in:_
- [PipelineRunTaskRunStatus](#pipelineruntaskrunstatus)
- [RetriesStatus](#retriesstatus)
- [TaskRun](#taskrun)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the 'Generation' of the Service that<br />was last processed by the controller. |  | Optional: \{\} <br /> |
| `conditions` _[Conditions](#conditions)_ | Conditions the latest available observations of a resource's current state. |  | Optional: \{\} <br /> |
| `annotations` _object (keys:string, values:string)_ | Annotations is additional Status fields for the Resource to save some<br />additional State as well as convey more information to the user. This is<br />roughly akin to Annotations on any k8s resource, just the reconciler conveying<br />richer information outwards. |  |  |
| `podName` _string_ | PodName is the name of the pod responsible for executing this task's steps. |  |  |
| `startTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#time-v1-meta)_ | StartTime is the time the build is actually started. |  |  |
| `completionTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#time-v1-meta)_ | CompletionTime is the time the build completed. |  |  |
| `steps` _[StepState](#stepstate) array_ | Steps describes the state of each build step container. |  | Optional: \{\} <br /> |
| `cloudEvents` _[CloudEventDelivery](#cloudeventdelivery) array_ | CloudEvents describe the state of each cloud event requested via a<br />CloudEventResource.<br />Deprecated: No content written to it. To be Removed (since v0.44.0).<br />Use kubectl describe (CloudEventSent/CloudEventFailed k8s Events) or the<br />tekton_events_sent_total Prometheus metric for delivery visibility instead. |  | Optional: \{\} <br /> |
| `retriesStatus` _[RetriesStatus](#retriesstatus)_ | RetriesStatus contains the history of TaskRunStatus in case of a retry in order to keep record of failures.<br />All TaskRunStatus stored in RetriesStatus will have no date within the RetriesStatus as is redundant.<br />See TaskRun.status (API version: tekton.dev/v1beta1) |  | Schemaless: \{\} <br />Optional: \{\} <br /> |
| `resourcesResult` _[PipelineResourceResult](#pipelineresourceresult) array_ | Results from Resources built during the TaskRun.<br />This is tomb-stoned along with the removal of pipelineResources<br />Deprecated: this field is not populated and is preserved only for backwards compatibility |  | Optional: \{\} <br /> |
| `taskResults` _[TaskRunResult](#taskrunresult) array_ | TaskRunResults are the list of results written out by the task's containers |  | Optional: \{\} <br /> |
| `sidecars` _[SidecarState](#sidecarstate) array_ | The list has one entry per sidecar in the manifest. Each entry is<br />represents the imageid of the corresponding sidecar. |  |  |
| `taskSpec` _[TaskSpec](#taskspec)_ | TaskSpec contains the Spec from the dereferenced Task definition used to instantiate this TaskRun.<br />See Task.spec (API version tekton.dev/v1beta1) |  | Schemaless: \{\} <br /> |
| `provenance` _[Provenance](#provenance)_ | Provenance contains some key authenticated metadata about how a software artifact was built (what sources, what inputs/outputs, etc.). |  | Optional: \{\} <br /> |
| `spanContext` _object (keys:string, values:string)_ | SpanContext contains tracing span context fields |  |  |


#### TaskRunStatusFields



TaskRunStatusFields holds the fields of TaskRun's status.  This is defined
separately and inlined so that other types can readily consume these fields
via duck typing.



_Appears in:_
- [TaskRunStatus](#taskrunstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podName` _string_ | PodName is the name of the pod responsible for executing this task's steps. |  |  |
| `startTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#time-v1-meta)_ | StartTime is the time the build is actually started. |  |  |
| `completionTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#time-v1-meta)_ | CompletionTime is the time the build completed. |  |  |
| `steps` _[StepState](#stepstate) array_ | Steps describes the state of each build step container. |  | Optional: \{\} <br /> |
| `cloudEvents` _[CloudEventDelivery](#cloudeventdelivery) array_ | CloudEvents describe the state of each cloud event requested via a<br />CloudEventResource.<br />Deprecated: No content written to it. To be Removed (since v0.44.0).<br />Use kubectl describe (CloudEventSent/CloudEventFailed k8s Events) or the<br />tekton_events_sent_total Prometheus metric for delivery visibility instead. |  | Optional: \{\} <br /> |
| `retriesStatus` _[RetriesStatus](#retriesstatus)_ | RetriesStatus contains the history of TaskRunStatus in case of a retry in order to keep record of failures.<br />All TaskRunStatus stored in RetriesStatus will have no date within the RetriesStatus as is redundant.<br />See TaskRun.status (API version: tekton.dev/v1beta1) |  | Schemaless: \{\} <br />Optional: \{\} <br /> |
| `resourcesResult` _[PipelineResourceResult](#pipelineresourceresult) array_ | Results from Resources built during the TaskRun.<br />This is tomb-stoned along with the removal of pipelineResources<br />Deprecated: this field is not populated and is preserved only for backwards compatibility |  | Optional: \{\} <br /> |
| `taskResults` _[TaskRunResult](#taskrunresult) array_ | TaskRunResults are the list of results written out by the task's containers |  | Optional: \{\} <br /> |
| `sidecars` _[SidecarState](#sidecarstate) array_ | The list has one entry per sidecar in the manifest. Each entry is<br />represents the imageid of the corresponding sidecar. |  |  |
| `taskSpec` _[TaskSpec](#taskspec)_ | TaskSpec contains the Spec from the dereferenced Task definition used to instantiate this TaskRun.<br />See Task.spec (API version tekton.dev/v1beta1) |  | Schemaless: \{\} <br /> |
| `provenance` _[Provenance](#provenance)_ | Provenance contains some key authenticated metadata about how a software artifact was built (what sources, what inputs/outputs, etc.). |  | Optional: \{\} <br /> |
| `spanContext` _object (keys:string, values:string)_ | SpanContext contains tracing span context fields |  |  |




#### TaskRunStepOverride



TaskRunStepOverride is used to override the values of a Step in the corresponding Task.



_Appears in:_
- [PipelineTaskRunSpec](#pipelinetaskrunspec)
- [TaskRunSpec](#taskrunspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | The name of the Step to override. |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | The resource requirements to apply to the Step. |  |  |




#### TaskSpec



TaskSpec defines the desired state of Task.



_Appears in:_
- [EmbeddedTask](#embeddedtask)
- [Task](#task)
- [TaskRunSpec](#taskrunspec)
- [TaskRunStatus](#taskrunstatus)
- [TaskRunStatusFields](#taskrunstatusfields)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `resources` _[TaskResources](#taskresources)_ | Resources is a list input and output resource to run the task<br />Resources are represented in TaskRuns as bindings to instances of<br />PipelineResources.<br />Deprecated: Unused, preserved only for backwards compatibility |  | Optional: \{\} <br /> |
| `params` _[ParamSpecs](#paramspecs)_ | Params is a list of input parameters required to run the task. Params<br />must be supplied as inputs in TaskRuns unless they declare a default<br />value. |  | Optional: \{\} <br /> |
| `displayName` _string_ | DisplayName is a user-facing name of the task that may be<br />used to populate a UI. |  | Optional: \{\} <br /> |
| `description` _string_ | Description is a user-facing description of the task that may be<br />used to populate a UI. |  | Optional: \{\} <br /> |
| `steps` _[Step](#step) array_ | Steps are the steps of the build; each step is run sequentially with the<br />source mounted into /workspace. |  |  |
| `volumes` _[Volumes](#volumes)_ | Volumes is a collection of volumes that are available to mount into the<br />steps of the build.<br />See Pod.spec.volumes (API version: v1) |  | Schemaless: \{\} <br /> |
| `stepTemplate` _[StepTemplate](#steptemplate)_ | StepTemplate can be used as the basis for all step containers within the<br />Task, so that the steps inherit settings on the base container. |  |  |
| `sidecars` _[Sidecar](#sidecar) array_ | Sidecars are run alongside the Task's step containers. They begin before<br />the steps start and end after the steps complete. |  |  |
| `workspaces` _[WorkspaceDeclaration](#workspacedeclaration) array_ | Workspaces are the volumes that this Task requires. |  |  |
| `results` _[TaskResult](#taskresult) array_ | Results are values that this Task can output |  |  |


#### TimeoutFields



TimeoutFields allows granular specification of pipeline, task, and finally timeouts



_Appears in:_
- [PipelineRunSpec](#pipelinerunspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `pipeline` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | Pipeline sets the maximum allowed duration for execution of the entire pipeline. The sum of individual timeouts for tasks and finally must not exceed this value. |  |  |
| `tasks` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | Tasks sets the maximum allowed duration of this pipeline's tasks |  |  |
| `finally` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | Finally sets the maximum allowed duration of this pipeline's finally |  |  |


#### Volumes

_Underlying type:_ _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volume-v1-core)_





_Appears in:_
- [EmbeddedTask](#embeddedtask)
- [TaskSpec](#taskspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | name of the volume.<br />Must be a DNS_LABEL and unique within the pod.<br />More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names |  |  |
| `hostPath` _[HostPathVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#hostpathvolumesource-v1-core)_ | hostPath represents a pre-existing file or directory on the host<br />machine that is directly exposed to the container. This is generally<br />used for system agents or other privileged things that are allowed<br />to see the host machine. Most containers will NOT need this.<br />More info: https://kubernetes.io/docs/concepts/storage/volumes#hostpath |  | Optional: \{\} <br /> |
| `emptyDir` _[EmptyDirVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#emptydirvolumesource-v1-core)_ | emptyDir represents a temporary directory that shares a pod's lifetime.<br />More info: https://kubernetes.io/docs/concepts/storage/volumes#emptydir |  | Optional: \{\} <br /> |
| `gcePersistentDisk` _[GCEPersistentDiskVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#gcepersistentdiskvolumesource-v1-core)_ | gcePersistentDisk represents a GCE Disk resource that is attached to a<br />kubelet's host machine and then exposed to the pod.<br />Deprecated: GCEPersistentDisk is deprecated. All operations for the in-tree<br />gcePersistentDisk type are redirected to the pd.csi.storage.gke.io CSI driver.<br />More info: https://kubernetes.io/docs/concepts/storage/volumes#gcepersistentdisk |  | Optional: \{\} <br /> |
| `awsElasticBlockStore` _[AWSElasticBlockStoreVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#awselasticblockstorevolumesource-v1-core)_ | awsElasticBlockStore represents an AWS Disk resource that is attached to a<br />kubelet's host machine and then exposed to the pod.<br />Deprecated: AWSElasticBlockStore is deprecated. All operations for the in-tree<br />awsElasticBlockStore type are redirected to the ebs.csi.aws.com CSI driver.<br />More info: https://kubernetes.io/docs/concepts/storage/volumes#awselasticblockstore |  | Optional: \{\} <br /> |
| `gitRepo` _[GitRepoVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#gitrepovolumesource-v1-core)_ | gitRepo represents a git repository at a particular revision.<br />Deprecated: GitRepo is deprecated. To provision a container with a git repo, mount an<br />EmptyDir into an InitContainer that clones the repo using git, then mount the EmptyDir<br />into the Pod's container. |  | Optional: \{\} <br /> |
| `secret` _[SecretVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#secretvolumesource-v1-core)_ | secret represents a secret that should populate this volume.<br />More info: https://kubernetes.io/docs/concepts/storage/volumes#secret |  | Optional: \{\} <br /> |
| `nfs` _[NFSVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#nfsvolumesource-v1-core)_ | nfs represents an NFS mount on the host that shares a pod's lifetime<br />More info: https://kubernetes.io/docs/concepts/storage/volumes#nfs |  | Optional: \{\} <br /> |
| `iscsi` _[ISCSIVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#iscsivolumesource-v1-core)_ | iscsi represents an ISCSI Disk resource that is attached to a<br />kubelet's host machine and then exposed to the pod.<br />More info: https://kubernetes.io/docs/concepts/storage/volumes/#iscsi |  | Optional: \{\} <br /> |
| `glusterfs` _[GlusterfsVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#glusterfsvolumesource-v1-core)_ | glusterfs represents a Glusterfs mount on the host that shares a pod's lifetime.<br />Deprecated: Glusterfs is deprecated and the in-tree glusterfs type is no longer supported. |  | Optional: \{\} <br /> |
| `persistentVolumeClaim` _[PersistentVolumeClaimVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#persistentvolumeclaimvolumesource-v1-core)_ | persistentVolumeClaimVolumeSource represents a reference to a<br />PersistentVolumeClaim in the same namespace.<br />More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#persistentvolumeclaims |  | Optional: \{\} <br /> |
| `rbd` _[RBDVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#rbdvolumesource-v1-core)_ | rbd represents a Rados Block Device mount on the host that shares a pod's lifetime.<br />Deprecated: RBD is deprecated and the in-tree rbd type is no longer supported. |  | Optional: \{\} <br /> |
| `flexVolume` _[FlexVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#flexvolumesource-v1-core)_ | flexVolume represents a generic volume resource that is<br />provisioned/attached using an exec based plugin.<br />Deprecated: FlexVolume is deprecated. Consider using a CSIDriver instead. |  | Optional: \{\} <br /> |
| `cinder` _[CinderVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#cindervolumesource-v1-core)_ | cinder represents a cinder volume attached and mounted on kubelets host machine.<br />Deprecated: Cinder is deprecated. All operations for the in-tree cinder type<br />are redirected to the cinder.csi.openstack.org CSI driver.<br />More info: https://examples.k8s.io/mysql-cinder-pd/README.md |  | Optional: \{\} <br /> |
| `cephfs` _[CephFSVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#cephfsvolumesource-v1-core)_ | cephFS represents a Ceph FS mount on the host that shares a pod's lifetime.<br />Deprecated: CephFS is deprecated and the in-tree cephfs type is no longer supported. |  | Optional: \{\} <br /> |
| `flocker` _[FlockerVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#flockervolumesource-v1-core)_ | flocker represents a Flocker volume attached to a kubelet's host machine. This depends on the Flocker control service being running.<br />Deprecated: Flocker is deprecated and the in-tree flocker type is no longer supported. |  | Optional: \{\} <br /> |
| `downwardAPI` _[DownwardAPIVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#downwardapivolumesource-v1-core)_ | downwardAPI represents downward API about the pod that should populate this volume |  | Optional: \{\} <br /> |
| `fc` _[FCVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#fcvolumesource-v1-core)_ | fc represents a Fibre Channel resource that is attached to a kubelet's host machine and then exposed to the pod. |  | Optional: \{\} <br /> |
| `azureFile` _[AzureFileVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#azurefilevolumesource-v1-core)_ | azureFile represents an Azure File Service mount on the host and bind mount to the pod.<br />Deprecated: AzureFile is deprecated. All operations for the in-tree azureFile type<br />are redirected to the file.csi.azure.com CSI driver. |  | Optional: \{\} <br /> |
| `configMap` _[ConfigMapVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#configmapvolumesource-v1-core)_ | configMap represents a configMap that should populate this volume |  | Optional: \{\} <br /> |
| `vsphereVolume` _[VsphereVirtualDiskVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#vspherevirtualdiskvolumesource-v1-core)_ | vsphereVolume represents a vSphere volume attached and mounted on kubelets host machine.<br />Deprecated: VsphereVolume is deprecated. All operations for the in-tree vsphereVolume type<br />are redirected to the csi.vsphere.vmware.com CSI driver. |  | Optional: \{\} <br /> |
| `quobyte` _[QuobyteVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#quobytevolumesource-v1-core)_ | quobyte represents a Quobyte mount on the host that shares a pod's lifetime.<br />Deprecated: Quobyte is deprecated and the in-tree quobyte type is no longer supported. |  | Optional: \{\} <br /> |
| `azureDisk` _[AzureDiskVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#azurediskvolumesource-v1-core)_ | azureDisk represents an Azure Data Disk mount on the host and bind mount to the pod.<br />Deprecated: AzureDisk is deprecated. All operations for the in-tree azureDisk type<br />are redirected to the disk.csi.azure.com CSI driver. |  | Optional: \{\} <br /> |
| `photonPersistentDisk` _[PhotonPersistentDiskVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#photonpersistentdiskvolumesource-v1-core)_ | photonPersistentDisk represents a PhotonController persistent disk attached and mounted on kubelets host machine.<br />Deprecated: PhotonPersistentDisk is deprecated and the in-tree photonPersistentDisk type is no longer supported. |  |  |
| `projected` _[ProjectedVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#projectedvolumesource-v1-core)_ | projected items for all in one resources secrets, configmaps, and downward API |  |  |
| `portworxVolume` _[PortworxVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#portworxvolumesource-v1-core)_ | portworxVolume represents a portworx volume attached and mounted on kubelets host machine.<br />Deprecated: PortworxVolume is deprecated. All operations for the in-tree portworxVolume type<br />are redirected to the pxd.portworx.com CSI driver when the CSIMigrationPortworx feature-gate<br />is on. |  | Optional: \{\} <br /> |
| `scaleIO` _[ScaleIOVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#scaleiovolumesource-v1-core)_ | scaleIO represents a ScaleIO persistent volume attached and mounted on Kubernetes nodes.<br />Deprecated: ScaleIO is deprecated and the in-tree scaleIO type is no longer supported. |  | Optional: \{\} <br /> |
| `storageos` _[StorageOSVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#storageosvolumesource-v1-core)_ | storageOS represents a StorageOS volume attached and mounted on Kubernetes nodes.<br />Deprecated: StorageOS is deprecated and the in-tree storageos type is no longer supported. |  | Optional: \{\} <br /> |
| `csi` _[CSIVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#csivolumesource-v1-core)_ | csi (Container Storage Interface) represents ephemeral storage that is handled by certain external CSI drivers. |  | Optional: \{\} <br /> |
| `ephemeral` _[EphemeralVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#ephemeralvolumesource-v1-core)_ | ephemeral represents a volume that is handled by a cluster storage driver.<br />The volume's lifecycle is tied to the pod that defines it - it will be created before the pod starts,<br />and deleted when the pod is removed.<br />Use this if:<br />a) the volume is only needed while the pod runs,<br />b) features of normal volumes like restoring from snapshot or capacity<br />   tracking are needed,<br />c) the storage driver is specified through a storage class, and<br />d) the storage driver supports dynamic volume provisioning through<br />   a PersistentVolumeClaim (see EphemeralVolumeSource for more<br />   information on the connection between this volume type<br />   and PersistentVolumeClaim).<br />Use PersistentVolumeClaim or one of the vendor-specific<br />APIs for volumes that persist for longer than the lifecycle<br />of an individual pod.<br />Use CSI for light-weight local ephemeral volumes if the CSI driver is meant to<br />be used that way - see the documentation of the driver for<br />more information.<br />A pod can use both types of ephemeral volumes and<br />persistent volumes at the same time. |  | Optional: \{\} <br /> |
| `image` _[ImageVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#imagevolumesource-v1-core)_ | image represents an OCI object (a container image or artifact) pulled and mounted on the kubelet's host machine.<br />The volume is resolved at pod startup depending on which PullPolicy value is provided:<br />- Always: the kubelet always attempts to pull the reference. Container creation will fail If the pull fails.<br />- Never: the kubelet never pulls the reference and only uses a local image or artifact. Container creation will fail if the reference isn't present.<br />- IfNotPresent: the kubelet pulls if the reference isn't already present on disk. Container creation will fail if the reference isn't present and the pull fails.<br />The volume gets re-resolved if the pod gets deleted and recreated, which means that new remote content will become available on pod recreation.<br />A failure to resolve or pull the image during pod startup will block containers from starting and may add significant latency. Failures will be retried using normal volume backoff and will be reported on the pod reason and message.<br />The types of objects that may be mounted by this volume are defined by the container runtime implementation on a host machine and at minimum must include all valid types supported by the container image field.<br />The OCI object gets mounted in a single directory (spec.containers[*].volumeMounts.mountPath) by merging the manifest layers in the same way as for container images.<br />The volume will be mounted read-only (ro) and non-executable files (noexec).<br />Sub path mounts for containers are not supported (spec.containers[*].volumeMounts.subpath) before 1.33.<br />The field spec.securityContext.fsGroupChangePolicy has no effect on this volume type. |  | Optional: \{\} <br /> |


#### WhenExpression



WhenExpression allows a PipelineTask to declare expressions to be evaluated before the Task is run
to determine whether the Task should be executed or skipped



_Appears in:_
- [ChildStatusReference](#childstatusreference)
- [PipelineRunRunStatus](#pipelinerunrunstatus)
- [PipelineRunTaskRunStatus](#pipelineruntaskrunstatus)
- [SkippedTask](#skippedtask)
- [WhenExpressions](#whenexpressions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `input` _string_ | Input is the string for guard checking which can be a static input or an output from a parent Task |  |  |
| `operator` _[Operator](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#operator-selection-pkg)_ | Operator that represents an Input's relationship to the values |  |  |
| `values` _string array_ | Values is an array of strings, which is compared against the input, for guard checking<br />It must be non-empty |  |  |
| `cel` _string_ | CEL is a string of Common Language Expression, which can be used to conditionally execute<br />the task based on the result of the expression evaluation<br />More info about CEL syntax: https://github.com/google/cel-spec/blob/master/doc/langdef.md |  | Optional: \{\} <br /> |


#### WhenExpressions

_Underlying type:_ _[WhenExpression](#whenexpression)_

WhenExpressions are used to specify whether a Task should be executed or skipped
All of them need to evaluate to True for a guarded Task to be executed.



_Appears in:_
- [PipelineTask](#pipelinetask)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `input` _string_ | Input is the string for guard checking which can be a static input or an output from a parent Task |  |  |
| `operator` _[Operator](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#operator-selection-pkg)_ | Operator that represents an Input's relationship to the values |  |  |
| `values` _string array_ | Values is an array of strings, which is compared against the input, for guard checking<br />It must be non-empty |  |  |
| `cel` _string_ | CEL is a string of Common Language Expression, which can be used to conditionally execute<br />the task based on the result of the expression evaluation<br />More info about CEL syntax: https://github.com/google/cel-spec/blob/master/doc/langdef.md |  | Optional: \{\} <br /> |


#### WorkspaceBinding



WorkspaceBinding maps a Task's declared workspace to a Volume.



_Appears in:_
- [CustomRunSpec](#customrunspec)
- [PipelineRunSpec](#pipelinerunspec)
- [RunSpec](#runspec)
- [TaskRunSpec](#taskrunspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the workspace populated by the volume. |  |  |
| `subPath` _string_ | SubPath is optionally a directory on the volume which should be used<br />for this binding (i.e. the volume will be mounted at this sub directory). |  | Optional: \{\} <br /> |
| `volumeClaimTemplate` _[PersistentVolumeClaim](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#persistentvolumeclaim-v1-core)_ | VolumeClaimTemplate is a template for a claim that will be created in the same namespace.<br />The PipelineRun controller is responsible for creating a unique claim for each instance of PipelineRun.<br />See PersistentVolumeClaim (API version: v1) |  | Schemaless: \{\} <br />Optional: \{\} <br /> |
| `persistentVolumeClaim` _[PersistentVolumeClaimVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#persistentvolumeclaimvolumesource-v1-core)_ | PersistentVolumeClaimVolumeSource represents a reference to a<br />PersistentVolumeClaim in the same namespace. Either this OR EmptyDir can be used. |  | Optional: \{\} <br /> |
| `emptyDir` _[EmptyDirVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#emptydirvolumesource-v1-core)_ | EmptyDir represents a temporary directory that shares a Task's lifetime.<br />More info: https://kubernetes.io/docs/concepts/storage/volumes#emptydir<br />Either this OR PersistentVolumeClaim can be used. |  | Optional: \{\} <br /> |
| `configMap` _[ConfigMapVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#configmapvolumesource-v1-core)_ | ConfigMap represents a configMap that should populate this workspace. |  | Optional: \{\} <br /> |
| `secret` _[SecretVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#secretvolumesource-v1-core)_ | Secret represents a secret that should populate this workspace. |  | Optional: \{\} <br /> |
| `projected` _[ProjectedVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#projectedvolumesource-v1-core)_ | Projected represents a projected volume that should populate this workspace. |  | Optional: \{\} <br /> |
| `csi` _[CSIVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#csivolumesource-v1-core)_ | CSI (Container Storage Interface) represents ephemeral storage that is handled by certain external CSI drivers. |  | Optional: \{\} <br /> |


#### WorkspaceDeclaration



WorkspaceDeclaration is a declaration of a volume that a Task requires.



_Appears in:_
- [EmbeddedTask](#embeddedtask)
- [TaskSpec](#taskspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name by which you can bind the volume at runtime. |  |  |
| `description` _string_ | Description is an optional human readable description of this volume. |  | Optional: \{\} <br /> |
| `mountPath` _string_ | MountPath overrides the directory that the volume will be made available at. |  | Optional: \{\} <br /> |
| `readOnly` _boolean_ | ReadOnly dictates whether a mounted volume is writable. By default this<br />field is false and so mounted volumes are writable. |  |  |
| `optional` _boolean_ | Optional marks a Workspace as not being required in TaskRuns. By default<br />this field is false and so declared workspaces are required. |  |  |




#### WorkspacePipelineTaskBinding



WorkspacePipelineTaskBinding describes how a workspace passed into the pipeline should be
mapped to a task's declared workspace.



_Appears in:_
- [PipelineTask](#pipelinetask)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the workspace as declared by the task |  |  |
| `workspace` _string_ | Workspace is the name of the workspace declared by the pipeline |  | Optional: \{\} <br /> |
| `subPath` _string_ | SubPath is optionally a directory on the volume which should be used<br />for this binding (i.e. the volume will be mounted at this sub directory). |  | Optional: \{\} <br /> |


#### WorkspaceUsage



WorkspaceUsage is used by a Step or Sidecar to declare that it wants isolated access
to a Workspace defined in a Task.



_Appears in:_
- [Sidecar](#sidecar)
- [Step](#step)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the workspace this Step or Sidecar wants access to. |  |  |
| `mountPath` _string_ | MountPath is the path that the workspace should be mounted to inside the Step or Sidecar,<br />overriding any MountPath specified in the Task's WorkspaceDeclaration. |  |  |


