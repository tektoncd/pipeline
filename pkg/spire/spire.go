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

package spire

import (
	"context"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
)

const (
	// TaskRunStatusHashAnnotation TaskRun status annotation Hash Key
	TaskRunStatusHashAnnotation = "tekton.dev/status-hash"
	// taskRunStatusHashSigAnnotation TaskRun status annotation hash signature Key
	taskRunStatusHashSigAnnotation = "tekton.dev/status-hash-sig"
	// controllerSvidAnnotation TaskRun status annotation controller SVID Key
	controllerSvidAnnotation = "tekton.dev/controller-svid"
	// NotVerifiedAnnotation TaskRun status annotation not verified by spire key that get set when status match fails
	NotVerifiedAnnotation = "tekton.dev/not-verified"
	// KeySVID key used by TaskRun SVID
	KeySVID = "SVID"
	// KeySignatureSuffix is the suffix of the keys that contain signatures
	KeySignatureSuffix = ".sig"
	// KeyResultManifest key used to get the result manifest from the results
	KeyResultManifest = "RESULT_MANIFEST"
)

// ControllerAPIClient interface maps to the spire controller API to interact with spire
type ControllerAPIClient interface {
	AppendStatusInternalAnnotation(ctx context.Context, tr *v1beta1.TaskRun) error
	CheckSpireVerifiedFlag(tr *v1beta1.TaskRun) bool
	Close()
	CreateEntries(ctx context.Context, tr *v1beta1.TaskRun, pod *corev1.Pod, ttl int) error
	DeleteEntry(ctx context.Context, tr *v1beta1.TaskRun, pod *corev1.Pod) error
	VerifyStatusInternalAnnotation(ctx context.Context, tr *v1beta1.TaskRun, logger *zap.SugaredLogger) error
	VerifyTaskRunResults(ctx context.Context, prs []v1beta1.PipelineResourceResult, tr *v1beta1.TaskRun) error
}

// EntrypointerAPIClient interface maps to the spire entrypointer API to interact with spire
type EntrypointerAPIClient interface {
	Close()
	// Sign returns the signature material to be put in the PipelineResourceResult to append to the output results
	Sign(ctx context.Context, results []v1beta1.PipelineResourceResult) ([]v1beta1.PipelineResourceResult, error)
}
