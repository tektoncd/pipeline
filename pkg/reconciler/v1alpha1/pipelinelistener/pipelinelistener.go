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
	"github.com/kubernetes/apimachinery/pkg/util/json"
	"github.com/tektoncd/pipeline/pkg/logging"

	v1alpha1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
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
	// buildclientset is a clientset for our own API group
	clientset clientset.Interface
	// Listing cloud event listeners
	eventsListenerLister listers.PipelineListenerLister
	// logger for inner info
	logger *zap.SugaredLogger
}

// Check that we implement the controller.Reconciler interface.
var _ controller.Reconciler = (*Reconciler)(nil)

// NewController returns a new cloud events listener controller
func NewController(
	kubeclientset kubernetes.Interface,
	clientset clientset.Interface,
	pipelineListenerInformer informers.PipelineListenerInformer,
) *controller.Impl {
	// Enrich the logs with controller name
	logger, _ := logging.NewLogger("", "pipeline-listener")

	r := &Reconciler{
		kubeclientset:        kubeclientset,
		clientset:            clientset,
		eventsListenerLister: pipelineListenerInformer.Lister(),
		logger:               logger,
	}
	impl := controller.NewImpl(r, logger, "CloudEventsListener",
		reconciler.MustNewStatsReporter("CloudEventsListener", r.logger))

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

	cel, err := c.eventsListenerLister.PipelineListeners(namespace).Get(name)
	if errors.IsNotFound(err) {
		c.logger.Errorf("cloud event listener %q in work queue no longer exists", key)
		return nil
	} else if err != nil {
		return err
	}

	cel = cel.DeepCopy()
	setName := cel.Name + "-statefulset"

	buildData, err := json.Marshal(cel.Spec.Build)
	if err != nil {
		c.logger.Errorf("could not marshal cloudevent: %q", key)
		return err

	}

	containerEnv := []corev1.EnvVar{
		{
			Name:  "EVENT_TYPE",
			Value: cel.Spec.CloudEventType,
		},
		{
			Name:  "BRANCH",
			Value: cel.Spec.Branch,
		},
		{
			Name:  "NAMESPACE",
			Value: cel.Spec.Namespace,
		},
	}

	c.logger.Infof("launching listener with type: %s branch: %s namespace: %s service account %s",
		cel.Spec.CloudEventType,
		cel.Spec.Branch,
		cel.Spec.Namespace,
		cel.Spec.ServiceAccount,
	)

	secretName := "knative-cloud-event-listener-secret-" + name

	// mount a secret to house the desired build definition
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: cel.Namespace,
		},
		Data: map[string][]byte{
			"build.json": []byte(buildData),
		},
	}

	_, err = c.kubeclientset.Core().Secrets(cel.Namespace).Get(secretName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err = c.kubeclientset.Core().Secrets(cel.Namespace).Create(&secret)
			if err != nil {
				c.logger.Errorf("Unable to create build secret:", err)
				return err
			}

		} else {
			c.logger.Errorf("Unable to get build secret:", err)
			return err
		}
	}

	// Create a stateful set for the listener. It mounts a secret containing the build information.
	// The build spec may contain sensetive data and therefore the whole thing seems safest/easiest as a secret
	set := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      setName,
			Namespace: cel.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"statefulset": cel.Name + "-statefulset"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
					"statefulset": cel.Name + "-statefulset",
				}},
				Spec: corev1.PodSpec{
					ServiceAccountName: cel.Spec.ServiceAccount,
					Containers: []corev1.Container{
						{
							Name:  "cloud-events-listener",
							Image: *listenerImage,
							Env:   containerEnv,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "build-volume",
									MountPath: "/root/builddata",
									ReadOnly:  true,
								},
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "listener-port",
									ContainerPort: int32(8082),
									HostPort:      int32(8082),
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "build-volume",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretName,
								},
							},
						},
					},
				},
			},
		},
	}

	found, err := c.kubeclientset.AppsV1().StatefulSets(cel.Namespace).Get(setName, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		c.logger.Info("Creating StatefulSet", "namespace", set.Namespace, "name", set.Name)
		created, err := c.kubeclientset.AppsV1().StatefulSets(cel.Namespace).Create(set)
		cel.Status = v1alpha1.PipelineListenerStatus{
			Namespace:       cel.Namespace,
			StatefulSetName: created.Name,
		}

		return err
	} else if err != nil {
		return err
	}

	if !reflect.DeepEqual(set.Spec, found.Spec) {
		found.Spec = set.Spec
		c.logger.Info("Updating Stateful Set", "namespace", set.Namespace, "name", set.Name)
		updated, err := c.kubeclientset.AppsV1().StatefulSets(cel.Namespace).Update(found)
		if err != nil {
			return err
		}
		cel.Status = v1alpha1.PipelineListenerStatus{
			Namespace:       cel.Namespace,
			StatefulSetName: updated.Name,
		}
	}
	return nil
}
