/*
Copyright 2019 The Knative Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pipelinelistener

import (
	"context"
	"flag"
	"reflect"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/knative/pkg/controller"
	"github.com/tektoncd/pipeline/pkg/logging"

	v1alpha1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	informers "github.com/tektoncd/pipeline/pkg/client/informers/externalversions/pipeline/v1alpha1"
	listers "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler"
	appsv1 "k8s.io/api/apps/v1"
)

const controllerAgentName = "cloudeventslistener-controller"

var (
	// The container used to accept cloud events and generate builds.
	listenerImage = flag.String("cloud-events-listener-image", "override:latest",
		"The container image for the cloud event listener.")
)

// Reconciler is the controller.Reconciler implementation for CloudEventsListener resources
type Reconciler struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// Listing cloud event listeners
	pipelineListenerLister listers.PipelineListenerLister
	// logger for inner info
	logger *zap.SugaredLogger
}

// Check that we implement the controller.Reconciler interface.
var _ controller.Reconciler = (*Reconciler)(nil)

// NewController returns a new cloud events listener controller
func NewController(
	kubeclientset kubernetes.Interface,
	pipelineListenerInformer informers.PipelineListenerInformer,
) *controller.Impl {
	// Enrich the logs with controller name
	logger, _ := logging.NewLogger("", "pipeline-listener")

	r := &Reconciler{
		kubeclientset:          kubeclientset,
		pipelineListenerLister: pipelineListenerInformer.Lister(),
		logger:                 logger,
	}
	impl := controller.NewImpl(r, logger, "PipelineListener",
		reconciler.MustNewStatsReporter("PipelineListener", r.logger))

	logger.Info("Setting up pipeline-listener event handler")
	// Set up an event handler for when PipelineListener resources change
	pipelineListenerInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    impl.Enqueue,
		UpdateFunc: controller.PassNew(impl.Enqueue),
	})

	return impl
}

// Reconcile will create the necessary statefulset to manage the listener process
func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	c.logger.Info("pipeline-listener-reconcile")

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		c.logger.Errorf("invalid resource key: %s", key)
		return nil
	}

	pl, err := c.pipelineListenerLister.PipelineListeners(namespace).Get(name)
	if errors.IsNotFound(err) {
		c.logger.Errorf("listener %q in work queue no longer exists", key)
		return nil
	} else if err != nil {
		return err
	}

	pl = pl.DeepCopy()
	setName := pl.Name + "-statefulset"

	containerEnv := []corev1.EnvVar{
		{
			Name:  "EVENT_TYPE",
			Value: pl.Spec.EventType,
		},
		{
			Name:  "Event",
			Value: pl.Spec.Event,
		},
		{
			Name:  "NAMESPACE",
			Value: pl.Spec.Namespace,
		},
		{
			Name:  "LISTENER_RESOURCE",
			Value: pl.Name,
		},
		{
			Name:  "Port",
			Value: string(pl.Spec.Port),
		},
	}

	c.logger.Infof("launching pipeline-listener %s with type: %s namespace: %s service account %s",
		pl.Name,
		pl.Spec.EventType,
		pl.Spec.Namespace,
	)

	// Create a stateful set for the listener. It mounts a secret containing the build information.
	// The build spec may contain sensetive data and therefore the whole thing seems safest/easiest as a secret
	set := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      setName,
			Namespace: pl.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"statefulset": pl.Name + "-statefulset"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
					"statefulset": pl.Name + "-statefulset",
				}},
				Spec: corev1.PodSpec{
					ServiceAccountName: pl.Spec.PipelineRunSpec.ServiceAccount,
					Containers: []corev1.Container{
						{
							Name:  "pipeline-listener",
							Image: *listenerImage,
							Env:   containerEnv,
							Ports: []corev1.ContainerPort{
								{
									Name:          "listener-port",
									ContainerPort: int32(8082),
									HostPort:      int32(8082),
								},
							},
						},
					},
				},
			},
		},
	}

	found, err := c.kubeclientset.AppsV1().StatefulSets(pl.Namespace).Get(setName, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		c.logger.Info("Creating StatefulSet", "namespace", set.Namespace, "name", set.Name)
		created, err := c.kubeclientset.AppsV1().StatefulSets(pl.Namespace).Create(set)
		pl.Status = v1alpha1.PipelineListenerStatus{
			Namespace:       pl.Namespace,
			StatefulSetName: created.Name,
		}

		return err
	} else if err != nil {
		return err
	}

	if !reflect.DeepEqual(set.Spec, found.Spec) {
		found.Spec = set.Spec
		c.logger.Info("Updating Stateful Set", "namespace", set.Namespace, "name", set.Name)
		updated, err := c.kubeclientset.AppsV1().StatefulSets(pl.Namespace).Update(found)
		if err != nil {
			return err
		}
		pl.Status = v1alpha1.PipelineListenerStatus{
			Namespace:       pl.Namespace,
			StatefulSetName: updated.Name,
		}
	}
	return nil
}
