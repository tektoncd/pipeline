## Kubernetes objects

Before learning about how Tekton works, it's useful to take some time to understand what a Kubernetes object is.
Please see [Understanding Kubernetes objects](https://kubernetes.io/docs/concepts/overview/working-with-objects/kubernetes-objects/)
for an overview of working with objects.
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
[In-depth introduction to Kubernetes admission webhooks](https://banzaicloud.com/blog/k8s-admission-webhooks/).

### Generated Code

Tekton uses [knative/pkg](https://pkg.go.dev/knative.dev/pkg) to generate client code for its CRDs, as well as code for its controllers and webhooks.
Code generation scripts are found in the [hack folder](../../hack/README.md). These scripts write generated code to [pkg/client](../../pkg/client).
The Knative [docs on dependency injection](https://github.com/knative/pkg/tree/main/injection) describe the contracts of the
[functions that must be implemented](https://github.com/knative/pkg/tree/main/injection#generated-reconciler-responsibilities)
for generated reconcilers and webhooks, as well as the concept of ["informers"](https://github.com/knative/pkg/tree/main/injection#consuming-informers),
which notify the reconcilers about other objects in the cluster.

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
