package node

import (
	"context"
	"fmt"

	"github.com/tektoncd/pipeline/pkg/workspace"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	nodereconciler "knative.dev/pkg/client/injection/kube/reconciler/core/v1/node"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

// Reconciler implements controller.Reconciler for Configuration resources.
type Reconciler struct {
	KubeClientSet kubernetes.Interface
}

// Check that our Reconciler implements noderunreconciler.Interface
var (
	_ nodereconciler.Interface = (*Reconciler)(nil)
)

// ReconcileKind deletes any affinity assistant pods on unschedulable nodes
func (c *Reconciler) ReconcileKind(ctx context.Context, n *corev1.Node) pkgreconciler.Event {
	if !n.Spec.Unschedulable {
		return nil
	}
	logger := logging.FromContext(ctx)
	logger.Debugf("found unschedulable node %s", n.Name)

	// Find all affinity assistant pods on the node
	pods, err := c.KubeClientSet.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		LabelSelector: workspace.LabelComponent + "=" + workspace.ComponentNameAffinityAssistant,
		FieldSelector: "spec.nodeName=" + n.Name,
	})
	if err != nil {
		return fmt.Errorf("couldn't list affinity assistant pods on node %s: %s", n.Name, err)
	}

	// Delete affinity assistant pods, allowing their statefulset controllers to recreate them.
	// Pods are deleted individually since API server rejects DeleteCollection
	for _, pod := range pods.Items {
		err = c.KubeClientSet.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("error deleting affinity assistant pod %s in ns %s: %s", pod.Name, pod.Namespace, err)
		}
	}

	return nil
}
