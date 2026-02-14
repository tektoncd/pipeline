## Kubernetes objects

Before learning about how Tekton works, it's useful to take some time to understand what a Kubernetes object is.
Please see [Understanding Kubernetes objects](https://kubernetes.io/docs/concepts/overview/working-with-objects/kubernetes-objects/)
for an overview of working with objects.
<!-- wokeignore:rule=master -->
Kubernetes [API conventions docs](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#types-kinds)
are another useful resource for understanding object terminology.

## Tekton CRDs, Webhooks, and Controllers

### CRDs

Tekton objects, including Tasks, TaskRuns, Pipelines, PipelineRuns, and other resources, are implemented as
Kubernetes [CustomResourceDefinitions](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/),
which are extensions of the Kubernetes API.
To better understand what it means to create a CustomResourceDefinition (CRD), check out the tutorial
[Extend the Kubernetes API with CustomResourceDefinitions](https://kubernetes.io/docs/tasks/access-kubernetes-api/custom-resources/custom-resource-definitions/).
Tekton CRDs are defined in the [config folder](../../config/), and the schemas of these CRDs are defined as Go structs in the [apis folder](../../pkg/apis/pipeline/v1).

### Controllers

Creating an instance of a CRD doesn't "do" much. As shown in the [CRD tutorial](https://kubernetes.io/docs/tasks/access-kubernetes-api/custom-resources/custom-resource-definitions/),
creating a CRD allows you to list Objects of that Kind but doesn't result in anything executing.
As described in ["custom controllers"](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#custom-controllers),
you must implement a controller to change a Kubernetes cluster's state when an instance of a CRD is created, a process called "reconciling".
Reconcilers change the cluster based on the desired behavior defined in an object's "spec", and update the object's "status" to reflect what happened.

The [Kubebuilder tutorial](https://book.kubebuilder.io/introduction.html) is a helpful resource for understanding what it means to build a controller
that reconciles CRDs. (Note: Tekton does not use Kubebuilder.)

Not all Tekton CRDs have controllers. For example, there is no reconciler for Tasks, meaning that creating a Task doesn't "do" anything.
To run a Task, you must create an instance of a TaskRun referencing it, and the TaskRun is executed by the TaskRun reconciler.

The TaskRun reconciler code is found in pkg/reconciler/taskrun/taskrun.go, and the PipelineRun reconciler code is found in pkg/reconciler/pipelinerun/pipelinerun.go.

### Webhooks

All Tekton CRDs use validating [admission webhooks](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/)
to validate instances of CRDs. Some CRDs also use mutating admission webhooks to set default values for some fields.
Validation and defaulting code is found in the [apis folder](../../pkg/apis/pipeline/v1).
For a useful overview and tutorial on admission webhooks, see
[In-depth introduction to Kubernetes admission webhooks](https://web.archive.org/web/20230928184501/https://banzaicloud.com/blog/k8s-admission-webhooks/).

### Generated Code

Tekton uses [knative/pkg](https://pkg.go.dev/knative.dev/pkg) to generate client code for its CRDs, as well as code for its controllers and webhooks.
Code generation scripts are found in the [hack folder](../../hack/README.md). These scripts write generated code to [pkg/client](../../pkg/client).
The Knative [docs on dependency injection](https://github.com/knative/pkg/tree/main/injection) describe the contracts of the
[functions that must be implemented](https://github.com/knative/pkg/tree/main/injection#generated-reconciler-responsibilities)
for generated reconcilers and webhooks, as well as the concept of ["informers"](https://github.com/knative/pkg/tree/main/injection#consuming-informers),
which notify the reconcilers about other objects in the cluster.

> **Note on Informer Cache Transforms**: Tekton applies [cache transform functions](../tekton-controller-performance-configuration.md#informer-cache-transform-memory-optimization)
> to reduce memory usage. Objects retrieved from the informer cache (via listers) may have certain fields stripped.
> Some fields are stripped from **all** objects (e.g., `managedFields`, `last-applied-configuration`), while additional
> fields are stripped only from **completed** objects (e.g., `status.taskSpec`, `status.steps`).
> The full objects remain in etcd. If you need access to stripped fields, use the Kubernetes API client directly instead of the lister.
>
> **Developer Warning**: If you add reconciliation logic that reads a field from cached objects, verify that field
> is not stripped by the transform. See `pkg/informer/transform/transform.go` for the complete list of stripped fields.

#### Example: How does the TaskRun reconciler work?

Please read ["Generated reconciler responsibilities"](https://github.com/knative/pkg/tree/main/injection#generated-reconciler-responsibilities)
from the Knative documentation before reading this section.

The TaskRun controller handles [any events related to TaskRuns](https://github.com/tektoncd/pipeline/blob/988cdd50c43cc7333dd0f646f19449e4e7041206/pkg/reconciler/taskrun/controller.go#L87),
and [any events related to a pod owned by a TaskRun](https://github.com/tektoncd/pipeline/blob/988cdd50c43cc7333dd0f646f19449e4e7041206/pkg/reconciler/taskrun/controller.go#L89-L92).

When one of these events occurs, the TaskRun name is added to the reconciler working queue. When an event (a TaskRun) is popped off the queue,
the reconciler [runs a reconcile loop](https://github.com/tektoncd/pipeline/blob/988cdd50c43cc7333dd0f646f19449e4e7041206/pkg/reconciler/taskrun/taskrun.go#L95)
with that TaskRun. It does not know what event occurred; it simply brings the cluster state in line with the TaskRun spec, and updates the TaskRun status to
reflect what happened.

When a TaskRun is created, the reconciler sees that the TaskRun has not run yet, and has no pod associated with it.
It creates a pod to run the TaskRun and exits the reconcile loop.

Later, the pod completes, resulting in another event that triggers reconciliation of the TaskRun that owns it.
The reconciler sees that the TaskRun has a pod associated with it, and that the pod has completed. It updates the TaskRun status to mark it as completed
and exits the reconcile loop.

### Informer Cache Transforms

To reduce controller memory usage, Tekton applies cache transform functions that strip unnecessary fields from objects before they are stored in the informer cache. This is implemented in `pkg/informer/transform/`.

#### How It Works

The transform package registers custom informer factories that override the default factories using Knative's dependency injection. These factories apply transform functions via the `WithTransform` option when creating shared informers.

#### Fields Stripped

**All Objects** (PipelineRuns, TaskRuns, CustomRuns, Pods):
- `metadata.managedFields` - Server-side apply metadata (can be 25-30% of object size)
- `kubectl.kubernetes.io/last-applied-configuration` annotation

**Completed PipelineRuns** (additional fields):
- `status.pipelineSpec` - The resolved Pipeline specification
- `status.provenance` - Build provenance metadata
- `status.spanContext` - Tracing context

**Completed TaskRuns** (additional fields):
- `status.taskSpec` - The resolved Task specification
- `status.provenance` - Build provenance metadata
- `status.spanContext` - Tracing context
- `status.steps` - Detailed step execution state
- `status.sidecars` - Detailed sidecar execution state

**Note:** `status.retriesStatus`, `status.podName`, and `status.completionTime` are **preserved** for completed TaskRuns because they are required for retry tracking in `retryTaskRun()`.

**Completed CustomRuns** (additional fields):
- `status.retriesStatus` - Retry history
- `status.completionTime` - Completion timestamp

**Pods** (stripped aggressively):
- Most of `spec` is stripped, keeping only `spec.containers[].name` for status sorting
- `metadata.managedFields` and `metadata.finalizers` are stripped
- `status` and essential metadata (labels, ownerReferences, annotations) are preserved

#### Configuration

The transforms are enabled by default and can be disabled by setting `enable-informer-cache-transforms: "false"` in the `feature-flags` ConfigMap. Changes require a controller restart.

#### Developer Guidelines

When adding new reconciliation logic:

1. **Check if the field is stripped**: Review `pkg/informer/transform/transform.go` to see if the field you need is stripped from cached objects.

2. **Use API client for stripped fields**: If you need a stripped field, use the Kubernetes API client directly instead of the lister to get the full object from etcd.

3. **Update transforms if needed**: If a field should not be stripped because it's needed for reconciliation, update the transform functions and add tests.

4. **Consider memory impact**: Before preserving additional fields, consider the memory impact—each preserved field increases controller memory usage across all cached objects.
