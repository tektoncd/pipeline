package v1alpha1

import (
	"fmt"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/controller"
	"knative.dev/pkg/apis"
)

func (tc *ExpirationController) AddPipelineRun(obj interface{}) {
	pr := obj.(*PipelineRun)
	klog.V(4).Infof("Adding PipelineRun %s/%s", pr.Namespace, pr.Name)

	if pr.DeletionTimestamp == nil && pipelineRunCleanup(pr) {
		tc.PrEnqueue(pr)
	}
}

func (tc *ExpirationController) UpdatePipelineRun(old, cur interface{}) {
	pr := cur.(*PipelineRun)
	klog.V(4).Infof("Updating PipelineRun %s/%s", pr.Namespace, pr.Name)

	if pr.DeletionTimestamp == nil && pipelineRunCleanup(pr) {
		tc.PrEnqueue(pr)
	}
}

func (tc *ExpirationController) PrEnqueue(pr *PipelineRun) {
	klog.V(4).Infof("Add PipelineRun %s/%s to cleanup", pr.Namespace, pr.Name)
	key, err := controller.KeyFunc(pr)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", pr, err))
		return
	}

	tc.queue.Add(key)
}

func (tc *ExpirationController) PrEnqueueAfter(pr *PipelineRun, after time.Duration) {
	key, err := controller.KeyFunc(pr)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", pr, err))
		return
	}

	tc.queue.AddAfter(key, after)
}

// processPipelineRun will check the PipelineRun's state and TTL and delete the PipelineRun when it
// finishes and its TTL after succeeded has expired. If the PipelineRun hasn't succeeded or
// its TTL hasn't expired, it will be added to the queue after the TTL is expected
// to expire.
// This function is not meant to be invoked concurrently with the same key.
func (tc *ExpirationController) processPipelineRun(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	klog.V(4).Infof("Checking if PipelineRun %s/%s is ready for cleanup", namespace, name)
	// Ignore the PipelineRuns that are already deleted or being deleted, or the ones that don't need clean up.
	pr, err := tc.prLister.PipelineRuns(namespace).Get(name)
	if errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	if expired, err := tc.processPrTTL(pr); err != nil {
		return err
	} else if !expired {
		return nil
	}

	// The PipelineRun's TTL is assumed to have expired, but the PipelineRun TTL might be stale.
	// Before deleting the PipelineRun, do a final sanity check.
	// If TTL is modified before we do this check, we cannot be sure if the TTL truly expires.
	// The latest PipelineRun may have a different UID, but it's fine because the checks will be run again.
	fresh, err := tc.client.TektonV1alpha1().PipelineRuns(namespace).Get(name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	// Use the latest PipelineRun TTL to see if the TTL truly expires.
	if expired, err := tc.processPrTTL(fresh); err != nil {
		return err
	} else if !expired {
		return nil
	}
	// Cascade deletes the PipelineRuns if TTL truly expires.
	policy := metav1.DeletePropagationForeground
	options := &metav1.DeleteOptions{
		PropagationPolicy: &policy,
		Preconditions:     &metav1.Preconditions{UID: &fresh.UID},
	}
	klog.V(4).Infof("Cleaning up PipelineRun %s/%s", namespace, name)
	return tc.client.TektonV1alpha1().PipelineRuns(fresh.Namespace).Delete(fresh.Name, options)
}

// processTTL checks whether a given PipelineRun's TTL has expired, and add it to the queue after the TTL is expected to expire
// if the TTL will expire later.
func (tc *ExpirationController) processPrTTL(pr *PipelineRun) (expired bool, err error) {
	// We don't care about the PipelineRuns that are going to be deleted, or the ones that don't need clean up.
	if pr.DeletionTimestamp != nil || !pipelineRunCleanup(pr) {
		return false, nil
	}

	now := tc.clock.Now()
	t, err := prTimeLeft(pr, &now)
	if err != nil {
		return false, err
	}

	// TTL has expired
	if *t <= 0 {
		return true, nil
	}

	tc.PrEnqueueAfter(pr, *t)
	return false, nil
}

func IsPipelineRunSucceeded(pr *PipelineRun) bool {
	for _, con := range pr.Status.Conditions {
		if con.Type == apis.ConditionSucceeded && con.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

func getPrFinishAndExpireTime(pr *PipelineRun) (*time.Time, *time.Time, error) {
	if !pipelineRunCleanup(pr) {
		return nil, nil, fmt.Errorf("PipelineRun %s/%s should not be cleaned up", pr.Namespace, pr.Name)
	}
	finishAt, err := PipelineRunFinishTime(pr)
	if err != nil {
		return nil, nil, err
	}
	finishAtUTC := finishAt.UTC()
	expireAtUTC := finishAtUTC.Add(pr.Spec.ExpirationSecondsTTL.Duration * time.Second)
	return &finishAtUTC, &expireAtUTC, nil
}

func prTimeLeft(pr *PipelineRun, since *time.Time) (*time.Duration, error) {
	finishAt, expireAt, err := getPrFinishAndExpireTime(pr)
	if err != nil {
		return nil, err
	}
	if finishAt.UTC().After(since.UTC()) {
		klog.Warningf("Warning: Found PipelineRun %s/%s succeeded in the future. This is likely due to time skew in the cluster. PipelineRun cleanup will be deferred.", pr.Namespace, pr.Name)
	}
	remaining := expireAt.UTC().Sub(since.UTC())
	klog.V(4).Infof("Found PipelineRun %s/%s succeeded at %v, remaining TTL %v since %v, TTL will expire at %v", pr.Namespace, pr.Name, finishAt.UTC(), remaining, since.UTC(), expireAt.UTC())
	return &remaining, nil
}

// PipelineRunFinishTime takes an already succeeded PipelineRun and returns the time it finishes.
func PipelineRunFinishTime(pr *PipelineRun) (metav1.Time, error) {
	for _, con := range pr.Status.Conditions {
		if con.Type == apis.ConditionSucceeded && con.Status == v1.ConditionTrue {
			finishAt := con.LastTransitionTime
			if finishAt.Inner.IsZero() {
				return metav1.Time{}, fmt.Errorf("unable to find the time when the PipelineRun %s/%s succeeded", pr.Namespace, pr.Name)
			}
			return con.LastTransitionTime.Inner, nil
		}
	}

	// This should never happen if the PipelineRuns has succeeded
	return metav1.Time{}, fmt.Errorf("unable to find the status of the succeeded PipelineRun %s/%s", pr.Namespace, pr.Name)
}

// pipelineRunCleanup checks whether a PipelineRun or PipelineRun has succeeded and has a TTL set.
func pipelineRunCleanup(pr *PipelineRun) bool {
	return pr.Spec.ExpirationSecondsTTL != nil && IsPipelineRunSucceeded(pr)
}
