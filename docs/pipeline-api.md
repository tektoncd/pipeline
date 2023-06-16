<!--
---
title: Pipeline API
linkTitle: Pipeline API
weight: 404
---
-->

<p>Packages:</p>
<ul>
<li>
<a href="#resolution.tekton.dev%2fv1alpha1">resolution.tekton.dev/v1alpha1</a>
</li>
<li>
<a href="#resolution.tekton.dev%2fv1beta1">resolution.tekton.dev/v1beta1</a>
</li>
<li>
<a href="#tekton.dev%2fv1">tekton.dev/v1</a>
</li>
<li>
<a href="#tekton.dev%2fv1alpha1">tekton.dev/v1alpha1</a>
</li>
<li>
<a href="#tekton.dev%2fv1beta1">tekton.dev/v1beta1</a>
</li>
</ul>
<h2 id="resolution.tekton.dev/v1alpha1">resolution.tekton.dev/v1alpha1</h2>
<div>
</div>
Resource Types:
<ul></ul>
<h3 id="resolution.tekton.dev/v1alpha1.ResolutionRequest">ResolutionRequest
</h3>
<div>
<p>ResolutionRequest is an object for requesting the content of
a Tekton resource like a pipeline.yaml.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
<em>(Optional)</em>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#resolution.tekton.dev/v1alpha1.ResolutionRequestSpec">
ResolutionRequestSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Spec holds the information for the request part of the resource request.</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>params</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Parameters are the runtime attributes passed to
the resolver to help it figure out how to resolve the
resource being requested. For example: repo URL, commit SHA,
path to file, the kind of authentication to leverage, etc.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#resolution.tekton.dev/v1alpha1.ResolutionRequestStatus">
ResolutionRequestStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Status communicates the state of the request and, ultimately,
the content of the resolved resource.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="resolution.tekton.dev/v1alpha1.ResolutionRequestSpec">ResolutionRequestSpec
</h3>
<p>
(<em>Appears on:</em><a href="#resolution.tekton.dev/v1alpha1.ResolutionRequest">ResolutionRequest</a>)
</p>
<div>
<p>ResolutionRequestSpec are all the fields in the spec of the
ResolutionRequest CRD.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>params</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Parameters are the runtime attributes passed to
the resolver to help it figure out how to resolve the
resource being requested. For example: repo URL, commit SHA,
path to file, the kind of authentication to leverage, etc.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="resolution.tekton.dev/v1alpha1.ResolutionRequestStatus">ResolutionRequestStatus
</h3>
<p>
(<em>Appears on:</em><a href="#resolution.tekton.dev/v1alpha1.ResolutionRequest">ResolutionRequest</a>)
</p>
<div>
<p>ResolutionRequestStatus are all the fields in a ResolutionRequest&rsquo;s
status subresource.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>Status</code><br/>
<em>
<a href="https://pkg.go.dev/knative.dev/pkg/apis/duck/v1#Status">
knative.dev/pkg/apis/duck/v1.Status
</a>
</em>
</td>
<td>
<p>
(Members of <code>Status</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>ResolutionRequestStatusFields</code><br/>
<em>
<a href="#resolution.tekton.dev/v1alpha1.ResolutionRequestStatusFields">
ResolutionRequestStatusFields
</a>
</em>
</td>
<td>
<p>
(Members of <code>ResolutionRequestStatusFields</code> are embedded into this type.)
</p>
</td>
</tr>
</tbody>
</table>
<h3 id="resolution.tekton.dev/v1alpha1.ResolutionRequestStatusFields">ResolutionRequestStatusFields
</h3>
<p>
(<em>Appears on:</em><a href="#resolution.tekton.dev/v1alpha1.ResolutionRequestStatus">ResolutionRequestStatus</a>)
</p>
<div>
<p>ResolutionRequestStatusFields are the ResolutionRequest-specific fields
for the status subresource.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>data</code><br/>
<em>
string
</em>
</td>
<td>
<p>Data is a string representation of the resolved content
of the requested resource in-lined into the ResolutionRequest
object.</p>
</td>
</tr>
<tr>
<td>
<code>refSource</code><br/>
<em>
<a href="#tekton.dev/v1.RefSource">
RefSource
</a>
</em>
</td>
<td>
<p>RefSource is the source reference of the remote data that records where the remote
file came from including the url, digest and the entrypoint.</p>
</td>
</tr>
</tbody>
</table>
<hr/>
<h2 id="resolution.tekton.dev/v1beta1">resolution.tekton.dev/v1beta1</h2>
<div>
</div>
Resource Types:
<ul></ul>
<h3 id="resolution.tekton.dev/v1beta1.ResolutionRequest">ResolutionRequest
</h3>
<div>
<p>ResolutionRequest is an object for requesting the content of
a Tekton resource like a pipeline.yaml.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
<em>(Optional)</em>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#resolution.tekton.dev/v1beta1.ResolutionRequestSpec">
ResolutionRequestSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Spec holds the information for the request part of the resource request.</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>params</code><br/>
<em>
<a href="#tekton.dev/v1.Param">
[]Param
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Parameters are the runtime attributes passed to
the resolver to help it figure out how to resolve the
resource being requested. For example: repo URL, commit SHA,
path to file, the kind of authentication to leverage, etc.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#resolution.tekton.dev/v1beta1.ResolutionRequestStatus">
ResolutionRequestStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Status communicates the state of the request and, ultimately,
the content of the resolved resource.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="resolution.tekton.dev/v1beta1.ResolutionRequestSpec">ResolutionRequestSpec
</h3>
<p>
(<em>Appears on:</em><a href="#resolution.tekton.dev/v1beta1.ResolutionRequest">ResolutionRequest</a>)
</p>
<div>
<p>ResolutionRequestSpec are all the fields in the spec of the
ResolutionRequest CRD.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>params</code><br/>
<em>
<a href="#tekton.dev/v1.Param">
[]Param
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Parameters are the runtime attributes passed to
the resolver to help it figure out how to resolve the
resource being requested. For example: repo URL, commit SHA,
path to file, the kind of authentication to leverage, etc.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="resolution.tekton.dev/v1beta1.ResolutionRequestStatus">ResolutionRequestStatus
</h3>
<p>
(<em>Appears on:</em><a href="#resolution.tekton.dev/v1beta1.ResolutionRequest">ResolutionRequest</a>)
</p>
<div>
<p>ResolutionRequestStatus are all the fields in a ResolutionRequest&rsquo;s
status subresource.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>Status</code><br/>
<em>
<a href="https://pkg.go.dev/knative.dev/pkg/apis/duck/v1#Status">
knative.dev/pkg/apis/duck/v1.Status
</a>
</em>
</td>
<td>
<p>
(Members of <code>Status</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>ResolutionRequestStatusFields</code><br/>
<em>
<a href="#resolution.tekton.dev/v1beta1.ResolutionRequestStatusFields">
ResolutionRequestStatusFields
</a>
</em>
</td>
<td>
<p>
(Members of <code>ResolutionRequestStatusFields</code> are embedded into this type.)
</p>
</td>
</tr>
</tbody>
</table>
<h3 id="resolution.tekton.dev/v1beta1.ResolutionRequestStatusFields">ResolutionRequestStatusFields
</h3>
<p>
(<em>Appears on:</em><a href="#resolution.tekton.dev/v1beta1.ResolutionRequestStatus">ResolutionRequestStatus</a>)
</p>
<div>
<p>ResolutionRequestStatusFields are the ResolutionRequest-specific fields
for the status subresource.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>data</code><br/>
<em>
string
</em>
</td>
<td>
<p>Data is a string representation of the resolved content
of the requested resource in-lined into the ResolutionRequest
object.</p>
</td>
</tr>
<tr>
<td>
<code>source</code><br/>
<em>
<a href="#tekton.dev/v1.RefSource">
RefSource
</a>
</em>
</td>
<td>
<p>Deprecated: Use RefSource instead</p>
</td>
</tr>
<tr>
<td>
<code>refSource</code><br/>
<em>
<a href="#tekton.dev/v1.RefSource">
RefSource
</a>
</em>
</td>
<td>
<p>RefSource is the source reference of the remote data that records the url, digest
and the entrypoint.</p>
</td>
</tr>
</tbody>
</table>
<hr/>
<h2 id="tekton.dev/v1">tekton.dev/v1</h2>
<div>
<p>Package v1 contains API Schema definitions for the pipeline v1 API group</p>
</div>
Resource Types:
<ul><li>
<a href="#tekton.dev/v1.Pipeline">Pipeline</a>
</li><li>
<a href="#tekton.dev/v1.PipelineRun">PipelineRun</a>
</li><li>
<a href="#tekton.dev/v1.Task">Task</a>
</li><li>
<a href="#tekton.dev/v1.TaskRun">TaskRun</a>
</li></ul>
<h3 id="tekton.dev/v1.Pipeline">Pipeline
</h3>
<div>
<p>Pipeline describes a list of Tasks to execute. It expresses how outputs
of tasks feed into inputs of subsequent tasks.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code><br/>
string</td>
<td>
<code>
tekton.dev/v1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>Pipeline</code></td>
</tr>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
<em>(Optional)</em>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#tekton.dev/v1.PipelineSpec">
PipelineSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Spec holds the desired state of the Pipeline from the client</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>displayName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>DisplayName is a user-facing name of the pipeline that may be
used to populate a UI.</p>
</td>
</tr>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Description is a user-facing description of the pipeline that may be
used to populate a UI.</p>
</td>
</tr>
<tr>
<td>
<code>tasks</code><br/>
<em>
<a href="#tekton.dev/v1.PipelineTask">
[]PipelineTask
</a>
</em>
</td>
<td>
<p>Tasks declares the graph of Tasks that execute when this Pipeline is run.</p>
</td>
</tr>
<tr>
<td>
<code>params</code><br/>
<em>
<a href="#tekton.dev/v1.ParamSpecs">
ParamSpecs
</a>
</em>
</td>
<td>
<p>Params declares a list of input parameters that must be supplied when
this Pipeline is run.</p>
</td>
</tr>
<tr>
<td>
<code>workspaces</code><br/>
<em>
<a href="#tekton.dev/v1.PipelineWorkspaceDeclaration">
[]PipelineWorkspaceDeclaration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Workspaces declares a set of named workspaces that are expected to be
provided by a PipelineRun.</p>
</td>
</tr>
<tr>
<td>
<code>results</code><br/>
<em>
<a href="#tekton.dev/v1.PipelineResult">
[]PipelineResult
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Results are values that this pipeline can output once run</p>
</td>
</tr>
<tr>
<td>
<code>finally</code><br/>
<em>
<a href="#tekton.dev/v1.PipelineTask">
[]PipelineTask
</a>
</em>
</td>
<td>
<p>Finally declares the list of Tasks that execute just before leaving the Pipeline
i.e. either after all Tasks are finished executing successfully
or after a failure which would result in ending the Pipeline</p>
</td>
</tr>
</table>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.PipelineRun">PipelineRun
</h3>
<div>
<p>PipelineRun represents a single execution of a Pipeline. PipelineRuns are how
the graph of Tasks declared in a Pipeline are executed; they specify inputs
to Pipelines such as parameter values and capture operational aspects of the
Tasks execution such as service account and tolerations. Creating a
PipelineRun creates TaskRuns for Tasks in the referenced Pipeline.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code><br/>
string</td>
<td>
<code>
tekton.dev/v1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>PipelineRun</code></td>
</tr>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
<em>(Optional)</em>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#tekton.dev/v1.PipelineRunSpec">
PipelineRunSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<br/>
<br/>
<table>
<tr>
<td>
<code>pipelineRef</code><br/>
<em>
<a href="#tekton.dev/v1.PipelineRef">
PipelineRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>pipelineSpec</code><br/>
<em>
<a href="#tekton.dev/v1.PipelineSpec">
PipelineSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>params</code><br/>
<em>
<a href="#tekton.dev/v1.Params">
Params
</a>
</em>
</td>
<td>
<p>Params is a list of parameter names and values.</p>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#tekton.dev/v1.PipelineRunSpecStatus">
PipelineRunSpecStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Used for cancelling a pipelinerun (and maybe more later on)</p>
</td>
</tr>
<tr>
<td>
<code>timeouts</code><br/>
<em>
<a href="#tekton.dev/v1.TimeoutFields">
TimeoutFields
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Time after which the Pipeline times out.
Currently three keys are accepted in the map
pipeline, tasks and finally
with Timeouts.pipeline &gt;= Timeouts.tasks + Timeouts.finally</p>
</td>
</tr>
<tr>
<td>
<code>taskRunTemplate</code><br/>
<em>
<a href="#tekton.dev/v1.PipelineTaskRunTemplate">
PipelineTaskRunTemplate
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>TaskRunTemplate represent template of taskrun</p>
</td>
</tr>
<tr>
<td>
<code>workspaces</code><br/>
<em>
<a href="#tekton.dev/v1.WorkspaceBinding">
[]WorkspaceBinding
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Workspaces holds a set of workspace bindings that must match names
with those declared in the pipeline.</p>
</td>
</tr>
<tr>
<td>
<code>taskRunSpecs</code><br/>
<em>
<a href="#tekton.dev/v1.PipelineTaskRunSpec">
[]PipelineTaskRunSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>TaskRunSpecs holds a set of runtime specs</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#tekton.dev/v1.PipelineRunStatus">
PipelineRunStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.Task">Task
</h3>
<div>
<p>Task represents a collection of sequential steps that are run as part of a
Pipeline using a set of inputs and producing a set of outputs. Tasks execute
when TaskRuns are created that provide the input parameters and resources and
output resources the Task requires.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code><br/>
string</td>
<td>
<code>
tekton.dev/v1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>Task</code></td>
</tr>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
<em>(Optional)</em>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#tekton.dev/v1.TaskSpec">
TaskSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Spec holds the desired state of the Task from the client</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>params</code><br/>
<em>
<a href="#tekton.dev/v1.ParamSpecs">
ParamSpecs
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Params is a list of input parameters required to run the task. Params
must be supplied as inputs in TaskRuns unless they declare a default
value.</p>
</td>
</tr>
<tr>
<td>
<code>displayName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>DisplayName is a user-facing name of the task that may be
used to populate a UI.</p>
</td>
</tr>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Description is a user-facing description of the task that may be
used to populate a UI.</p>
</td>
</tr>
<tr>
<td>
<code>steps</code><br/>
<em>
<a href="#tekton.dev/v1.Step">
[]Step
</a>
</em>
</td>
<td>
<p>Steps are the steps of the build; each step is run sequentially with the
source mounted into /workspace.</p>
</td>
</tr>
<tr>
<td>
<code>volumes</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#volume-v1-core">
[]Kubernetes core/v1.Volume
</a>
</em>
</td>
<td>
<p>Volumes is a collection of volumes that are available to mount into the
steps of the build.</p>
</td>
</tr>
<tr>
<td>
<code>stepTemplate</code><br/>
<em>
<a href="#tekton.dev/v1.StepTemplate">
StepTemplate
</a>
</em>
</td>
<td>
<p>StepTemplate can be used as the basis for all step containers within the
Task, so that the steps inherit settings on the base container.</p>
</td>
</tr>
<tr>
<td>
<code>sidecars</code><br/>
<em>
<a href="#tekton.dev/v1.Sidecar">
[]Sidecar
</a>
</em>
</td>
<td>
<p>Sidecars are run alongside the Task&rsquo;s step containers. They begin before
the steps start and end after the steps complete.</p>
</td>
</tr>
<tr>
<td>
<code>workspaces</code><br/>
<em>
<a href="#tekton.dev/v1.WorkspaceDeclaration">
[]WorkspaceDeclaration
</a>
</em>
</td>
<td>
<p>Workspaces are the volumes that this Task requires.</p>
</td>
</tr>
<tr>
<td>
<code>results</code><br/>
<em>
<a href="#tekton.dev/v1.TaskResult">
[]TaskResult
</a>
</em>
</td>
<td>
<p>Results are values that this Task can output</p>
</td>
</tr>
</table>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.TaskRun">TaskRun
</h3>
<div>
<p>TaskRun represents a single execution of a Task. TaskRuns are how the steps
specified in a Task are executed; they specify the parameters and resources
used to run the steps in a Task.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code><br/>
string</td>
<td>
<code>
tekton.dev/v1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>TaskRun</code></td>
</tr>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
<em>(Optional)</em>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#tekton.dev/v1.TaskRunSpec">
TaskRunSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<br/>
<br/>
<table>
<tr>
<td>
<code>debug</code><br/>
<em>
<a href="#tekton.dev/v1.TaskRunDebug">
TaskRunDebug
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>params</code><br/>
<em>
<a href="#tekton.dev/v1.Params">
Params
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>serviceAccountName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>taskRef</code><br/>
<em>
<a href="#tekton.dev/v1.TaskRef">
TaskRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>no more than one of the TaskRef and TaskSpec may be specified.</p>
</td>
</tr>
<tr>
<td>
<code>taskSpec</code><br/>
<em>
<a href="#tekton.dev/v1.TaskSpec">
TaskSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#tekton.dev/v1.TaskRunSpecStatus">
TaskRunSpecStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Used for cancelling a TaskRun (and maybe more later on)</p>
</td>
</tr>
<tr>
<td>
<code>statusMessage</code><br/>
<em>
<a href="#tekton.dev/v1.TaskRunSpecStatusMessage">
TaskRunSpecStatusMessage
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Status message for cancellation.</p>
</td>
</tr>
<tr>
<td>
<code>retries</code><br/>
<em>
int
</em>
</td>
<td>
<em>(Optional)</em>
<p>Retries represents how many times this TaskRun should be retried in the event of task failure.</p>
</td>
</tr>
<tr>
<td>
<code>timeout</code><br/>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Time after which one retry attempt times out. Defaults to 1 hour.
Refer Go&rsquo;s ParseDuration documentation for expected format: <a href="https://golang.org/pkg/time/#ParseDuration">https://golang.org/pkg/time/#ParseDuration</a></p>
</td>
</tr>
<tr>
<td>
<code>podTemplate</code><br/>
<em>
<a href="#tekton.dev/unversioned.Template">
Template
</a>
</em>
</td>
<td>
<p>PodTemplate holds pod specific configuration</p>
</td>
</tr>
<tr>
<td>
<code>workspaces</code><br/>
<em>
<a href="#tekton.dev/v1.WorkspaceBinding">
[]WorkspaceBinding
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Workspaces is a list of WorkspaceBindings from volumes to workspaces.</p>
</td>
</tr>
<tr>
<td>
<code>stepSpecs</code><br/>
<em>
<a href="#tekton.dev/v1.TaskRunStepSpec">
[]TaskRunStepSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specs to apply to Steps in this TaskRun.
If a field is specified in both a Step and a StepSpec,
the value from the StepSpec will be used.
This field is only supported when the alpha feature gate is enabled.</p>
</td>
</tr>
<tr>
<td>
<code>sidecarSpecs</code><br/>
<em>
<a href="#tekton.dev/v1.TaskRunSidecarSpec">
[]TaskRunSidecarSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specs to apply to Sidecars in this TaskRun.
If a field is specified in both a Sidecar and a SidecarSpec,
the value from the SidecarSpec will be used.
This field is only supported when the alpha feature gate is enabled.</p>
</td>
</tr>
<tr>
<td>
<code>computeResources</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#resourcerequirements-v1-core">
Kubernetes core/v1.ResourceRequirements
</a>
</em>
</td>
<td>
<p>Compute resources to use for this TaskRun</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#tekton.dev/v1.TaskRunStatus">
TaskRunStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.ChildStatusReference">ChildStatusReference
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.PipelineRunStatusFields">PipelineRunStatusFields</a>)
</p>
<div>
<p>ChildStatusReference is used to point to the statuses of individual TaskRuns and Runs within this PipelineRun.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the TaskRun or Run this is referencing.</p>
</td>
</tr>
<tr>
<td>
<code>pipelineTaskName</code><br/>
<em>
string
</em>
</td>
<td>
<p>PipelineTaskName is the name of the PipelineTask this is referencing.</p>
</td>
</tr>
<tr>
<td>
<code>whenExpressions</code><br/>
<em>
<a href="#tekton.dev/v1.WhenExpression">
[]WhenExpression
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>WhenExpressions is the list of checks guarding the execution of the PipelineTask</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.Combination">Combination
(<code>map[string]string</code> alias)</h3>
<div>
<p>Combination is a map, mainly defined to hold a single combination from a Matrix with key as param.Name and value as param.Value</p>
</div>
<h3 id="tekton.dev/v1.Combinations">Combinations
(<code>[]github.com/tektoncd/pipeline/pkg/apis/pipeline/v1.Combination</code> alias)</h3>
<div>
<p>Combinations is a Combination list</p>
</div>
<h3 id="tekton.dev/v1.EmbeddedTask">EmbeddedTask
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.PipelineTask">PipelineTask</a>)
</p>
<div>
<p>EmbeddedTask is used to define a Task inline within a Pipeline&rsquo;s PipelineTasks.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>spec</code><br/>
<em>
k8s.io/apimachinery/pkg/runtime.RawExtension
</em>
</td>
<td>
<em>(Optional)</em>
<p>Spec is a specification of a custom task</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>-</code><br/>
<em>
[]byte
</em>
</td>
<td>
<p>Raw is the underlying serialization of this object.</p>
<p>TODO: Determine how to detect ContentType and ContentEncoding of &lsquo;Raw&rsquo; data.</p>
</td>
</tr>
<tr>
<td>
<code>-</code><br/>
<em>
k8s.io/apimachinery/pkg/runtime.Object
</em>
</td>
<td>
<p>Object can hold a representation of this extension - useful for working with versioned
structs.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="#tekton.dev/v1.PipelineTaskMetadata">
PipelineTaskMetadata
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>TaskSpec</code><br/>
<em>
<a href="#tekton.dev/v1.TaskSpec">
TaskSpec
</a>
</em>
</td>
<td>
<p>
(Members of <code>TaskSpec</code> are embedded into this type.)
</p>
<em>(Optional)</em>
<p>TaskSpec is a specification of a task</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.IncludeParams">IncludeParams
</h3>
<div>
<p>IncludeParams allows passing in a specific combinations of Parameters into the Matrix.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name the specified combination</p>
</td>
</tr>
<tr>
<td>
<code>params</code><br/>
<em>
<a href="#tekton.dev/v1.Params">
Params
</a>
</em>
</td>
<td>
<p>Params takes only <code>Parameters</code> of type <code>&quot;string&quot;</code>
The names of the <code>params</code> must match the names of the <code>params</code> in the underlying <code>Task</code></p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.Matrix">Matrix
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.PipelineTask">PipelineTask</a>)
</p>
<div>
<p>Matrix is used to fan out Tasks in a Pipeline</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>params</code><br/>
<em>
<a href="#tekton.dev/v1.Params">
Params
</a>
</em>
</td>
<td>
<p>Params is a list of parameters used to fan out the pipelineTask
Params takes only <code>Parameters</code> of type <code>&quot;array&quot;</code>
Each array element is supplied to the <code>PipelineTask</code> by substituting <code>params</code> of type <code>&quot;string&quot;</code> in the underlying <code>Task</code>.
The names of the <code>params</code> in the <code>Matrix</code> must match the names of the <code>params</code> in the underlying <code>Task</code> that they will be substituting.</p>
</td>
</tr>
<tr>
<td>
<code>include</code><br/>
<em>
<a href="#tekton.dev/v1.IncludeParamsList">
IncludeParamsList
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Include is a list of IncludeParams which allows passing in specific combinations of Parameters into the Matrix.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.OnErrorType">OnErrorType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.Step">Step</a>)
</p>
<div>
<p>OnErrorType defines a list of supported exiting behavior of a container on error</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;continue&#34;</p></td>
<td><p>Continue indicates continue executing the rest of the steps irrespective of the container exit code</p>
</td>
</tr><tr><td><p>&#34;stopAndFail&#34;</p></td>
<td><p>StopAndFail indicates exit the taskRun if the container exits with non-zero exit code</p>
</td>
</tr></tbody>
</table>
<h3 id="tekton.dev/v1.Param">Param
</h3>
<p>
(<em>Appears on:</em><a href="#resolution.tekton.dev/v1beta1.ResolutionRequestSpec">ResolutionRequestSpec</a>)
</p>
<div>
<p>Param declares an ParamValues to use for the parameter called name.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>value</code><br/>
<em>
<a href="#tekton.dev/v1.ParamValue">
ParamValue
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.ParamSpec">ParamSpec
</h3>
<div>
<p>ParamSpec defines arbitrary parameters needed beyond typed inputs (such as
resources). Parameter values are provided by users as inputs on a TaskRun
or PipelineRun.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name declares the name by which a parameter is referenced.</p>
</td>
</tr>
<tr>
<td>
<code>type</code><br/>
<em>
<a href="#tekton.dev/v1.ParamType">
ParamType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Type is the user-specified type of the parameter. The possible types
are currently &ldquo;string&rdquo;, &ldquo;array&rdquo; and &ldquo;object&rdquo;, and &ldquo;string&rdquo; is the default.</p>
</td>
</tr>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Description is a user-facing description of the parameter that may be
used to populate a UI.</p>
</td>
</tr>
<tr>
<td>
<code>properties</code><br/>
<em>
<a href="#tekton.dev/v1.PropertySpec">
map[string]github.com/tektoncd/pipeline/pkg/apis/pipeline/v1.PropertySpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Properties is the JSON Schema properties to support key-value pairs parameter.</p>
</td>
</tr>
<tr>
<td>
<code>default</code><br/>
<em>
<a href="#tekton.dev/v1.ParamValue">
ParamValue
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Default is the value a parameter takes if no input value is supplied. If
default is set, a Task may be executed without a supplied value for the
parameter.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.ParamSpecs">ParamSpecs
(<code>[]github.com/tektoncd/pipeline/pkg/apis/pipeline/v1.ParamSpec</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.PipelineSpec">PipelineSpec</a>, <a href="#tekton.dev/v1.TaskSpec">TaskSpec</a>)
</p>
<div>
<p>ParamSpecs is a list of ParamSpec</p>
</div>
<h3 id="tekton.dev/v1.ParamType">ParamType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.ParamSpec">ParamSpec</a>, <a href="#tekton.dev/v1.ParamValue">ParamValue</a>, <a href="#tekton.dev/v1.PropertySpec">PropertySpec</a>)
</p>
<div>
<p>ParamType indicates the type of an input parameter;
Used to distinguish between a single string and an array of strings.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;array&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;object&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;string&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="tekton.dev/v1.ParamValue">ParamValue
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.Param">Param</a>, <a href="#tekton.dev/v1.ParamSpec">ParamSpec</a>, <a href="#tekton.dev/v1.PipelineResult">PipelineResult</a>, <a href="#tekton.dev/v1.PipelineRunResult">PipelineRunResult</a>, <a href="#tekton.dev/v1.TaskRunResult">TaskRunResult</a>)
</p>
<div>
<p>ResultValue is a type alias of ParamValue</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>Type</code><br/>
<em>
<a href="#tekton.dev/v1.ParamType">
ParamType
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>StringVal</code><br/>
<em>
string
</em>
</td>
<td>
<p>Represents the stored type of ParamValues.</p>
</td>
</tr>
<tr>
<td>
<code>ArrayVal</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>ObjectVal</code><br/>
<em>
map[string]string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.Params">Params
(<code>[]github.com/tektoncd/pipeline/pkg/apis/pipeline/v1.Param</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.IncludeParams">IncludeParams</a>, <a href="#tekton.dev/v1.Matrix">Matrix</a>, <a href="#tekton.dev/v1.PipelineRunSpec">PipelineRunSpec</a>, <a href="#tekton.dev/v1.PipelineTask">PipelineTask</a>, <a href="#tekton.dev/v1.ResolverRef">ResolverRef</a>, <a href="#tekton.dev/v1.TaskRunInputs">TaskRunInputs</a>, <a href="#tekton.dev/v1.TaskRunSpec">TaskRunSpec</a>)
</p>
<div>
<p>Params is a list of Param</p>
</div>
<h3 id="tekton.dev/v1.PipelineRef">PipelineRef
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.PipelineRunSpec">PipelineRunSpec</a>)
</p>
<div>
<p>PipelineRef can be used to refer to a specific instance of a Pipeline.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name of the referent; More info: <a href="http://kubernetes.io/docs/user-guide/identifiers#names">http://kubernetes.io/docs/user-guide/identifiers#names</a></p>
</td>
</tr>
<tr>
<td>
<code>apiVersion</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>API version of the referent</p>
</td>
</tr>
<tr>
<td>
<code>ResolverRef</code><br/>
<em>
<a href="#tekton.dev/v1.ResolverRef">
ResolverRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ResolverRef allows referencing a Pipeline in a remote location
like a git repo. This field is only supported when the alpha
feature gate is enabled.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.PipelineResult">PipelineResult
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.PipelineSpec">PipelineSpec</a>)
</p>
<div>
<p>PipelineResult used to describe the results of a pipeline</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name the given name</p>
</td>
</tr>
<tr>
<td>
<code>type</code><br/>
<em>
<a href="#tekton.dev/v1.ResultsType">
ResultsType
</a>
</em>
</td>
<td>
<p>Type is the user-specified type of the result.
The possible types are &lsquo;string&rsquo;, &lsquo;array&rsquo;, and &lsquo;object&rsquo;, with &lsquo;string&rsquo; as the default.
&lsquo;array&rsquo; and &lsquo;object&rsquo; types are alpha features.</p>
</td>
</tr>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Description is a human-readable description of the result</p>
</td>
</tr>
<tr>
<td>
<code>value</code><br/>
<em>
<a href="#tekton.dev/v1.ParamValue">
ParamValue
</a>
</em>
</td>
<td>
<p>Value the expression used to retrieve the value</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.PipelineRunReason">PipelineRunReason
(<code>string</code> alias)</h3>
<div>
<p>PipelineRunReason represents a reason for the pipeline run &ldquo;Succeeded&rdquo; condition</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Cancelled&#34;</p></td>
<td><p>PipelineRunReasonCancelled is the reason set when the PipelineRun cancelled by the user
This reason may be found with a corev1.ConditionFalse status, if the cancellation was processed successfully
This reason may be found with a corev1.ConditionUnknown status, if the cancellation is being processed or failed</p>
</td>
</tr><tr><td><p>&#34;CancelledRunningFinally&#34;</p></td>
<td><p>PipelineRunReasonCancelledRunningFinally indicates that pipeline has been gracefully cancelled
and no new Tasks will be scheduled by the controller, but final tasks are now running</p>
</td>
</tr><tr><td><p>&#34;Completed&#34;</p></td>
<td><p>PipelineRunReasonCompleted is the reason set when the PipelineRun completed successfully with one or more skipped Tasks</p>
</td>
</tr><tr><td><p>&#34;Failed&#34;</p></td>
<td><p>PipelineRunReasonFailed is the reason set when the PipelineRun completed with a failure</p>
</td>
</tr><tr><td><p>&#34;PipelineRunPending&#34;</p></td>
<td><p>PipelineRunReasonPending is the reason set when the PipelineRun is in the pending state</p>
</td>
</tr><tr><td><p>&#34;Running&#34;</p></td>
<td><p>PipelineRunReasonRunning is the reason set when the PipelineRun is running</p>
</td>
</tr><tr><td><p>&#34;Started&#34;</p></td>
<td><p>PipelineRunReasonStarted is the reason set when the PipelineRun has just started</p>
</td>
</tr><tr><td><p>&#34;StoppedRunningFinally&#34;</p></td>
<td><p>PipelineRunReasonStoppedRunningFinally indicates that pipeline has been gracefully stopped
and no new Tasks will be scheduled by the controller, but final tasks are now running</p>
</td>
</tr><tr><td><p>&#34;PipelineRunStopping&#34;</p></td>
<td><p>PipelineRunReasonStopping indicates that no new Tasks will be scheduled by the controller, and the
pipeline will stop once all running tasks complete their work</p>
</td>
</tr><tr><td><p>&#34;Succeeded&#34;</p></td>
<td><p>PipelineRunReasonSuccessful is the reason set when the PipelineRun completed successfully</p>
</td>
</tr><tr><td><p>&#34;PipelineRunTimeout&#34;</p></td>
<td><p>PipelineRunReasonTimedOut is the reason set when the PipelineRun has timed out</p>
</td>
</tr></tbody>
</table>
<h3 id="tekton.dev/v1.PipelineRunResult">PipelineRunResult
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.PipelineRunStatusFields">PipelineRunStatusFields</a>)
</p>
<div>
<p>PipelineRunResult used to describe the results of a pipeline</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is the result&rsquo;s name as declared by the Pipeline</p>
</td>
</tr>
<tr>
<td>
<code>value</code><br/>
<em>
<a href="#tekton.dev/v1.ParamValue">
ParamValue
</a>
</em>
</td>
<td>
<p>Value is the result returned from the execution of this PipelineRun</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.PipelineRunRunStatus">PipelineRunRunStatus
</h3>
<div>
<p>PipelineRunRunStatus contains the name of the PipelineTask for this Run and the Run&rsquo;s Status</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>pipelineTaskName</code><br/>
<em>
string
</em>
</td>
<td>
<p>PipelineTaskName is the name of the PipelineTask.</p>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#tekton.dev/v1beta1.CustomRunStatus">
CustomRunStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Status is the RunStatus for the corresponding Run</p>
</td>
</tr>
<tr>
<td>
<code>whenExpressions</code><br/>
<em>
<a href="#tekton.dev/v1.WhenExpression">
[]WhenExpression
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>WhenExpressions is the list of checks guarding the execution of the PipelineTask</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.PipelineRunSpec">PipelineRunSpec
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.PipelineRun">PipelineRun</a>)
</p>
<div>
<p>PipelineRunSpec defines the desired state of PipelineRun</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>pipelineRef</code><br/>
<em>
<a href="#tekton.dev/v1.PipelineRef">
PipelineRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>pipelineSpec</code><br/>
<em>
<a href="#tekton.dev/v1.PipelineSpec">
PipelineSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>params</code><br/>
<em>
<a href="#tekton.dev/v1.Params">
Params
</a>
</em>
</td>
<td>
<p>Params is a list of parameter names and values.</p>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#tekton.dev/v1.PipelineRunSpecStatus">
PipelineRunSpecStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Used for cancelling a pipelinerun (and maybe more later on)</p>
</td>
</tr>
<tr>
<td>
<code>timeouts</code><br/>
<em>
<a href="#tekton.dev/v1.TimeoutFields">
TimeoutFields
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Time after which the Pipeline times out.
Currently three keys are accepted in the map
pipeline, tasks and finally
with Timeouts.pipeline &gt;= Timeouts.tasks + Timeouts.finally</p>
</td>
</tr>
<tr>
<td>
<code>taskRunTemplate</code><br/>
<em>
<a href="#tekton.dev/v1.PipelineTaskRunTemplate">
PipelineTaskRunTemplate
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>TaskRunTemplate represent template of taskrun</p>
</td>
</tr>
<tr>
<td>
<code>workspaces</code><br/>
<em>
<a href="#tekton.dev/v1.WorkspaceBinding">
[]WorkspaceBinding
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Workspaces holds a set of workspace bindings that must match names
with those declared in the pipeline.</p>
</td>
</tr>
<tr>
<td>
<code>taskRunSpecs</code><br/>
<em>
<a href="#tekton.dev/v1.PipelineTaskRunSpec">
[]PipelineTaskRunSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>TaskRunSpecs holds a set of runtime specs</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.PipelineRunSpecStatus">PipelineRunSpecStatus
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.PipelineRunSpec">PipelineRunSpec</a>)
</p>
<div>
<p>PipelineRunSpecStatus defines the pipelinerun spec status the user can provide</p>
</div>
<h3 id="tekton.dev/v1.PipelineRunStatus">PipelineRunStatus
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.PipelineRun">PipelineRun</a>)
</p>
<div>
<p>PipelineRunStatus defines the observed state of PipelineRun</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>Status</code><br/>
<em>
<a href="https://pkg.go.dev/knative.dev/pkg/apis/duck/v1#Status">
knative.dev/pkg/apis/duck/v1.Status
</a>
</em>
</td>
<td>
<p>
(Members of <code>Status</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>PipelineRunStatusFields</code><br/>
<em>
<a href="#tekton.dev/v1.PipelineRunStatusFields">
PipelineRunStatusFields
</a>
</em>
</td>
<td>
<p>
(Members of <code>PipelineRunStatusFields</code> are embedded into this type.)
</p>
<p>PipelineRunStatusFields inlines the status fields.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.PipelineRunStatusFields">PipelineRunStatusFields
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.PipelineRunStatus">PipelineRunStatus</a>)
</p>
<div>
<p>PipelineRunStatusFields holds the fields of PipelineRunStatus&rsquo; status.
This is defined separately and inlined so that other types can readily
consume these fields via duck typing.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>startTime</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<p>StartTime is the time the PipelineRun is actually started.</p>
</td>
</tr>
<tr>
<td>
<code>completionTime</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<p>CompletionTime is the time the PipelineRun completed.</p>
</td>
</tr>
<tr>
<td>
<code>results</code><br/>
<em>
<a href="#tekton.dev/v1.PipelineRunResult">
[]PipelineRunResult
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Results are the list of results written out by the pipeline task&rsquo;s containers</p>
</td>
</tr>
<tr>
<td>
<code>pipelineSpec</code><br/>
<em>
<a href="#tekton.dev/v1.PipelineSpec">
PipelineSpec
</a>
</em>
</td>
<td>
<p>PipelineRunSpec contains the exact spec used to instantiate the run</p>
</td>
</tr>
<tr>
<td>
<code>skippedTasks</code><br/>
<em>
<a href="#tekton.dev/v1.SkippedTask">
[]SkippedTask
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>list of tasks that were skipped due to when expressions evaluating to false</p>
</td>
</tr>
<tr>
<td>
<code>childReferences</code><br/>
<em>
<a href="#tekton.dev/v1.ChildStatusReference">
[]ChildStatusReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>list of TaskRun and Run names, PipelineTask names, and API versions/kinds for children of this PipelineRun.</p>
</td>
</tr>
<tr>
<td>
<code>finallyStartTime</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>FinallyStartTime is when all non-finally tasks have been completed and only finally tasks are being executed.</p>
</td>
</tr>
<tr>
<td>
<code>provenance</code><br/>
<em>
<a href="#tekton.dev/v1.Provenance">
Provenance
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Provenance contains some key authenticated metadata about how a software artifact was built (what sources, what inputs/outputs, etc.).</p>
</td>
</tr>
<tr>
<td>
<code>spanContext</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<p>SpanContext contains tracing span context fields</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.PipelineRunTaskRunStatus">PipelineRunTaskRunStatus
</h3>
<div>
<p>PipelineRunTaskRunStatus contains the name of the PipelineTask for this TaskRun and the TaskRun&rsquo;s Status</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>pipelineTaskName</code><br/>
<em>
string
</em>
</td>
<td>
<p>PipelineTaskName is the name of the PipelineTask.</p>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#tekton.dev/v1.TaskRunStatus">
TaskRunStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Status is the TaskRunStatus for the corresponding TaskRun</p>
</td>
</tr>
<tr>
<td>
<code>whenExpressions</code><br/>
<em>
<a href="#tekton.dev/v1.WhenExpression">
[]WhenExpression
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>WhenExpressions is the list of checks guarding the execution of the PipelineTask</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.PipelineSpec">PipelineSpec
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.Pipeline">Pipeline</a>, <a href="#tekton.dev/v1.PipelineRunSpec">PipelineRunSpec</a>, <a href="#tekton.dev/v1.PipelineRunStatusFields">PipelineRunStatusFields</a>)
</p>
<div>
<p>PipelineSpec defines the desired state of Pipeline.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>displayName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>DisplayName is a user-facing name of the pipeline that may be
used to populate a UI.</p>
</td>
</tr>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Description is a user-facing description of the pipeline that may be
used to populate a UI.</p>
</td>
</tr>
<tr>
<td>
<code>tasks</code><br/>
<em>
<a href="#tekton.dev/v1.PipelineTask">
[]PipelineTask
</a>
</em>
</td>
<td>
<p>Tasks declares the graph of Tasks that execute when this Pipeline is run.</p>
</td>
</tr>
<tr>
<td>
<code>params</code><br/>
<em>
<a href="#tekton.dev/v1.ParamSpecs">
ParamSpecs
</a>
</em>
</td>
<td>
<p>Params declares a list of input parameters that must be supplied when
this Pipeline is run.</p>
</td>
</tr>
<tr>
<td>
<code>workspaces</code><br/>
<em>
<a href="#tekton.dev/v1.PipelineWorkspaceDeclaration">
[]PipelineWorkspaceDeclaration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Workspaces declares a set of named workspaces that are expected to be
provided by a PipelineRun.</p>
</td>
</tr>
<tr>
<td>
<code>results</code><br/>
<em>
<a href="#tekton.dev/v1.PipelineResult">
[]PipelineResult
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Results are values that this pipeline can output once run</p>
</td>
</tr>
<tr>
<td>
<code>finally</code><br/>
<em>
<a href="#tekton.dev/v1.PipelineTask">
[]PipelineTask
</a>
</em>
</td>
<td>
<p>Finally declares the list of Tasks that execute just before leaving the Pipeline
i.e. either after all Tasks are finished executing successfully
or after a failure which would result in ending the Pipeline</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.PipelineTask">PipelineTask
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.PipelineSpec">PipelineSpec</a>)
</p>
<div>
<p>PipelineTask defines a task in a Pipeline, passing inputs from both
Params and from the output of previous tasks.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is the name of this task within the context of a Pipeline. Name is
used as a coordinate with the <code>from</code> and <code>runAfter</code> fields to establish
the execution order of tasks relative to one another.</p>
</td>
</tr>
<tr>
<td>
<code>displayName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>DisplayName is the display name of this task within the context of a Pipeline.
This display name may be used to populate a UI.</p>
</td>
</tr>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Description is the description of this task within the context of a Pipeline.
This description may be used to populate a UI.</p>
</td>
</tr>
<tr>
<td>
<code>taskRef</code><br/>
<em>
<a href="#tekton.dev/v1.TaskRef">
TaskRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>TaskRef is a reference to a task definition.</p>
</td>
</tr>
<tr>
<td>
<code>taskSpec</code><br/>
<em>
<a href="#tekton.dev/v1.EmbeddedTask">
EmbeddedTask
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>TaskSpec is a specification of a task</p>
</td>
</tr>
<tr>
<td>
<code>when</code><br/>
<em>
<a href="#tekton.dev/v1.WhenExpressions">
WhenExpressions
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>When is a list of when expressions that need to be true for the task to run</p>
</td>
</tr>
<tr>
<td>
<code>retries</code><br/>
<em>
int
</em>
</td>
<td>
<em>(Optional)</em>
<p>Retries represents how many times this task should be retried in case of task failure: ConditionSucceeded set to False</p>
</td>
</tr>
<tr>
<td>
<code>runAfter</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>RunAfter is the list of PipelineTask names that should be executed before
this Task executes. (Used to force a specific ordering in graph execution.)</p>
</td>
</tr>
<tr>
<td>
<code>params</code><br/>
<em>
<a href="#tekton.dev/v1.Params">
Params
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Parameters declares parameters passed to this task.</p>
</td>
</tr>
<tr>
<td>
<code>matrix</code><br/>
<em>
<a href="#tekton.dev/v1.Matrix">
Matrix
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Matrix declares parameters used to fan out this task.</p>
</td>
</tr>
<tr>
<td>
<code>workspaces</code><br/>
<em>
<a href="#tekton.dev/v1.WorkspacePipelineTaskBinding">
[]WorkspacePipelineTaskBinding
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Workspaces maps workspaces from the pipeline spec to the workspaces
declared in the Task.</p>
</td>
</tr>
<tr>
<td>
<code>timeout</code><br/>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Time after which the TaskRun times out. Defaults to 1 hour.
Refer Go&rsquo;s ParseDuration documentation for expected format: <a href="https://golang.org/pkg/time/#ParseDuration">https://golang.org/pkg/time/#ParseDuration</a></p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.PipelineTaskMetadata">PipelineTaskMetadata
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.EmbeddedTask">EmbeddedTask</a>, <a href="#tekton.dev/v1.PipelineTaskRunSpec">PipelineTaskRunSpec</a>)
</p>
<div>
<p>PipelineTaskMetadata contains the labels or annotations for an EmbeddedTask</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>labels</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>annotations</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.PipelineTaskParam">PipelineTaskParam
</h3>
<div>
<p>PipelineTaskParam is used to provide arbitrary string parameters to a Task.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>value</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.PipelineTaskRun">PipelineTaskRun
</h3>
<div>
<p>PipelineTaskRun reports the results of running a step in the Task. Each
task has the potential to succeed or fail (based on the exit code)
and produces logs.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.PipelineTaskRunSpec">PipelineTaskRunSpec
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.PipelineRunSpec">PipelineRunSpec</a>)
</p>
<div>
<p>PipelineTaskRunSpec  can be used to configure specific
specs for a concrete Task</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>pipelineTaskName</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>serviceAccountName</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>podTemplate</code><br/>
<em>
<a href="#tekton.dev/unversioned.Template">
Template
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>stepSpecs</code><br/>
<em>
<a href="#tekton.dev/v1.TaskRunStepSpec">
[]TaskRunStepSpec
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>sidecarSpecs</code><br/>
<em>
<a href="#tekton.dev/v1.TaskRunSidecarSpec">
[]TaskRunSidecarSpec
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="#tekton.dev/v1.PipelineTaskMetadata">
PipelineTaskMetadata
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>computeResources</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#resourcerequirements-v1-core">
Kubernetes core/v1.ResourceRequirements
</a>
</em>
</td>
<td>
<p>Compute resources to use for this TaskRun</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.PipelineTaskRunTemplate">PipelineTaskRunTemplate
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.PipelineRunSpec">PipelineRunSpec</a>)
</p>
<div>
<p>PipelineTaskRunTemplate is used to specify run specifications for all Task in pipelinerun.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>podTemplate</code><br/>
<em>
<a href="#tekton.dev/unversioned.Template">
Template
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>serviceAccountName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.PipelineWorkspaceDeclaration">PipelineWorkspaceDeclaration
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.PipelineSpec">PipelineSpec</a>)
</p>
<div>
<p>WorkspacePipelineDeclaration creates a named slot in a Pipeline that a PipelineRun
is expected to populate with a workspace binding.</p>
<p>Deprecated: use PipelineWorkspaceDeclaration type instead</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is the name of a workspace to be provided by a PipelineRun.</p>
</td>
</tr>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Description is a human readable string describing how the workspace will be
used in the Pipeline. It can be useful to include a bit of detail about which
tasks are intended to have access to the data on the workspace.</p>
</td>
</tr>
<tr>
<td>
<code>optional</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Optional marks a Workspace as not being required in PipelineRuns. By default
this field is false and so declared workspaces are required.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.PropertySpec">PropertySpec
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.ParamSpec">ParamSpec</a>, <a href="#tekton.dev/v1.TaskResult">TaskResult</a>)
</p>
<div>
<p>PropertySpec defines the struct for object keys</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>type</code><br/>
<em>
<a href="#tekton.dev/v1.ParamType">
ParamType
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.Provenance">Provenance
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.PipelineRunStatusFields">PipelineRunStatusFields</a>, <a href="#tekton.dev/v1.TaskRunStatusFields">TaskRunStatusFields</a>)
</p>
<div>
<p>Provenance contains metadata about resources used in the TaskRun/PipelineRun
such as the source from where a remote build definition was fetched.
This field aims to carry minimum amoumt of metadata in *Run status so that
Tekton Chains can capture them in the provenance.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>refSource</code><br/>
<em>
<a href="#tekton.dev/v1.RefSource">
RefSource
</a>
</em>
</td>
<td>
<p>RefSource identifies the source where a remote task/pipeline came from.</p>
</td>
</tr>
<tr>
<td>
<code>featureFlags</code><br/>
<em>
github.com/tektoncd/pipeline/pkg/apis/config.FeatureFlags
</em>
</td>
<td>
<p>FeatureFlags identifies the feature flags that were used during the task/pipeline run</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.RefSource">RefSource
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.Provenance">Provenance</a>, <a href="#resolution.tekton.dev/v1alpha1.ResolutionRequestStatusFields">ResolutionRequestStatusFields</a>, <a href="#resolution.tekton.dev/v1beta1.ResolutionRequestStatusFields">ResolutionRequestStatusFields</a>)
</p>
<div>
<p>RefSource contains the information that can uniquely identify where a remote
built definition came from i.e. Git repositories, Tekton Bundles in OCI registry
and hub.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>uri</code><br/>
<em>
string
</em>
</td>
<td>
<p>URI indicates the identity of the source of the build definition.
Example: &ldquo;<a href="https://github.com/tektoncd/catalog&quot;">https://github.com/tektoncd/catalog&rdquo;</a></p>
</td>
</tr>
<tr>
<td>
<code>digest</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<p>Digest is a collection of cryptographic digests for the contents of the artifact specified by URI.
Example: {&ldquo;sha1&rdquo;: &ldquo;f99d13e554ffcb696dee719fa85b695cb5b0f428&rdquo;}</p>
</td>
</tr>
<tr>
<td>
<code>entryPoint</code><br/>
<em>
string
</em>
</td>
<td>
<p>EntryPoint identifies the entry point into the build. This is often a path to a
build definition file and/or a target label within that file.
Example: &ldquo;task/git-clone/0.8/git-clone.yaml&rdquo;</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.ResolverName">ResolverName
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.ResolverRef">ResolverRef</a>)
</p>
<div>
<p>ResolverName is the name of a resolver from which a resource can be
requested.</p>
</div>
<h3 id="tekton.dev/v1.ResolverRef">ResolverRef
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.PipelineRef">PipelineRef</a>, <a href="#tekton.dev/v1.TaskRef">TaskRef</a>)
</p>
<div>
<p>ResolverRef can be used to refer to a Pipeline or Task in a remote
location like a git repo. This feature is in beta and these fields
are only available when the beta feature gate is enabled.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>resolver</code><br/>
<em>
<a href="#tekton.dev/v1.ResolverName">
ResolverName
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resolver is the name of the resolver that should perform
resolution of the referenced Tekton resource, such as &ldquo;git&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>params</code><br/>
<em>
<a href="#tekton.dev/v1.Params">
Params
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Params contains the parameters used to identify the
referenced Tekton resource. Example entries might include
&ldquo;repo&rdquo; or &ldquo;path&rdquo; but the set of params ultimately depends on
the chosen resolver.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.ResultRef">ResultRef
</h3>
<div>
<p>ResultRef is a type that represents a reference to a task run result</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>pipelineTask</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>result</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>resultsIndex</code><br/>
<em>
int
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>property</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.ResultsType">ResultsType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.PipelineResult">PipelineResult</a>, <a href="#tekton.dev/v1.TaskResult">TaskResult</a>, <a href="#tekton.dev/v1.TaskRunResult">TaskRunResult</a>)
</p>
<div>
<p>ResultsType indicates the type of a result;
Used to distinguish between a single string and an array of strings.
Note that there is ResultType used to find out whether a
RunResult is from a task result or not, which is different from
this ResultsType.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;array&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;object&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;string&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="tekton.dev/v1.Sidecar">Sidecar
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.TaskSpec">TaskSpec</a>)
</p>
<div>
<p>Sidecar has nearly the same data structure as Step but does not have the ability to timeout.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name of the Sidecar specified as a DNS_LABEL.
Each Sidecar in a Task must have a unique name (DNS_LABEL).
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>image</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Image reference name.
More info: <a href="https://kubernetes.io/docs/concepts/containers/images">https://kubernetes.io/docs/concepts/containers/images</a>
This field is optional to allow higher level config management to default or override
container images in workload controllers like Deployments and StatefulSets.</p>
</td>
</tr>
<tr>
<td>
<code>command</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Entrypoint array. Not executed within a shell.
The image&rsquo;s ENTRYPOINT is used if this is not provided.
Variable references $(VAR_NAME) are expanded using the Sidecar&rsquo;s environment. If a variable
cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced
to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. &ldquo;$$(VAR_NAME)&rdquo; will
produce the string literal &ldquo;$(VAR_NAME)&rdquo;. Escaped references will never be expanded, regardless
of whether the variable exists or not. Cannot be updated.
More info: <a href="https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell">https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell</a></p>
</td>
</tr>
<tr>
<td>
<code>args</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Arguments to the entrypoint.
The image&rsquo;s CMD is used if this is not provided.
Variable references $(VAR_NAME) are expanded using the Sidecar&rsquo;s environment. If a variable
cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced
to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. &ldquo;$$(VAR_NAME)&rdquo; will
produce the string literal &ldquo;$(VAR_NAME)&rdquo;. Escaped references will never be expanded, regardless
of whether the variable exists or not. Cannot be updated.
More info: <a href="https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell">https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell</a></p>
</td>
</tr>
<tr>
<td>
<code>workingDir</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Sidecar&rsquo;s working directory.
If not specified, the container runtime&rsquo;s default will be used, which
might be configured in the container image.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>ports</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#containerport-v1-core">
[]Kubernetes core/v1.ContainerPort
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>List of ports to expose from the Sidecar. Exposing a port here gives
the system additional information about the network connections a
container uses, but is primarily informational. Not specifying a port here
DOES NOT prevent that port from being exposed. Any port which is
listening on the default &ldquo;0.0.0.0&rdquo; address inside a container will be
accessible from the network.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>envFrom</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#envfromsource-v1-core">
[]Kubernetes core/v1.EnvFromSource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>List of sources to populate environment variables in the Sidecar.
The keys defined within a source must be a C_IDENTIFIER. All invalid keys
will be reported as an event when the container is starting. When a key exists in multiple
sources, the value associated with the last source will take precedence.
Values defined by an Env with a duplicate key will take precedence.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>env</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#envvar-v1-core">
[]Kubernetes core/v1.EnvVar
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>List of environment variables to set in the Sidecar.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>computeResources</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#resourcerequirements-v1-core">
Kubernetes core/v1.ResourceRequirements
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ComputeResources required by this Sidecar.
Cannot be updated.
More info: <a href="https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/">https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/</a></p>
</td>
</tr>
<tr>
<td>
<code>volumeMounts</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#volumemount-v1-core">
[]Kubernetes core/v1.VolumeMount
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Volumes to mount into the Sidecar&rsquo;s filesystem.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>volumeDevices</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#volumedevice-v1-core">
[]Kubernetes core/v1.VolumeDevice
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>volumeDevices is the list of block devices to be used by the Sidecar.</p>
</td>
</tr>
<tr>
<td>
<code>livenessProbe</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#probe-v1-core">
Kubernetes core/v1.Probe
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Periodic probe of Sidecar liveness.
Container will be restarted if the probe fails.
Cannot be updated.
More info: <a href="https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes">https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes</a></p>
</td>
</tr>
<tr>
<td>
<code>readinessProbe</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#probe-v1-core">
Kubernetes core/v1.Probe
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Periodic probe of Sidecar service readiness.
Container will be removed from service endpoints if the probe fails.
Cannot be updated.
More info: <a href="https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes">https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes</a></p>
</td>
</tr>
<tr>
<td>
<code>startupProbe</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#probe-v1-core">
Kubernetes core/v1.Probe
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>StartupProbe indicates that the Pod the Sidecar is running in has successfully initialized.
If specified, no other probes are executed until this completes successfully.
If this probe fails, the Pod will be restarted, just as if the livenessProbe failed.
This can be used to provide different probe parameters at the beginning of a Pod&rsquo;s lifecycle,
when it might take a long time to load data or warm a cache, than during steady-state operation.
This cannot be updated.
More info: <a href="https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes">https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes</a></p>
</td>
</tr>
<tr>
<td>
<code>lifecycle</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#lifecycle-v1-core">
Kubernetes core/v1.Lifecycle
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Actions that the management system should take in response to Sidecar lifecycle events.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>terminationMessagePath</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Optional: Path at which the file to which the Sidecar&rsquo;s termination message
will be written is mounted into the Sidecar&rsquo;s filesystem.
Message written is intended to be brief final status, such as an assertion failure message.
Will be truncated by the node if greater than 4096 bytes. The total message length across
all containers will be limited to 12kb.
Defaults to /dev/termination-log.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>terminationMessagePolicy</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#terminationmessagepolicy-v1-core">
Kubernetes core/v1.TerminationMessagePolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Indicate how the termination message should be populated. File will use the contents of
terminationMessagePath to populate the Sidecar status message on both success and failure.
FallbackToLogsOnError will use the last chunk of Sidecar log output if the termination
message file is empty and the Sidecar exited with an error.
The log output is limited to 2048 bytes or 80 lines, whichever is smaller.
Defaults to File.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>imagePullPolicy</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#pullpolicy-v1-core">
Kubernetes core/v1.PullPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Image pull policy.
One of Always, Never, IfNotPresent.
Defaults to Always if :latest tag is specified, or IfNotPresent otherwise.
Cannot be updated.
More info: <a href="https://kubernetes.io/docs/concepts/containers/images#updating-images">https://kubernetes.io/docs/concepts/containers/images#updating-images</a></p>
</td>
</tr>
<tr>
<td>
<code>securityContext</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#securitycontext-v1-core">
Kubernetes core/v1.SecurityContext
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SecurityContext defines the security options the Sidecar should be run with.
If set, the fields of SecurityContext override the equivalent fields of PodSecurityContext.
More info: <a href="https://kubernetes.io/docs/tasks/configure-pod-container/security-context/">https://kubernetes.io/docs/tasks/configure-pod-container/security-context/</a></p>
</td>
</tr>
<tr>
<td>
<code>stdin</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Whether this Sidecar should allocate a buffer for stdin in the container runtime. If this
is not set, reads from stdin in the Sidecar will always result in EOF.
Default is false.</p>
</td>
</tr>
<tr>
<td>
<code>stdinOnce</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Whether the container runtime should close the stdin channel after it has been opened by
a single attach. When stdin is true the stdin stream will remain open across multiple attach
sessions. If stdinOnce is set to true, stdin is opened on Sidecar start, is empty until the
first client attaches to stdin, and then remains open and accepts data until the client disconnects,
at which time stdin is closed and remains closed until the Sidecar is restarted. If this
flag is false, a container processes that reads from stdin will never receive an EOF.
Default is false</p>
</td>
</tr>
<tr>
<td>
<code>tty</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Whether this Sidecar should allocate a TTY for itself, also requires &lsquo;stdin&rsquo; to be true.
Default is false.</p>
</td>
</tr>
<tr>
<td>
<code>script</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Script is the contents of an executable file to execute.</p>
<p>If Script is not empty, the Step cannot have an Command or Args.</p>
</td>
</tr>
<tr>
<td>
<code>workspaces</code><br/>
<em>
<a href="#tekton.dev/v1.WorkspaceUsage">
[]WorkspaceUsage
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>This is an alpha field. You must set the &ldquo;enable-api-fields&rdquo; feature flag to &ldquo;alpha&rdquo;
for this field to be supported.</p>
<p>Workspaces is a list of workspaces from the Task that this Sidecar wants
exclusive access to. Adding a workspace to this list means that any
other Step or Sidecar that does not also request this Workspace will
not have access to it.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.SidecarState">SidecarState
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.TaskRunStatusFields">TaskRunStatusFields</a>)
</p>
<div>
<p>SidecarState reports the results of running a sidecar in a Task.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>ContainerState</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#containerstate-v1-core">
Kubernetes core/v1.ContainerState
</a>
</em>
</td>
<td>
<p>
(Members of <code>ContainerState</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>container</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>imageID</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.SkippedTask">SkippedTask
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.PipelineRunStatusFields">PipelineRunStatusFields</a>)
</p>
<div>
<p>SkippedTask is used to describe the Tasks that were skipped due to their When Expressions
evaluating to False. This is a struct because we are looking into including more details
about the When Expressions that caused this Task to be skipped.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is the Pipeline Task name</p>
</td>
</tr>
<tr>
<td>
<code>reason</code><br/>
<em>
<a href="#tekton.dev/v1.SkippingReason">
SkippingReason
</a>
</em>
</td>
<td>
<p>Reason is the cause of the PipelineTask being skipped.</p>
</td>
</tr>
<tr>
<td>
<code>whenExpressions</code><br/>
<em>
<a href="#tekton.dev/v1.WhenExpression">
[]WhenExpression
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>WhenExpressions is the list of checks guarding the execution of the PipelineTask</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.SkippingReason">SkippingReason
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.SkippedTask">SkippedTask</a>)
</p>
<div>
<p>SkippingReason explains why a PipelineTask was skipped.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Matrix Parameters have an empty array&#34;</p></td>
<td><p>EmptyArrayInMatrixParams means the task was skipped because Matrix parameters contain empty array.</p>
</td>
</tr><tr><td><p>&#34;PipelineRun Finally timeout has been reached&#34;</p></td>
<td><p>FinallyTimedOutSkip means the task was skipped because the PipelineRun has passed its Timeouts.Finally.</p>
</td>
</tr><tr><td><p>&#34;PipelineRun was gracefully cancelled&#34;</p></td>
<td><p>GracefullyCancelledSkip means the task was skipped because the pipeline run has been gracefully cancelled</p>
</td>
</tr><tr><td><p>&#34;PipelineRun was gracefully stopped&#34;</p></td>
<td><p>GracefullyStoppedSkip means the task was skipped because the pipeline run has been gracefully stopped</p>
</td>
</tr><tr><td><p>&#34;Results were missing&#34;</p></td>
<td><p>MissingResultsSkip means the task was skipped because it&rsquo;s missing necessary results</p>
</td>
</tr><tr><td><p>&#34;None&#34;</p></td>
<td><p>None means the task was not skipped</p>
</td>
</tr><tr><td><p>&#34;Parent Tasks were skipped&#34;</p></td>
<td><p>ParentTasksSkip means the task was skipped because its parent was skipped</p>
</td>
</tr><tr><td><p>&#34;PipelineRun timeout has been reached&#34;</p></td>
<td><p>PipelineTimedOutSkip means the task was skipped because the PipelineRun has passed its overall timeout.</p>
</td>
</tr><tr><td><p>&#34;PipelineRun was stopping&#34;</p></td>
<td><p>StoppingSkip means the task was skipped because the pipeline run is stopping</p>
</td>
</tr><tr><td><p>&#34;PipelineRun Tasks timeout has been reached&#34;</p></td>
<td><p>TasksTimedOutSkip means the task was skipped because the PipelineRun has passed its Timeouts.Tasks.</p>
</td>
</tr><tr><td><p>&#34;When Expressions evaluated to false&#34;</p></td>
<td><p>WhenExpressionsSkip means the task was skipped due to at least one of its when expressions evaluating to false</p>
</td>
</tr></tbody>
</table>
<h3 id="tekton.dev/v1.Step">Step
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.TaskSpec">TaskSpec</a>)
</p>
<div>
<p>Step runs a subcomponent of a Task</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name of the Step specified as a DNS_LABEL.
Each Step in a Task must have a unique name.</p>
</td>
</tr>
<tr>
<td>
<code>image</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Docker image name.
More info: <a href="https://kubernetes.io/docs/concepts/containers/images">https://kubernetes.io/docs/concepts/containers/images</a></p>
</td>
</tr>
<tr>
<td>
<code>command</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Entrypoint array. Not executed within a shell.
The image&rsquo;s ENTRYPOINT is used if this is not provided.
Variable references $(VAR_NAME) are expanded using the container&rsquo;s environment. If a variable
cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced
to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. &ldquo;$$(VAR_NAME)&rdquo; will
produce the string literal &ldquo;$(VAR_NAME)&rdquo;. Escaped references will never be expanded, regardless
of whether the variable exists or not. Cannot be updated.
More info: <a href="https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell">https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell</a></p>
</td>
</tr>
<tr>
<td>
<code>args</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Arguments to the entrypoint.
The image&rsquo;s CMD is used if this is not provided.
Variable references $(VAR_NAME) are expanded using the container&rsquo;s environment. If a variable
cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced
to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. &ldquo;$$(VAR_NAME)&rdquo; will
produce the string literal &ldquo;$(VAR_NAME)&rdquo;. Escaped references will never be expanded, regardless
of whether the variable exists or not. Cannot be updated.
More info: <a href="https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell">https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell</a></p>
</td>
</tr>
<tr>
<td>
<code>workingDir</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Step&rsquo;s working directory.
If not specified, the container runtime&rsquo;s default will be used, which
might be configured in the container image.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>envFrom</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#envfromsource-v1-core">
[]Kubernetes core/v1.EnvFromSource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>List of sources to populate environment variables in the Step.
The keys defined within a source must be a C_IDENTIFIER. All invalid keys
will be reported as an event when the Step is starting. When a key exists in multiple
sources, the value associated with the last source will take precedence.
Values defined by an Env with a duplicate key will take precedence.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>env</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#envvar-v1-core">
[]Kubernetes core/v1.EnvVar
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>List of environment variables to set in the Step.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>computeResources</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#resourcerequirements-v1-core">
Kubernetes core/v1.ResourceRequirements
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ComputeResources required by this Step.
Cannot be updated.
More info: <a href="https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/">https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/</a></p>
</td>
</tr>
<tr>
<td>
<code>volumeMounts</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#volumemount-v1-core">
[]Kubernetes core/v1.VolumeMount
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Volumes to mount into the Step&rsquo;s filesystem.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>volumeDevices</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#volumedevice-v1-core">
[]Kubernetes core/v1.VolumeDevice
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>volumeDevices is the list of block devices to be used by the Step.</p>
</td>
</tr>
<tr>
<td>
<code>imagePullPolicy</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#pullpolicy-v1-core">
Kubernetes core/v1.PullPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Image pull policy.
One of Always, Never, IfNotPresent.
Defaults to Always if :latest tag is specified, or IfNotPresent otherwise.
Cannot be updated.
More info: <a href="https://kubernetes.io/docs/concepts/containers/images#updating-images">https://kubernetes.io/docs/concepts/containers/images#updating-images</a></p>
</td>
</tr>
<tr>
<td>
<code>securityContext</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#securitycontext-v1-core">
Kubernetes core/v1.SecurityContext
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SecurityContext defines the security options the Step should be run with.
If set, the fields of SecurityContext override the equivalent fields of PodSecurityContext.
More info: <a href="https://kubernetes.io/docs/tasks/configure-pod-container/security-context/">https://kubernetes.io/docs/tasks/configure-pod-container/security-context/</a></p>
</td>
</tr>
<tr>
<td>
<code>script</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Script is the contents of an executable file to execute.</p>
<p>If Script is not empty, the Step cannot have an Command and the Args will be passed to the Script.</p>
</td>
</tr>
<tr>
<td>
<code>timeout</code><br/>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Timeout is the time after which the step times out. Defaults to never.
Refer to Go&rsquo;s ParseDuration documentation for expected format: <a href="https://golang.org/pkg/time/#ParseDuration">https://golang.org/pkg/time/#ParseDuration</a></p>
</td>
</tr>
<tr>
<td>
<code>workspaces</code><br/>
<em>
<a href="#tekton.dev/v1.WorkspaceUsage">
[]WorkspaceUsage
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>This is an alpha field. You must set the &ldquo;enable-api-fields&rdquo; feature flag to &ldquo;alpha&rdquo;
for this field to be supported.</p>
<p>Workspaces is a list of workspaces from the Task that this Step wants
exclusive access to. Adding a workspace to this list means that any
other Step or Sidecar that does not also request this Workspace will
not have access to it.</p>
</td>
</tr>
<tr>
<td>
<code>onError</code><br/>
<em>
<a href="#tekton.dev/v1.OnErrorType">
OnErrorType
</a>
</em>
</td>
<td>
<p>OnError defines the exiting behavior of a container on error
can be set to [ continue | stopAndFail ]</p>
</td>
</tr>
<tr>
<td>
<code>stdoutConfig</code><br/>
<em>
<a href="#tekton.dev/v1.StepOutputConfig">
StepOutputConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Stores configuration for the stdout stream of the step.</p>
</td>
</tr>
<tr>
<td>
<code>stderrConfig</code><br/>
<em>
<a href="#tekton.dev/v1.StepOutputConfig">
StepOutputConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Stores configuration for the stderr stream of the step.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.StepOutputConfig">StepOutputConfig
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.Step">Step</a>)
</p>
<div>
<p>StepOutputConfig stores configuration for a step output stream.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>path</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Path to duplicate stdout stream to on container&rsquo;s local filesystem.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.StepState">StepState
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.TaskRunStatusFields">TaskRunStatusFields</a>)
</p>
<div>
<p>StepState reports the results of running a step in a Task.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>ContainerState</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#containerstate-v1-core">
Kubernetes core/v1.ContainerState
</a>
</em>
</td>
<td>
<p>
(Members of <code>ContainerState</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>container</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>imageID</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.StepTemplate">StepTemplate
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.TaskSpec">TaskSpec</a>)
</p>
<div>
<p>StepTemplate is a template for a Step</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>image</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Image reference name.
More info: <a href="https://kubernetes.io/docs/concepts/containers/images">https://kubernetes.io/docs/concepts/containers/images</a>
This field is optional to allow higher level config management to default or override
container images in workload controllers like Deployments and StatefulSets.</p>
</td>
</tr>
<tr>
<td>
<code>command</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Entrypoint array. Not executed within a shell.
The image&rsquo;s ENTRYPOINT is used if this is not provided.
Variable references $(VAR_NAME) are expanded using the Step&rsquo;s environment. If a variable
cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced
to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. &ldquo;$$(VAR_NAME)&rdquo; will
produce the string literal &ldquo;$(VAR_NAME)&rdquo;. Escaped references will never be expanded, regardless
of whether the variable exists or not. Cannot be updated.
More info: <a href="https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell">https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell</a></p>
</td>
</tr>
<tr>
<td>
<code>args</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Arguments to the entrypoint.
The image&rsquo;s CMD is used if this is not provided.
Variable references $(VAR_NAME) are expanded using the Step&rsquo;s environment. If a variable
cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced
to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. &ldquo;$$(VAR_NAME)&rdquo; will
produce the string literal &ldquo;$(VAR_NAME)&rdquo;. Escaped references will never be expanded, regardless
of whether the variable exists or not. Cannot be updated.
More info: <a href="https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell">https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell</a></p>
</td>
</tr>
<tr>
<td>
<code>workingDir</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Step&rsquo;s working directory.
If not specified, the container runtime&rsquo;s default will be used, which
might be configured in the container image.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>envFrom</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#envfromsource-v1-core">
[]Kubernetes core/v1.EnvFromSource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>List of sources to populate environment variables in the Step.
The keys defined within a source must be a C_IDENTIFIER. All invalid keys
will be reported as an event when the Step is starting. When a key exists in multiple
sources, the value associated with the last source will take precedence.
Values defined by an Env with a duplicate key will take precedence.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>env</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#envvar-v1-core">
[]Kubernetes core/v1.EnvVar
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>List of environment variables to set in the Step.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>computeResources</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#resourcerequirements-v1-core">
Kubernetes core/v1.ResourceRequirements
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ComputeResources required by this Step.
Cannot be updated.
More info: <a href="https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/">https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/</a></p>
</td>
</tr>
<tr>
<td>
<code>volumeMounts</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#volumemount-v1-core">
[]Kubernetes core/v1.VolumeMount
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Volumes to mount into the Step&rsquo;s filesystem.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>volumeDevices</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#volumedevice-v1-core">
[]Kubernetes core/v1.VolumeDevice
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>volumeDevices is the list of block devices to be used by the Step.</p>
</td>
</tr>
<tr>
<td>
<code>imagePullPolicy</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#pullpolicy-v1-core">
Kubernetes core/v1.PullPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Image pull policy.
One of Always, Never, IfNotPresent.
Defaults to Always if :latest tag is specified, or IfNotPresent otherwise.
Cannot be updated.
More info: <a href="https://kubernetes.io/docs/concepts/containers/images#updating-images">https://kubernetes.io/docs/concepts/containers/images#updating-images</a></p>
</td>
</tr>
<tr>
<td>
<code>securityContext</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#securitycontext-v1-core">
Kubernetes core/v1.SecurityContext
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SecurityContext defines the security options the Step should be run with.
If set, the fields of SecurityContext override the equivalent fields of PodSecurityContext.
More info: <a href="https://kubernetes.io/docs/tasks/configure-pod-container/security-context/">https://kubernetes.io/docs/tasks/configure-pod-container/security-context/</a></p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.TaskKind">TaskKind
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.TaskRef">TaskRef</a>)
</p>
<div>
<p>TaskKind defines the type of Task used by the pipeline.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;ClusterTask&#34;</p></td>
<td><p>ClusterTaskRefKind is the task type for a reference to a task with cluster scope.
ClusterTasks are not supported in v1, but v1 types may reference ClusterTasks.</p>
</td>
</tr><tr><td><p>&#34;Task&#34;</p></td>
<td><p>NamespacedTaskKind indicates that the task type has a namespaced scope.</p>
</td>
</tr></tbody>
</table>
<h3 id="tekton.dev/v1.TaskRef">TaskRef
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.PipelineTask">PipelineTask</a>, <a href="#tekton.dev/v1.TaskRunSpec">TaskRunSpec</a>)
</p>
<div>
<p>TaskRef can be used to refer to a specific instance of a task.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name of the referent; More info: <a href="http://kubernetes.io/docs/user-guide/identifiers#names">http://kubernetes.io/docs/user-guide/identifiers#names</a></p>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
<em>
<a href="#tekton.dev/v1.TaskKind">
TaskKind
</a>
</em>
</td>
<td>
<p>TaskKind indicates the Kind of the Task:
1. Namespaced Task when Kind is set to &ldquo;Task&rdquo;. If Kind is &ldquo;&rdquo;, it defaults to &ldquo;Task&rdquo;.
2. Custom Task when Kind is non-empty and APIVersion is non-empty</p>
</td>
</tr>
<tr>
<td>
<code>apiVersion</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>API version of the referent
Note: A Task with non-empty APIVersion and Kind is considered a Custom Task</p>
</td>
</tr>
<tr>
<td>
<code>ResolverRef</code><br/>
<em>
<a href="#tekton.dev/v1.ResolverRef">
ResolverRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ResolverRef allows referencing a Task in a remote location
like a git repo. This field is only supported when the alpha
feature gate is enabled.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.TaskResult">TaskResult
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.TaskSpec">TaskSpec</a>)
</p>
<div>
<p>TaskResult used to describe the results of a task</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name the given name</p>
</td>
</tr>
<tr>
<td>
<code>type</code><br/>
<em>
<a href="#tekton.dev/v1.ResultsType">
ResultsType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Type is the user-specified type of the result. The possible type
is currently &ldquo;string&rdquo; and will support &ldquo;array&rdquo; in following work.</p>
</td>
</tr>
<tr>
<td>
<code>properties</code><br/>
<em>
<a href="#tekton.dev/v1.PropertySpec">
map[string]github.com/tektoncd/pipeline/pkg/apis/pipeline/v1.PropertySpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Properties is the JSON Schema properties to support key-value pairs results.</p>
</td>
</tr>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Description is a human-readable description of the result</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.TaskRunDebug">TaskRunDebug
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.TaskRunSpec">TaskRunSpec</a>)
</p>
<div>
<p>TaskRunDebug defines the breakpoint config for a particular TaskRun</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>breakpoint</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.TaskRunInputs">TaskRunInputs
</h3>
<div>
<p>TaskRunInputs holds the input values that this task was invoked with.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>params</code><br/>
<em>
<a href="#tekton.dev/v1.Params">
Params
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.TaskRunReason">TaskRunReason
(<code>string</code> alias)</h3>
<div>
<p>TaskRunReason is an enum used to store all TaskRun reason for
the Succeeded condition that are controlled by the TaskRun itself. Failure
reasons that emerge from underlying resources are not included here</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;TaskRunCancelled&#34;</p></td>
<td><p>TaskRunReasonCancelled is the reason set when the TaskRun is cancelled by the user</p>
</td>
</tr><tr><td><p>&#34;Failed&#34;</p></td>
<td><p>TaskRunReasonFailed is the reason set when the TaskRun completed with a failure</p>
</td>
</tr><tr><td><p>&#34;TaskRunImagePullFailed&#34;</p></td>
<td><p>TaskRunReasonImagePullFailed is the reason set when the step of a task fails due to image not being pulled</p>
</td>
</tr><tr><td><p>&#34;TaskRunResultLargerThanAllowedLimit&#34;</p></td>
<td><p>TaskRunReasonResultLargerThanAllowedLimit is the reason set when one of the results exceeds its maximum allowed limit of 1 KB</p>
</td>
</tr><tr><td><p>&#34;Running&#34;</p></td>
<td><p>TaskRunReasonRunning is the reason set when the TaskRun is running</p>
</td>
</tr><tr><td><p>&#34;Started&#34;</p></td>
<td><p>TaskRunReasonStarted is the reason set when the TaskRun has just started</p>
</td>
</tr><tr><td><p>&#34;Succeeded&#34;</p></td>
<td><p>TaskRunReasonSuccessful is the reason set when the TaskRun completed successfully</p>
</td>
</tr><tr><td><p>&#34;TaskRunTimeout&#34;</p></td>
<td><p>TaskRunReasonTimedOut is the reason set when one TaskRun execution has timed out</p>
</td>
</tr><tr><td><p>&#34;ToBeRetried&#34;</p></td>
<td><p>TaskRunReasonToBeRetried is the reason set when the last TaskRun execution failed, and will be retried</p>
</td>
</tr></tbody>
</table>
<h3 id="tekton.dev/v1.TaskRunResult">TaskRunResult
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.TaskRunStatusFields">TaskRunStatusFields</a>)
</p>
<div>
<p>TaskRunResult used to describe the results of a task</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name the given name</p>
</td>
</tr>
<tr>
<td>
<code>type</code><br/>
<em>
<a href="#tekton.dev/v1.ResultsType">
ResultsType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Type is the user-specified type of the result. The possible type
is currently &ldquo;string&rdquo; and will support &ldquo;array&rdquo; in following work.</p>
</td>
</tr>
<tr>
<td>
<code>value</code><br/>
<em>
<a href="#tekton.dev/v1.ParamValue">
ParamValue
</a>
</em>
</td>
<td>
<p>Value the given value of the result</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.TaskRunSidecarSpec">TaskRunSidecarSpec
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.PipelineTaskRunSpec">PipelineTaskRunSpec</a>, <a href="#tekton.dev/v1.TaskRunSpec">TaskRunSpec</a>)
</p>
<div>
<p>TaskRunSidecarSpec is used to override the values of a Sidecar in the corresponding Task.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>The name of the Sidecar to override.</p>
</td>
</tr>
<tr>
<td>
<code>computeResources</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#resourcerequirements-v1-core">
Kubernetes core/v1.ResourceRequirements
</a>
</em>
</td>
<td>
<p>The resource requirements to apply to the Sidecar.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.TaskRunSpec">TaskRunSpec
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.TaskRun">TaskRun</a>)
</p>
<div>
<p>TaskRunSpec defines the desired state of TaskRun</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>debug</code><br/>
<em>
<a href="#tekton.dev/v1.TaskRunDebug">
TaskRunDebug
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>params</code><br/>
<em>
<a href="#tekton.dev/v1.Params">
Params
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>serviceAccountName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>taskRef</code><br/>
<em>
<a href="#tekton.dev/v1.TaskRef">
TaskRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>no more than one of the TaskRef and TaskSpec may be specified.</p>
</td>
</tr>
<tr>
<td>
<code>taskSpec</code><br/>
<em>
<a href="#tekton.dev/v1.TaskSpec">
TaskSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#tekton.dev/v1.TaskRunSpecStatus">
TaskRunSpecStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Used for cancelling a TaskRun (and maybe more later on)</p>
</td>
</tr>
<tr>
<td>
<code>statusMessage</code><br/>
<em>
<a href="#tekton.dev/v1.TaskRunSpecStatusMessage">
TaskRunSpecStatusMessage
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Status message for cancellation.</p>
</td>
</tr>
<tr>
<td>
<code>retries</code><br/>
<em>
int
</em>
</td>
<td>
<em>(Optional)</em>
<p>Retries represents how many times this TaskRun should be retried in the event of task failure.</p>
</td>
</tr>
<tr>
<td>
<code>timeout</code><br/>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Time after which one retry attempt times out. Defaults to 1 hour.
Refer Go&rsquo;s ParseDuration documentation for expected format: <a href="https://golang.org/pkg/time/#ParseDuration">https://golang.org/pkg/time/#ParseDuration</a></p>
</td>
</tr>
<tr>
<td>
<code>podTemplate</code><br/>
<em>
<a href="#tekton.dev/unversioned.Template">
Template
</a>
</em>
</td>
<td>
<p>PodTemplate holds pod specific configuration</p>
</td>
</tr>
<tr>
<td>
<code>workspaces</code><br/>
<em>
<a href="#tekton.dev/v1.WorkspaceBinding">
[]WorkspaceBinding
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Workspaces is a list of WorkspaceBindings from volumes to workspaces.</p>
</td>
</tr>
<tr>
<td>
<code>stepSpecs</code><br/>
<em>
<a href="#tekton.dev/v1.TaskRunStepSpec">
[]TaskRunStepSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specs to apply to Steps in this TaskRun.
If a field is specified in both a Step and a StepSpec,
the value from the StepSpec will be used.
This field is only supported when the alpha feature gate is enabled.</p>
</td>
</tr>
<tr>
<td>
<code>sidecarSpecs</code><br/>
<em>
<a href="#tekton.dev/v1.TaskRunSidecarSpec">
[]TaskRunSidecarSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specs to apply to Sidecars in this TaskRun.
If a field is specified in both a Sidecar and a SidecarSpec,
the value from the SidecarSpec will be used.
This field is only supported when the alpha feature gate is enabled.</p>
</td>
</tr>
<tr>
<td>
<code>computeResources</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#resourcerequirements-v1-core">
Kubernetes core/v1.ResourceRequirements
</a>
</em>
</td>
<td>
<p>Compute resources to use for this TaskRun</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.TaskRunSpecStatus">TaskRunSpecStatus
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.TaskRunSpec">TaskRunSpec</a>)
</p>
<div>
<p>TaskRunSpecStatus defines the TaskRun spec status the user can provide</p>
</div>
<h3 id="tekton.dev/v1.TaskRunSpecStatusMessage">TaskRunSpecStatusMessage
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.TaskRunSpec">TaskRunSpec</a>)
</p>
<div>
<p>TaskRunSpecStatusMessage defines human readable status messages for the TaskRun.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;TaskRun cancelled as the PipelineRun it belongs to has been cancelled.&#34;</p></td>
<td><p>TaskRunCancelledByPipelineMsg indicates that the PipelineRun of which this
TaskRun was a part of has been cancelled.</p>
</td>
</tr><tr><td><p>&#34;TaskRun cancelled as the PipelineRun it belongs to has timed out.&#34;</p></td>
<td><p>TaskRunCancelledByPipelineTimeoutMsg indicates that the TaskRun was cancelled because the PipelineRun running it timed out.</p>
</td>
</tr></tbody>
</table>
<h3 id="tekton.dev/v1.TaskRunStatus">TaskRunStatus
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.TaskRun">TaskRun</a>, <a href="#tekton.dev/v1.PipelineRunTaskRunStatus">PipelineRunTaskRunStatus</a>, <a href="#tekton.dev/v1.TaskRunStatusFields">TaskRunStatusFields</a>)
</p>
<div>
<p>TaskRunStatus defines the observed state of TaskRun</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>Status</code><br/>
<em>
<a href="https://pkg.go.dev/knative.dev/pkg/apis/duck/v1#Status">
knative.dev/pkg/apis/duck/v1.Status
</a>
</em>
</td>
<td>
<p>
(Members of <code>Status</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>TaskRunStatusFields</code><br/>
<em>
<a href="#tekton.dev/v1.TaskRunStatusFields">
TaskRunStatusFields
</a>
</em>
</td>
<td>
<p>
(Members of <code>TaskRunStatusFields</code> are embedded into this type.)
</p>
<p>TaskRunStatusFields inlines the status fields.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.TaskRunStatusFields">TaskRunStatusFields
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.TaskRunStatus">TaskRunStatus</a>)
</p>
<div>
<p>TaskRunStatusFields holds the fields of TaskRun&rsquo;s status.  This is defined
separately and inlined so that other types can readily consume these fields
via duck typing.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>podName</code><br/>
<em>
string
</em>
</td>
<td>
<p>PodName is the name of the pod responsible for executing this task&rsquo;s steps.</p>
</td>
</tr>
<tr>
<td>
<code>startTime</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<p>StartTime is the time the build is actually started.</p>
</td>
</tr>
<tr>
<td>
<code>completionTime</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<p>CompletionTime is the time the build completed.</p>
</td>
</tr>
<tr>
<td>
<code>steps</code><br/>
<em>
<a href="#tekton.dev/v1.StepState">
[]StepState
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Steps describes the state of each build step container.</p>
</td>
</tr>
<tr>
<td>
<code>retriesStatus</code><br/>
<em>
<a href="#tekton.dev/v1.TaskRunStatus">
[]TaskRunStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>RetriesStatus contains the history of TaskRunStatus in case of a retry in order to keep record of failures.
All TaskRunStatus stored in RetriesStatus will have no date within the RetriesStatus as is redundant.</p>
</td>
</tr>
<tr>
<td>
<code>results</code><br/>
<em>
<a href="#tekton.dev/v1.TaskRunResult">
[]TaskRunResult
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Results are the list of results written out by the task&rsquo;s containers</p>
</td>
</tr>
<tr>
<td>
<code>sidecars</code><br/>
<em>
<a href="#tekton.dev/v1.SidecarState">
[]SidecarState
</a>
</em>
</td>
<td>
<p>The list has one entry per sidecar in the manifest. Each entry is
represents the imageid of the corresponding sidecar.</p>
</td>
</tr>
<tr>
<td>
<code>taskSpec</code><br/>
<em>
<a href="#tekton.dev/v1.TaskSpec">
TaskSpec
</a>
</em>
</td>
<td>
<p>TaskSpec contains the Spec from the dereferenced Task definition used to instantiate this TaskRun.</p>
</td>
</tr>
<tr>
<td>
<code>provenance</code><br/>
<em>
<a href="#tekton.dev/v1.Provenance">
Provenance
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Provenance contains some key authenticated metadata about how a software artifact was built (what sources, what inputs/outputs, etc.).</p>
</td>
</tr>
<tr>
<td>
<code>spanContext</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<p>SpanContext contains tracing span context fields</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.TaskRunStepSpec">TaskRunStepSpec
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.PipelineTaskRunSpec">PipelineTaskRunSpec</a>, <a href="#tekton.dev/v1.TaskRunSpec">TaskRunSpec</a>)
</p>
<div>
<p>TaskRunStepSpec is used to override the values of a Step in the corresponding Task.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>The name of the Step to override.</p>
</td>
</tr>
<tr>
<td>
<code>computeResources</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#resourcerequirements-v1-core">
Kubernetes core/v1.ResourceRequirements
</a>
</em>
</td>
<td>
<p>The resource requirements to apply to the Step.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.TaskSpec">TaskSpec
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.Task">Task</a>, <a href="#tekton.dev/v1.EmbeddedTask">EmbeddedTask</a>, <a href="#tekton.dev/v1.TaskRunSpec">TaskRunSpec</a>, <a href="#tekton.dev/v1.TaskRunStatusFields">TaskRunStatusFields</a>)
</p>
<div>
<p>TaskSpec defines the desired state of Task.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>params</code><br/>
<em>
<a href="#tekton.dev/v1.ParamSpecs">
ParamSpecs
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Params is a list of input parameters required to run the task. Params
must be supplied as inputs in TaskRuns unless they declare a default
value.</p>
</td>
</tr>
<tr>
<td>
<code>displayName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>DisplayName is a user-facing name of the task that may be
used to populate a UI.</p>
</td>
</tr>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Description is a user-facing description of the task that may be
used to populate a UI.</p>
</td>
</tr>
<tr>
<td>
<code>steps</code><br/>
<em>
<a href="#tekton.dev/v1.Step">
[]Step
</a>
</em>
</td>
<td>
<p>Steps are the steps of the build; each step is run sequentially with the
source mounted into /workspace.</p>
</td>
</tr>
<tr>
<td>
<code>volumes</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#volume-v1-core">
[]Kubernetes core/v1.Volume
</a>
</em>
</td>
<td>
<p>Volumes is a collection of volumes that are available to mount into the
steps of the build.</p>
</td>
</tr>
<tr>
<td>
<code>stepTemplate</code><br/>
<em>
<a href="#tekton.dev/v1.StepTemplate">
StepTemplate
</a>
</em>
</td>
<td>
<p>StepTemplate can be used as the basis for all step containers within the
Task, so that the steps inherit settings on the base container.</p>
</td>
</tr>
<tr>
<td>
<code>sidecars</code><br/>
<em>
<a href="#tekton.dev/v1.Sidecar">
[]Sidecar
</a>
</em>
</td>
<td>
<p>Sidecars are run alongside the Task&rsquo;s step containers. They begin before
the steps start and end after the steps complete.</p>
</td>
</tr>
<tr>
<td>
<code>workspaces</code><br/>
<em>
<a href="#tekton.dev/v1.WorkspaceDeclaration">
[]WorkspaceDeclaration
</a>
</em>
</td>
<td>
<p>Workspaces are the volumes that this Task requires.</p>
</td>
</tr>
<tr>
<td>
<code>results</code><br/>
<em>
<a href="#tekton.dev/v1.TaskResult">
[]TaskResult
</a>
</em>
</td>
<td>
<p>Results are values that this Task can output</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.TimeoutFields">TimeoutFields
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.PipelineRunSpec">PipelineRunSpec</a>)
</p>
<div>
<p>TimeoutFields allows granular specification of pipeline, task, and finally timeouts</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>pipeline</code><br/>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<p>Pipeline sets the maximum allowed duration for execution of the entire pipeline. The sum of individual timeouts for tasks and finally must not exceed this value.</p>
</td>
</tr>
<tr>
<td>
<code>tasks</code><br/>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<p>Tasks sets the maximum allowed duration of this pipeline&rsquo;s tasks</p>
</td>
</tr>
<tr>
<td>
<code>finally</code><br/>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<p>Finally sets the maximum allowed duration of this pipeline&rsquo;s finally</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.WhenExpression">WhenExpression
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.ChildStatusReference">ChildStatusReference</a>, <a href="#tekton.dev/v1.PipelineRunRunStatus">PipelineRunRunStatus</a>, <a href="#tekton.dev/v1.PipelineRunTaskRunStatus">PipelineRunTaskRunStatus</a>, <a href="#tekton.dev/v1.SkippedTask">SkippedTask</a>)
</p>
<div>
<p>WhenExpression allows a PipelineTask to declare expressions to be evaluated before the Task is run
to determine whether the Task should be executed or skipped</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>input</code><br/>
<em>
string
</em>
</td>
<td>
<p>Input is the string for guard checking which can be a static input or an output from a parent Task</p>
</td>
</tr>
<tr>
<td>
<code>operator</code><br/>
<em>
k8s.io/apimachinery/pkg/selection.Operator
</em>
</td>
<td>
<p>Operator that represents an Input&rsquo;s relationship to the values</p>
</td>
</tr>
<tr>
<td>
<code>values</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Values is an array of strings, which is compared against the input, for guard checking
It must be non-empty</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.WhenExpressions">WhenExpressions
(<code>[]github.com/tektoncd/pipeline/pkg/apis/pipeline/v1.WhenExpression</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.PipelineTask">PipelineTask</a>)
</p>
<div>
<p>WhenExpressions are used to specify whether a Task should be executed or skipped
All of them need to evaluate to True for a guarded Task to be executed.</p>
</div>
<h3 id="tekton.dev/v1.WorkspaceBinding">WorkspaceBinding
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.PipelineRunSpec">PipelineRunSpec</a>, <a href="#tekton.dev/v1.TaskRunSpec">TaskRunSpec</a>)
</p>
<div>
<p>WorkspaceBinding maps a Task&rsquo;s declared workspace to a Volume.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the workspace populated by the volume.</p>
</td>
</tr>
<tr>
<td>
<code>subPath</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>SubPath is optionally a directory on the volume which should be used
for this binding (i.e. the volume will be mounted at this sub directory).</p>
</td>
</tr>
<tr>
<td>
<code>volumeClaimTemplate</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#persistentvolumeclaim-v1-core">
Kubernetes core/v1.PersistentVolumeClaim
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>VolumeClaimTemplate is a template for a claim that will be created in the same namespace.
The PipelineRun controller is responsible for creating a unique claim for each instance of PipelineRun.</p>
</td>
</tr>
<tr>
<td>
<code>persistentVolumeClaim</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#persistentvolumeclaimvolumesource-v1-core">
Kubernetes core/v1.PersistentVolumeClaimVolumeSource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>PersistentVolumeClaimVolumeSource represents a reference to a
PersistentVolumeClaim in the same namespace. Either this OR EmptyDir can be used.</p>
</td>
</tr>
<tr>
<td>
<code>emptyDir</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#emptydirvolumesource-v1-core">
Kubernetes core/v1.EmptyDirVolumeSource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>EmptyDir represents a temporary directory that shares a Task&rsquo;s lifetime.
More info: <a href="https://kubernetes.io/docs/concepts/storage/volumes#emptydir">https://kubernetes.io/docs/concepts/storage/volumes#emptydir</a>
Either this OR PersistentVolumeClaim can be used.</p>
</td>
</tr>
<tr>
<td>
<code>configMap</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#configmapvolumesource-v1-core">
Kubernetes core/v1.ConfigMapVolumeSource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ConfigMap represents a configMap that should populate this workspace.</p>
</td>
</tr>
<tr>
<td>
<code>secret</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#secretvolumesource-v1-core">
Kubernetes core/v1.SecretVolumeSource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Secret represents a secret that should populate this workspace.</p>
</td>
</tr>
<tr>
<td>
<code>projected</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#projectedvolumesource-v1-core">
Kubernetes core/v1.ProjectedVolumeSource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Projected represents a projected volume that should populate this workspace.</p>
</td>
</tr>
<tr>
<td>
<code>csi</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#csivolumesource-v1-core">
Kubernetes core/v1.CSIVolumeSource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CSI (Container Storage Interface) represents ephemeral storage that is handled by certain external CSI drivers.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.WorkspaceDeclaration">WorkspaceDeclaration
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.TaskSpec">TaskSpec</a>)
</p>
<div>
<p>WorkspaceDeclaration is a declaration of a volume that a Task requires.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is the name by which you can bind the volume at runtime.</p>
</td>
</tr>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Description is an optional human readable description of this volume.</p>
</td>
</tr>
<tr>
<td>
<code>mountPath</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>MountPath overrides the directory that the volume will be made available at.</p>
</td>
</tr>
<tr>
<td>
<code>readOnly</code><br/>
<em>
bool
</em>
</td>
<td>
<p>ReadOnly dictates whether a mounted volume is writable. By default this
field is false and so mounted volumes are writable.</p>
</td>
</tr>
<tr>
<td>
<code>optional</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Optional marks a Workspace as not being required in TaskRuns. By default
this field is false and so declared workspaces are required.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.WorkspacePipelineTaskBinding">WorkspacePipelineTaskBinding
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.PipelineTask">PipelineTask</a>)
</p>
<div>
<p>WorkspacePipelineTaskBinding describes how a workspace passed into the pipeline should be
mapped to a task&rsquo;s declared workspace.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the workspace as declared by the task</p>
</td>
</tr>
<tr>
<td>
<code>workspace</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Workspace is the name of the workspace declared by the pipeline</p>
</td>
</tr>
<tr>
<td>
<code>subPath</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>SubPath is optionally a directory on the volume which should be used
for this binding (i.e. the volume will be mounted at this sub directory).</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1.WorkspaceUsage">WorkspaceUsage
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1.Sidecar">Sidecar</a>, <a href="#tekton.dev/v1.Step">Step</a>)
</p>
<div>
<p>WorkspaceUsage is used by a Step or Sidecar to declare that it wants isolated access
to a Workspace defined in a Task.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the workspace this Step or Sidecar wants access to.</p>
</td>
</tr>
<tr>
<td>
<code>mountPath</code><br/>
<em>
string
</em>
</td>
<td>
<p>MountPath is the path that the workspace should be mounted to inside the Step or Sidecar,
overriding any MountPath specified in the Task&rsquo;s WorkspaceDeclaration.</p>
</td>
</tr>
</tbody>
</table>
<hr/>
<h2 id="tekton.dev/v1alpha1">tekton.dev/v1alpha1</h2>
<div>
<p>Package v1alpha1 contains API Schema definitions for the pipeline v1alpha1 API group</p>
</div>
Resource Types:
<ul><li>
<a href="#tekton.dev/v1alpha1.Run">Run</a>
</li><li>
<a href="#tekton.dev/v1alpha1.VerificationPolicy">VerificationPolicy</a>
</li><li>
<a href="#tekton.dev/v1alpha1.PipelineResource">PipelineResource</a>
</li></ul>
<h3 id="tekton.dev/v1alpha1.Run">Run
</h3>
<div>
<p>Run represents a single execution of a Custom Task.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code><br/>
string</td>
<td>
<code>
tekton.dev/v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>Run</code></td>
</tr>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
<em>(Optional)</em>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#tekton.dev/v1alpha1.RunSpec">
RunSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<br/>
<br/>
<table>
<tr>
<td>
<code>ref</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskRef">
TaskRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#tekton.dev/v1alpha1.EmbeddedRunSpec">
EmbeddedRunSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Spec is a specification of a custom task</p>
<br/>
<br/>
<table>
</table>
</td>
</tr>
<tr>
<td>
<code>params</code><br/>
<em>
<a href="#tekton.dev/v1beta1.Params">
Params
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#tekton.dev/v1alpha1.RunSpecStatus">
RunSpecStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Used for cancelling a run (and maybe more later on)</p>
</td>
</tr>
<tr>
<td>
<code>statusMessage</code><br/>
<em>
<a href="#tekton.dev/v1alpha1.RunSpecStatusMessage">
RunSpecStatusMessage
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Status message for cancellation.</p>
</td>
</tr>
<tr>
<td>
<code>retries</code><br/>
<em>
int
</em>
</td>
<td>
<em>(Optional)</em>
<p>Used for propagating retries count to custom tasks</p>
</td>
</tr>
<tr>
<td>
<code>serviceAccountName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>podTemplate</code><br/>
<em>
<a href="#tekton.dev/unversioned.Template">
Template
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>PodTemplate holds pod specific configuration</p>
</td>
</tr>
<tr>
<td>
<code>timeout</code><br/>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Time after which the custom-task times out.
Refer Go&rsquo;s ParseDuration documentation for expected format: <a href="https://golang.org/pkg/time/#ParseDuration">https://golang.org/pkg/time/#ParseDuration</a></p>
</td>
</tr>
<tr>
<td>
<code>workspaces</code><br/>
<em>
<a href="#tekton.dev/v1beta1.WorkspaceBinding">
[]WorkspaceBinding
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Workspaces is a list of WorkspaceBindings from volumes to workspaces.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#tekton.dev/v1alpha1.RunStatus">
RunStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1alpha1.VerificationPolicy">VerificationPolicy
</h3>
<div>
<p>VerificationPolicy defines the rules to verify Tekton resources.
VerificationPolicy can config the mapping from resources to a list of public
keys, so when verifying the resources we can use the corresponding public keys.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code><br/>
string</td>
<td>
<code>
tekton.dev/v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>VerificationPolicy</code></td>
</tr>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
<em>(Optional)</em>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#tekton.dev/v1alpha1.VerificationPolicySpec">
VerificationPolicySpec
</a>
</em>
</td>
<td>
<p>Spec holds the desired state of the VerificationPolicy.</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>resources</code><br/>
<em>
<a href="#tekton.dev/v1alpha1.ResourcePattern">
[]ResourcePattern
</a>
</em>
</td>
<td>
<p>Resources defines the patterns of resources sources that should be subject to this policy.
For example, we may want to apply this Policy from a certain GitHub repo.
Then the ResourcesPattern should be valid regex. E.g. If using gitresolver, and we want to config keys from a certain git repo.
<code>ResourcesPattern</code> can be <code>https://github.com/tektoncd/catalog.git</code>, we will use regex to filter out those resources.</p>
</td>
</tr>
<tr>
<td>
<code>authorities</code><br/>
<em>
<a href="#tekton.dev/v1alpha1.Authority">
[]Authority
</a>
</em>
</td>
<td>
<p>Authorities defines the rules for validating signatures.</p>
</td>
</tr>
<tr>
<td>
<code>mode</code><br/>
<em>
<a href="#tekton.dev/v1alpha1.ModeType">
ModeType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Mode controls whether a failing policy will fail the taskrun/pipelinerun, or only log the warnings
enforce - fail the taskrun/pipelinerun if verification fails (default)
warn - don&rsquo;t fail the taskrun/pipelinerun if verification fails but log warnings</p>
</td>
</tr>
</table>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1alpha1.PipelineResource">PipelineResource
</h3>
<div>
<p>PipelineResource describes a resource that is an input to or output from a
Task.</p>
<p>Deprecated: Unused, preserved only for backwards compatibility</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code><br/>
string</td>
<td>
<code>
tekton.dev/v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>PipelineResource</code></td>
</tr>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
<em>(Optional)</em>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#tekton.dev/v1alpha1.PipelineResourceSpec">
PipelineResourceSpec
</a>
</em>
</td>
<td>
<p>Spec holds the desired state of the PipelineResource from the client</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Description is a user-facing description of the resource that may be
used to populate a UI.</p>
</td>
</tr>
<tr>
<td>
<code>type</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>params</code><br/>
<em>
<a href="#tekton.dev/v1alpha1.ResourceParam">
[]ResourceParam
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>secrets</code><br/>
<em>
<a href="#tekton.dev/v1alpha1.SecretParam">
[]SecretParam
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Secrets to fetch to populate some of resource fields</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#tekton.dev/v1alpha1.PipelineResourceStatus">
PipelineResourceStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Status is used to communicate the observed state of the PipelineResource from
the controller, but was unused as there is no controller for PipelineResource.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1alpha1.Authority">Authority
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1alpha1.VerificationPolicySpec">VerificationPolicySpec</a>)
</p>
<div>
<p>The Authority block defines the keys for validating signatures.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is the name for this authority.</p>
</td>
</tr>
<tr>
<td>
<code>key</code><br/>
<em>
<a href="#tekton.dev/v1alpha1.KeyRef">
KeyRef
</a>
</em>
</td>
<td>
<p>Key contains the public key to validate the resource.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1alpha1.EmbeddedRunSpec">EmbeddedRunSpec
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1alpha1.RunSpec">RunSpec</a>)
</p>
<div>
<p>EmbeddedRunSpec allows custom task definitions to be embedded</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="#tekton.dev/v1beta1.PipelineTaskMetadata">
PipelineTaskMetadata
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
k8s.io/apimachinery/pkg/runtime.RawExtension
</em>
</td>
<td>
<em>(Optional)</em>
<p>Spec is a specification of a custom task</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>-</code><br/>
<em>
[]byte
</em>
</td>
<td>
<p>Raw is the underlying serialization of this object.</p>
<p>TODO: Determine how to detect ContentType and ContentEncoding of &lsquo;Raw&rsquo; data.</p>
</td>
</tr>
<tr>
<td>
<code>-</code><br/>
<em>
k8s.io/apimachinery/pkg/runtime.Object
</em>
</td>
<td>
<p>Object can hold a representation of this extension - useful for working with versioned
structs.</p>
</td>
</tr>
</table>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1alpha1.HashAlgorithm">HashAlgorithm
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1alpha1.KeyRef">KeyRef</a>)
</p>
<div>
<p>HashAlgorithm defines the hash algorithm used for the public key</p>
</div>
<h3 id="tekton.dev/v1alpha1.KeyRef">KeyRef
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1alpha1.Authority">Authority</a>)
</p>
<div>
<p>KeyRef defines the reference to a public key</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>secretRef</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SecretRef sets a reference to a secret with the key.</p>
</td>
</tr>
<tr>
<td>
<code>data</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Data contains the inline public key.</p>
</td>
</tr>
<tr>
<td>
<code>kms</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>KMS contains the KMS url of the public key
Supported formats differ based on the KMS system used.
One example of a KMS url could be:
gcpkms://projects/[PROJECT]/locations/[LOCATION]&gt;/keyRings/[KEYRING]/cryptoKeys/[KEY]/cryptoKeyVersions/[KEY_VERSION]
For more examples please refer <a href="https://docs.sigstore.dev/cosign/kms_support">https://docs.sigstore.dev/cosign/kms_support</a>.
Note that the KMS is not supported yet.</p>
</td>
</tr>
<tr>
<td>
<code>hashAlgorithm</code><br/>
<em>
<a href="#tekton.dev/v1alpha1.HashAlgorithm">
HashAlgorithm
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>HashAlgorithm always defaults to sha256 if the algorithm hasn&rsquo;t been explicitly set</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1alpha1.ModeType">ModeType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1alpha1.VerificationPolicySpec">VerificationPolicySpec</a>)
</p>
<div>
<p>ModeType indicates the type of a mode for VerificationPolicy</p>
</div>
<h3 id="tekton.dev/v1alpha1.ResourcePattern">ResourcePattern
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1alpha1.VerificationPolicySpec">VerificationPolicySpec</a>)
</p>
<div>
<p>ResourcePattern defines the pattern of the resource source</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>pattern</code><br/>
<em>
string
</em>
</td>
<td>
<p>Pattern defines a resource pattern. Regex is created to filter resources based on <code>Pattern</code>
Example patterns:
GitHub resource: <a href="https://github.com/tektoncd/catalog.git">https://github.com/tektoncd/catalog.git</a>, <a href="https://github.com/tektoncd/*">https://github.com/tektoncd/*</a>
Bundle resource: gcr.io/tekton-releases/catalog/upstream/git-clone, gcr.io/tekton-releases/catalog/upstream/*
Hub resource: <a href="https://artifacthub.io/*">https://artifacthub.io/*</a>,</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1alpha1.RunReason">RunReason
(<code>string</code> alias)</h3>
<div>
<p>RunReason is an enum used to store all Run reason for the Succeeded condition that are controlled by the Run itself.</p>
</div>
<h3 id="tekton.dev/v1alpha1.RunSpec">RunSpec
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1alpha1.Run">Run</a>)
</p>
<div>
<p>RunSpec defines the desired state of Run</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>ref</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskRef">
TaskRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#tekton.dev/v1alpha1.EmbeddedRunSpec">
EmbeddedRunSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Spec is a specification of a custom task</p>
<br/>
<br/>
<table>
</table>
</td>
</tr>
<tr>
<td>
<code>params</code><br/>
<em>
<a href="#tekton.dev/v1beta1.Params">
Params
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#tekton.dev/v1alpha1.RunSpecStatus">
RunSpecStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Used for cancelling a run (and maybe more later on)</p>
</td>
</tr>
<tr>
<td>
<code>statusMessage</code><br/>
<em>
<a href="#tekton.dev/v1alpha1.RunSpecStatusMessage">
RunSpecStatusMessage
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Status message for cancellation.</p>
</td>
</tr>
<tr>
<td>
<code>retries</code><br/>
<em>
int
</em>
</td>
<td>
<em>(Optional)</em>
<p>Used for propagating retries count to custom tasks</p>
</td>
</tr>
<tr>
<td>
<code>serviceAccountName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>podTemplate</code><br/>
<em>
<a href="#tekton.dev/unversioned.Template">
Template
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>PodTemplate holds pod specific configuration</p>
</td>
</tr>
<tr>
<td>
<code>timeout</code><br/>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Time after which the custom-task times out.
Refer Go&rsquo;s ParseDuration documentation for expected format: <a href="https://golang.org/pkg/time/#ParseDuration">https://golang.org/pkg/time/#ParseDuration</a></p>
</td>
</tr>
<tr>
<td>
<code>workspaces</code><br/>
<em>
<a href="#tekton.dev/v1beta1.WorkspaceBinding">
[]WorkspaceBinding
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Workspaces is a list of WorkspaceBindings from volumes to workspaces.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1alpha1.RunSpecStatus">RunSpecStatus
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1alpha1.RunSpec">RunSpec</a>)
</p>
<div>
<p>RunSpecStatus defines the taskrun spec status the user can provide</p>
</div>
<h3 id="tekton.dev/v1alpha1.RunSpecStatusMessage">RunSpecStatusMessage
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1alpha1.RunSpec">RunSpec</a>)
</p>
<div>
<p>RunSpecStatusMessage defines human readable status messages for the TaskRun.</p>
</div>
<h3 id="tekton.dev/v1alpha1.VerificationPolicySpec">VerificationPolicySpec
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1alpha1.VerificationPolicy">VerificationPolicy</a>)
</p>
<div>
<p>VerificationPolicySpec defines the patterns and authorities.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>resources</code><br/>
<em>
<a href="#tekton.dev/v1alpha1.ResourcePattern">
[]ResourcePattern
</a>
</em>
</td>
<td>
<p>Resources defines the patterns of resources sources that should be subject to this policy.
For example, we may want to apply this Policy from a certain GitHub repo.
Then the ResourcesPattern should be valid regex. E.g. If using gitresolver, and we want to config keys from a certain git repo.
<code>ResourcesPattern</code> can be <code>https://github.com/tektoncd/catalog.git</code>, we will use regex to filter out those resources.</p>
</td>
</tr>
<tr>
<td>
<code>authorities</code><br/>
<em>
<a href="#tekton.dev/v1alpha1.Authority">
[]Authority
</a>
</em>
</td>
<td>
<p>Authorities defines the rules for validating signatures.</p>
</td>
</tr>
<tr>
<td>
<code>mode</code><br/>
<em>
<a href="#tekton.dev/v1alpha1.ModeType">
ModeType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Mode controls whether a failing policy will fail the taskrun/pipelinerun, or only log the warnings
enforce - fail the taskrun/pipelinerun if verification fails (default)
warn - don&rsquo;t fail the taskrun/pipelinerun if verification fails but log warnings</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1alpha1.PipelineResourceSpec">PipelineResourceSpec
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1alpha1.PipelineResource">PipelineResource</a>, <a href="#tekton.dev/v1beta1.PipelineResourceBinding">PipelineResourceBinding</a>)
</p>
<div>
<p>PipelineResourceSpec defines an individual resources used in the pipeline.</p>
<p>Deprecated: Unused, preserved only for backwards compatibility</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Description is a user-facing description of the resource that may be
used to populate a UI.</p>
</td>
</tr>
<tr>
<td>
<code>type</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>params</code><br/>
<em>
<a href="#tekton.dev/v1alpha1.ResourceParam">
[]ResourceParam
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>secrets</code><br/>
<em>
<a href="#tekton.dev/v1alpha1.SecretParam">
[]SecretParam
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Secrets to fetch to populate some of resource fields</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1alpha1.PipelineResourceStatus">PipelineResourceStatus
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1alpha1.PipelineResource">PipelineResource</a>)
</p>
<div>
<p>PipelineResourceStatus does not contain anything because PipelineResources on their own
do not have a status</p>
<p>Deprecated: Unused, preserved only for backwards compatibility</p>
</div>
<h3 id="tekton.dev/v1alpha1.ResourceDeclaration">ResourceDeclaration
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.TaskResource">TaskResource</a>)
</p>
<div>
<p>ResourceDeclaration defines an input or output PipelineResource declared as a requirement
by another type such as a Task or Condition. The Name field will be used to refer to these
PipelineResources within the type&rsquo;s definition, and when provided as an Input, the Name will be the
path to the volume mounted containing this PipelineResource as an input (e.g.
an input Resource named <code>workspace</code> will be mounted at <code>/workspace</code>).</p>
<p>Deprecated: Unused, preserved only for backwards compatibility</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name declares the name by which a resource is referenced in the
definition. Resources may be referenced by name in the definition of a
Task&rsquo;s steps.</p>
</td>
</tr>
<tr>
<td>
<code>type</code><br/>
<em>
string
</em>
</td>
<td>
<p>Type is the type of this resource;</p>
</td>
</tr>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Description is a user-facing description of the declared resource that may be
used to populate a UI.</p>
</td>
</tr>
<tr>
<td>
<code>targetPath</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>TargetPath is the path in workspace directory where the resource
will be copied.</p>
</td>
</tr>
<tr>
<td>
<code>optional</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Optional declares the resource as optional.
By default optional is set to false which makes a resource required.
optional: true - the resource is considered optional
optional: false - the resource is considered required (equivalent of not specifying it)</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1alpha1.ResourceParam">ResourceParam
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1alpha1.PipelineResourceSpec">PipelineResourceSpec</a>)
</p>
<div>
<p>ResourceParam declares a string value to use for the parameter called Name, and is used in
the specific context of PipelineResources.</p>
<p>Deprecated: Unused, preserved only for backwards compatibility</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>value</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1alpha1.SecretParam">SecretParam
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1alpha1.PipelineResourceSpec">PipelineResourceSpec</a>)
</p>
<div>
<p>SecretParam indicates which secret can be used to populate a field of the resource</p>
<p>Deprecated: Unused, preserved only for backwards compatibility</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>fieldName</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>secretKey</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>secretName</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1alpha1.RunResult">RunResult
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1alpha1.RunStatusFields">RunStatusFields</a>)
</p>
<div>
<p>RunResult used to describe the results of a task</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name the given name</p>
</td>
</tr>
<tr>
<td>
<code>value</code><br/>
<em>
string
</em>
</td>
<td>
<p>Value the given value of the result</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1alpha1.RunStatus">RunStatus
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1alpha1.Run">Run</a>, <a href="#tekton.dev/v1alpha1.RunStatusFields">RunStatusFields</a>)
</p>
<div>
<p>RunStatus defines the observed state of Run</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>Status</code><br/>
<em>
<a href="https://pkg.go.dev/knative.dev/pkg/apis/duck/v1#Status">
knative.dev/pkg/apis/duck/v1.Status
</a>
</em>
</td>
<td>
<p>
(Members of <code>Status</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>RunStatusFields</code><br/>
<em>
<a href="#tekton.dev/v1alpha1.RunStatusFields">
RunStatusFields
</a>
</em>
</td>
<td>
<p>
(Members of <code>RunStatusFields</code> are embedded into this type.)
</p>
<p>RunStatusFields inlines the status fields.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1alpha1.RunStatusFields">RunStatusFields
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1alpha1.RunStatus">RunStatus</a>)
</p>
<div>
<p>RunStatusFields holds the fields of Run&rsquo;s status.  This is defined
separately and inlined so that other types can readily consume these fields
via duck typing.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>startTime</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>StartTime is the time the build is actually started.</p>
</td>
</tr>
<tr>
<td>
<code>completionTime</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CompletionTime is the time the build completed.</p>
</td>
</tr>
<tr>
<td>
<code>results</code><br/>
<em>
<a href="#tekton.dev/v1alpha1.RunResult">
[]RunResult
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Results reports any output result values to be consumed by later
tasks in a pipeline.</p>
</td>
</tr>
<tr>
<td>
<code>retriesStatus</code><br/>
<em>
<a href="#tekton.dev/v1alpha1.RunStatus">
[]RunStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>RetriesStatus contains the history of RunStatus, in case of a retry.</p>
</td>
</tr>
<tr>
<td>
<code>extraFields</code><br/>
<em>
k8s.io/apimachinery/pkg/runtime.RawExtension
</em>
</td>
<td>
<p>ExtraFields holds arbitrary fields provided by the custom task
controller.</p>
</td>
</tr>
</tbody>
</table>
<hr/>
<h2 id="tekton.dev/v1beta1">tekton.dev/v1beta1</h2>
<div>
<p>Package v1beta1 contains API Schema definitions for the pipeline v1beta1 API group</p>
</div>
Resource Types:
<ul><li>
<a href="#tekton.dev/v1beta1.ClusterTask">ClusterTask</a>
</li><li>
<a href="#tekton.dev/v1beta1.CustomRun">CustomRun</a>
</li><li>
<a href="#tekton.dev/v1beta1.Pipeline">Pipeline</a>
</li><li>
<a href="#tekton.dev/v1beta1.PipelineRun">PipelineRun</a>
</li><li>
<a href="#tekton.dev/v1beta1.Task">Task</a>
</li><li>
<a href="#tekton.dev/v1beta1.TaskRun">TaskRun</a>
</li></ul>
<h3 id="tekton.dev/v1beta1.ClusterTask">ClusterTask
</h3>
<div>
<p>ClusterTask is a Task with a cluster scope. ClusterTasks are used to
represent Tasks that should be publicly addressable from any namespace in the
cluster.</p>
<p>Deprecated: Please use the cluster resolver instead.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code><br/>
string</td>
<td>
<code>
tekton.dev/v1beta1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>ClusterTask</code></td>
</tr>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
<em>(Optional)</em>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskSpec">
TaskSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Spec holds the desired state of the Task from the client</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>resources</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskResources">
TaskResources
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resources is a list input and output resource to run the task
Resources are represented in TaskRuns as bindings to instances of
PipelineResources.</p>
<p>Deprecated: Unused, preserved only for backwards compatibility</p>
</td>
</tr>
<tr>
<td>
<code>params</code><br/>
<em>
<a href="#tekton.dev/v1beta1.ParamSpecs">
ParamSpecs
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Params is a list of input parameters required to run the task. Params
must be supplied as inputs in TaskRuns unless they declare a default
value.</p>
</td>
</tr>
<tr>
<td>
<code>displayName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>DisplayName is a user-facing name of the task that may be
used to populate a UI.</p>
</td>
</tr>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Description is a user-facing description of the task that may be
used to populate a UI.</p>
</td>
</tr>
<tr>
<td>
<code>steps</code><br/>
<em>
<a href="#tekton.dev/v1beta1.Step">
[]Step
</a>
</em>
</td>
<td>
<p>Steps are the steps of the build; each step is run sequentially with the
source mounted into /workspace.</p>
</td>
</tr>
<tr>
<td>
<code>volumes</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#volume-v1-core">
[]Kubernetes core/v1.Volume
</a>
</em>
</td>
<td>
<p>Volumes is a collection of volumes that are available to mount into the
steps of the build.</p>
</td>
</tr>
<tr>
<td>
<code>stepTemplate</code><br/>
<em>
<a href="#tekton.dev/v1beta1.StepTemplate">
StepTemplate
</a>
</em>
</td>
<td>
<p>StepTemplate can be used as the basis for all step containers within the
Task, so that the steps inherit settings on the base container.</p>
</td>
</tr>
<tr>
<td>
<code>sidecars</code><br/>
<em>
<a href="#tekton.dev/v1beta1.Sidecar">
[]Sidecar
</a>
</em>
</td>
<td>
<p>Sidecars are run alongside the Task&rsquo;s step containers. They begin before
the steps start and end after the steps complete.</p>
</td>
</tr>
<tr>
<td>
<code>workspaces</code><br/>
<em>
<a href="#tekton.dev/v1beta1.WorkspaceDeclaration">
[]WorkspaceDeclaration
</a>
</em>
</td>
<td>
<p>Workspaces are the volumes that this Task requires.</p>
</td>
</tr>
<tr>
<td>
<code>results</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskResult">
[]TaskResult
</a>
</em>
</td>
<td>
<p>Results are values that this Task can output</p>
</td>
</tr>
</table>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.CustomRun">CustomRun
</h3>
<div>
<p>CustomRun represents a single execution of a Custom Task.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code><br/>
string</td>
<td>
<code>
tekton.dev/v1beta1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>CustomRun</code></td>
</tr>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
<em>(Optional)</em>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#tekton.dev/v1beta1.CustomRunSpec">
CustomRunSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<br/>
<br/>
<table>
<tr>
<td>
<code>customRef</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskRef">
TaskRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>customSpec</code><br/>
<em>
<a href="#tekton.dev/v1beta1.EmbeddedCustomRunSpec">
EmbeddedCustomRunSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Spec is a specification of a custom task</p>
</td>
</tr>
<tr>
<td>
<code>params</code><br/>
<em>
<a href="#tekton.dev/v1beta1.Params">
Params
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#tekton.dev/v1beta1.CustomRunSpecStatus">
CustomRunSpecStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Used for cancelling a customrun (and maybe more later on)</p>
</td>
</tr>
<tr>
<td>
<code>statusMessage</code><br/>
<em>
<a href="#tekton.dev/v1beta1.CustomRunSpecStatusMessage">
CustomRunSpecStatusMessage
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Status message for cancellation.</p>
</td>
</tr>
<tr>
<td>
<code>retries</code><br/>
<em>
int
</em>
</td>
<td>
<em>(Optional)</em>
<p>Used for propagating retries count to custom tasks</p>
</td>
</tr>
<tr>
<td>
<code>serviceAccountName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>timeout</code><br/>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Time after which the custom-task times out.
Refer Go&rsquo;s ParseDuration documentation for expected format: <a href="https://golang.org/pkg/time/#ParseDuration">https://golang.org/pkg/time/#ParseDuration</a></p>
</td>
</tr>
<tr>
<td>
<code>workspaces</code><br/>
<em>
<a href="#tekton.dev/v1beta1.WorkspaceBinding">
[]WorkspaceBinding
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Workspaces is a list of WorkspaceBindings from volumes to workspaces.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#tekton.dev/v1beta1.CustomRunStatus">
CustomRunStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.Pipeline">Pipeline
</h3>
<div>
<p>Pipeline describes a list of Tasks to execute. It expresses how outputs
of tasks feed into inputs of subsequent tasks.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code><br/>
string</td>
<td>
<code>
tekton.dev/v1beta1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>Pipeline</code></td>
</tr>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
<em>(Optional)</em>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#tekton.dev/v1beta1.PipelineSpec">
PipelineSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Spec holds the desired state of the Pipeline from the client</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>displayName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>DisplayName is a user-facing name of the pipeline that may be
used to populate a UI.</p>
</td>
</tr>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Description is a user-facing description of the pipeline that may be
used to populate a UI.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code><br/>
<em>
<a href="#tekton.dev/v1beta1.PipelineDeclaredResource">
[]PipelineDeclaredResource
</a>
</em>
</td>
<td>
<p>Deprecated: Unused, preserved only for backwards compatibility</p>
</td>
</tr>
<tr>
<td>
<code>tasks</code><br/>
<em>
<a href="#tekton.dev/v1beta1.PipelineTask">
[]PipelineTask
</a>
</em>
</td>
<td>
<p>Tasks declares the graph of Tasks that execute when this Pipeline is run.</p>
</td>
</tr>
<tr>
<td>
<code>params</code><br/>
<em>
<a href="#tekton.dev/v1beta1.ParamSpecs">
ParamSpecs
</a>
</em>
</td>
<td>
<p>Params declares a list of input parameters that must be supplied when
this Pipeline is run.</p>
</td>
</tr>
<tr>
<td>
<code>workspaces</code><br/>
<em>
<a href="#tekton.dev/v1beta1.PipelineWorkspaceDeclaration">
[]PipelineWorkspaceDeclaration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Workspaces declares a set of named workspaces that are expected to be
provided by a PipelineRun.</p>
</td>
</tr>
<tr>
<td>
<code>results</code><br/>
<em>
<a href="#tekton.dev/v1beta1.PipelineResult">
[]PipelineResult
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Results are values that this pipeline can output once run</p>
</td>
</tr>
<tr>
<td>
<code>finally</code><br/>
<em>
<a href="#tekton.dev/v1beta1.PipelineTask">
[]PipelineTask
</a>
</em>
</td>
<td>
<p>Finally declares the list of Tasks that execute just before leaving the Pipeline
i.e. either after all Tasks are finished executing successfully
or after a failure which would result in ending the Pipeline</p>
</td>
</tr>
</table>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.PipelineRun">PipelineRun
</h3>
<div>
<p>PipelineRun represents a single execution of a Pipeline. PipelineRuns are how
the graph of Tasks declared in a Pipeline are executed; they specify inputs
to Pipelines such as parameter values and capture operational aspects of the
Tasks execution such as service account and tolerations. Creating a
PipelineRun creates TaskRuns for Tasks in the referenced Pipeline.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code><br/>
string</td>
<td>
<code>
tekton.dev/v1beta1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>PipelineRun</code></td>
</tr>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
<em>(Optional)</em>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#tekton.dev/v1beta1.PipelineRunSpec">
PipelineRunSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<br/>
<br/>
<table>
<tr>
<td>
<code>pipelineRef</code><br/>
<em>
<a href="#tekton.dev/v1beta1.PipelineRef">
PipelineRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>pipelineSpec</code><br/>
<em>
<a href="#tekton.dev/v1beta1.PipelineSpec">
PipelineSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>resources</code><br/>
<em>
<a href="#tekton.dev/v1beta1.PipelineResourceBinding">
[]PipelineResourceBinding
</a>
</em>
</td>
<td>
<p>Resources is a list of bindings specifying which actual instances of
PipelineResources to use for the resources the Pipeline has declared
it needs.</p>
<p>Deprecated: Unused, preserved only for backwards compatibility</p>
</td>
</tr>
<tr>
<td>
<code>params</code><br/>
<em>
<a href="#tekton.dev/v1beta1.Params">
Params
</a>
</em>
</td>
<td>
<p>Params is a list of parameter names and values.</p>
</td>
</tr>
<tr>
<td>
<code>serviceAccountName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#tekton.dev/v1beta1.PipelineRunSpecStatus">
PipelineRunSpecStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Used for cancelling a pipelinerun (and maybe more later on)</p>
</td>
</tr>
<tr>
<td>
<code>timeouts</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TimeoutFields">
TimeoutFields
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Time after which the Pipeline times out.
Currently three keys are accepted in the map
pipeline, tasks and finally
with Timeouts.pipeline &gt;= Timeouts.tasks + Timeouts.finally</p>
</td>
</tr>
<tr>
<td>
<code>timeout</code><br/>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Timeout is the Time after which the Pipeline times out.
Defaults to never.
Refer to Go&rsquo;s ParseDuration documentation for expected format: <a href="https://golang.org/pkg/time/#ParseDuration">https://golang.org/pkg/time/#ParseDuration</a></p>
<p>Deprecated: use pipelineRunSpec.Timeouts.Pipeline instead</p>
</td>
</tr>
<tr>
<td>
<code>podTemplate</code><br/>
<em>
<a href="#tekton.dev/unversioned.Template">
Template
</a>
</em>
</td>
<td>
<p>PodTemplate holds pod specific configuration</p>
</td>
</tr>
<tr>
<td>
<code>workspaces</code><br/>
<em>
<a href="#tekton.dev/v1beta1.WorkspaceBinding">
[]WorkspaceBinding
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Workspaces holds a set of workspace bindings that must match names
with those declared in the pipeline.</p>
</td>
</tr>
<tr>
<td>
<code>taskRunSpecs</code><br/>
<em>
<a href="#tekton.dev/v1beta1.PipelineTaskRunSpec">
[]PipelineTaskRunSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>TaskRunSpecs holds a set of runtime specs</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#tekton.dev/v1beta1.PipelineRunStatus">
PipelineRunStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.Task">Task
</h3>
<div>
<p>Task represents a collection of sequential steps that are run as part of a
Pipeline using a set of inputs and producing a set of outputs. Tasks execute
when TaskRuns are created that provide the input parameters and resources and
output resources the Task requires.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code><br/>
string</td>
<td>
<code>
tekton.dev/v1beta1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>Task</code></td>
</tr>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
<em>(Optional)</em>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskSpec">
TaskSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Spec holds the desired state of the Task from the client</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>resources</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskResources">
TaskResources
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resources is a list input and output resource to run the task
Resources are represented in TaskRuns as bindings to instances of
PipelineResources.</p>
<p>Deprecated: Unused, preserved only for backwards compatibility</p>
</td>
</tr>
<tr>
<td>
<code>params</code><br/>
<em>
<a href="#tekton.dev/v1beta1.ParamSpecs">
ParamSpecs
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Params is a list of input parameters required to run the task. Params
must be supplied as inputs in TaskRuns unless they declare a default
value.</p>
</td>
</tr>
<tr>
<td>
<code>displayName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>DisplayName is a user-facing name of the task that may be
used to populate a UI.</p>
</td>
</tr>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Description is a user-facing description of the task that may be
used to populate a UI.</p>
</td>
</tr>
<tr>
<td>
<code>steps</code><br/>
<em>
<a href="#tekton.dev/v1beta1.Step">
[]Step
</a>
</em>
</td>
<td>
<p>Steps are the steps of the build; each step is run sequentially with the
source mounted into /workspace.</p>
</td>
</tr>
<tr>
<td>
<code>volumes</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#volume-v1-core">
[]Kubernetes core/v1.Volume
</a>
</em>
</td>
<td>
<p>Volumes is a collection of volumes that are available to mount into the
steps of the build.</p>
</td>
</tr>
<tr>
<td>
<code>stepTemplate</code><br/>
<em>
<a href="#tekton.dev/v1beta1.StepTemplate">
StepTemplate
</a>
</em>
</td>
<td>
<p>StepTemplate can be used as the basis for all step containers within the
Task, so that the steps inherit settings on the base container.</p>
</td>
</tr>
<tr>
<td>
<code>sidecars</code><br/>
<em>
<a href="#tekton.dev/v1beta1.Sidecar">
[]Sidecar
</a>
</em>
</td>
<td>
<p>Sidecars are run alongside the Task&rsquo;s step containers. They begin before
the steps start and end after the steps complete.</p>
</td>
</tr>
<tr>
<td>
<code>workspaces</code><br/>
<em>
<a href="#tekton.dev/v1beta1.WorkspaceDeclaration">
[]WorkspaceDeclaration
</a>
</em>
</td>
<td>
<p>Workspaces are the volumes that this Task requires.</p>
</td>
</tr>
<tr>
<td>
<code>results</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskResult">
[]TaskResult
</a>
</em>
</td>
<td>
<p>Results are values that this Task can output</p>
</td>
</tr>
</table>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.TaskRun">TaskRun
</h3>
<div>
<p>TaskRun represents a single execution of a Task. TaskRuns are how the steps
specified in a Task are executed; they specify the parameters and resources
used to run the steps in a Task.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code><br/>
string</td>
<td>
<code>
tekton.dev/v1beta1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>TaskRun</code></td>
</tr>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
<em>(Optional)</em>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskRunSpec">
TaskRunSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<br/>
<br/>
<table>
<tr>
<td>
<code>debug</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskRunDebug">
TaskRunDebug
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>params</code><br/>
<em>
<a href="#tekton.dev/v1beta1.Params">
Params
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>resources</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskRunResources">
TaskRunResources
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Deprecated: Unused, preserved only for backwards compatibility</p>
</td>
</tr>
<tr>
<td>
<code>serviceAccountName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>taskRef</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskRef">
TaskRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>no more than one of the TaskRef and TaskSpec may be specified.</p>
</td>
</tr>
<tr>
<td>
<code>taskSpec</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskSpec">
TaskSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskRunSpecStatus">
TaskRunSpecStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Used for cancelling a TaskRun (and maybe more later on)</p>
</td>
</tr>
<tr>
<td>
<code>statusMessage</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskRunSpecStatusMessage">
TaskRunSpecStatusMessage
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Status message for cancellation.</p>
</td>
</tr>
<tr>
<td>
<code>retries</code><br/>
<em>
int
</em>
</td>
<td>
<em>(Optional)</em>
<p>Retries represents how many times this TaskRun should be retried in the event of Task failure.</p>
</td>
</tr>
<tr>
<td>
<code>timeout</code><br/>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Time after which one retry attempt times out. Defaults to 1 hour.
Refer Go&rsquo;s ParseDuration documentation for expected format: <a href="https://golang.org/pkg/time/#ParseDuration">https://golang.org/pkg/time/#ParseDuration</a></p>
</td>
</tr>
<tr>
<td>
<code>podTemplate</code><br/>
<em>
<a href="#tekton.dev/unversioned.Template">
Template
</a>
</em>
</td>
<td>
<p>PodTemplate holds pod specific configuration</p>
</td>
</tr>
<tr>
<td>
<code>workspaces</code><br/>
<em>
<a href="#tekton.dev/v1beta1.WorkspaceBinding">
[]WorkspaceBinding
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Workspaces is a list of WorkspaceBindings from volumes to workspaces.</p>
</td>
</tr>
<tr>
<td>
<code>stepOverrides</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskRunStepOverride">
[]TaskRunStepOverride
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Overrides to apply to Steps in this TaskRun.
If a field is specified in both a Step and a StepOverride,
the value from the StepOverride will be used.
This field is only supported when the alpha feature gate is enabled.</p>
</td>
</tr>
<tr>
<td>
<code>sidecarOverrides</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskRunSidecarOverride">
[]TaskRunSidecarOverride
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Overrides to apply to Sidecars in this TaskRun.
If a field is specified in both a Sidecar and a SidecarOverride,
the value from the SidecarOverride will be used.
This field is only supported when the alpha feature gate is enabled.</p>
</td>
</tr>
<tr>
<td>
<code>computeResources</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#resourcerequirements-v1-core">
Kubernetes core/v1.ResourceRequirements
</a>
</em>
</td>
<td>
<p>Compute resources to use for this TaskRun</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskRunStatus">
TaskRunStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.ChildStatusReference">ChildStatusReference
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.PipelineRunStatusFields">PipelineRunStatusFields</a>)
</p>
<div>
<p>ChildStatusReference is used to point to the statuses of individual TaskRuns and Runs within this PipelineRun.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the TaskRun or Run this is referencing.</p>
</td>
</tr>
<tr>
<td>
<code>pipelineTaskName</code><br/>
<em>
string
</em>
</td>
<td>
<p>PipelineTaskName is the name of the PipelineTask this is referencing.</p>
</td>
</tr>
<tr>
<td>
<code>whenExpressions</code><br/>
<em>
<a href="#tekton.dev/v1beta1.WhenExpression">
[]WhenExpression
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>WhenExpressions is the list of checks guarding the execution of the PipelineTask</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.CloudEventCondition">CloudEventCondition
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.CloudEventDeliveryState">CloudEventDeliveryState</a>)
</p>
<div>
<p>CloudEventCondition is a string that represents the condition of the event.</p>
</div>
<h3 id="tekton.dev/v1beta1.CloudEventDelivery">CloudEventDelivery
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.TaskRunStatusFields">TaskRunStatusFields</a>)
</p>
<div>
<p>CloudEventDelivery is the target of a cloud event along with the state of
delivery.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>target</code><br/>
<em>
string
</em>
</td>
<td>
<p>Target points to an addressable</p>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#tekton.dev/v1beta1.CloudEventDeliveryState">
CloudEventDeliveryState
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.CloudEventDeliveryState">CloudEventDeliveryState
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.CloudEventDelivery">CloudEventDelivery</a>)
</p>
<div>
<p>CloudEventDeliveryState reports the state of a cloud event to be sent.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>condition</code><br/>
<em>
<a href="#tekton.dev/v1beta1.CloudEventCondition">
CloudEventCondition
</a>
</em>
</td>
<td>
<p>Current status</p>
</td>
</tr>
<tr>
<td>
<code>sentAt</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SentAt is the time at which the last attempt to send the event was made</p>
</td>
</tr>
<tr>
<td>
<code>message</code><br/>
<em>
string
</em>
</td>
<td>
<p>Error is the text of error (if any)</p>
</td>
</tr>
<tr>
<td>
<code>retryCount</code><br/>
<em>
int32
</em>
</td>
<td>
<p>RetryCount is the number of attempts of sending the cloud event</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.Combination">Combination
(<code>map[string]string</code> alias)</h3>
<div>
<p>Combination is a map, mainly defined to hold a single combination from a Matrix with key as param.Name and value as param.Value</p>
</div>
<h3 id="tekton.dev/v1beta1.Combinations">Combinations
(<code>[]github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1.Combination</code> alias)</h3>
<div>
<p>Combinations is a Combination list</p>
</div>
<h3 id="tekton.dev/v1beta1.ConfigSource">ConfigSource
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.Provenance">Provenance</a>)
</p>
<div>
<p>ConfigSource contains the information that can uniquely identify where a remote
built definition came from i.e. Git repositories, Tekton Bundles in OCI registry
and hub.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>uri</code><br/>
<em>
string
</em>
</td>
<td>
<p>URI indicates the identity of the source of the build definition.
Example: &ldquo;<a href="https://github.com/tektoncd/catalog&quot;">https://github.com/tektoncd/catalog&rdquo;</a></p>
</td>
</tr>
<tr>
<td>
<code>digest</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<p>Digest is a collection of cryptographic digests for the contents of the artifact specified by URI.
Example: {&ldquo;sha1&rdquo;: &ldquo;f99d13e554ffcb696dee719fa85b695cb5b0f428&rdquo;}</p>
</td>
</tr>
<tr>
<td>
<code>entryPoint</code><br/>
<em>
string
</em>
</td>
<td>
<p>EntryPoint identifies the entry point into the build. This is often a path to a
build definition file and/or a target label within that file.
Example: &ldquo;task/git-clone/0.8/git-clone.yaml&rdquo;</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.CustomRunReason">CustomRunReason
(<code>string</code> alias)</h3>
<div>
<p>CustomRunReason is an enum used to store all Run reason for the Succeeded condition that are controlled by the CustomRun itself.</p>
</div>
<h3 id="tekton.dev/v1beta1.CustomRunSpec">CustomRunSpec
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.CustomRun">CustomRun</a>)
</p>
<div>
<p>CustomRunSpec defines the desired state of CustomRun</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>customRef</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskRef">
TaskRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>customSpec</code><br/>
<em>
<a href="#tekton.dev/v1beta1.EmbeddedCustomRunSpec">
EmbeddedCustomRunSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Spec is a specification of a custom task</p>
</td>
</tr>
<tr>
<td>
<code>params</code><br/>
<em>
<a href="#tekton.dev/v1beta1.Params">
Params
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#tekton.dev/v1beta1.CustomRunSpecStatus">
CustomRunSpecStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Used for cancelling a customrun (and maybe more later on)</p>
</td>
</tr>
<tr>
<td>
<code>statusMessage</code><br/>
<em>
<a href="#tekton.dev/v1beta1.CustomRunSpecStatusMessage">
CustomRunSpecStatusMessage
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Status message for cancellation.</p>
</td>
</tr>
<tr>
<td>
<code>retries</code><br/>
<em>
int
</em>
</td>
<td>
<em>(Optional)</em>
<p>Used for propagating retries count to custom tasks</p>
</td>
</tr>
<tr>
<td>
<code>serviceAccountName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>timeout</code><br/>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Time after which the custom-task times out.
Refer Go&rsquo;s ParseDuration documentation for expected format: <a href="https://golang.org/pkg/time/#ParseDuration">https://golang.org/pkg/time/#ParseDuration</a></p>
</td>
</tr>
<tr>
<td>
<code>workspaces</code><br/>
<em>
<a href="#tekton.dev/v1beta1.WorkspaceBinding">
[]WorkspaceBinding
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Workspaces is a list of WorkspaceBindings from volumes to workspaces.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.CustomRunSpecStatus">CustomRunSpecStatus
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.CustomRunSpec">CustomRunSpec</a>)
</p>
<div>
<p>CustomRunSpecStatus defines the taskrun spec status the user can provide</p>
</div>
<h3 id="tekton.dev/v1beta1.CustomRunSpecStatusMessage">CustomRunSpecStatusMessage
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.CustomRunSpec">CustomRunSpec</a>)
</p>
<div>
<p>CustomRunSpecStatusMessage defines human readable status messages for the TaskRun.</p>
</div>
<h3 id="tekton.dev/v1beta1.EmbeddedCustomRunSpec">EmbeddedCustomRunSpec
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.CustomRunSpec">CustomRunSpec</a>)
</p>
<div>
<p>EmbeddedCustomRunSpec allows custom task definitions to be embedded</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="#tekton.dev/v1beta1.PipelineTaskMetadata">
PipelineTaskMetadata
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
k8s.io/apimachinery/pkg/runtime.RawExtension
</em>
</td>
<td>
<em>(Optional)</em>
<p>Spec is a specification of a custom task</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>-</code><br/>
<em>
[]byte
</em>
</td>
<td>
<p>Raw is the underlying serialization of this object.</p>
<p>TODO: Determine how to detect ContentType and ContentEncoding of &lsquo;Raw&rsquo; data.</p>
</td>
</tr>
<tr>
<td>
<code>-</code><br/>
<em>
k8s.io/apimachinery/pkg/runtime.Object
</em>
</td>
<td>
<p>Object can hold a representation of this extension - useful for working with versioned
structs.</p>
</td>
</tr>
</table>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.EmbeddedTask">EmbeddedTask
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.PipelineTask">PipelineTask</a>)
</p>
<div>
<p>EmbeddedTask is used to define a Task inline within a Pipeline&rsquo;s PipelineTasks.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>spec</code><br/>
<em>
k8s.io/apimachinery/pkg/runtime.RawExtension
</em>
</td>
<td>
<em>(Optional)</em>
<p>Spec is a specification of a custom task</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>-</code><br/>
<em>
[]byte
</em>
</td>
<td>
<p>Raw is the underlying serialization of this object.</p>
<p>TODO: Determine how to detect ContentType and ContentEncoding of &lsquo;Raw&rsquo; data.</p>
</td>
</tr>
<tr>
<td>
<code>-</code><br/>
<em>
k8s.io/apimachinery/pkg/runtime.Object
</em>
</td>
<td>
<p>Object can hold a representation of this extension - useful for working with versioned
structs.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="#tekton.dev/v1beta1.PipelineTaskMetadata">
PipelineTaskMetadata
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>TaskSpec</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskSpec">
TaskSpec
</a>
</em>
</td>
<td>
<p>
(Members of <code>TaskSpec</code> are embedded into this type.)
</p>
<em>(Optional)</em>
<p>TaskSpec is a specification of a task</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.IncludeParams">IncludeParams
</h3>
<div>
<p>IncludeParams allows passing in a specific combinations of Parameters into the Matrix.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name the specified combination</p>
</td>
</tr>
<tr>
<td>
<code>params</code><br/>
<em>
<a href="#tekton.dev/v1beta1.Params">
Params
</a>
</em>
</td>
<td>
<p>Params takes only <code>Parameters</code> of type <code>&quot;string&quot;</code>
The names of the <code>params</code> must match the names of the <code>params</code> in the underlying <code>Task</code></p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.InternalTaskModifier">InternalTaskModifier
</h3>
<div>
<p>InternalTaskModifier implements TaskModifier for resources that are built-in to Tekton Pipelines.</p>
<p>Deprecated: Unused, preserved only for backwards compatibility</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>stepsToPrepend</code><br/>
<em>
<a href="#tekton.dev/v1beta1.Step">
[]Step
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>stepsToAppend</code><br/>
<em>
<a href="#tekton.dev/v1beta1.Step">
[]Step
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>volumes</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#volume-v1-core">
[]Kubernetes core/v1.Volume
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.Matrix">Matrix
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.PipelineTask">PipelineTask</a>)
</p>
<div>
<p>Matrix is used to fan out Tasks in a Pipeline</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>params</code><br/>
<em>
<a href="#tekton.dev/v1beta1.Params">
Params
</a>
</em>
</td>
<td>
<p>Params is a list of parameters used to fan out the pipelineTask
Params takes only <code>Parameters</code> of type <code>&quot;array&quot;</code>
Each array element is supplied to the <code>PipelineTask</code> by substituting <code>params</code> of type <code>&quot;string&quot;</code> in the underlying <code>Task</code>.
The names of the <code>params</code> in the <code>Matrix</code> must match the names of the <code>params</code> in the underlying <code>Task</code> that they will be substituting.</p>
</td>
</tr>
<tr>
<td>
<code>include</code><br/>
<em>
<a href="#tekton.dev/v1beta1.IncludeParamsList">
IncludeParamsList
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Include is a list of IncludeParams which allows passing in specific combinations of Parameters into the Matrix.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.OnErrorType">OnErrorType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.Step">Step</a>)
</p>
<div>
<p>OnErrorType defines a list of supported exiting behavior of a container on error</p>
</div>
<h3 id="tekton.dev/v1beta1.Param">Param
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.TaskRunInputs">TaskRunInputs</a>)
</p>
<div>
<p>Param declares an ParamValues to use for the parameter called name.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>value</code><br/>
<em>
<a href="#tekton.dev/v1beta1.ParamValue">
ParamValue
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.ParamSpec">ParamSpec
</h3>
<div>
<p>ParamSpec defines arbitrary parameters needed beyond typed inputs (such as
resources). Parameter values are provided by users as inputs on a TaskRun
or PipelineRun.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name declares the name by which a parameter is referenced.</p>
</td>
</tr>
<tr>
<td>
<code>type</code><br/>
<em>
<a href="#tekton.dev/v1beta1.ParamType">
ParamType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Type is the user-specified type of the parameter. The possible types
are currently &ldquo;string&rdquo;, &ldquo;array&rdquo; and &ldquo;object&rdquo;, and &ldquo;string&rdquo; is the default.</p>
</td>
</tr>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Description is a user-facing description of the parameter that may be
used to populate a UI.</p>
</td>
</tr>
<tr>
<td>
<code>properties</code><br/>
<em>
<a href="#tekton.dev/v1beta1.PropertySpec">
map[string]github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1.PropertySpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Properties is the JSON Schema properties to support key-value pairs parameter.</p>
</td>
</tr>
<tr>
<td>
<code>default</code><br/>
<em>
<a href="#tekton.dev/v1beta1.ParamValue">
ParamValue
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Default is the value a parameter takes if no input value is supplied. If
default is set, a Task may be executed without a supplied value for the
parameter.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.ParamSpecs">ParamSpecs
(<code>[]github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1.ParamSpec</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.PipelineSpec">PipelineSpec</a>, <a href="#tekton.dev/v1beta1.TaskSpec">TaskSpec</a>)
</p>
<div>
<p>ParamSpecs is a list of ParamSpec</p>
</div>
<h3 id="tekton.dev/v1beta1.ParamType">ParamType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.ParamSpec">ParamSpec</a>, <a href="#tekton.dev/v1beta1.ParamValue">ParamValue</a>, <a href="#tekton.dev/v1beta1.PropertySpec">PropertySpec</a>)
</p>
<div>
<p>ParamType indicates the type of an input parameter;
Used to distinguish between a single string and an array of strings.</p>
</div>
<h3 id="tekton.dev/v1beta1.ParamValue">ParamValue
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.Param">Param</a>, <a href="#tekton.dev/v1beta1.ParamSpec">ParamSpec</a>, <a href="#tekton.dev/v1beta1.PipelineResult">PipelineResult</a>, <a href="#tekton.dev/v1beta1.PipelineRunResult">PipelineRunResult</a>, <a href="#tekton.dev/v1beta1.TaskRunResult">TaskRunResult</a>)
</p>
<div>
<p>ResultValue is a type alias of ParamValue</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>Type</code><br/>
<em>
<a href="#tekton.dev/v1beta1.ParamType">
ParamType
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>StringVal</code><br/>
<em>
string
</em>
</td>
<td>
<p>Represents the stored type of ParamValues.</p>
</td>
</tr>
<tr>
<td>
<code>ArrayVal</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>ObjectVal</code><br/>
<em>
map[string]string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.Params">Params
(<code>[]github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1.Param</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1alpha1.RunSpec">RunSpec</a>, <a href="#tekton.dev/v1beta1.CustomRunSpec">CustomRunSpec</a>, <a href="#tekton.dev/v1beta1.IncludeParams">IncludeParams</a>, <a href="#tekton.dev/v1beta1.Matrix">Matrix</a>, <a href="#tekton.dev/v1beta1.PipelineRunSpec">PipelineRunSpec</a>, <a href="#tekton.dev/v1beta1.PipelineTask">PipelineTask</a>, <a href="#tekton.dev/v1beta1.ResolverRef">ResolverRef</a>, <a href="#tekton.dev/v1beta1.TaskRunSpec">TaskRunSpec</a>)
</p>
<div>
<p>Params is a list of Param</p>
</div>
<h3 id="tekton.dev/v1beta1.PipelineDeclaredResource">PipelineDeclaredResource
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.PipelineSpec">PipelineSpec</a>)
</p>
<div>
<p>PipelineDeclaredResource is used by a Pipeline to declare the types of the
PipelineResources that it will required to run and names which can be used to
refer to these PipelineResources in PipelineTaskResourceBindings.</p>
<p>Deprecated: Unused, preserved only for backwards compatibility</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is the name that will be used by the Pipeline to refer to this resource.
It does not directly correspond to the name of any PipelineResources Task
inputs or outputs, and it does not correspond to the actual names of the
PipelineResources that will be bound in the PipelineRun.</p>
</td>
</tr>
<tr>
<td>
<code>type</code><br/>
<em>
string
</em>
</td>
<td>
<p>Type is the type of the PipelineResource.</p>
</td>
</tr>
<tr>
<td>
<code>optional</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Optional declares the resource as optional.
optional: true - the resource is considered optional
optional: false - the resource is considered required (default/equivalent of not specifying it)</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.PipelineObject">PipelineObject
</h3>
<div>
<p>PipelineObject is implemented by Pipeline</p>
</div>
<h3 id="tekton.dev/v1beta1.PipelineRef">PipelineRef
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.PipelineRunSpec">PipelineRunSpec</a>)
</p>
<div>
<p>PipelineRef can be used to refer to a specific instance of a Pipeline.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name of the referent; More info: <a href="http://kubernetes.io/docs/user-guide/identifiers#names">http://kubernetes.io/docs/user-guide/identifiers#names</a></p>
</td>
</tr>
<tr>
<td>
<code>apiVersion</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>API version of the referent</p>
</td>
</tr>
<tr>
<td>
<code>bundle</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Bundle url reference to a Tekton Bundle.</p>
<p>Deprecated: Please use ResolverRef with the bundles resolver instead.</p>
</td>
</tr>
<tr>
<td>
<code>ResolverRef</code><br/>
<em>
<a href="#tekton.dev/v1beta1.ResolverRef">
ResolverRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ResolverRef allows referencing a Pipeline in a remote location
like a git repo. This field is only supported when the alpha
feature gate is enabled.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.PipelineResourceBinding">PipelineResourceBinding
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.PipelineRunSpec">PipelineRunSpec</a>, <a href="#tekton.dev/v1beta1.TaskResourceBinding">TaskResourceBinding</a>)
</p>
<div>
<p>PipelineResourceBinding connects a reference to an instance of a PipelineResource
with a PipelineResource dependency that the Pipeline has declared</p>
<p>Deprecated: Unused, preserved only for backwards compatibility</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the PipelineResource in the Pipeline&rsquo;s declaration</p>
</td>
</tr>
<tr>
<td>
<code>resourceRef</code><br/>
<em>
<a href="#tekton.dev/v1beta1.PipelineResourceRef">
PipelineResourceRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ResourceRef is a reference to the instance of the actual PipelineResource
that should be used</p>
</td>
</tr>
<tr>
<td>
<code>resourceSpec</code><br/>
<em>
<a href="#tekton.dev/v1alpha1.PipelineResourceSpec">
PipelineResourceSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ResourceSpec is specification of a resource that should be created and
consumed by the task</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.PipelineResourceInterface">PipelineResourceInterface
</h3>
<div>
<p>PipelineResourceInterface interface to be implemented by different PipelineResource types</p>
<p>Deprecated: Unused, preserved only for backwards compatibility</p>
</div>
<h3 id="tekton.dev/v1beta1.PipelineResourceRef">PipelineResourceRef
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.PipelineResourceBinding">PipelineResourceBinding</a>)
</p>
<div>
<p>PipelineResourceRef can be used to refer to a specific instance of a Resource</p>
<p>Deprecated: Unused, preserved only for backwards compatibility</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name of the referent; More info: <a href="http://kubernetes.io/docs/user-guide/identifiers#names">http://kubernetes.io/docs/user-guide/identifiers#names</a></p>
</td>
</tr>
<tr>
<td>
<code>apiVersion</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>API version of the referent</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.PipelineResult">PipelineResult
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.PipelineSpec">PipelineSpec</a>)
</p>
<div>
<p>PipelineResult used to describe the results of a pipeline</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name the given name</p>
</td>
</tr>
<tr>
<td>
<code>type</code><br/>
<em>
<a href="#tekton.dev/v1beta1.ResultsType">
ResultsType
</a>
</em>
</td>
<td>
<p>Type is the user-specified type of the result.
The possible types are &lsquo;string&rsquo;, &lsquo;array&rsquo;, and &lsquo;object&rsquo;, with &lsquo;string&rsquo; as the default.
&lsquo;array&rsquo; and &lsquo;object&rsquo; types are alpha features.</p>
</td>
</tr>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Description is a human-readable description of the result</p>
</td>
</tr>
<tr>
<td>
<code>value</code><br/>
<em>
<a href="#tekton.dev/v1beta1.ParamValue">
ParamValue
</a>
</em>
</td>
<td>
<p>Value the expression used to retrieve the value</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.PipelineRunReason">PipelineRunReason
(<code>string</code> alias)</h3>
<div>
<p>PipelineRunReason represents a reason for the pipeline run &ldquo;Succeeded&rdquo; condition</p>
</div>
<h3 id="tekton.dev/v1beta1.PipelineRunResult">PipelineRunResult
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.PipelineRunStatusFields">PipelineRunStatusFields</a>)
</p>
<div>
<p>PipelineRunResult used to describe the results of a pipeline</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is the result&rsquo;s name as declared by the Pipeline</p>
</td>
</tr>
<tr>
<td>
<code>value</code><br/>
<em>
<a href="#tekton.dev/v1beta1.ParamValue">
ParamValue
</a>
</em>
</td>
<td>
<p>Value is the result returned from the execution of this PipelineRun</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.PipelineRunRunStatus">PipelineRunRunStatus
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.PipelineRunStatusFields">PipelineRunStatusFields</a>)
</p>
<div>
<p>PipelineRunRunStatus contains the name of the PipelineTask for this CustomRun or Run and the CustomRun or Run&rsquo;s Status</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>pipelineTaskName</code><br/>
<em>
string
</em>
</td>
<td>
<p>PipelineTaskName is the name of the PipelineTask.</p>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#tekton.dev/v1beta1.CustomRunStatus">
CustomRunStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Status is the CustomRunStatus for the corresponding CustomRun or Run</p>
</td>
</tr>
<tr>
<td>
<code>whenExpressions</code><br/>
<em>
<a href="#tekton.dev/v1beta1.WhenExpression">
[]WhenExpression
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>WhenExpressions is the list of checks guarding the execution of the PipelineTask</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.PipelineRunSpec">PipelineRunSpec
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.PipelineRun">PipelineRun</a>)
</p>
<div>
<p>PipelineRunSpec defines the desired state of PipelineRun</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>pipelineRef</code><br/>
<em>
<a href="#tekton.dev/v1beta1.PipelineRef">
PipelineRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>pipelineSpec</code><br/>
<em>
<a href="#tekton.dev/v1beta1.PipelineSpec">
PipelineSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>resources</code><br/>
<em>
<a href="#tekton.dev/v1beta1.PipelineResourceBinding">
[]PipelineResourceBinding
</a>
</em>
</td>
<td>
<p>Resources is a list of bindings specifying which actual instances of
PipelineResources to use for the resources the Pipeline has declared
it needs.</p>
<p>Deprecated: Unused, preserved only for backwards compatibility</p>
</td>
</tr>
<tr>
<td>
<code>params</code><br/>
<em>
<a href="#tekton.dev/v1beta1.Params">
Params
</a>
</em>
</td>
<td>
<p>Params is a list of parameter names and values.</p>
</td>
</tr>
<tr>
<td>
<code>serviceAccountName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#tekton.dev/v1beta1.PipelineRunSpecStatus">
PipelineRunSpecStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Used for cancelling a pipelinerun (and maybe more later on)</p>
</td>
</tr>
<tr>
<td>
<code>timeouts</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TimeoutFields">
TimeoutFields
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Time after which the Pipeline times out.
Currently three keys are accepted in the map
pipeline, tasks and finally
with Timeouts.pipeline &gt;= Timeouts.tasks + Timeouts.finally</p>
</td>
</tr>
<tr>
<td>
<code>timeout</code><br/>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Timeout is the Time after which the Pipeline times out.
Defaults to never.
Refer to Go&rsquo;s ParseDuration documentation for expected format: <a href="https://golang.org/pkg/time/#ParseDuration">https://golang.org/pkg/time/#ParseDuration</a></p>
<p>Deprecated: use pipelineRunSpec.Timeouts.Pipeline instead</p>
</td>
</tr>
<tr>
<td>
<code>podTemplate</code><br/>
<em>
<a href="#tekton.dev/unversioned.Template">
Template
</a>
</em>
</td>
<td>
<p>PodTemplate holds pod specific configuration</p>
</td>
</tr>
<tr>
<td>
<code>workspaces</code><br/>
<em>
<a href="#tekton.dev/v1beta1.WorkspaceBinding">
[]WorkspaceBinding
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Workspaces holds a set of workspace bindings that must match names
with those declared in the pipeline.</p>
</td>
</tr>
<tr>
<td>
<code>taskRunSpecs</code><br/>
<em>
<a href="#tekton.dev/v1beta1.PipelineTaskRunSpec">
[]PipelineTaskRunSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>TaskRunSpecs holds a set of runtime specs</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.PipelineRunSpecStatus">PipelineRunSpecStatus
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.PipelineRunSpec">PipelineRunSpec</a>)
</p>
<div>
<p>PipelineRunSpecStatus defines the pipelinerun spec status the user can provide</p>
</div>
<h3 id="tekton.dev/v1beta1.PipelineRunStatus">PipelineRunStatus
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.PipelineRun">PipelineRun</a>)
</p>
<div>
<p>PipelineRunStatus defines the observed state of PipelineRun</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>Status</code><br/>
<em>
<a href="https://pkg.go.dev/knative.dev/pkg/apis/duck/v1#Status">
knative.dev/pkg/apis/duck/v1.Status
</a>
</em>
</td>
<td>
<p>
(Members of <code>Status</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>PipelineRunStatusFields</code><br/>
<em>
<a href="#tekton.dev/v1beta1.PipelineRunStatusFields">
PipelineRunStatusFields
</a>
</em>
</td>
<td>
<p>
(Members of <code>PipelineRunStatusFields</code> are embedded into this type.)
</p>
<p>PipelineRunStatusFields inlines the status fields.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.PipelineRunStatusFields">PipelineRunStatusFields
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.PipelineRunStatus">PipelineRunStatus</a>)
</p>
<div>
<p>PipelineRunStatusFields holds the fields of PipelineRunStatus&rsquo; status.
This is defined separately and inlined so that other types can readily
consume these fields via duck typing.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>startTime</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<p>StartTime is the time the PipelineRun is actually started.</p>
</td>
</tr>
<tr>
<td>
<code>completionTime</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<p>CompletionTime is the time the PipelineRun completed.</p>
</td>
</tr>
<tr>
<td>
<code>taskRuns</code><br/>
<em>
<a href="#tekton.dev/v1beta1.PipelineRunTaskRunStatus">
map[string]*github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1.PipelineRunTaskRunStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>TaskRuns is a map of PipelineRunTaskRunStatus with the taskRun name as the key.</p>
<p>Deprecated: use ChildReferences instead. As of v0.45.0, this field is no
longer populated and is only included for backwards compatibility with
older server versions.</p>
</td>
</tr>
<tr>
<td>
<code>runs</code><br/>
<em>
<a href="#tekton.dev/v1beta1.PipelineRunRunStatus">
map[string]*github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1.PipelineRunRunStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Runs is a map of PipelineRunRunStatus with the run name as the key</p>
<p>Deprecated: use ChildReferences instead. As of v0.45.0, this field is no
longer populated and is only included for backwards compatibility with
older server versions.</p>
</td>
</tr>
<tr>
<td>
<code>pipelineResults</code><br/>
<em>
<a href="#tekton.dev/v1beta1.PipelineRunResult">
[]PipelineRunResult
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>PipelineResults are the list of results written out by the pipeline task&rsquo;s containers</p>
</td>
</tr>
<tr>
<td>
<code>pipelineSpec</code><br/>
<em>
<a href="#tekton.dev/v1beta1.PipelineSpec">
PipelineSpec
</a>
</em>
</td>
<td>
<p>PipelineRunSpec contains the exact spec used to instantiate the run</p>
</td>
</tr>
<tr>
<td>
<code>skippedTasks</code><br/>
<em>
<a href="#tekton.dev/v1beta1.SkippedTask">
[]SkippedTask
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>list of tasks that were skipped due to when expressions evaluating to false</p>
</td>
</tr>
<tr>
<td>
<code>childReferences</code><br/>
<em>
<a href="#tekton.dev/v1beta1.ChildStatusReference">
[]ChildStatusReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>list of TaskRun and Run names, PipelineTask names, and API versions/kinds for children of this PipelineRun.</p>
</td>
</tr>
<tr>
<td>
<code>finallyStartTime</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>FinallyStartTime is when all non-finally tasks have been completed and only finally tasks are being executed.</p>
</td>
</tr>
<tr>
<td>
<code>provenance</code><br/>
<em>
<a href="#tekton.dev/v1beta1.Provenance">
Provenance
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Provenance contains some key authenticated metadata about how a software artifact was built (what sources, what inputs/outputs, etc.).</p>
</td>
</tr>
<tr>
<td>
<code>spanContext</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<p>SpanContext contains tracing span context fields</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.PipelineRunTaskRunStatus">PipelineRunTaskRunStatus
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.PipelineRunStatusFields">PipelineRunStatusFields</a>)
</p>
<div>
<p>PipelineRunTaskRunStatus contains the name of the PipelineTask for this TaskRun and the TaskRun&rsquo;s Status</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>pipelineTaskName</code><br/>
<em>
string
</em>
</td>
<td>
<p>PipelineTaskName is the name of the PipelineTask.</p>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskRunStatus">
TaskRunStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Status is the TaskRunStatus for the corresponding TaskRun</p>
</td>
</tr>
<tr>
<td>
<code>whenExpressions</code><br/>
<em>
<a href="#tekton.dev/v1beta1.WhenExpression">
[]WhenExpression
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>WhenExpressions is the list of checks guarding the execution of the PipelineTask</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.PipelineSpec">PipelineSpec
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.Pipeline">Pipeline</a>, <a href="#tekton.dev/v1beta1.PipelineRunSpec">PipelineRunSpec</a>, <a href="#tekton.dev/v1beta1.PipelineRunStatusFields">PipelineRunStatusFields</a>)
</p>
<div>
<p>PipelineSpec defines the desired state of Pipeline.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>displayName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>DisplayName is a user-facing name of the pipeline that may be
used to populate a UI.</p>
</td>
</tr>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Description is a user-facing description of the pipeline that may be
used to populate a UI.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code><br/>
<em>
<a href="#tekton.dev/v1beta1.PipelineDeclaredResource">
[]PipelineDeclaredResource
</a>
</em>
</td>
<td>
<p>Deprecated: Unused, preserved only for backwards compatibility</p>
</td>
</tr>
<tr>
<td>
<code>tasks</code><br/>
<em>
<a href="#tekton.dev/v1beta1.PipelineTask">
[]PipelineTask
</a>
</em>
</td>
<td>
<p>Tasks declares the graph of Tasks that execute when this Pipeline is run.</p>
</td>
</tr>
<tr>
<td>
<code>params</code><br/>
<em>
<a href="#tekton.dev/v1beta1.ParamSpecs">
ParamSpecs
</a>
</em>
</td>
<td>
<p>Params declares a list of input parameters that must be supplied when
this Pipeline is run.</p>
</td>
</tr>
<tr>
<td>
<code>workspaces</code><br/>
<em>
<a href="#tekton.dev/v1beta1.PipelineWorkspaceDeclaration">
[]PipelineWorkspaceDeclaration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Workspaces declares a set of named workspaces that are expected to be
provided by a PipelineRun.</p>
</td>
</tr>
<tr>
<td>
<code>results</code><br/>
<em>
<a href="#tekton.dev/v1beta1.PipelineResult">
[]PipelineResult
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Results are values that this pipeline can output once run</p>
</td>
</tr>
<tr>
<td>
<code>finally</code><br/>
<em>
<a href="#tekton.dev/v1beta1.PipelineTask">
[]PipelineTask
</a>
</em>
</td>
<td>
<p>Finally declares the list of Tasks that execute just before leaving the Pipeline
i.e. either after all Tasks are finished executing successfully
or after a failure which would result in ending the Pipeline</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.PipelineTask">PipelineTask
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.PipelineSpec">PipelineSpec</a>)
</p>
<div>
<p>PipelineTask defines a task in a Pipeline, passing inputs from both
Params and from the output of previous tasks.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is the name of this task within the context of a Pipeline. Name is
used as a coordinate with the <code>from</code> and <code>runAfter</code> fields to establish
the execution order of tasks relative to one another.</p>
</td>
</tr>
<tr>
<td>
<code>displayName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>DisplayName is the display name of this task within the context of a Pipeline.
This display name may be used to populate a UI.</p>
</td>
</tr>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Description is the description of this task within the context of a Pipeline.
This description may be used to populate a UI.</p>
</td>
</tr>
<tr>
<td>
<code>taskRef</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskRef">
TaskRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>TaskRef is a reference to a task definition.</p>
</td>
</tr>
<tr>
<td>
<code>taskSpec</code><br/>
<em>
<a href="#tekton.dev/v1beta1.EmbeddedTask">
EmbeddedTask
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>TaskSpec is a specification of a task</p>
</td>
</tr>
<tr>
<td>
<code>when</code><br/>
<em>
<a href="#tekton.dev/v1beta1.WhenExpressions">
WhenExpressions
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>WhenExpressions is a list of when expressions that need to be true for the task to run</p>
</td>
</tr>
<tr>
<td>
<code>retries</code><br/>
<em>
int
</em>
</td>
<td>
<em>(Optional)</em>
<p>Retries represents how many times this task should be retried in case of task failure: ConditionSucceeded set to False</p>
</td>
</tr>
<tr>
<td>
<code>runAfter</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>RunAfter is the list of PipelineTask names that should be executed before
this Task executes. (Used to force a specific ordering in graph execution.)</p>
</td>
</tr>
<tr>
<td>
<code>resources</code><br/>
<em>
<a href="#tekton.dev/v1beta1.PipelineTaskResources">
PipelineTaskResources
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Deprecated: Unused, preserved only for backwards compatibility</p>
</td>
</tr>
<tr>
<td>
<code>params</code><br/>
<em>
<a href="#tekton.dev/v1beta1.Params">
Params
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Parameters declares parameters passed to this task.</p>
</td>
</tr>
<tr>
<td>
<code>matrix</code><br/>
<em>
<a href="#tekton.dev/v1beta1.Matrix">
Matrix
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Matrix declares parameters used to fan out this task.</p>
</td>
</tr>
<tr>
<td>
<code>workspaces</code><br/>
<em>
<a href="#tekton.dev/v1beta1.WorkspacePipelineTaskBinding">
[]WorkspacePipelineTaskBinding
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Workspaces maps workspaces from the pipeline spec to the workspaces
declared in the Task.</p>
</td>
</tr>
<tr>
<td>
<code>timeout</code><br/>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Time after which the TaskRun times out. Defaults to 1 hour.
Refer Go&rsquo;s ParseDuration documentation for expected format: <a href="https://golang.org/pkg/time/#ParseDuration">https://golang.org/pkg/time/#ParseDuration</a></p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.PipelineTaskInputResource">PipelineTaskInputResource
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.PipelineTaskResources">PipelineTaskResources</a>)
</p>
<div>
<p>PipelineTaskInputResource maps the name of a declared PipelineResource input
dependency in a Task to the resource in the Pipeline&rsquo;s DeclaredPipelineResources
that should be used. This input may come from a previous task.</p>
<p>Deprecated: Unused, preserved only for backwards compatibility</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the PipelineResource as declared by the Task.</p>
</td>
</tr>
<tr>
<td>
<code>resource</code><br/>
<em>
string
</em>
</td>
<td>
<p>Resource is the name of the DeclaredPipelineResource to use.</p>
</td>
</tr>
<tr>
<td>
<code>from</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>From is the list of PipelineTask names that the resource has to come from.
(Implies an ordering in the execution graph.)</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.PipelineTaskMetadata">PipelineTaskMetadata
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1alpha1.EmbeddedRunSpec">EmbeddedRunSpec</a>, <a href="#tekton.dev/v1beta1.EmbeddedCustomRunSpec">EmbeddedCustomRunSpec</a>, <a href="#tekton.dev/v1beta1.EmbeddedTask">EmbeddedTask</a>, <a href="#tekton.dev/v1beta1.PipelineTaskRunSpec">PipelineTaskRunSpec</a>)
</p>
<div>
<p>PipelineTaskMetadata contains the labels or annotations for an EmbeddedTask</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>labels</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>annotations</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.PipelineTaskOutputResource">PipelineTaskOutputResource
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.PipelineTaskResources">PipelineTaskResources</a>)
</p>
<div>
<p>PipelineTaskOutputResource maps the name of a declared PipelineResource output
dependency in a Task to the resource in the Pipeline&rsquo;s DeclaredPipelineResources
that should be used.</p>
<p>Deprecated: Unused, preserved only for backwards compatibility</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the PipelineResource as declared by the Task.</p>
</td>
</tr>
<tr>
<td>
<code>resource</code><br/>
<em>
string
</em>
</td>
<td>
<p>Resource is the name of the DeclaredPipelineResource to use.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.PipelineTaskParam">PipelineTaskParam
</h3>
<div>
<p>PipelineTaskParam is used to provide arbitrary string parameters to a Task.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>value</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.PipelineTaskResources">PipelineTaskResources
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.PipelineTask">PipelineTask</a>)
</p>
<div>
<p>PipelineTaskResources allows a Pipeline to declare how its DeclaredPipelineResources
should be provided to a Task as its inputs and outputs.</p>
<p>Deprecated: Unused, preserved only for backwards compatibility</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>inputs</code><br/>
<em>
<a href="#tekton.dev/v1beta1.PipelineTaskInputResource">
[]PipelineTaskInputResource
</a>
</em>
</td>
<td>
<p>Inputs holds the mapping from the PipelineResources declared in
DeclaredPipelineResources to the input PipelineResources required by the Task.</p>
</td>
</tr>
<tr>
<td>
<code>outputs</code><br/>
<em>
<a href="#tekton.dev/v1beta1.PipelineTaskOutputResource">
[]PipelineTaskOutputResource
</a>
</em>
</td>
<td>
<p>Outputs holds the mapping from the PipelineResources declared in
DeclaredPipelineResources to the input PipelineResources required by the Task.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.PipelineTaskRun">PipelineTaskRun
</h3>
<div>
<p>PipelineTaskRun reports the results of running a step in the Task. Each
task has the potential to succeed or fail (based on the exit code)
and produces logs.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.PipelineTaskRunSpec">PipelineTaskRunSpec
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.PipelineRunSpec">PipelineRunSpec</a>)
</p>
<div>
<p>PipelineTaskRunSpec  can be used to configure specific
specs for a concrete Task</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>pipelineTaskName</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>taskServiceAccountName</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>taskPodTemplate</code><br/>
<em>
<a href="#tekton.dev/unversioned.Template">
Template
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>stepOverrides</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskRunStepOverride">
[]TaskRunStepOverride
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>sidecarOverrides</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskRunSidecarOverride">
[]TaskRunSidecarOverride
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="#tekton.dev/v1beta1.PipelineTaskMetadata">
PipelineTaskMetadata
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>computeResources</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#resourcerequirements-v1-core">
Kubernetes core/v1.ResourceRequirements
</a>
</em>
</td>
<td>
<p>Compute resources to use for this TaskRun</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.PipelineWorkspaceDeclaration">PipelineWorkspaceDeclaration
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.PipelineSpec">PipelineSpec</a>)
</p>
<div>
<p>WorkspacePipelineDeclaration creates a named slot in a Pipeline that a PipelineRun
is expected to populate with a workspace binding.</p>
<p>Deprecated: use PipelineWorkspaceDeclaration type instead</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is the name of a workspace to be provided by a PipelineRun.</p>
</td>
</tr>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Description is a human readable string describing how the workspace will be
used in the Pipeline. It can be useful to include a bit of detail about which
tasks are intended to have access to the data on the workspace.</p>
</td>
</tr>
<tr>
<td>
<code>optional</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Optional marks a Workspace as not being required in PipelineRuns. By default
this field is false and so declared workspaces are required.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.PropertySpec">PropertySpec
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.ParamSpec">ParamSpec</a>, <a href="#tekton.dev/v1beta1.TaskResult">TaskResult</a>)
</p>
<div>
<p>PropertySpec defines the struct for object keys</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>type</code><br/>
<em>
<a href="#tekton.dev/v1beta1.ParamType">
ParamType
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.Provenance">Provenance
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.PipelineRunStatusFields">PipelineRunStatusFields</a>, <a href="#tekton.dev/v1beta1.TaskRunStatusFields">TaskRunStatusFields</a>)
</p>
<div>
<p>Provenance contains metadata about resources used in the TaskRun/PipelineRun
such as the source from where a remote build definition was fetched.
This field aims to carry minimum amoumt of metadata in *Run status so that
Tekton Chains can capture them in the provenance.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>configSource</code><br/>
<em>
<a href="#tekton.dev/v1beta1.ConfigSource">
ConfigSource
</a>
</em>
</td>
<td>
<p>Deprecated: Use RefSource instead</p>
</td>
</tr>
<tr>
<td>
<code>refSource</code><br/>
<em>
<a href="#tekton.dev/v1beta1.RefSource">
RefSource
</a>
</em>
</td>
<td>
<p>RefSource identifies the source where a remote task/pipeline came from.</p>
</td>
</tr>
<tr>
<td>
<code>featureFlags</code><br/>
<em>
github.com/tektoncd/pipeline/pkg/apis/config.FeatureFlags
</em>
</td>
<td>
<p>FeatureFlags identifies the feature flags that were used during the task/pipeline run</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.RefSource">RefSource
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.Provenance">Provenance</a>)
</p>
<div>
<p>RefSource contains the information that can uniquely identify where a remote
built definition came from i.e. Git repositories, Tekton Bundles in OCI registry
and hub.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>uri</code><br/>
<em>
string
</em>
</td>
<td>
<p>URI indicates the identity of the source of the build definition.
Example: &ldquo;<a href="https://github.com/tektoncd/catalog&quot;">https://github.com/tektoncd/catalog&rdquo;</a></p>
</td>
</tr>
<tr>
<td>
<code>digest</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<p>Digest is a collection of cryptographic digests for the contents of the artifact specified by URI.
Example: {&ldquo;sha1&rdquo;: &ldquo;f99d13e554ffcb696dee719fa85b695cb5b0f428&rdquo;}</p>
</td>
</tr>
<tr>
<td>
<code>entryPoint</code><br/>
<em>
string
</em>
</td>
<td>
<p>EntryPoint identifies the entry point into the build. This is often a path to a
build definition file and/or a target label within that file.
Example: &ldquo;task/git-clone/0.8/git-clone.yaml&rdquo;</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.ResolverName">ResolverName
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.ResolverRef">ResolverRef</a>)
</p>
<div>
<p>ResolverName is the name of a resolver from which a resource can be
requested.</p>
</div>
<h3 id="tekton.dev/v1beta1.ResolverRef">ResolverRef
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.PipelineRef">PipelineRef</a>, <a href="#tekton.dev/v1beta1.TaskRef">TaskRef</a>)
</p>
<div>
<p>ResolverRef can be used to refer to a Pipeline or Task in a remote
location like a git repo.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>resolver</code><br/>
<em>
<a href="#tekton.dev/v1beta1.ResolverName">
ResolverName
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resolver is the name of the resolver that should perform
resolution of the referenced Tekton resource, such as &ldquo;git&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>params</code><br/>
<em>
<a href="#tekton.dev/v1beta1.Params">
Params
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Params contains the parameters used to identify the
referenced Tekton resource. Example entries might include
&ldquo;repo&rdquo; or &ldquo;path&rdquo; but the set of params ultimately depends on
the chosen resolver.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.ResultRef">ResultRef
</h3>
<div>
<p>ResultRef is a type that represents a reference to a task run result</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>pipelineTask</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>result</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>resultsIndex</code><br/>
<em>
int
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>property</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.ResultsType">ResultsType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.PipelineResult">PipelineResult</a>, <a href="#tekton.dev/v1beta1.TaskResult">TaskResult</a>, <a href="#tekton.dev/v1beta1.TaskRunResult">TaskRunResult</a>)
</p>
<div>
<p>ResultsType indicates the type of a result;
Used to distinguish between a single string and an array of strings.
Note that there is ResultType used to find out whether a
RunResult is from a task result or not, which is different from
this ResultsType.</p>
</div>
<h3 id="tekton.dev/v1beta1.RunObject">RunObject
</h3>
<div>
<p>RunObject is implemented by CustomRun and Run</p>
</div>
<h3 id="tekton.dev/v1beta1.Sidecar">Sidecar
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.TaskSpec">TaskSpec</a>)
</p>
<div>
<p>Sidecar has nearly the same data structure as Step but does not have the ability to timeout.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name of the Sidecar specified as a DNS_LABEL.
Each Sidecar in a Task must have a unique name (DNS_LABEL).
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>image</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Image name to be used by the Sidecar.
More info: <a href="https://kubernetes.io/docs/concepts/containers/images">https://kubernetes.io/docs/concepts/containers/images</a></p>
</td>
</tr>
<tr>
<td>
<code>command</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Entrypoint array. Not executed within a shell.
The image&rsquo;s ENTRYPOINT is used if this is not provided.
Variable references $(VAR_NAME) are expanded using the Sidecar&rsquo;s environment. If a variable
cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced
to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. &ldquo;$$(VAR_NAME)&rdquo; will
produce the string literal &ldquo;$(VAR_NAME)&rdquo;. Escaped references will never be expanded, regardless
of whether the variable exists or not. Cannot be updated.
More info: <a href="https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell">https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell</a></p>
</td>
</tr>
<tr>
<td>
<code>args</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Arguments to the entrypoint.
The image&rsquo;s CMD is used if this is not provided.
Variable references $(VAR_NAME) are expanded using the container&rsquo;s environment. If a variable
cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced
to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. &ldquo;$$(VAR_NAME)&rdquo; will
produce the string literal &ldquo;$(VAR_NAME)&rdquo;. Escaped references will never be expanded, regardless
of whether the variable exists or not. Cannot be updated.
More info: <a href="https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell">https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell</a></p>
</td>
</tr>
<tr>
<td>
<code>workingDir</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Sidecar&rsquo;s working directory.
If not specified, the container runtime&rsquo;s default will be used, which
might be configured in the container image.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>ports</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#containerport-v1-core">
[]Kubernetes core/v1.ContainerPort
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>List of ports to expose from the Sidecar. Exposing a port here gives
the system additional information about the network connections a
container uses, but is primarily informational. Not specifying a port here
DOES NOT prevent that port from being exposed. Any port which is
listening on the default &ldquo;0.0.0.0&rdquo; address inside a container will be
accessible from the network.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>envFrom</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#envfromsource-v1-core">
[]Kubernetes core/v1.EnvFromSource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>List of sources to populate environment variables in the Sidecar.
The keys defined within a source must be a C_IDENTIFIER. All invalid keys
will be reported as an event when the Sidecar is starting. When a key exists in multiple
sources, the value associated with the last source will take precedence.
Values defined by an Env with a duplicate key will take precedence.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>env</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#envvar-v1-core">
[]Kubernetes core/v1.EnvVar
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>List of environment variables to set in the Sidecar.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#resourcerequirements-v1-core">
Kubernetes core/v1.ResourceRequirements
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Compute Resources required by this Sidecar.
Cannot be updated.
More info: <a href="https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/">https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/</a></p>
</td>
</tr>
<tr>
<td>
<code>volumeMounts</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#volumemount-v1-core">
[]Kubernetes core/v1.VolumeMount
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Volumes to mount into the Sidecar&rsquo;s filesystem.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>volumeDevices</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#volumedevice-v1-core">
[]Kubernetes core/v1.VolumeDevice
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>volumeDevices is the list of block devices to be used by the Sidecar.</p>
</td>
</tr>
<tr>
<td>
<code>livenessProbe</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#probe-v1-core">
Kubernetes core/v1.Probe
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Periodic probe of Sidecar liveness.
Container will be restarted if the probe fails.
Cannot be updated.
More info: <a href="https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes">https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes</a></p>
</td>
</tr>
<tr>
<td>
<code>readinessProbe</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#probe-v1-core">
Kubernetes core/v1.Probe
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Periodic probe of Sidecar service readiness.
Container will be removed from service endpoints if the probe fails.
Cannot be updated.
More info: <a href="https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes">https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes</a></p>
</td>
</tr>
<tr>
<td>
<code>startupProbe</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#probe-v1-core">
Kubernetes core/v1.Probe
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>StartupProbe indicates that the Pod the Sidecar is running in has successfully initialized.
If specified, no other probes are executed until this completes successfully.
If this probe fails, the Pod will be restarted, just as if the livenessProbe failed.
This can be used to provide different probe parameters at the beginning of a Pod&rsquo;s lifecycle,
when it might take a long time to load data or warm a cache, than during steady-state operation.
This cannot be updated.
More info: <a href="https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes">https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes</a></p>
</td>
</tr>
<tr>
<td>
<code>lifecycle</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#lifecycle-v1-core">
Kubernetes core/v1.Lifecycle
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Actions that the management system should take in response to Sidecar lifecycle events.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>terminationMessagePath</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Optional: Path at which the file to which the Sidecar&rsquo;s termination message
will be written is mounted into the Sidecar&rsquo;s filesystem.
Message written is intended to be brief final status, such as an assertion failure message.
Will be truncated by the node if greater than 4096 bytes. The total message length across
all containers will be limited to 12kb.
Defaults to /dev/termination-log.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>terminationMessagePolicy</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#terminationmessagepolicy-v1-core">
Kubernetes core/v1.TerminationMessagePolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Indicate how the termination message should be populated. File will use the contents of
terminationMessagePath to populate the Sidecar status message on both success and failure.
FallbackToLogsOnError will use the last chunk of Sidecar log output if the termination
message file is empty and the Sidecar exited with an error.
The log output is limited to 2048 bytes or 80 lines, whichever is smaller.
Defaults to File.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>imagePullPolicy</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#pullpolicy-v1-core">
Kubernetes core/v1.PullPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Image pull policy.
One of Always, Never, IfNotPresent.
Defaults to Always if :latest tag is specified, or IfNotPresent otherwise.
Cannot be updated.
More info: <a href="https://kubernetes.io/docs/concepts/containers/images#updating-images">https://kubernetes.io/docs/concepts/containers/images#updating-images</a></p>
</td>
</tr>
<tr>
<td>
<code>securityContext</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#securitycontext-v1-core">
Kubernetes core/v1.SecurityContext
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SecurityContext defines the security options the Sidecar should be run with.
If set, the fields of SecurityContext override the equivalent fields of PodSecurityContext.
More info: <a href="https://kubernetes.io/docs/tasks/configure-pod-container/security-context/">https://kubernetes.io/docs/tasks/configure-pod-container/security-context/</a></p>
</td>
</tr>
<tr>
<td>
<code>stdin</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Whether this Sidecar should allocate a buffer for stdin in the container runtime. If this
is not set, reads from stdin in the Sidecar will always result in EOF.
Default is false.</p>
</td>
</tr>
<tr>
<td>
<code>stdinOnce</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Whether the container runtime should close the stdin channel after it has been opened by
a single attach. When stdin is true the stdin stream will remain open across multiple attach
sessions. If stdinOnce is set to true, stdin is opened on Sidecar start, is empty until the
first client attaches to stdin, and then remains open and accepts data until the client disconnects,
at which time stdin is closed and remains closed until the Sidecar is restarted. If this
flag is false, a container processes that reads from stdin will never receive an EOF.
Default is false</p>
</td>
</tr>
<tr>
<td>
<code>tty</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Whether this Sidecar should allocate a TTY for itself, also requires &lsquo;stdin&rsquo; to be true.
Default is false.</p>
</td>
</tr>
<tr>
<td>
<code>script</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Script is the contents of an executable file to execute.</p>
<p>If Script is not empty, the Step cannot have an Command or Args.</p>
</td>
</tr>
<tr>
<td>
<code>workspaces</code><br/>
<em>
<a href="#tekton.dev/v1beta1.WorkspaceUsage">
[]WorkspaceUsage
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>This is an alpha field. You must set the &ldquo;enable-api-fields&rdquo; feature flag to &ldquo;alpha&rdquo;
for this field to be supported.</p>
<p>Workspaces is a list of workspaces from the Task that this Sidecar wants
exclusive access to. Adding a workspace to this list means that any
other Step or Sidecar that does not also request this Workspace will
not have access to it.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.SidecarState">SidecarState
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.TaskRunStatusFields">TaskRunStatusFields</a>)
</p>
<div>
<p>SidecarState reports the results of running a sidecar in a Task.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>ContainerState</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#containerstate-v1-core">
Kubernetes core/v1.ContainerState
</a>
</em>
</td>
<td>
<p>
(Members of <code>ContainerState</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>container</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>imageID</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.SkippedTask">SkippedTask
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.PipelineRunStatusFields">PipelineRunStatusFields</a>)
</p>
<div>
<p>SkippedTask is used to describe the Tasks that were skipped due to their When Expressions
evaluating to False. This is a struct because we are looking into including more details
about the When Expressions that caused this Task to be skipped.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is the Pipeline Task name</p>
</td>
</tr>
<tr>
<td>
<code>reason</code><br/>
<em>
<a href="#tekton.dev/v1beta1.SkippingReason">
SkippingReason
</a>
</em>
</td>
<td>
<p>Reason is the cause of the PipelineTask being skipped.</p>
</td>
</tr>
<tr>
<td>
<code>whenExpressions</code><br/>
<em>
<a href="#tekton.dev/v1beta1.WhenExpression">
[]WhenExpression
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>WhenExpressions is the list of checks guarding the execution of the PipelineTask</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.SkippingReason">SkippingReason
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.SkippedTask">SkippedTask</a>)
</p>
<div>
<p>SkippingReason explains why a PipelineTask was skipped.</p>
</div>
<h3 id="tekton.dev/v1beta1.Step">Step
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.InternalTaskModifier">InternalTaskModifier</a>, <a href="#tekton.dev/v1beta1.TaskSpec">TaskSpec</a>)
</p>
<div>
<p>Step runs a subcomponent of a Task</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name of the Step specified as a DNS_LABEL.
Each Step in a Task must have a unique name.</p>
</td>
</tr>
<tr>
<td>
<code>image</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Image reference name to run for this Step.
More info: <a href="https://kubernetes.io/docs/concepts/containers/images">https://kubernetes.io/docs/concepts/containers/images</a></p>
</td>
</tr>
<tr>
<td>
<code>command</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Entrypoint array. Not executed within a shell.
The image&rsquo;s ENTRYPOINT is used if this is not provided.
Variable references $(VAR_NAME) are expanded using the container&rsquo;s environment. If a variable
cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced
to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. &ldquo;$$(VAR_NAME)&rdquo; will
produce the string literal &ldquo;$(VAR_NAME)&rdquo;. Escaped references will never be expanded, regardless
of whether the variable exists or not. Cannot be updated.
More info: <a href="https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell">https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell</a></p>
</td>
</tr>
<tr>
<td>
<code>args</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Arguments to the entrypoint.
The image&rsquo;s CMD is used if this is not provided.
Variable references $(VAR_NAME) are expanded using the container&rsquo;s environment. If a variable
cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced
to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. &ldquo;$$(VAR_NAME)&rdquo; will
produce the string literal &ldquo;$(VAR_NAME)&rdquo;. Escaped references will never be expanded, regardless
of whether the variable exists or not. Cannot be updated.
More info: <a href="https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell">https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell</a></p>
</td>
</tr>
<tr>
<td>
<code>workingDir</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Step&rsquo;s working directory.
If not specified, the container runtime&rsquo;s default will be used, which
might be configured in the container image.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>ports</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#containerport-v1-core">
[]Kubernetes core/v1.ContainerPort
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>List of ports to expose from the Step&rsquo;s container. Exposing a port here gives
the system additional information about the network connections a
container uses, but is primarily informational. Not specifying a port here
DOES NOT prevent that port from being exposed. Any port which is
listening on the default &ldquo;0.0.0.0&rdquo; address inside a container will be
accessible from the network.
Cannot be updated.</p>
<p>Deprecated: This field will be removed in a future release.</p>
</td>
</tr>
<tr>
<td>
<code>envFrom</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#envfromsource-v1-core">
[]Kubernetes core/v1.EnvFromSource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>List of sources to populate environment variables in the container.
The keys defined within a source must be a C_IDENTIFIER. All invalid keys
will be reported as an event when the container is starting. When a key exists in multiple
sources, the value associated with the last source will take precedence.
Values defined by an Env with a duplicate key will take precedence.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>env</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#envvar-v1-core">
[]Kubernetes core/v1.EnvVar
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>List of environment variables to set in the container.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#resourcerequirements-v1-core">
Kubernetes core/v1.ResourceRequirements
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Compute Resources required by this Step.
Cannot be updated.
More info: <a href="https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/">https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/</a></p>
</td>
</tr>
<tr>
<td>
<code>volumeMounts</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#volumemount-v1-core">
[]Kubernetes core/v1.VolumeMount
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Volumes to mount into the Step&rsquo;s filesystem.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>volumeDevices</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#volumedevice-v1-core">
[]Kubernetes core/v1.VolumeDevice
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>volumeDevices is the list of block devices to be used by the Step.</p>
</td>
</tr>
<tr>
<td>
<code>livenessProbe</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#probe-v1-core">
Kubernetes core/v1.Probe
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Periodic probe of container liveness.
Step will be restarted if the probe fails.
Cannot be updated.
More info: <a href="https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes">https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes</a></p>
<p>Deprecated: This field will be removed in a future release.</p>
</td>
</tr>
<tr>
<td>
<code>readinessProbe</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#probe-v1-core">
Kubernetes core/v1.Probe
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Periodic probe of container service readiness.
Step will be removed from service endpoints if the probe fails.
Cannot be updated.
More info: <a href="https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes">https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes</a></p>
<p>Deprecated: This field will be removed in a future release.</p>
</td>
</tr>
<tr>
<td>
<code>startupProbe</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#probe-v1-core">
Kubernetes core/v1.Probe
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>DeprecatedStartupProbe indicates that the Pod this Step runs in has successfully initialized.
If specified, no other probes are executed until this completes successfully.
If this probe fails, the Pod will be restarted, just as if the livenessProbe failed.
This can be used to provide different probe parameters at the beginning of a Pod&rsquo;s lifecycle,
when it might take a long time to load data or warm a cache, than during steady-state operation.
This cannot be updated.
More info: <a href="https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes">https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes</a></p>
<p>Deprecated: This field will be removed in a future release.</p>
</td>
</tr>
<tr>
<td>
<code>lifecycle</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#lifecycle-v1-core">
Kubernetes core/v1.Lifecycle
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Actions that the management system should take in response to container lifecycle events.
Cannot be updated.</p>
<p>Deprecated: This field will be removed in a future release.</p>
</td>
</tr>
<tr>
<td>
<code>terminationMessagePath</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Deprecated: This field will be removed in a future release and can&rsquo;t be meaningfully used.</p>
</td>
</tr>
<tr>
<td>
<code>terminationMessagePolicy</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#terminationmessagepolicy-v1-core">
Kubernetes core/v1.TerminationMessagePolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Deprecated: This field will be removed in a future release and can&rsquo;t be meaningfully used.</p>
</td>
</tr>
<tr>
<td>
<code>imagePullPolicy</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#pullpolicy-v1-core">
Kubernetes core/v1.PullPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Image pull policy.
One of Always, Never, IfNotPresent.
Defaults to Always if :latest tag is specified, or IfNotPresent otherwise.
Cannot be updated.
More info: <a href="https://kubernetes.io/docs/concepts/containers/images#updating-images">https://kubernetes.io/docs/concepts/containers/images#updating-images</a></p>
</td>
</tr>
<tr>
<td>
<code>securityContext</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#securitycontext-v1-core">
Kubernetes core/v1.SecurityContext
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SecurityContext defines the security options the Step should be run with.
If set, the fields of SecurityContext override the equivalent fields of PodSecurityContext.
More info: <a href="https://kubernetes.io/docs/tasks/configure-pod-container/security-context/">https://kubernetes.io/docs/tasks/configure-pod-container/security-context/</a></p>
</td>
</tr>
<tr>
<td>
<code>stdin</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Whether this container should allocate a buffer for stdin in the container runtime. If this
is not set, reads from stdin in the container will always result in EOF.
Default is false.</p>
<p>Deprecated: This field will be removed in a future release.</p>
</td>
</tr>
<tr>
<td>
<code>stdinOnce</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Whether the container runtime should close the stdin channel after it has been opened by
a single attach. When stdin is true the stdin stream will remain open across multiple attach
sessions. If stdinOnce is set to true, stdin is opened on container start, is empty until the
first client attaches to stdin, and then remains open and accepts data until the client disconnects,
at which time stdin is closed and remains closed until the container is restarted. If this
flag is false, a container processes that reads from stdin will never receive an EOF.
Default is false</p>
<p>Deprecated: This field will be removed in a future release.</p>
</td>
</tr>
<tr>
<td>
<code>tty</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Whether this container should allocate a DeprecatedTTY for itself, also requires &lsquo;stdin&rsquo; to be true.
Default is false.</p>
<p>Deprecated: This field will be removed in a future release.</p>
</td>
</tr>
<tr>
<td>
<code>script</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Script is the contents of an executable file to execute.</p>
<p>If Script is not empty, the Step cannot have an Command and the Args will be passed to the Script.</p>
</td>
</tr>
<tr>
<td>
<code>timeout</code><br/>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Timeout is the time after which the step times out. Defaults to never.
Refer to Go&rsquo;s ParseDuration documentation for expected format: <a href="https://golang.org/pkg/time/#ParseDuration">https://golang.org/pkg/time/#ParseDuration</a></p>
</td>
</tr>
<tr>
<td>
<code>workspaces</code><br/>
<em>
<a href="#tekton.dev/v1beta1.WorkspaceUsage">
[]WorkspaceUsage
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>This is an alpha field. You must set the &ldquo;enable-api-fields&rdquo; feature flag to &ldquo;alpha&rdquo;
for this field to be supported.</p>
<p>Workspaces is a list of workspaces from the Task that this Step wants
exclusive access to. Adding a workspace to this list means that any
other Step or Sidecar that does not also request this Workspace will
not have access to it.</p>
</td>
</tr>
<tr>
<td>
<code>onError</code><br/>
<em>
<a href="#tekton.dev/v1beta1.OnErrorType">
OnErrorType
</a>
</em>
</td>
<td>
<p>OnError defines the exiting behavior of a container on error
can be set to [ continue | stopAndFail ]</p>
</td>
</tr>
<tr>
<td>
<code>stdoutConfig</code><br/>
<em>
<a href="#tekton.dev/v1beta1.StepOutputConfig">
StepOutputConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Stores configuration for the stdout stream of the step.</p>
</td>
</tr>
<tr>
<td>
<code>stderrConfig</code><br/>
<em>
<a href="#tekton.dev/v1beta1.StepOutputConfig">
StepOutputConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Stores configuration for the stderr stream of the step.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.StepOutputConfig">StepOutputConfig
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.Step">Step</a>)
</p>
<div>
<p>StepOutputConfig stores configuration for a step output stream.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>path</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Path to duplicate stdout stream to on container&rsquo;s local filesystem.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.StepState">StepState
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.TaskRunStatusFields">TaskRunStatusFields</a>)
</p>
<div>
<p>StepState reports the results of running a step in a Task.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>ContainerState</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#containerstate-v1-core">
Kubernetes core/v1.ContainerState
</a>
</em>
</td>
<td>
<p>
(Members of <code>ContainerState</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>container</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>imageID</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.StepTemplate">StepTemplate
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.TaskSpec">TaskSpec</a>)
</p>
<div>
<p>StepTemplate is a template for a Step</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Default name for each Step specified as a DNS_LABEL.
Each Step in a Task must have a unique name.
Cannot be updated.</p>
<p>Deprecated: This field will be removed in a future release.</p>
</td>
</tr>
<tr>
<td>
<code>image</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Default image name to use for each Step.
More info: <a href="https://kubernetes.io/docs/concepts/containers/images">https://kubernetes.io/docs/concepts/containers/images</a>
This field is optional to allow higher level config management to default or override
container images in workload controllers like Deployments and StatefulSets.</p>
</td>
</tr>
<tr>
<td>
<code>command</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Entrypoint array. Not executed within a shell.
The docker image&rsquo;s ENTRYPOINT is used if this is not provided.
Variable references $(VAR_NAME) are expanded using the Step&rsquo;s environment. If a variable
cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced
to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. &ldquo;$$(VAR_NAME)&rdquo; will
produce the string literal &ldquo;$(VAR_NAME)&rdquo;. Escaped references will never be expanded, regardless
of whether the variable exists or not. Cannot be updated.
More info: <a href="https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell">https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell</a></p>
</td>
</tr>
<tr>
<td>
<code>args</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Arguments to the entrypoint.
The image&rsquo;s CMD is used if this is not provided.
Variable references $(VAR_NAME) are expanded using the Step&rsquo;s environment. If a variable
cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced
to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. &ldquo;$$(VAR_NAME)&rdquo; will
produce the string literal &ldquo;$(VAR_NAME)&rdquo;. Escaped references will never be expanded, regardless
of whether the variable exists or not. Cannot be updated.
More info: <a href="https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell">https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell</a></p>
</td>
</tr>
<tr>
<td>
<code>workingDir</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Step&rsquo;s working directory.
If not specified, the container runtime&rsquo;s default will be used, which
might be configured in the container image.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>ports</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#containerport-v1-core">
[]Kubernetes core/v1.ContainerPort
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>List of ports to expose from the Step&rsquo;s container. Exposing a port here gives
the system additional information about the network connections a
container uses, but is primarily informational. Not specifying a port here
DOES NOT prevent that port from being exposed. Any port which is
listening on the default &ldquo;0.0.0.0&rdquo; address inside a container will be
accessible from the network.
Cannot be updated.</p>
<p>Deprecated: This field will be removed in a future release.</p>
</td>
</tr>
<tr>
<td>
<code>envFrom</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#envfromsource-v1-core">
[]Kubernetes core/v1.EnvFromSource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>List of sources to populate environment variables in the Step.
The keys defined within a source must be a C_IDENTIFIER. All invalid keys
will be reported as an event when the container is starting. When a key exists in multiple
sources, the value associated with the last source will take precedence.
Values defined by an Env with a duplicate key will take precedence.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>env</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#envvar-v1-core">
[]Kubernetes core/v1.EnvVar
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>List of environment variables to set in the container.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#resourcerequirements-v1-core">
Kubernetes core/v1.ResourceRequirements
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Compute Resources required by this Step.
Cannot be updated.
More info: <a href="https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/">https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/</a></p>
</td>
</tr>
<tr>
<td>
<code>volumeMounts</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#volumemount-v1-core">
[]Kubernetes core/v1.VolumeMount
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Volumes to mount into the Step&rsquo;s filesystem.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>volumeDevices</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#volumedevice-v1-core">
[]Kubernetes core/v1.VolumeDevice
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>volumeDevices is the list of block devices to be used by the Step.</p>
</td>
</tr>
<tr>
<td>
<code>livenessProbe</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#probe-v1-core">
Kubernetes core/v1.Probe
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Periodic probe of container liveness.
Container will be restarted if the probe fails.
Cannot be updated.
More info: <a href="https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes">https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes</a></p>
<p>Deprecated: This field will be removed in a future release.</p>
</td>
</tr>
<tr>
<td>
<code>readinessProbe</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#probe-v1-core">
Kubernetes core/v1.Probe
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Periodic probe of container service readiness.
Container will be removed from service endpoints if the probe fails.
Cannot be updated.
More info: <a href="https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes">https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes</a></p>
<p>Deprecated: This field will be removed in a future release.</p>
</td>
</tr>
<tr>
<td>
<code>startupProbe</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#probe-v1-core">
Kubernetes core/v1.Probe
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>DeprecatedStartupProbe indicates that the Pod has successfully initialized.
If specified, no other probes are executed until this completes successfully.
If this probe fails, the Pod will be restarted, just as if the livenessProbe failed.
This can be used to provide different probe parameters at the beginning of a Pod&rsquo;s lifecycle,
when it might take a long time to load data or warm a cache, than during steady-state operation.
This cannot be updated.
More info: <a href="https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes">https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes</a></p>
<p>Deprecated: This field will be removed in a future release.</p>
</td>
</tr>
<tr>
<td>
<code>lifecycle</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#lifecycle-v1-core">
Kubernetes core/v1.Lifecycle
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Actions that the management system should take in response to container lifecycle events.
Cannot be updated.</p>
<p>Deprecated: This field will be removed in a future release.</p>
</td>
</tr>
<tr>
<td>
<code>terminationMessagePath</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Deprecated: This field will be removed in a future release and cannot be meaningfully used.</p>
</td>
</tr>
<tr>
<td>
<code>terminationMessagePolicy</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#terminationmessagepolicy-v1-core">
Kubernetes core/v1.TerminationMessagePolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Deprecated: This field will be removed in a future release and cannot be meaningfully used.</p>
</td>
</tr>
<tr>
<td>
<code>imagePullPolicy</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#pullpolicy-v1-core">
Kubernetes core/v1.PullPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Image pull policy.
One of Always, Never, IfNotPresent.
Defaults to Always if :latest tag is specified, or IfNotPresent otherwise.
Cannot be updated.
More info: <a href="https://kubernetes.io/docs/concepts/containers/images#updating-images">https://kubernetes.io/docs/concepts/containers/images#updating-images</a></p>
</td>
</tr>
<tr>
<td>
<code>securityContext</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#securitycontext-v1-core">
Kubernetes core/v1.SecurityContext
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SecurityContext defines the security options the Step should be run with.
If set, the fields of SecurityContext override the equivalent fields of PodSecurityContext.
More info: <a href="https://kubernetes.io/docs/tasks/configure-pod-container/security-context/">https://kubernetes.io/docs/tasks/configure-pod-container/security-context/</a></p>
</td>
</tr>
<tr>
<td>
<code>stdin</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Whether this Step should allocate a buffer for stdin in the container runtime. If this
is not set, reads from stdin in the Step will always result in EOF.
Default is false.</p>
<p>Deprecated: This field will be removed in a future release.</p>
</td>
</tr>
<tr>
<td>
<code>stdinOnce</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Whether the container runtime should close the stdin channel after it has been opened by
a single attach. When stdin is true the stdin stream will remain open across multiple attach
sessions. If stdinOnce is set to true, stdin is opened on container start, is empty until the
first client attaches to stdin, and then remains open and accepts data until the client disconnects,
at which time stdin is closed and remains closed until the container is restarted. If this
flag is false, a container processes that reads from stdin will never receive an EOF.
Default is false</p>
<p>Deprecated: This field will be removed in a future release.</p>
</td>
</tr>
<tr>
<td>
<code>tty</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Whether this Step should allocate a DeprecatedTTY for itself, also requires &lsquo;stdin&rsquo; to be true.
Default is false.</p>
<p>Deprecated: This field will be removed in a future release.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.TaskKind">TaskKind
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.TaskRef">TaskRef</a>)
</p>
<div>
<p>TaskKind defines the type of Task used by the pipeline.</p>
</div>
<h3 id="tekton.dev/v1beta1.TaskModifier">TaskModifier
</h3>
<div>
<p>TaskModifier is an interface to be implemented by different PipelineResources</p>
<p>Deprecated: Unused, preserved only for backwards compatibility</p>
</div>
<h3 id="tekton.dev/v1beta1.TaskObject">TaskObject
</h3>
<div>
<p>TaskObject is implemented by Task and ClusterTask</p>
</div>
<h3 id="tekton.dev/v1beta1.TaskRef">TaskRef
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1alpha1.RunSpec">RunSpec</a>, <a href="#tekton.dev/v1beta1.CustomRunSpec">CustomRunSpec</a>, <a href="#tekton.dev/v1beta1.PipelineTask">PipelineTask</a>, <a href="#tekton.dev/v1beta1.TaskRunSpec">TaskRunSpec</a>)
</p>
<div>
<p>TaskRef can be used to refer to a specific instance of a task.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name of the referent; More info: <a href="http://kubernetes.io/docs/user-guide/identifiers#names">http://kubernetes.io/docs/user-guide/identifiers#names</a></p>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskKind">
TaskKind
</a>
</em>
</td>
<td>
<p>TaskKind indicates the Kind of the Task:
1. Namespaced Task when Kind is set to &ldquo;Task&rdquo;. If Kind is &ldquo;&rdquo;, it defaults to &ldquo;Task&rdquo;.
2. Cluster-Scoped Task when Kind is set to &ldquo;ClusterTask&rdquo;
3. Custom Task when Kind is non-empty and APIVersion is non-empty</p>
</td>
</tr>
<tr>
<td>
<code>apiVersion</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>API version of the referent
Note: A Task with non-empty APIVersion and Kind is considered a Custom Task</p>
</td>
</tr>
<tr>
<td>
<code>bundle</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Bundle url reference to a Tekton Bundle.</p>
<p>Deprecated: Please use ResolverRef with the bundles resolver instead.</p>
</td>
</tr>
<tr>
<td>
<code>ResolverRef</code><br/>
<em>
<a href="#tekton.dev/v1beta1.ResolverRef">
ResolverRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ResolverRef allows referencing a Task in a remote location
like a git repo. This field is only supported when the alpha
feature gate is enabled.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.TaskResource">TaskResource
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.TaskResources">TaskResources</a>)
</p>
<div>
<p>TaskResource defines an input or output Resource declared as a requirement
by a Task. The Name field will be used to refer to these Resources within
the Task definition, and when provided as an Input, the Name will be the
path to the volume mounted containing this Resource as an input (e.g.
an input Resource named <code>workspace</code> will be mounted at <code>/workspace</code>).</p>
<p>Deprecated: Unused, preserved only for backwards compatibility</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>ResourceDeclaration</code><br/>
<em>
<a href="#tekton.dev/v1alpha1.ResourceDeclaration">
ResourceDeclaration
</a>
</em>
</td>
<td>
<p>
(Members of <code>ResourceDeclaration</code> are embedded into this type.)
</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.TaskResourceBinding">TaskResourceBinding
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.TaskRunInputs">TaskRunInputs</a>, <a href="#tekton.dev/v1beta1.TaskRunOutputs">TaskRunOutputs</a>, <a href="#tekton.dev/v1beta1.TaskRunResources">TaskRunResources</a>)
</p>
<div>
<p>TaskResourceBinding points to the PipelineResource that
will be used for the Task input or output called Name.</p>
<p>Deprecated: Unused, preserved only for backwards compatibility</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>PipelineResourceBinding</code><br/>
<em>
<a href="#tekton.dev/v1beta1.PipelineResourceBinding">
PipelineResourceBinding
</a>
</em>
</td>
<td>
<p>
(Members of <code>PipelineResourceBinding</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>paths</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Paths will probably be removed in #1284, and then PipelineResourceBinding can be used instead.
The optional Path field corresponds to a path on disk at which the Resource can be found
(used when providing the resource via mounted volume, overriding the default logic to fetch the Resource).</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.TaskResources">TaskResources
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.TaskSpec">TaskSpec</a>)
</p>
<div>
<p>TaskResources allows a Pipeline to declare how its DeclaredPipelineResources
should be provided to a Task as its inputs and outputs.</p>
<p>Deprecated: Unused, preserved only for backwards compatibility</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>inputs</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskResource">
[]TaskResource
</a>
</em>
</td>
<td>
<p>Inputs holds the mapping from the PipelineResources declared in
DeclaredPipelineResources to the input PipelineResources required by the Task.</p>
</td>
</tr>
<tr>
<td>
<code>outputs</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskResource">
[]TaskResource
</a>
</em>
</td>
<td>
<p>Outputs holds the mapping from the PipelineResources declared in
DeclaredPipelineResources to the input PipelineResources required by the Task.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.TaskResult">TaskResult
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.TaskSpec">TaskSpec</a>)
</p>
<div>
<p>TaskResult used to describe the results of a task</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name the given name</p>
</td>
</tr>
<tr>
<td>
<code>type</code><br/>
<em>
<a href="#tekton.dev/v1beta1.ResultsType">
ResultsType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Type is the user-specified type of the result. The possible type
is currently &ldquo;string&rdquo; and will support &ldquo;array&rdquo; in following work.</p>
</td>
</tr>
<tr>
<td>
<code>properties</code><br/>
<em>
<a href="#tekton.dev/v1beta1.PropertySpec">
map[string]github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1.PropertySpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Properties is the JSON Schema properties to support key-value pairs results.</p>
</td>
</tr>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Description is a human-readable description of the result</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.TaskRunConditionType">TaskRunConditionType
(<code>string</code> alias)</h3>
<div>
<p>TaskRunConditionType is an enum used to store TaskRun custom
conditions such as one used in spire results verification</p>
</div>
<h3 id="tekton.dev/v1beta1.TaskRunDebug">TaskRunDebug
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.TaskRunSpec">TaskRunSpec</a>)
</p>
<div>
<p>TaskRunDebug defines the breakpoint config for a particular TaskRun</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>breakpoint</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.TaskRunInputs">TaskRunInputs
</h3>
<div>
<p>TaskRunInputs holds the input values that this task was invoked with.</p>
<p>Deprecated: Unused, preserved only for backwards compatibility</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>resources</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskResourceBinding">
[]TaskResourceBinding
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>params</code><br/>
<em>
<a href="#tekton.dev/v1beta1.Param">
[]Param
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.TaskRunOutputs">TaskRunOutputs
</h3>
<div>
<p>TaskRunOutputs holds the output values that this task was invoked with.</p>
<p>Deprecated: Unused, preserved only for backwards compatibility</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>resources</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskResourceBinding">
[]TaskResourceBinding
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.TaskRunReason">TaskRunReason
(<code>string</code> alias)</h3>
<div>
<p>TaskRunReason is an enum used to store all TaskRun reason for
the Succeeded condition that are controlled by the TaskRun itself. Failure
reasons that emerge from underlying resources are not included here</p>
</div>
<h3 id="tekton.dev/v1beta1.TaskRunResources">TaskRunResources
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.TaskRunSpec">TaskRunSpec</a>)
</p>
<div>
<p>TaskRunResources allows a TaskRun to declare inputs and outputs TaskResourceBinding</p>
<p>Deprecated: Unused, preserved only for backwards compatibility</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>inputs</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskResourceBinding">
[]TaskResourceBinding
</a>
</em>
</td>
<td>
<p>Inputs holds the inputs resources this task was invoked with</p>
</td>
</tr>
<tr>
<td>
<code>outputs</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskResourceBinding">
[]TaskResourceBinding
</a>
</em>
</td>
<td>
<p>Outputs holds the inputs resources this task was invoked with</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.TaskRunResult">TaskRunResult
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.TaskRunStatusFields">TaskRunStatusFields</a>)
</p>
<div>
<p>TaskRunResult used to describe the results of a task</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name the given name</p>
</td>
</tr>
<tr>
<td>
<code>type</code><br/>
<em>
<a href="#tekton.dev/v1beta1.ResultsType">
ResultsType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Type is the user-specified type of the result. The possible type
is currently &ldquo;string&rdquo; and will support &ldquo;array&rdquo; in following work.</p>
</td>
</tr>
<tr>
<td>
<code>value</code><br/>
<em>
<a href="#tekton.dev/v1beta1.ParamValue">
ParamValue
</a>
</em>
</td>
<td>
<p>Value the given value of the result</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.TaskRunSidecarOverride">TaskRunSidecarOverride
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.PipelineTaskRunSpec">PipelineTaskRunSpec</a>, <a href="#tekton.dev/v1beta1.TaskRunSpec">TaskRunSpec</a>)
</p>
<div>
<p>TaskRunSidecarOverride is used to override the values of a Sidecar in the corresponding Task.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>The name of the Sidecar to override.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#resourcerequirements-v1-core">
Kubernetes core/v1.ResourceRequirements
</a>
</em>
</td>
<td>
<p>The resource requirements to apply to the Sidecar.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.TaskRunSpec">TaskRunSpec
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.TaskRun">TaskRun</a>)
</p>
<div>
<p>TaskRunSpec defines the desired state of TaskRun</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>debug</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskRunDebug">
TaskRunDebug
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>params</code><br/>
<em>
<a href="#tekton.dev/v1beta1.Params">
Params
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>resources</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskRunResources">
TaskRunResources
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Deprecated: Unused, preserved only for backwards compatibility</p>
</td>
</tr>
<tr>
<td>
<code>serviceAccountName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>taskRef</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskRef">
TaskRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>no more than one of the TaskRef and TaskSpec may be specified.</p>
</td>
</tr>
<tr>
<td>
<code>taskSpec</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskSpec">
TaskSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskRunSpecStatus">
TaskRunSpecStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Used for cancelling a TaskRun (and maybe more later on)</p>
</td>
</tr>
<tr>
<td>
<code>statusMessage</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskRunSpecStatusMessage">
TaskRunSpecStatusMessage
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Status message for cancellation.</p>
</td>
</tr>
<tr>
<td>
<code>retries</code><br/>
<em>
int
</em>
</td>
<td>
<em>(Optional)</em>
<p>Retries represents how many times this TaskRun should be retried in the event of Task failure.</p>
</td>
</tr>
<tr>
<td>
<code>timeout</code><br/>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Time after which one retry attempt times out. Defaults to 1 hour.
Refer Go&rsquo;s ParseDuration documentation for expected format: <a href="https://golang.org/pkg/time/#ParseDuration">https://golang.org/pkg/time/#ParseDuration</a></p>
</td>
</tr>
<tr>
<td>
<code>podTemplate</code><br/>
<em>
<a href="#tekton.dev/unversioned.Template">
Template
</a>
</em>
</td>
<td>
<p>PodTemplate holds pod specific configuration</p>
</td>
</tr>
<tr>
<td>
<code>workspaces</code><br/>
<em>
<a href="#tekton.dev/v1beta1.WorkspaceBinding">
[]WorkspaceBinding
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Workspaces is a list of WorkspaceBindings from volumes to workspaces.</p>
</td>
</tr>
<tr>
<td>
<code>stepOverrides</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskRunStepOverride">
[]TaskRunStepOverride
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Overrides to apply to Steps in this TaskRun.
If a field is specified in both a Step and a StepOverride,
the value from the StepOverride will be used.
This field is only supported when the alpha feature gate is enabled.</p>
</td>
</tr>
<tr>
<td>
<code>sidecarOverrides</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskRunSidecarOverride">
[]TaskRunSidecarOverride
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Overrides to apply to Sidecars in this TaskRun.
If a field is specified in both a Sidecar and a SidecarOverride,
the value from the SidecarOverride will be used.
This field is only supported when the alpha feature gate is enabled.</p>
</td>
</tr>
<tr>
<td>
<code>computeResources</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#resourcerequirements-v1-core">
Kubernetes core/v1.ResourceRequirements
</a>
</em>
</td>
<td>
<p>Compute resources to use for this TaskRun</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.TaskRunSpecStatus">TaskRunSpecStatus
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.TaskRunSpec">TaskRunSpec</a>)
</p>
<div>
<p>TaskRunSpecStatus defines the TaskRun spec status the user can provide</p>
</div>
<h3 id="tekton.dev/v1beta1.TaskRunSpecStatusMessage">TaskRunSpecStatusMessage
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.TaskRunSpec">TaskRunSpec</a>)
</p>
<div>
<p>TaskRunSpecStatusMessage defines human readable status messages for the TaskRun.</p>
</div>
<h3 id="tekton.dev/v1beta1.TaskRunStatus">TaskRunStatus
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.TaskRun">TaskRun</a>, <a href="#tekton.dev/v1beta1.PipelineRunTaskRunStatus">PipelineRunTaskRunStatus</a>, <a href="#tekton.dev/v1beta1.TaskRunStatusFields">TaskRunStatusFields</a>)
</p>
<div>
<p>TaskRunStatus defines the observed state of TaskRun</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>Status</code><br/>
<em>
<a href="https://pkg.go.dev/knative.dev/pkg/apis/duck/v1#Status">
knative.dev/pkg/apis/duck/v1.Status
</a>
</em>
</td>
<td>
<p>
(Members of <code>Status</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>TaskRunStatusFields</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskRunStatusFields">
TaskRunStatusFields
</a>
</em>
</td>
<td>
<p>
(Members of <code>TaskRunStatusFields</code> are embedded into this type.)
</p>
<p>TaskRunStatusFields inlines the status fields.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.TaskRunStatusFields">TaskRunStatusFields
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.TaskRunStatus">TaskRunStatus</a>)
</p>
<div>
<p>TaskRunStatusFields holds the fields of TaskRun&rsquo;s status.  This is defined
separately and inlined so that other types can readily consume these fields
via duck typing.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>podName</code><br/>
<em>
string
</em>
</td>
<td>
<p>PodName is the name of the pod responsible for executing this task&rsquo;s steps.</p>
</td>
</tr>
<tr>
<td>
<code>startTime</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<p>StartTime is the time the build is actually started.</p>
</td>
</tr>
<tr>
<td>
<code>completionTime</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<p>CompletionTime is the time the build completed.</p>
</td>
</tr>
<tr>
<td>
<code>steps</code><br/>
<em>
<a href="#tekton.dev/v1beta1.StepState">
[]StepState
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Steps describes the state of each build step container.</p>
</td>
</tr>
<tr>
<td>
<code>cloudEvents</code><br/>
<em>
<a href="#tekton.dev/v1beta1.CloudEventDelivery">
[]CloudEventDelivery
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CloudEvents describe the state of each cloud event requested via a
CloudEventResource.</p>
<p>Deprecated: Removed in v0.44.0.</p>
</td>
</tr>
<tr>
<td>
<code>retriesStatus</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskRunStatus">
[]TaskRunStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>RetriesStatus contains the history of TaskRunStatus in case of a retry in order to keep record of failures.
All TaskRunStatus stored in RetriesStatus will have no date within the RetriesStatus as is redundant.</p>
</td>
</tr>
<tr>
<td>
<code>resourcesResult</code><br/>
<em>
[]github.com/tektoncd/pipeline/pkg/result.RunResult
</em>
</td>
<td>
<em>(Optional)</em>
<p>Results from Resources built during the TaskRun.
This is tomb-stoned along with the removal of pipelineResources
Deprecated: this field is not populated and is preserved only for backwards compatibility</p>
</td>
</tr>
<tr>
<td>
<code>taskResults</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskRunResult">
[]TaskRunResult
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>TaskRunResults are the list of results written out by the task&rsquo;s containers</p>
</td>
</tr>
<tr>
<td>
<code>sidecars</code><br/>
<em>
<a href="#tekton.dev/v1beta1.SidecarState">
[]SidecarState
</a>
</em>
</td>
<td>
<p>The list has one entry per sidecar in the manifest. Each entry is
represents the imageid of the corresponding sidecar.</p>
</td>
</tr>
<tr>
<td>
<code>taskSpec</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskSpec">
TaskSpec
</a>
</em>
</td>
<td>
<p>TaskSpec contains the Spec from the dereferenced Task definition used to instantiate this TaskRun.</p>
</td>
</tr>
<tr>
<td>
<code>provenance</code><br/>
<em>
<a href="#tekton.dev/v1beta1.Provenance">
Provenance
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Provenance contains some key authenticated metadata about how a software artifact was built (what sources, what inputs/outputs, etc.).</p>
</td>
</tr>
<tr>
<td>
<code>spanContext</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<p>SpanContext contains tracing span context fields</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.TaskRunStepOverride">TaskRunStepOverride
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.PipelineTaskRunSpec">PipelineTaskRunSpec</a>, <a href="#tekton.dev/v1beta1.TaskRunSpec">TaskRunSpec</a>)
</p>
<div>
<p>TaskRunStepOverride is used to override the values of a Step in the corresponding Task.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>The name of the Step to override.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#resourcerequirements-v1-core">
Kubernetes core/v1.ResourceRequirements
</a>
</em>
</td>
<td>
<p>The resource requirements to apply to the Step.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.TaskSpec">TaskSpec
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.ClusterTask">ClusterTask</a>, <a href="#tekton.dev/v1beta1.Task">Task</a>, <a href="#tekton.dev/v1beta1.EmbeddedTask">EmbeddedTask</a>, <a href="#tekton.dev/v1beta1.TaskRunSpec">TaskRunSpec</a>, <a href="#tekton.dev/v1beta1.TaskRunStatusFields">TaskRunStatusFields</a>)
</p>
<div>
<p>TaskSpec defines the desired state of Task.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>resources</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskResources">
TaskResources
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resources is a list input and output resource to run the task
Resources are represented in TaskRuns as bindings to instances of
PipelineResources.</p>
<p>Deprecated: Unused, preserved only for backwards compatibility</p>
</td>
</tr>
<tr>
<td>
<code>params</code><br/>
<em>
<a href="#tekton.dev/v1beta1.ParamSpecs">
ParamSpecs
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Params is a list of input parameters required to run the task. Params
must be supplied as inputs in TaskRuns unless they declare a default
value.</p>
</td>
</tr>
<tr>
<td>
<code>displayName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>DisplayName is a user-facing name of the task that may be
used to populate a UI.</p>
</td>
</tr>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Description is a user-facing description of the task that may be
used to populate a UI.</p>
</td>
</tr>
<tr>
<td>
<code>steps</code><br/>
<em>
<a href="#tekton.dev/v1beta1.Step">
[]Step
</a>
</em>
</td>
<td>
<p>Steps are the steps of the build; each step is run sequentially with the
source mounted into /workspace.</p>
</td>
</tr>
<tr>
<td>
<code>volumes</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#volume-v1-core">
[]Kubernetes core/v1.Volume
</a>
</em>
</td>
<td>
<p>Volumes is a collection of volumes that are available to mount into the
steps of the build.</p>
</td>
</tr>
<tr>
<td>
<code>stepTemplate</code><br/>
<em>
<a href="#tekton.dev/v1beta1.StepTemplate">
StepTemplate
</a>
</em>
</td>
<td>
<p>StepTemplate can be used as the basis for all step containers within the
Task, so that the steps inherit settings on the base container.</p>
</td>
</tr>
<tr>
<td>
<code>sidecars</code><br/>
<em>
<a href="#tekton.dev/v1beta1.Sidecar">
[]Sidecar
</a>
</em>
</td>
<td>
<p>Sidecars are run alongside the Task&rsquo;s step containers. They begin before
the steps start and end after the steps complete.</p>
</td>
</tr>
<tr>
<td>
<code>workspaces</code><br/>
<em>
<a href="#tekton.dev/v1beta1.WorkspaceDeclaration">
[]WorkspaceDeclaration
</a>
</em>
</td>
<td>
<p>Workspaces are the volumes that this Task requires.</p>
</td>
</tr>
<tr>
<td>
<code>results</code><br/>
<em>
<a href="#tekton.dev/v1beta1.TaskResult">
[]TaskResult
</a>
</em>
</td>
<td>
<p>Results are values that this Task can output</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.TimeoutFields">TimeoutFields
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.PipelineRunSpec">PipelineRunSpec</a>)
</p>
<div>
<p>TimeoutFields allows granular specification of pipeline, task, and finally timeouts</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>pipeline</code><br/>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<p>Pipeline sets the maximum allowed duration for execution of the entire pipeline. The sum of individual timeouts for tasks and finally must not exceed this value.</p>
</td>
</tr>
<tr>
<td>
<code>tasks</code><br/>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<p>Tasks sets the maximum allowed duration of this pipeline&rsquo;s tasks</p>
</td>
</tr>
<tr>
<td>
<code>finally</code><br/>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<p>Finally sets the maximum allowed duration of this pipeline&rsquo;s finally</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.WhenExpression">WhenExpression
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.ChildStatusReference">ChildStatusReference</a>, <a href="#tekton.dev/v1beta1.PipelineRunRunStatus">PipelineRunRunStatus</a>, <a href="#tekton.dev/v1beta1.PipelineRunTaskRunStatus">PipelineRunTaskRunStatus</a>, <a href="#tekton.dev/v1beta1.SkippedTask">SkippedTask</a>)
</p>
<div>
<p>WhenExpression allows a PipelineTask to declare expressions to be evaluated before the Task is run
to determine whether the Task should be executed or skipped</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>input</code><br/>
<em>
string
</em>
</td>
<td>
<p>Input is the string for guard checking which can be a static input or an output from a parent Task</p>
</td>
</tr>
<tr>
<td>
<code>operator</code><br/>
<em>
k8s.io/apimachinery/pkg/selection.Operator
</em>
</td>
<td>
<p>Operator that represents an Input&rsquo;s relationship to the values</p>
</td>
</tr>
<tr>
<td>
<code>values</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Values is an array of strings, which is compared against the input, for guard checking
It must be non-empty</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.WhenExpressions">WhenExpressions
(<code>[]github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1.WhenExpression</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.PipelineTask">PipelineTask</a>)
</p>
<div>
<p>WhenExpressions are used to specify whether a Task should be executed or skipped
All of them need to evaluate to True for a guarded Task to be executed.</p>
</div>
<h3 id="tekton.dev/v1beta1.WorkspaceBinding">WorkspaceBinding
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1alpha1.RunSpec">RunSpec</a>, <a href="#tekton.dev/v1beta1.CustomRunSpec">CustomRunSpec</a>, <a href="#tekton.dev/v1beta1.PipelineRunSpec">PipelineRunSpec</a>, <a href="#tekton.dev/v1beta1.TaskRunSpec">TaskRunSpec</a>)
</p>
<div>
<p>WorkspaceBinding maps a Task&rsquo;s declared workspace to a Volume.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the workspace populated by the volume.</p>
</td>
</tr>
<tr>
<td>
<code>subPath</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>SubPath is optionally a directory on the volume which should be used
for this binding (i.e. the volume will be mounted at this sub directory).</p>
</td>
</tr>
<tr>
<td>
<code>volumeClaimTemplate</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#persistentvolumeclaim-v1-core">
Kubernetes core/v1.PersistentVolumeClaim
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>VolumeClaimTemplate is a template for a claim that will be created in the same namespace.
The PipelineRun controller is responsible for creating a unique claim for each instance of PipelineRun.</p>
</td>
</tr>
<tr>
<td>
<code>persistentVolumeClaim</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#persistentvolumeclaimvolumesource-v1-core">
Kubernetes core/v1.PersistentVolumeClaimVolumeSource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>PersistentVolumeClaimVolumeSource represents a reference to a
PersistentVolumeClaim in the same namespace. Either this OR EmptyDir can be used.</p>
</td>
</tr>
<tr>
<td>
<code>emptyDir</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#emptydirvolumesource-v1-core">
Kubernetes core/v1.EmptyDirVolumeSource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>EmptyDir represents a temporary directory that shares a Task&rsquo;s lifetime.
More info: <a href="https://kubernetes.io/docs/concepts/storage/volumes#emptydir">https://kubernetes.io/docs/concepts/storage/volumes#emptydir</a>
Either this OR PersistentVolumeClaim can be used.</p>
</td>
</tr>
<tr>
<td>
<code>configMap</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#configmapvolumesource-v1-core">
Kubernetes core/v1.ConfigMapVolumeSource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ConfigMap represents a configMap that should populate this workspace.</p>
</td>
</tr>
<tr>
<td>
<code>secret</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#secretvolumesource-v1-core">
Kubernetes core/v1.SecretVolumeSource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Secret represents a secret that should populate this workspace.</p>
</td>
</tr>
<tr>
<td>
<code>projected</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#projectedvolumesource-v1-core">
Kubernetes core/v1.ProjectedVolumeSource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Projected represents a projected volume that should populate this workspace.</p>
</td>
</tr>
<tr>
<td>
<code>csi</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#csivolumesource-v1-core">
Kubernetes core/v1.CSIVolumeSource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CSI (Container Storage Interface) represents ephemeral storage that is handled by certain external CSI drivers.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.WorkspaceDeclaration">WorkspaceDeclaration
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.TaskSpec">TaskSpec</a>)
</p>
<div>
<p>WorkspaceDeclaration is a declaration of a volume that a Task requires.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is the name by which you can bind the volume at runtime.</p>
</td>
</tr>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Description is an optional human readable description of this volume.</p>
</td>
</tr>
<tr>
<td>
<code>mountPath</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>MountPath overrides the directory that the volume will be made available at.</p>
</td>
</tr>
<tr>
<td>
<code>readOnly</code><br/>
<em>
bool
</em>
</td>
<td>
<p>ReadOnly dictates whether a mounted volume is writable. By default this
field is false and so mounted volumes are writable.</p>
</td>
</tr>
<tr>
<td>
<code>optional</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Optional marks a Workspace as not being required in TaskRuns. By default
this field is false and so declared workspaces are required.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.WorkspacePipelineTaskBinding">WorkspacePipelineTaskBinding
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.PipelineTask">PipelineTask</a>)
</p>
<div>
<p>WorkspacePipelineTaskBinding describes how a workspace passed into the pipeline should be
mapped to a task&rsquo;s declared workspace.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the workspace as declared by the task</p>
</td>
</tr>
<tr>
<td>
<code>workspace</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Workspace is the name of the workspace declared by the pipeline</p>
</td>
</tr>
<tr>
<td>
<code>subPath</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>SubPath is optionally a directory on the volume which should be used
for this binding (i.e. the volume will be mounted at this sub directory).</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.WorkspaceUsage">WorkspaceUsage
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.Sidecar">Sidecar</a>, <a href="#tekton.dev/v1beta1.Step">Step</a>)
</p>
<div>
<p>WorkspaceUsage is used by a Step or Sidecar to declare that it wants isolated access
to a Workspace defined in a Task.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the workspace this Step or Sidecar wants access to.</p>
</td>
</tr>
<tr>
<td>
<code>mountPath</code><br/>
<em>
string
</em>
</td>
<td>
<p>MountPath is the path that the workspace should be mounted to inside the Step or Sidecar,
overriding any MountPath specified in the Task&rsquo;s WorkspaceDeclaration.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.CustomRunResult">CustomRunResult
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.CustomRunStatusFields">CustomRunStatusFields</a>)
</p>
<div>
<p>CustomRunResult used to describe the results of a task</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name the given name</p>
</td>
</tr>
<tr>
<td>
<code>value</code><br/>
<em>
string
</em>
</td>
<td>
<p>Value the given value of the result</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.CustomRunStatus">CustomRunStatus
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.CustomRun">CustomRun</a>, <a href="#tekton.dev/v1.PipelineRunRunStatus">PipelineRunRunStatus</a>, <a href="#tekton.dev/v1beta1.PipelineRunRunStatus">PipelineRunRunStatus</a>, <a href="#tekton.dev/v1beta1.CustomRunStatusFields">CustomRunStatusFields</a>)
</p>
<div>
<p>CustomRunStatus defines the observed state of CustomRun</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>Status</code><br/>
<em>
<a href="https://pkg.go.dev/knative.dev/pkg/apis/duck/v1#Status">
knative.dev/pkg/apis/duck/v1.Status
</a>
</em>
</td>
<td>
<p>
(Members of <code>Status</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>CustomRunStatusFields</code><br/>
<em>
<a href="#tekton.dev/v1beta1.CustomRunStatusFields">
CustomRunStatusFields
</a>
</em>
</td>
<td>
<p>
(Members of <code>CustomRunStatusFields</code> are embedded into this type.)
</p>
<p>CustomRunStatusFields inlines the status fields.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="tekton.dev/v1beta1.CustomRunStatusFields">CustomRunStatusFields
</h3>
<p>
(<em>Appears on:</em><a href="#tekton.dev/v1beta1.CustomRunStatus">CustomRunStatus</a>)
</p>
<div>
<p>CustomRunStatusFields holds the fields of CustomRun&rsquo;s status.  This is defined
separately and inlined so that other types can readily consume these fields
via duck typing.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>startTime</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>StartTime is the time the build is actually started.</p>
</td>
</tr>
<tr>
<td>
<code>completionTime</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CompletionTime is the time the build completed.</p>
</td>
</tr>
<tr>
<td>
<code>results</code><br/>
<em>
<a href="#tekton.dev/v1beta1.CustomRunResult">
[]CustomRunResult
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Results reports any output result values to be consumed by later
tasks in a pipeline.</p>
</td>
</tr>
<tr>
<td>
<code>retriesStatus</code><br/>
<em>
<a href="#tekton.dev/v1beta1.CustomRunStatus">
[]CustomRunStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>RetriesStatus contains the history of CustomRunStatus, in case of a retry.</p>
</td>
</tr>
<tr>
<td>
<code>extraFields</code><br/>
<em>
k8s.io/apimachinery/pkg/runtime.RawExtension
</em>
</td>
<td>
<p>ExtraFields holds arbitrary fields provided by the custom task
controller.</p>
</td>
</tr>
</tbody>
</table>
<hr/>
<p><em>
Generated with <code>gen-crd-api-reference-docs</code>
.
</em></p>
