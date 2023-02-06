/*
 *
 * Copyright 2019 The Tekton Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 * /
 */

package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/queue/v1alpha1"
	"gomodules.xyz/jsonpatch/v2"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var (
	cancelPipelineRunPatchBytes           []byte
	gracefullyCancelPipelineRunPatchBytes []byte
	gracefullyStopPipelineRunPatchBytes   []byte
	concurrencyControlsAppliedLabel       = "tekton.dev/concurrency"
)

func init() {
	var err error
	cancelPipelineRunPatchBytes, err = json.Marshal([]jsonpatch.JsonPatchOperation{
		{
			Operation: "add",
			Path:      "/spec/status",
			Value:     v1beta1.PipelineRunSpecStatusCancelled,
		}})
	if err != nil {
		log.Fatalf("failed to marshal PipelineRun cancel patch bytes: %v", err)
	}
	gracefullyCancelPipelineRunPatchBytes, err = json.Marshal([]jsonpatch.JsonPatchOperation{
		{
			Operation: "add",
			Path:      "/spec/status",
			Value:     v1beta1.PipelineRunSpecStatusCancelledRunFinally,
		}})
	if err != nil {
		log.Fatalf("failed to marshal PipelineRun gracefully cancel patch bytes: %v", err)
	}
	gracefullyStopPipelineRunPatchBytes, err = json.Marshal([]jsonpatch.JsonPatchOperation{
		{
			Operation: "add",
			Path:      "/spec/status",
			Value:     v1beta1.PipelineRunSpecStatusStoppedRunFinally,
		}})
	if err != nil {
		log.Fatalf("failed to marshal PipelineRun gracefully stop patch bytes: %v", err)
	}
}

// TODO: cancelPipelineRun is copied from cancel.go
// (see https://github.com/tektoncd/pipeline/blob/main/pkg/reconciler/pipelinerun/cancel.go).
func (q *defaultQueue) cancelPipelineRun(ctx context.Context, namespace, name string, s v1alpha1.Strategy) error {
	var bytes []byte
	switch s {
	case v1alpha1.StrategyCancel:
		bytes = cancelPipelineRunPatchBytes
	case v1alpha1.StrategyGracefullyCancel:
		bytes = gracefullyCancelPipelineRunPatchBytes
	case v1alpha1.StrategyGracefullyStop:
		bytes = gracefullyStopPipelineRunPatchBytes
	default:
		return fmt.Errorf("unsupported operation: %s", s)
	}
	_, err := q.pipelineClientSet.TektonV1beta1().PipelineRuns(namespace).Patch(ctx, name, types.JSONPatchType, bytes, metav1.PatchOptions{})
	if errors.IsNotFound(err) {
		// The PipelineRun may have been deleted in the meantime
		return nil
	} else if err != nil {
		return fmt.Errorf("error patching PipelineRun %s using strategy %s: %s", name, s, err)
	}
	return nil
}
