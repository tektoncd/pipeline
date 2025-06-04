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

package volumeclaim

import (
	"context"
	"fmt"
	"testing"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	fakek8s "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/typed/core/v1/fake"
	client_go_testing "k8s.io/client-go/testing"
)

const (
	actionCreate = "create"
	actionGet    = "get"
)

// check that defaultPVCHandler implements PvcHandler
var _ PvcHandler = (*defaultPVCHandler)(nil)

// TestCreatePersistentVolumeClaimsForWorkspaces tests that given a TaskRun with volumeClaimTemplate workspace,
// a PVC is created, with the expected name and that it has the expected OwnerReference.
func TestCreatePersistentVolumeClaimsForWorkspaces(t *testing.T) {
	// given

	// 2 workspaces with volumeClaimTemplate
	claimName1 := "pvc1"
	ws1 := "myws1"
	ownerName := "taskrun1"
	workspaces := []v1.WorkspaceBinding{{
		Name: ws1,
		VolumeClaimTemplate: &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name: claimName1,
			},
			Spec: corev1.PersistentVolumeClaimSpec{},
		},
	}, {
		Name: "bring-my-own-pvc",
		PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
			ClaimName: "myown",
		},
	}, {
		Name: "myws2",
		VolumeClaimTemplate: &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pvc2",
			},
			Spec: corev1.PersistentVolumeClaimSpec{},
		},
	}}
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ownerRef := metav1.OwnerReference{UID: types.UID(ownerName)}
	namespace := "ns"
	fakekubeclient := fakek8s.NewSimpleClientset()
	pvcHandler := defaultPVCHandler{fakekubeclient, zap.NewExample().Sugar()}

	// when

	for _, ws := range workspaces {
		err := pvcHandler.CreatePVCFromVolumeClaimTemplate(ctx, ws, ownerRef, namespace)
		if err != nil {
			t.Fatalf("unexpexted error: %v", err)
		}
	}

	expectedPVCName := claimName1 + "-ad02547921"
	pvc, err := fakekubeclient.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, expectedPVCName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	createActions := 0
	for _, action := range fakekubeclient.Fake.Actions() {
		if actionCreate == action.GetVerb() {
			createActions++
		}
	}

	// that

	expectedNumberOfCreateActions := 2
	if createActions != expectedNumberOfCreateActions {
		t.Fatalf("unexpected numer of 'create' PVC actions; expected: %d got: %d", expectedNumberOfCreateActions, createActions)
	}

	if pvc.Name != expectedPVCName {
		t.Fatalf("unexpected PVC name on created PVC; exptected: %s got: %s", expectedPVCName, pvc.Name)
	}

	expectedNumberOfOwnerRefs := 1
	if len(pvc.OwnerReferences) != expectedNumberOfOwnerRefs {
		t.Fatalf("unexpected number of ownerreferences on created PVC; expected: %d got %d", expectedNumberOfOwnerRefs, len(pvc.OwnerReferences))
	}

	if string(pvc.OwnerReferences[0].UID) != ownerName {
		t.Fatalf("unexptected name in ownerreference on created PVC; expected: %s got %s", ownerName, pvc.OwnerReferences[0].Name)
	}
}

// TestCreatePersistentVolumeClaimsForWorkspacesWithoutMetadata tests that given a volumeClaimTemplate workspace
// without a metadata part, a PVC is created, with the expected name.
func TestCreatePersistentVolumeClaimsForWorkspacesWithoutMetadata(t *testing.T) {
	// given

	// workspace with volumeClaimTemplate without metadata
	workspaceName := "ws-with-volume-claim-template-without-metadata"
	ownerName := "taskrun1"
	workspaces := []v1.WorkspaceBinding{{
		Name: workspaceName,
		VolumeClaimTemplate: &corev1.PersistentVolumeClaim{
			Spec: corev1.PersistentVolumeClaimSpec{},
		},
	}}
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ownerRef := metav1.OwnerReference{UID: types.UID(ownerName)}
	namespace := "ns"
	fakekubeclient := fakek8s.NewSimpleClientset()
	pvcHandler := defaultPVCHandler{fakekubeclient, zap.NewExample().Sugar()}

	// when

	for _, ws := range workspaces {
		err := pvcHandler.CreatePVCFromVolumeClaimTemplate(ctx, ws, ownerRef, namespace)
		if err != nil {
			t.Fatalf("unexpexted error: %v", err)
		}
	}

	expectedPVCName := fmt.Sprintf("%s-%s", "pvc", "3fc56c2bb2")
	pvc, err := fakekubeclient.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, expectedPVCName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// that

	if pvc.Name != expectedPVCName {
		t.Fatalf("unexpected PVC name on created PVC; exptected: %s got: %s", expectedPVCName, pvc.Name)
	}
}

