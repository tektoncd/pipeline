/*
Copyright 2019 The Tekton Authors

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
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	// Link in the fakes so they get injected into injection.Fake
	"github.com/tektoncd/pipeline/pkg/apis/config"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resolutionv1alpha1 "github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	fakepipelineclientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/fake"
	informersv1 "github.com/tektoncd/pipeline/pkg/client/informers/externalversions/pipeline/v1"
	informersv1alpha1 "github.com/tektoncd/pipeline/pkg/client/informers/externalversions/pipeline/v1alpha1"
	informersv1beta1 "github.com/tektoncd/pipeline/pkg/client/informers/externalversions/pipeline/v1beta1"
	fakepipelineclient "github.com/tektoncd/pipeline/pkg/client/injection/client/fake"
	fakepipelineinformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1/pipeline/fake"
	fakepipelineruninformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1/pipelinerun/fake"
	faketaskinformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1/task/fake"
	faketaskruninformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1/taskrun/fake"
	fakeverificationpolicyinformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1alpha1/verificationpolicy/fake"
	fakeclustertaskinformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1beta1/clustertask/fake"
	fakecustomruninformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1beta1/customrun/fake"
	fakeresolutionclientset "github.com/tektoncd/pipeline/pkg/client/resolution/clientset/versioned/fake"
	resolutioninformersv1alpha1 "github.com/tektoncd/pipeline/pkg/client/resolution/informers/externalversions/resolution/v1beta1"
	fakeresolutionrequestclient "github.com/tektoncd/pipeline/pkg/client/resolution/injection/client/fake"
	fakeresolutionrequestinformer "github.com/tektoncd/pipeline/pkg/client/resolution/injection/informers/resolution/v1beta1/resolutionrequest/fake"
	cloudeventclient "github.com/tektoncd/pipeline/pkg/reconciler/events/cloudevent"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	coreinformers "k8s.io/client-go/informers/core/v1"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	fakeconfigmapinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/configmap/fake"
	fakelimitrangeinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/limitrange/fake"
	fakefilteredpodinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/pod/filtered/fake"
	fakeserviceaccountinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/serviceaccount/fake"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/system"
)

// Data represents the desired state of the system (i.e. existing resources) to seed controllers
// with.
type Data struct {
	PipelineRuns            []*v1.PipelineRun
	Pipelines               []*v1.Pipeline
	TaskRuns                []*v1.TaskRun
	Tasks                   []*v1.Task
	ClusterTasks            []*v1beta1.ClusterTask
	CustomRuns              []*v1beta1.CustomRun
	Pods                    []*corev1.Pod
	Namespaces              []*corev1.Namespace
	ConfigMaps              []*corev1.ConfigMap
	ServiceAccounts         []*corev1.ServiceAccount
	LimitRange              []*corev1.LimitRange
	ResolutionRequests      []*resolutionv1alpha1.ResolutionRequest
	ExpectedCloudEventCount int
	VerificationPolicies    []*v1alpha1.VerificationPolicy
}

// Clients holds references to clients which are useful for reconciler tests.
type Clients struct {
	Pipeline           *fakepipelineclientset.Clientset
	Kube               *fakekubeclientset.Clientset
	CloudEvents        cloudeventclient.CEClient
	ResolutionRequests *fakeresolutionclientset.Clientset
}

// Informers holds references to informers which are useful for reconciler tests.
type Informers struct {
	PipelineRun        informersv1.PipelineRunInformer
	Pipeline           informersv1.PipelineInformer
	TaskRun            informersv1.TaskRunInformer
	Run                informersv1alpha1.RunInformer
	CustomRun          informersv1beta1.CustomRunInformer
	Task               informersv1.TaskInformer
	ClusterTask        informersv1beta1.ClusterTaskInformer
	Pod                coreinformers.PodInformer
	ConfigMap          coreinformers.ConfigMapInformer
	ServiceAccount     coreinformers.ServiceAccountInformer
	LimitRange         coreinformers.LimitRangeInformer
	ResolutionRequest  resolutioninformersv1alpha1.ResolutionRequestInformer
	VerificationPolicy informersv1alpha1.VerificationPolicyInformer
}

// Assets holds references to the controller, logs, clients, and informers.
type Assets struct {
	Logger     *zap.SugaredLogger
	Controller *controller.Impl
	Clients    Clients
	Informers  Informers
	Recorder   *record.FakeRecorder
	Ctx        context.Context
}

// AddToInformer returns a function to add ktesting.Actions to the cache store
func AddToInformer(t *testing.T, store cache.Store) func(ktesting.Action) (bool, runtime.Object, error) {
	t.Helper()
	return func(action ktesting.Action) (bool, runtime.Object, error) {
		switch a := action.(type) {
		case ktesting.CreateActionImpl:
			if err := store.Add(a.GetObject()); err != nil {
				t.Fatal(err)
			}

		case ktesting.UpdateActionImpl:
			objMeta, err := meta.Accessor(a.GetObject())
			if err != nil {
				return true, nil, err
			}

			// Look up the old copy of this resource and perform the optimistic concurrency check.
			old, exists, err := store.GetByKey(objMeta.GetNamespace() + "/" + objMeta.GetName())
			if err != nil {
				return true, nil, err
			} else if !exists {
				// Let the client return the error.
				return false, nil, nil
			}
			oldMeta, err := meta.Accessor(old)
			if err != nil {
				return true, nil, err
			}
			// If the resource version is mismatched, then fail with a conflict.
			if oldMeta.GetResourceVersion() != objMeta.GetResourceVersion() {
				return true, nil, apierrs.NewConflict(
					a.Resource.GroupResource(), objMeta.GetName(),
					fmt.Errorf("resourceVersion mismatch, got: %v, wanted: %v",
						objMeta.GetResourceVersion(), oldMeta.GetResourceVersion()))
			}

			// Update the store with the new object when it's fine.
			if err := store.Update(a.GetObject()); err != nil {
				t.Fatal(err)
			}
		}
		return false, nil, nil
	}
}

// SeedTestData returns Clients and Informers populated with the
// given Data.
//
//nolint:revive
func SeedTestData(t *testing.T, ctx context.Context, d Data) (Clients, Informers) {
	t.Helper()
	c := Clients{
		Kube:               fakekubeclient.Get(ctx),
		Pipeline:           fakepipelineclient.Get(ctx),
		CloudEvents:        cloudeventclient.Get(ctx),
		ResolutionRequests: fakeresolutionrequestclient.Get(ctx),
	}
	// Every time a resource is modified, change the metadata.resourceVersion.
	PrependResourceVersionReactor(&c.Pipeline.Fake)

	i := Informers{
		PipelineRun:        fakepipelineruninformer.Get(ctx),
		Pipeline:           fakepipelineinformer.Get(ctx),
		TaskRun:            faketaskruninformer.Get(ctx),
		CustomRun:          fakecustomruninformer.Get(ctx),
		Task:               faketaskinformer.Get(ctx),
		ClusterTask:        fakeclustertaskinformer.Get(ctx),
		Pod:                fakefilteredpodinformer.Get(ctx, v1.ManagedByLabelKey),
		ConfigMap:          fakeconfigmapinformer.Get(ctx),
		ServiceAccount:     fakeserviceaccountinformer.Get(ctx),
		LimitRange:         fakelimitrangeinformer.Get(ctx),
		ResolutionRequest:  fakeresolutionrequestinformer.Get(ctx),
		VerificationPolicy: fakeverificationpolicyinformer.Get(ctx),
	}

	// Attach reactors that add resource mutations to the appropriate
	// informer index, and simulate optimistic concurrency failures when
	// the resource version is mismatched.
	c.Pipeline.PrependReactor("*", "pipelineruns", AddToInformer(t, i.PipelineRun.Informer().GetIndexer()))
	for _, pr := range d.PipelineRuns {
		pr := pr.DeepCopy() // Avoid assumptions that the informer's copy is modified.
		if _, err := c.Pipeline.TektonV1().PipelineRuns(pr.Namespace).Create(ctx, pr, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	c.Pipeline.PrependReactor("*", "pipelines", AddToInformer(t, i.Pipeline.Informer().GetIndexer()))
	for _, p := range d.Pipelines {
		p := p.DeepCopy() // Avoid assumptions that the informer's copy is modified.
		if _, err := c.Pipeline.TektonV1().Pipelines(p.Namespace).Create(ctx, p, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	c.Pipeline.PrependReactor("*", "taskruns", AddToInformer(t, i.TaskRun.Informer().GetIndexer()))
	for _, tr := range d.TaskRuns {
		tr := tr.DeepCopy() // Avoid assumptions that the informer's copy is modified.
		if _, err := c.Pipeline.TektonV1().TaskRuns(tr.Namespace).Create(ctx, tr, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	c.Pipeline.PrependReactor("*", "tasks", AddToInformer(t, i.Task.Informer().GetIndexer()))
	for _, ta := range d.Tasks {
		ta := ta.DeepCopy() // Avoid assumptions that the informer's copy is modified.
		if _, err := c.Pipeline.TektonV1().Tasks(ta.Namespace).Create(ctx, ta, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	c.Pipeline.PrependReactor("*", "clustertasks", AddToInformer(t, i.ClusterTask.Informer().GetIndexer()))
	for _, ct := range d.ClusterTasks {
		ct := ct.DeepCopy() // Avoid assumptions that the informer's copy is modified.
		if _, err := c.Pipeline.TektonV1beta1().ClusterTasks().Create(ctx, ct, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	c.Pipeline.PrependReactor("*", "customruns", AddToInformer(t, i.CustomRun.Informer().GetIndexer()))
	for _, customRun := range d.CustomRuns {
		customRun := customRun.DeepCopy() // Avoid assumptions that the informer's copy is modified.
		if _, err := c.Pipeline.TektonV1beta1().CustomRuns(customRun.Namespace).Create(ctx, customRun, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	c.Kube.PrependReactor("*", "pods", AddToInformer(t, i.Pod.Informer().GetIndexer()))
	for _, p := range d.Pods {
		p := p.DeepCopy() // Avoid assumptions that the informer's copy is modified.
		if _, err := c.Kube.CoreV1().Pods(p.Namespace).Create(ctx, p, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	for _, n := range d.Namespaces {
		n := n.DeepCopy() // Avoid assumptions that the informer's copy is modified.
		if _, err := c.Kube.CoreV1().Namespaces().Create(ctx, n, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	c.Kube.PrependReactor("*", "configmaps", AddToInformer(t, i.ConfigMap.Informer().GetIndexer()))
	for _, cm := range d.ConfigMaps {
		cm := cm.DeepCopy() // Avoid assumptions that the informer's copy is modified.
		if _, err := c.Kube.CoreV1().ConfigMaps(cm.Namespace).Create(ctx, cm, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	c.Kube.PrependReactor("*", "serviceaccounts", AddToInformer(t, i.ServiceAccount.Informer().GetIndexer()))
	for _, sa := range d.ServiceAccounts {
		sa := sa.DeepCopy() // Avoid assumptions that the informer's copy is modified.
		if _, err := c.Kube.CoreV1().ServiceAccounts(sa.Namespace).Create(ctx, sa, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	c.ResolutionRequests.PrependReactor("*", "resolutionrequests", AddToInformer(t, i.ResolutionRequest.Informer().GetIndexer()))
	for _, rr := range d.ResolutionRequests {
		rr := rr.DeepCopy() // Avoid assumptions that the informer's copy is modified.
		if _, err := c.ResolutionRequests.ResolutionV1beta1().ResolutionRequests(rr.Namespace).Create(ctx, rr, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}

	c.Pipeline.PrependReactor("*", "verificationpolicies", AddToInformer(t, i.VerificationPolicy.Informer().GetIndexer()))
	for _, vp := range d.VerificationPolicies {
		vp := vp.DeepCopy() // Avoid assumptions that the informer's copy is modified.
		if _, err := c.Pipeline.TektonV1alpha1().VerificationPolicies(vp.Namespace).Create(ctx, vp, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	c.Pipeline.ClearActions()
	c.Kube.ClearActions()
	c.ResolutionRequests.ClearActions()
	return c, i
}

// ResourceVersionReactor is an implementation of Reactor for our tests
type ResourceVersionReactor struct {
	count int64
}

// Handles returns whether our test reactor can handle a given ktesting.Action
func (r *ResourceVersionReactor) Handles(action ktesting.Action) bool {
	body := func(o runtime.Object) bool {
		objMeta, err := meta.Accessor(o)
		if err != nil {
			return false
		}
		val := atomic.AddInt64(&r.count, 1)
		objMeta.SetResourceVersion(fmt.Sprintf("%05d", val))
		return false
	}

	switch o := action.(type) {
	case ktesting.CreateActionImpl:
		return body(o.GetObject())
	case ktesting.UpdateActionImpl:
		return body(o.GetObject())
	default:
		return false
	}
}

// React is noop-function
func (r *ResourceVersionReactor) React(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
	return false, nil, nil
}

var _ ktesting.Reactor = (*ResourceVersionReactor)(nil)

// PrependResourceVersionReactor will instrument a client-go testing Fake
// with a reactor that simulates resourceVersion changes on mutations.
// This does not work with patches.
func PrependResourceVersionReactor(f *ktesting.Fake) {
	f.ReactionChain = append([]ktesting.Reactor{&ResourceVersionReactor{}}, f.ReactionChain...)
}

// EnsureConfigurationConfigMapsExist makes sure all the configmaps exists.
func EnsureConfigurationConfigMapsExist(d *Data) {
	var defaultsExists, featureFlagsExists, metricsExists, spireconfigExists bool
	for _, cm := range d.ConfigMaps {
		if cm.Name == config.GetDefaultsConfigName() {
			defaultsExists = true
		}
		if cm.Name == config.GetFeatureFlagsConfigName() {
			featureFlagsExists = true
		}
		if cm.Name == config.GetMetricsConfigName() {
			metricsExists = true
		}
		if cm.Name == config.GetSpireConfigName() {
			spireconfigExists = true
		}
	}
	if !defaultsExists {
		d.ConfigMaps = append(d.ConfigMaps, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetDefaultsConfigName(), Namespace: system.Namespace()},
			Data:       map[string]string{},
		})
	}
	if !featureFlagsExists {
		d.ConfigMaps = append(d.ConfigMaps, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.Namespace()},
			Data:       map[string]string{},
		})
	}
	if !metricsExists {
		d.ConfigMaps = append(d.ConfigMaps, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetMetricsConfigName(), Namespace: system.Namespace()},
			Data:       map[string]string{},
		})
	}
	if !spireconfigExists {
		d.ConfigMaps = append(d.ConfigMaps, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetSpireConfigName(), Namespace: system.Namespace()},
			Data:       map[string]string{},
		})
	}
}
