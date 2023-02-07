<!--
---
linkTitle: "Pod templates"
weight: 409
---
-->

# Pod templates

A Pod template defines a portion of a [`PodSpec`](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#pod-v1-core)
configuration that Tekton can use as "boilerplate" for a Pod that runs your `Tasks` and `Pipelines`.

You can specify a Pod template for `TaskRuns` and `PipelineRuns`. In the template, you can specify custom values for fields governing
the execution of individual `Tasks` or for all `Tasks` executed by a given `PipelineRun`.

You also have the option to define a global Pod template [in your Tekton config](./install.md#customizing-basic-execution-parameters) using the key `default-pod-template`.
However, this global template is going to be merged with any templates
you specify in your `TaskRuns` and `PipelineRuns`. Any field that is
present in both the global template and the `TaskRun`'s or
`PipelineRun`'s template will be taken from the `TaskRun` or `PipelineRun`.

See the following for examples of specifying a Pod template:
- [Specifying a Pod template for a `TaskRun`](./taskruns.md#specifying-a-pod-template)
- [Specifying a Pod template for a `PipelineRun`](./pipelineruns.md#specifying-a-pod-template)

## Affinity Assistant Pod templates

The Pod templates specified in the `TaskRuns` and `PipelineRuns `also apply to
the [affinity assistant Pods](#./workspaces.md#specifying-workspace-order-in-a-pipeline-and-affinity-assistants)
that are created when using Workspaces, but only on select fields.

The supported fields are: `tolerations`, `nodeSelector`, and
`imagePullSecrets` (see the table below for more details).

Similarily to Pod templates, you have the option to define a global affinity
assistant Pod template [in your Tekton config](./install.md#customizing-basic-execution-parameters)
using the key `default-affinity-assistant-pod-template`. The merge strategy is
the same as the one described above.

## Supported fields

Pod templates support fields listed in the table below.

<table>
	<thead>
		<th>Field</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td><code>env</code></td>
			<td>Environment variables defined in the Pod template at <code>TaskRun</code> and <code>PipelineRun</code> level take precedence over the ones defined in <code>steps</code> and <code>stepTemplate</code></td>
		</tr>
		<tr>
			<td><code>nodeSelector</code></td>
			<td>Must be true for <a href=https://kubernetes.io/docs/concepts/configuration/assign-pod-node/>the Pod to fit on a node</a>.</td>
		</tr>
		<tr>
			<td><code>tolerations</code></td>
			<td>Allows (but does not require) the Pods to schedule onto nodes with matching taints.</td>
		</tr>
		<tr>
			<td><code>affinity</code></td>
			<td>Allows constraining the set of nodes for which the Pod can be scheduled based on the labels present on the node.</td>
		</tr>
		<tr>
			<td><code>securityContext</code></td>
			<td>Specifies Pod-level security attributes and common container settings such as <code>runAsUser</code> and <code>selinux</code>.</td>
		</tr>
		<tr>
			<td><code>volumes</code></td>
			<td>Specifies a list of volumes that containers within the Pod can mount. This allows you to specify a volume type for each <code>volumeMount</code> in a <code>Task</code>.</td>
		</tr>
		<tr>
			<td><code>runtimeClassName</code></td>
			<td>Specifies the <a href=https://kubernetes.io/docs/concepts/containers/runtime-class/>runtime class</a> for the Pod.</td>
		</tr>
		<tr>
			<td><code>automountServiceAccountToken</code></td>
			<td><b>Default:</b> <code>true</code>. Determines whether Tekton automatically provides the token for the service account used by the Pod inside containers at a predefined path.</td>
		</tr>
		<tr>
			<td><code>dnsPolicy</code></td>
			<td><b>Default:</b> <code>ClusterFirst</code>. Specifies the <a href=https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/#pod-s-dns-policy>DNS policy</a>
                for the Pod. Legal values are <code>ClusterFirst</code>, <code>Default</code>, and <code>None</code>. Does <b>not</b> support <code>ClusterFirstWithHostNet</code>
                because Tekton Pods cannot run with host networking.</td>
		</tr>
		<tr>
			<td><code>dnsConfig</code></td>
			<td>Specifies <a href=https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/#pod-s-dns-config>additional DNS configuration for the Pod</a>, such as name servers and search domains.</td>
		</tr>
		<tr>
			<td><code>enableServiceLinks</code></td>
			<td><b>Default:</b> <code>true</code>. Determines whether services in the Pod's namespace are exposed as environment variables to the Pod, similarly to Docker service links.</td>
		</tr>
		<tr>
			<td><code>priorityClassName</code></td>
			<td>Specifies the <a href=https://kubernetes.io/docs/concepts/configuration/pod-priority-preemption/>priority class</a> for the Pod. Allows you to selectively enable preemption on lower-priority workloads.</td>
		</tr>
		<tr>
			<td><code>schedulerName</code></td>
			<td>Specifies the <a href=https://kubernetes.io/docs/tasks/administer-cluster/configure-multiple-schedulers/>scheduler</a> to use when dispatching the Pod. You can specify different schedulers for different types of
                workloads, such as <code>volcano.sh</code> for machine learning workloads.</td>
		</tr>
		<tr>
			<td><code>imagePullSecrets</code></td>
			<td>Specifies the <a href=https://kubernetes.io/docs/concepts/configuration/secret/>secret</a> to use when <a href=https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/>
                pulling a container image</a>.</td>
		</tr>
		<tr>
			<td><code>hostNetwork</code></td>
			<td><b>Default:</b> <code>false</code>. Determines whether to use the host network namespace.</td>
		</tr>
		<tr>
			<td><code>hostAliases</code></td>
			<td>Adds entries to a Pod's `/etc/hosts` to provide Pod-level overrides of hostnames. For further info see [Kubernetes' docs for this field](https://kubernetes.io/docs/tasks/network/customize-hosts-file-for-pods/).</td>
		</tr>
        <tr>
            <td><code>topologySpreadConstraints</code></td>
            <td>Specify how Pods are spread across your cluster among topology domains.</td>
        </tr>
	</tbody>
</table>

## Use `imagePullSecrets` to lookup entrypoint

If no command is configured in `task` and `imagePullSecrets` is configured in `podTemplate`, the Tekton Controller will look up the entrypoint of image with `imagePullSecrets`. The Tekton controller's service account is given access to secrets by default. See [this](https://github.com/tektoncd/pipeline/blob/main/config/200-clusterrole.yaml) for reference. If the Tekton controller's service account is not granted the access to secrets in different namespace, you need to grant the access via `RoleBinding`:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: creds-getter
  namespace: my-ns
rules:
- apiGroups: [""]
  resources: ["secrets"]
  resourceNames: ["creds"]
  verbs: ["get"]
```

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: creds-getter-binding
  namespace: my-ns
subjects:
- kind: ServiceAccount
  name: tekton-pipelines-controller
  namespace: tekton-pipelines
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: Role
  name: creds-getter
  apiGroup: rbac.authorization.k8s.io
```

---

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