// TestCreateExistPersistentVolumeClaims tests that given a volumeClaimTemplate workspace.
// At first, a pvc could not be obtained, but when creating a pvc, it already existed.
// In this scenario, no new pvc will be created.
func TestCreateExistPersistentVolumeClaims(t *testing.T) {
	workspaceName := "ws-with-volume-claim-template"
	ownerName := "taskrun1"
	workspaces := []v1.WorkspaceBinding{{
		Name: workspaceName,
		VolumeClaimTemplate: &corev1.PersistentVolumeClaim{
			Spec: corev1.PersistentVolumeClaimSpec{},
		},
	}}
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ownerRef := metav1.OwnerReference{UID: types.UID(ownerName)}
	namespace := "ns"
	fakekubeclient := fakek8s.NewSimpleClientset()
	pvcHandler := defaultPVCHandler{fakekubeclient, zap.NewExample().Sugar()}

	for _, ws := range workspaces {
		claim := pvcHandler.getPVCFromVolumeClaimTemplate(ws, ownerRef, namespace)
		if claim == nil {
			t.Fatalf("expect PVC but got nil from workspace: %v", ws.Name)
		}
		_, err := fakekubeclient.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, claim, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	// A new ReactionFunc implementation.
	// This function is called when a get action occurs and returns a not found error
	fn := func(action client_go_testing.Action) (bool, runtime.Object, error) {
		switch action := action.(type) {
		case client_go_testing.GetActionImpl:
			return true, nil, errors.NewNotFound(schema.GroupResource{}, action.Name)
		default:
			return false, nil, fmt.Errorf("no reaction implemented for %s", action)
		}
	}
	fakekubeclient.Fake.PrependReactor(actionGet, "*", fn)

	for _, ws := range workspaces {
		err := pvcHandler.CreatePVCFromVolumeClaimTemplate(ctx, ws, ownerRef, namespace)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if len(fakekubeclient.Fake.Actions()) != 3 {
		t.Fatalf("unexpected numer of actions; expected: %d got: %d", 3, len(fakekubeclient.Fake.Actions()))
	}

	actions := []string{actionCreate, actionGet, actionCreate}
	for i, action := range fakekubeclient.Fake.Actions() {
		if actions[i] != action.GetVerb() {
			t.Fatalf("PVC action, expected: %s got: %s", actions[i], action.GetVerb())
		}
	}

	expectedPVCName := fmt.Sprintf("%s-%s", "pvc", "5435cf73f0")
	pvcList, err := fakekubeclient.CoreV1().PersistentVolumeClaims(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pvcList.Items) != 1 {
		t.Fatalf("unexpected number of created PVC; expected: %d got %d", 1, len(pvcList.Items))
	}
	if pvcList.Items[0].Name != expectedPVCName {
		t.Fatalf("unexpected PVC name on created PVC; expected: %s got: %s", expectedPVCName, pvcList.Items[0].Name)
	}
}

func TestPurgeFinalizerAndDeletePVCForWorkspace(t *testing.T) {
	ctx := context.Background()
	kubeClientSet := fakek8s.NewSimpleClientset()

	// seed data
	namespace := "my-ns"
	pvcName := "my-pvc"
	nonExistingPvcName := "non-existing-pvc"
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: namespace,
			Finalizers: []string{
				"kubernetes.io/pvc-protection",
			},
		}}
	if _, err := kubeClientSet.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, pvc, metav1.CreateOptions{}); err != nil {
		t.Fatalf("unexpected error when creating PVC: %v", err)
	}

	// mocks `kubernetes.io/pvc-protection` finalizer behavior by adding DeletionTimestamp when deleting pvcs with the finalizer
	// see details in: https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers/#how-finalizers-work
	pvc.DeletionTimestamp = &metav1.Time{
		Time: metav1.Now().Time,
	}
	kubeClientSet.CoreV1().(*fake.FakeCoreV1).PrependReactor("delete", "persistentvolumeclaims",
		func(action client_go_testing.Action) (handled bool, ret runtime.Object, err error) {
			return true, pvc, nil
		})

	// call PurgeFinalizerAndDeletePVCForWorkspace to delete pvc
	// note that the pvcs are not actually deleted in the unit test due to the mock limitation of fakek8s.NewSimpleClientset();
	// full pvc lifecycle is tested in TestAffinityAssistant_PerPipelineRun integration test
	pvcHandler := defaultPVCHandler{kubeClientSet, zap.NewExample().Sugar()}
	if err := pvcHandler.PurgeFinalizerAndDeletePVCForWorkspace(ctx, pvcName, namespace); err != nil {
		t.Fatalf("unexpected error when calling PurgeFinalizerAndDeletePVCForWorkspace: %v", err)
	}
	// check that pvc deletion is skipped when the pvc does not exist
	if err := pvcHandler.PurgeFinalizerAndDeletePVCForWorkspace(ctx, nonExistingPvcName, namespace); err != nil {
		t.Fatalf("the deletion of non existing pvc %s was not skipped; an unexpected error occurred: %v", nonExistingPvcName, err)
	}

	// validate the `kubernetes.io/pvc-protection` finalizer is removed
	pvc, err := kubeClientSet.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvcName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pvc.Finalizers) > 0 {
		t.Errorf("pvc %s kubernetes.io/pvc-protection finalizer is not removed properly", pvcName)
	}
}
