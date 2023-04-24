/*
Copyright 2020 The Tekton Authors

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

package test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	fakepipelineruninformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1/pipelinerun/fake"
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgotesting "k8s.io/client-go/testing"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	"knative.dev/pkg/system"
	_ "knative.dev/pkg/system/testing"
)

var deploymentsResource = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}

func TestResourceVersionReactor(t *testing.T) {
	tests := []struct {
		name       string
		deployment *appsv1.Deployment
	}{{
		name: "first resource",
		deployment: &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "basic",
				Namespace: "default",
			},
		},
	}, {
		name: "replace resource version",
		deployment: &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "basic",
				Namespace:       "default",
				ResourceVersion: "replace-me",
			},
		},
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lastHandlerInvoked := false

			ns := tc.deployment.Namespace

			ctx, _ := ttesting.SetupFakeContext(t)
			kc := fakekubeclient.Get(ctx)
			pri := fakepipelineruninformer.Get(ctx)

			var mutated *appsv1.Deployment
			kc.PrependReactor("*", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
				create, ok := action.(clientgotesting.CreateAction)
				if !ok {
					return false, nil, nil
				}
				deploy, ok := create.GetObject().(*appsv1.Deployment)
				if !ok {
					return false, nil, nil
				}
				lastHandlerInvoked = true
				mutated = deploy
				return false, nil, nil
			})

			PrependResourceVersionReactor(&kc.Fake)
			kc.PrependReactor("*", "deployments", AddToInformer(t, pri.Informer().GetIndexer()))

			// Good Create
			createAction := clientgotesting.NewCreateAction(deploymentsResource, ns, tc.deployment)
			updatedObj, err := kc.Fake.Invokes(createAction, &appsv1.Deployment{})
			if err != nil {
				t.Fatalf("Create() = %v", err)
			}
			if diff := cmp.Diff("00001", mutated.GetResourceVersion()); diff != "" {
				t.Error(diff)
			}
			if !lastHandlerInvoked {
				t.Error("ResourceVersionReactor should not interfere with the fake's ReactionChain")
			}
			lastHandlerInvoked = false

			// Good Update
			updateAction := clientgotesting.NewUpdateAction(deploymentsResource, ns, updatedObj)
			updatedObj, err = kc.Fake.Invokes(updateAction, &appsv1.Deployment{})
			if err != nil {
				t.Fatalf("Update() = %v", err)
			}
			if diff := cmp.Diff("00002", mutated.GetResourceVersion()); diff != "" {
				t.Error(diff)
			}
			if !lastHandlerInvoked {
				t.Error("ResourceVersionReactor should not interfere with the fake's ReactionChain")
			}
			lastHandlerInvoked = false

			// Bad Update
			bad := tc.deployment.DeepCopy()
			bad.ResourceVersion = "bad-version"
			updateAction = clientgotesting.NewUpdateAction(deploymentsResource, ns, bad)
			obj, err := kc.Fake.Invokes(updateAction, &appsv1.Deployment{})
			if err == nil {
				t.Fatalf("Update() = %#v", obj)
			}
			if lastHandlerInvoked {
				t.Error("Reactor chain should have aborted!")
			}
		})
	}
}

func TestEnsureConfigurationConfigMapsExist(t *testing.T) {
	d := Data{ConfigMaps: []*corev1.ConfigMap{}}
	expected := Data{ConfigMaps: []*corev1.ConfigMap{}}
	expected.ConfigMaps = append(expected.ConfigMaps, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: config.GetDefaultsConfigName(), Namespace: system.Namespace()},
		Data:       map[string]string{},
	})
	expected.ConfigMaps = append(expected.ConfigMaps, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.Namespace()},
		Data:       map[string]string{},
	})
	expected.ConfigMaps = append(expected.ConfigMaps, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: config.GetMetricsConfigName(), Namespace: system.Namespace()},
		Data:       map[string]string{},
	})
	expected.ConfigMaps = append(expected.ConfigMaps, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: config.GetSpireConfigName(), Namespace: system.Namespace()},
		Data:       map[string]string{},
	})

	EnsureConfigurationConfigMapsExist(&d)
	if d := cmp.Diff(expected, d); d != "" {
		t.Errorf("ConfigMaps: diff(-want,+got):\n%s", d)
	}
}
