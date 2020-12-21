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

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakek8s "k8s.io/client-go/kubernetes/fake"
)

const actionCreate = "create"

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
	workspaces := []v1beta1.WorkspaceBinding{{
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

	ownerRef := metav1.OwnerReference{Name: ownerName}
	namespace := "ns"
	fakekubeclient := fakek8s.NewSimpleClientset()
	pvcHandler := defaultPVCHandler{fakekubeclient, zap.NewExample().Sugar()}

	// when

	err := pvcHandler.CreatePersistentVolumeClaimsForWorkspaces(ctx, workspaces, ownerRef, namespace)
	if err != nil {
		t.Fatalf("unexpexted error: %v", err)
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

	if pvc.OwnerReferences[0].Name != ownerName {
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
	workspaces := []v1beta1.WorkspaceBinding{{
		Name: workspaceName,
		VolumeClaimTemplate: &corev1.PersistentVolumeClaim{
			Spec: corev1.PersistentVolumeClaimSpec{},
		},
	}}
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ownerRef := metav1.OwnerReference{Name: ownerName}
	namespace := "ns"
	fakekubeclient := fakek8s.NewSimpleClientset()
	pvcHandler := defaultPVCHandler{fakekubeclient, zap.NewExample().Sugar()}

	// when

	err := pvcHandler.CreatePersistentVolumeClaimsForWorkspaces(ctx, workspaces, ownerRef, namespace)
	if err != nil {
		t.Fatalf("unexpexted error: %v", err)
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
