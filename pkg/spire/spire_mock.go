/*
Copyright 2022 The Tekton Authors

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
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/result"
	spireconfig "github.com/tektoncd/pipeline/pkg/spire/config"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"knative.dev/pkg/injection"
)

func init() {
	injection.Fake.RegisterClient(withFakeControllerClient)
}

func withFakeControllerClient(ctx context.Context, cfg *rest.Config) context.Context {
	return context.WithValue(ctx, controllerKey{}, &spireControllerAPIClient{})
}

// MockClient is a client used for mocking the this package for unit testing
// other tekton components that use the spire entrypointer or controller client.
//
// The MockClient implements both SpireControllerApiClient and SpireEntrypointerApiClient
// and in addition to that provides the helper functions to define and query internal state.
type MockClient struct {
	// Entries is a dictionary of entries that mock the SPIRE server datastore (for function Sign only)
	Entries map[string]bool

	// SignIdentities represents the list of identities to use to sign (providing context of a caller to Sign)
	// when Sign is called, the identity is dequeued from the slice. A signature will only be provided if the
	// corresponding entry is in Entries. This only takes effect if SignOverride is nil.
	SignIdentities []string

	// VerifyAlwaysReturns defines whether to always verify successfully or to always fail verification if non-nil.
	// This only take effect on Verify functions:
	// - VerifyStatusInternalAnnotationOverride
	// - VerifyTaskRunResultsOverride
	VerifyAlwaysReturns *bool

	// VerifyStatusInternalAnnotationOverride contains the function to overwrite a call to VerifyStatusInternalAnnotation
	VerifyStatusInternalAnnotationOverride func(ctx context.Context, tr *v1beta1.TaskRun, logger *zap.SugaredLogger) error

	// VerifyTaskRunResultsOverride contains the function to overwrite a call to VerifyTaskRunResults
	VerifyTaskRunResultsOverride func(ctx context.Context, prs []result.RunResult, tr *v1beta1.TaskRun) error

	// AppendStatusInternalAnnotationOverride  contains the function to overwrite a call to AppendStatusInternalAnnotation
	AppendStatusInternalAnnotationOverride func(ctx context.Context, tr *v1beta1.TaskRun) error

	// CheckSpireVerifiedFlagOverride contains the function to overwrite a call to CheckSpireVerifiedFlag
	CheckSpireVerifiedFlagOverride func(tr *v1beta1.TaskRun) bool

	// SignOverride contains the function to overwrite a call to Sign
	SignOverride func(ctx context.Context, results []result.RunResult) ([]result.RunResult, error)
}

var _ ControllerAPIClient = (*MockClient)(nil)
var _ EntrypointerAPIClient = (*MockClient)(nil)

const controllerSvid = "CONTROLLER_SVID_DATA"

func (*MockClient) mockSign(content, signedBy string) string {
	return fmt.Sprintf("signed-by-%s:%x", signedBy, sha256.Sum256([]byte(content)))
}

func (sc *MockClient) mockVerify(content, sig, signedBy string) bool {
	return sig == sc.mockSign(content, signedBy)
}

// GetIdentity get the taskrun namespace and taskrun name that is used for signing and verifying in mocked spire
func (*MockClient) GetIdentity(tr *v1beta1.TaskRun) string {
	return fmt.Sprintf("/ns/%v/taskrun/%v", tr.Namespace, tr.Name)
}

// AppendStatusInternalAnnotation creates the status annotations which are used by the controller to verify the status hash
func (sc *MockClient) AppendStatusInternalAnnotation(ctx context.Context, tr *v1beta1.TaskRun) error {
	if sc.AppendStatusInternalAnnotationOverride != nil {
		return sc.AppendStatusInternalAnnotationOverride(ctx, tr)
	}
	// Add status hash
	currentHash, err := hashTaskrunStatusInternal(tr)
	if err != nil {
		return err
	}

	if tr.Status.Annotations == nil {
		tr.Status.Annotations = map[string]string{}
	}
	tr.Status.Annotations[controllerSvidAnnotation] = controllerSvid
	tr.Status.Annotations[TaskRunStatusHashAnnotation] = currentHash
	tr.Status.Annotations[taskRunStatusHashSigAnnotation] = sc.mockSign(currentHash, "controller")
	return nil
}

// CheckSpireVerifiedFlag checks if the verified status annotation is set which would result in spire verification failed
func (sc *MockClient) CheckSpireVerifiedFlag(tr *v1beta1.TaskRun) bool {
	if sc.CheckSpireVerifiedFlagOverride != nil {
		return sc.CheckSpireVerifiedFlagOverride(tr)
	}

	_, ok := tr.Status.Annotations[VerifiedAnnotation]
	return !ok
}

// CreateEntries adds entries to the dictionary of entries that mock the SPIRE server datastore
func (sc *MockClient) CreateEntries(ctx context.Context, tr *v1beta1.TaskRun, pod *corev1.Pod, ttl time.Duration) error {
	id := fmt.Sprintf("/ns/%v/taskrun/%v", tr.Namespace, tr.Name)
	if sc.Entries == nil {
		sc.Entries = map[string]bool{}
	}
	sc.Entries[id] = true
	return nil
}

// DeleteEntry removes the entry from the dictionary of entries that mock the SPIRE server datastore
func (sc *MockClient) DeleteEntry(ctx context.Context, tr *v1beta1.TaskRun, pod *corev1.Pod) error {
	id := fmt.Sprintf("/ns/%v/taskrun/%v", tr.Namespace, tr.Name)
	if sc.Entries != nil {
		delete(sc.Entries, id)
	}
	return nil
}

// VerifyStatusInternalAnnotation checks that the internal status annotations are valid by the mocked spire client
func (sc *MockClient) VerifyStatusInternalAnnotation(ctx context.Context, tr *v1beta1.TaskRun, logger *zap.SugaredLogger) error {
	if sc.VerifyStatusInternalAnnotationOverride != nil {
		return sc.VerifyStatusInternalAnnotationOverride(ctx, tr, logger)
	}

	if sc.VerifyAlwaysReturns != nil {
		if *sc.VerifyAlwaysReturns {
			return nil
		}
		return errors.New("failed to verify from mock VerifyAlwaysReturns")
	}

	if !sc.CheckSpireVerifiedFlag(tr) {
		return errors.New("annotation tekton.dev/not-verified = yes failed spire verification")
	}

	annotations := tr.Status.Annotations

	// Verify annotations are there
	if annotations[controllerSvidAnnotation] != controllerSvid {
		return errors.New("svid annotation missing")
	}

	// Check signature
	currentHash, err := hashTaskrunStatusInternal(tr)
	if err != nil {
		return err
	}
	if !sc.mockVerify(currentHash, annotations[taskRunStatusHashSigAnnotation], "controller") {
		return errors.New("signature was not able to be verified")
	}

	// check current status hash vs annotation status hash by controller
	return CheckStatusInternalAnnotation(tr)
}

// VerifyTaskRunResults checks that all the TaskRun results are valid by the mocked spire client
func (sc *MockClient) VerifyTaskRunResults(ctx context.Context, prs []result.RunResult, tr *v1beta1.TaskRun) error {
	if sc.VerifyTaskRunResultsOverride != nil {
		return sc.VerifyTaskRunResultsOverride(ctx, prs, tr)
	}

	if sc.VerifyAlwaysReturns != nil {
		if *sc.VerifyAlwaysReturns {
			return nil
		}
		return errors.New("failed to verify from mock VerifyAlwaysReturns")
	}

	resultMap := map[string]result.RunResult{}
	for _, r := range prs {
		if r.ResultType == result.TaskRunResultType {
			resultMap[r.Key] = r
		}
	}

	var identity string
	// Get SVID identity
	for k, p := range resultMap {
		if k == KeySVID {
			identity = p.Value
			break
		}
	}

	// Verify manifest
	if err := verifyManifest(resultMap); err != nil {
		return err
	}

	if identity != sc.GetIdentity(tr) {
		return errors.New("mock identity did not match")
	}

	for key, r := range resultMap {
		if strings.HasSuffix(key, KeySignatureSuffix) {
			continue
		}
		if key == KeySVID {
			continue
		}

		sigEntry, ok := resultMap[key+KeySignatureSuffix]
		sigValue, err := getResultValue(sigEntry)
		if err != nil {
			return err
		}
		resultValue, err := getResultValue(r)
		if err != nil {
			return err
		}
		if !ok || !sc.mockVerify(resultValue, sigValue, identity) {
			return errors.Errorf("failed to verify field: %v", key)
		}
	}

	return nil
}

// Sign signs and appends signatures to the RunResult based on the mocked spire client
func (sc *MockClient) Sign(ctx context.Context, results []result.RunResult) ([]result.RunResult, error) {
	if sc.SignOverride != nil {
		return sc.SignOverride(ctx, results)
	}

	if len(sc.SignIdentities) == 0 {
		return nil, errors.New("signIdentities empty, please provide identities to sign with the MockClient.GetIdentity function")
	}

	identity := sc.SignIdentities[0]
	sc.SignIdentities = sc.SignIdentities[1:]

	if !sc.Entries[identity] {
		return nil, errors.Errorf("entry doesn't exist for identity: %v", identity)
	}

	output := []result.RunResult{}
	output = append(output, result.RunResult{
		Key:        KeySVID,
		Value:      identity,
		ResultType: result.TaskRunResultType,
	})

	for _, r := range results {
		if r.ResultType == result.TaskRunResultType {
			resultValue, err := getResultValue(r)
			if err != nil {
				return nil, err
			}
			s := sc.mockSign(resultValue, identity)
			output = append(output, result.RunResult{
				Key:        r.Key + KeySignatureSuffix,
				Value:      s,
				ResultType: result.TaskRunResultType,
			})
		}
	}
	// get complete manifest of keys such that it can be verified
	manifest := getManifest(results)
	output = append(output, result.RunResult{
		Key:        KeyResultManifest,
		Value:      manifest,
		ResultType: result.TaskRunResultType,
	})
	manifestSig := sc.mockSign(manifest, identity)
	output = append(output, result.RunResult{
		Key:        KeyResultManifest + KeySignatureSuffix,
		Value:      manifestSig,
		ResultType: result.TaskRunResultType,
	})

	return output, nil
}

// Close mock closing the spire client connection
func (*MockClient) Close() error { return nil }

// SetConfig sets the spire configuration for MockClient
func (*MockClient) SetConfig(spireconfig.SpireConfig) {}
