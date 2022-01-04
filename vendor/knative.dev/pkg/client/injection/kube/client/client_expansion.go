/*
Copyright 2021 The Knative Authors

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

package client

import (
	context "context"
	fmt "fmt"

	certificatesv1beta1 "k8s.io/api/certificates/v1beta1"
	corev1 "k8s.io/api/core/v1"
	eventsv1beta1 "k8s.io/api/events/v1beta1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	policyv1 "k8s.io/api/policy/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	restclient "k8s.io/client-go/rest"
)

func (*wrapCoreV1NamespaceImpl) Finalize(context.Context, *corev1.Namespace, metav1.UpdateOptions) (*corev1.Namespace, error) {
	panic("NYI")
}

func (*wrapCoreV1ServiceImpl) ProxyGet(string, string, string, string, map[string]string) restclient.ResponseWrapper {
	panic("NYI")
}

func (e *wrapEventsV1beta1EventImpl) CreateWithEventNamespace(event *eventsv1beta1.Event) (*eventsv1beta1.Event, error) {
	if e.namespace != "" && event.Namespace != e.namespace {
		return nil, fmt.Errorf("can't create an event with namespace '%v' in namespace '%v'", event.Namespace, e.namespace)
	}
	ne := &wrapEventsV1beta1EventImpl{
		dyn:       e.dyn,
		namespace: event.Namespace,
	}
	return ne.Create(context.TODO(), event, metav1.CreateOptions{})
}

func (e *wrapEventsV1beta1EventImpl) UpdateWithEventNamespace(event *eventsv1beta1.Event) (*eventsv1beta1.Event, error) {
	if e.namespace != "" && event.Namespace != e.namespace {
		return nil, fmt.Errorf("can't update an event with namespace '%v' in namespace '%v'", event.Namespace, e.namespace)
	}
	ne := &wrapEventsV1beta1EventImpl{
		dyn:       e.dyn,
		namespace: event.Namespace,
	}
	return ne.Update(context.TODO(), event, metav1.UpdateOptions{})
}

func (e *wrapEventsV1beta1EventImpl) PatchWithEventNamespace(event *eventsv1beta1.Event, bytes []byte) (*eventsv1beta1.Event, error) {
	if e.namespace != "" && event.Namespace != e.namespace {
		return nil, fmt.Errorf("can't patch an event with namespace '%v' in namespace '%v'", event.Namespace, e.namespace)
	}
	ne := &wrapEventsV1beta1EventImpl{
		dyn:       e.dyn,
		namespace: event.Namespace,
	}
	return ne.Patch(context.TODO(), event.Name, types.StrategicMergePatchType, bytes, metav1.PatchOptions{})
}

func (e *wrapCoreV1EventImpl) CreateWithEventNamespace(event *corev1.Event) (*corev1.Event, error) {
	if e.namespace != "" && event.Namespace != e.namespace {
		return nil, fmt.Errorf("can't create an event with namespace '%v' in namespace '%v'", event.Namespace, e.namespace)
	}
	ne := &wrapCoreV1EventImpl{
		dyn:       e.dyn,
		namespace: event.Namespace,
	}
	return ne.Create(context.TODO(), event, metav1.CreateOptions{})
}

func (e *wrapCoreV1EventImpl) UpdateWithEventNamespace(event *corev1.Event) (*corev1.Event, error) {
	if e.namespace != "" && event.Namespace != e.namespace {
		return nil, fmt.Errorf("can't update an event with namespace '%v' in namespace '%v'", event.Namespace, e.namespace)
	}
	ne := &wrapCoreV1EventImpl{
		dyn:       e.dyn,
		namespace: event.Namespace,
	}
	return ne.Update(context.TODO(), event, metav1.UpdateOptions{})
}

func (e *wrapCoreV1EventImpl) PatchWithEventNamespace(event *corev1.Event, bytes []byte) (*corev1.Event, error) {
	if e.namespace != "" && event.Namespace != e.namespace {
		return nil, fmt.Errorf("can't patch an event with namespace '%v' in namespace '%v'", event.Namespace, e.namespace)
	}
	ne := &wrapCoreV1EventImpl{
		dyn:       e.dyn,
		namespace: event.Namespace,
	}
	return ne.Patch(context.TODO(), event.Name, types.StrategicMergePatchType, bytes, metav1.PatchOptions{})
}

func (*wrapCoreV1EventImpl) Search(*runtime.Scheme, runtime.Object) (*corev1.EventList, error) {
	panic("NYI")
}

func (*wrapCoreV1EventImpl) GetFieldSelector(*string, *string, *string, *string) fields.Selector {
	panic("NYI")
}

func (*wrapCoreV1NodeImpl) PatchStatus(context.Context, string, []byte) (*corev1.Node, error) {
	panic("NYI")
}

func (*wrapCoreV1PodImpl) Bind(context.Context, *corev1.Binding, metav1.CreateOptions) error {
	panic("NYI")
}

func (*wrapCoreV1PodImpl) Evict(context.Context, *policyv1beta1.Eviction) error {
	panic("NYI")
}

func (*wrapCoreV1PodImpl) EvictV1(context.Context, *policyv1.Eviction) error {
	panic("NYI")
}

func (*wrapCoreV1PodImpl) EvictV1beta1(context.Context, *policyv1beta1.Eviction) error {
	panic("NYI")
}

func (c *wrapPolicyV1EvictionImpl) Evict(ctx context.Context, eviction *policyv1.Eviction) error {
	panic("NYI")
}

func (*wrapCoreV1PodImpl) GetLogs(string, *corev1.PodLogOptions) *restclient.Request {
	panic("NYI")
}

func (*wrapCoreV1PodImpl) ProxyGet(string, string, string, string, map[string]string) restclient.ResponseWrapper {
	panic("NYI")
}

func (*wrapExtensionsV1beta1DeploymentImpl) Rollback(context.Context, *extensionsv1beta1.DeploymentRollback, metav1.CreateOptions) error {
	panic("NYI")
}

func (*wrapPolicyV1beta1EvictionImpl) Evict(context.Context, *policyv1beta1.Eviction) error {
	panic("NYI")
}

func (*wrapCertificatesV1beta1CertificateSigningRequestImpl) UpdateApproval(context.Context, *certificatesv1beta1.CertificateSigningRequest, metav1.UpdateOptions) (*certificatesv1beta1.CertificateSigningRequest, error) {
	panic("NYI")
}
