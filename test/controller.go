/*
Copyright 2018 The Knative Authors.

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
	"reflect"
	"testing"
	"time"

	"golang.org/x/net/context"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	controllerClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	timeout  = time.Second * 5
	interval = time.Millisecond * 10
)

// WaitForReconcile will wait for the reconcile request to be received on channel c or for a timeout to occur.
// If the request it recieved it will assert that the request is the expectedRequest.
func WaitForReconcile(t *testing.T, c chan reconcile.Request, expectedRequest reconcile.Request) {
	t.Helper()

	select {
	case request := <-c:
		if !reflect.DeepEqual(request, expectedRequest) {
			t.Errorf("Reconciled result was different from expected. Reconciled: %v. Expected: %v.", request, expectedRequest)
		}
	case <-time.After(timeout):
		t.Fatalf("Timed out waiting for reconcile loop to finish.")
	}
}

// PollDeployment will request the deployment referred to by depkey from the client and return it, or timeout.
func PollDeployment(t *testing.T, client controllerClient.Client, depKey types.NamespacedName) *appsv1.Deployment {
	t.Helper()

	deploy := &appsv1.Deployment{}
	c := context.Background()
	err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		err := client.Get(c, depKey, deploy)
		if err != nil {
			// We assume an error means this resource hasn't become ready yet and we should retry.
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		t.Fatalf("The deployment for the resource %s didn't become available: %s", depKey, err)
	}
	return deploy
}

// SetupTestReconcile returns a reconcile.Reconcile implementation that delegates to inner and
// writes the request to requests after Reconcile is finished.
func SetupTestReconcile(inner reconcile.Reconciler) (reconcile.Reconciler, chan reconcile.Request) {
	requests := make(chan reconcile.Request)
	fn := reconcile.Func(func(req reconcile.Request) (reconcile.Result, error) {
		result, err := inner.Reconcile(req)
		requests <- req
		return result, err
	})
	return fn, requests
}

// StartTestManager adds recFn to mgr.
func StartTestManager(t *testing.T, mgr manager.Manager) chan struct{} {
	stop := make(chan struct{})
	go func() {
		err := mgr.Start(stop)
		if err != nil {
			t.Fatalf("Failed to stop test manager: %s", err)
		}
	}()
	return stop
}
