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
	"fmt"
	"testing"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"github.com/tektoncd/pipeline/pkg/spire/config"
	"github.com/tektoncd/pipeline/pkg/spire/test"
	"github.com/tektoncd/pipeline/pkg/spire/test/fakeworkloadapi"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/logging"
)

var (
	trustDomain  = "example.org"
	td           = spiffeid.RequireTrustDomainFromString(trustDomain)
	fooID        = spiffeid.RequireFromPath(td, "/foo")
	controllerID = spiffeid.RequireFromPath(td, "/controller")
)

func TestSpire_TaskRunSign(t *testing.T) {
	ctx, _ := ttesting.SetupDefaultContext(t)

	ca := test.NewCA(t, td)
	wl := fakeworkloadapi.New(t)
	defer wl.Stop()

	wl.SetX509Bundles(ca.X509Bundle())

	resp := &fakeworkloadapi.X509SVIDResponse{
		Bundle: ca.X509Bundle(),
		SVIDs:  makeX509SVIDs(ca, controllerID),
	}
	wl.SetX509SVIDResponse(resp)

	cfg := &config.SpireConfig{}
	cfg.SocketPath = wl.Addr()
	cfg.TrustDomain = trustDomain
	spireControllerClient := GetControllerAPIClient(ctx)
	spireControllerClient.SetConfig(*cfg)

	logger := logging.FromContext(ctx)

	var (
		cc ControllerAPIClient = spireControllerClient
	)
	defer cc.Close()

	var err error

	for _, tr := range testTaskRuns() {
		err = cc.AppendStatusInternalAnnotation(ctx, tr)
		if err != nil {
			t.Fatalf("failed to sign TaskRun: %v", err)
		}

		err = cc.VerifyStatusInternalAnnotation(ctx, tr, logger)
		if err != nil {
			t.Fatalf("failed to verify TaskRun: %v", err)
		}
	}
}

func TestSpire_CheckSpireVerifiedFlag(t *testing.T) {
	ctx, _ := ttesting.SetupDefaultContext(t)

	ca := test.NewCA(t, td)
	wl := fakeworkloadapi.New(t)
	defer wl.Stop()

	wl.SetX509Bundles(ca.X509Bundle())

	resp := &fakeworkloadapi.X509SVIDResponse{
		Bundle: ca.X509Bundle(),
		SVIDs:  makeX509SVIDs(ca, controllerID),
	}
	wl.SetX509SVIDResponse(resp)

	cfg := &config.SpireConfig{}
	cfg.SocketPath = wl.Addr()
	cfg.TrustDomain = trustDomain
	spireControllerClient := GetControllerAPIClient(ctx)
	spireControllerClient.SetConfig(*cfg)

	var (
		cc ControllerAPIClient = spireControllerClient
	)
	defer cc.Close()

	trs := testTaskRuns()
	tr := trs[0]

	if !cc.CheckSpireVerifiedFlag(tr) {
		t.Fatalf("verified flag should be unset")
	}

	if tr.Status.Status.Annotations == nil {
		tr.Status.Status.Annotations = map[string]string{}
	}
	tr.Status.Status.Annotations[NotVerifiedAnnotation] = "yes"

	if cc.CheckSpireVerifiedFlag(tr) {
		t.Fatalf("verified flag should be unset")
	}
}

func TestSpire_CheckHashSimilarities(t *testing.T) {
	ctx, _ := ttesting.SetupDefaultContext(t)

	ca := test.NewCA(t, td)
	wl := fakeworkloadapi.New(t)
	defer wl.Stop()

	wl.SetX509Bundles(ca.X509Bundle())

	resp := &fakeworkloadapi.X509SVIDResponse{
		Bundle: ca.X509Bundle(),
		SVIDs:  makeX509SVIDs(ca, controllerID),
	}
	wl.SetX509SVIDResponse(resp)

	cfg := &config.SpireConfig{}
	cfg.SocketPath = wl.Addr()
	cfg.TrustDomain = trustDomain
	spireControllerClient := GetControllerAPIClient(ctx)
	spireControllerClient.SetConfig(*cfg)

	var (
		cc ControllerAPIClient = spireControllerClient
	)
	defer cc.Close()

	trs := testTaskRuns()
	tr1, tr2 := trs[0], trs[1]

	trs = testTaskRuns()
	tr1c, tr2c := trs[0], trs[1]

	tr2c.Status.Status.Annotations = map[string]string{"new": "value"}

	signTrs := []*v1beta1.TaskRun{tr1, tr1c, tr2, tr2c}

	for _, tr := range signTrs {
		err := cc.AppendStatusInternalAnnotation(ctx, tr)
		if err != nil {
			t.Fatalf("failed to sign TaskRun: %v", err)
		}
	}

	if getHash(tr1) != getHash(tr1c) {
		t.Fatalf("2 hashes of the same status should be same")
	}

	if getHash(tr1) == getHash(tr2) {
		t.Fatalf("2 hashes of different status should not be the same")
	}

	if getHash(tr2) != getHash(tr2c) {
		t.Fatalf("2 hashes of the same status should be same (ignoring Status.Status)")
	}
}

// Task run sign, modify signature/hash/svid/content and verify
func TestSpire_CheckTamper(t *testing.T) {
	ctx, _ := ttesting.SetupDefaultContext(t)

	ca := test.NewCA(t, td)
	wl := fakeworkloadapi.New(t)
	defer wl.Stop()

	wl.SetX509Bundles(ca.X509Bundle())

	resp := &fakeworkloadapi.X509SVIDResponse{
		Bundle: ca.X509Bundle(),
		SVIDs:  makeX509SVIDs(ca, controllerID),
	}
	wl.SetX509SVIDResponse(resp)

	cfg := &config.SpireConfig{}
	cfg.SocketPath = wl.Addr()
	cfg.TrustDomain = trustDomain
	spireControllerClient := GetControllerAPIClient(ctx)
	spireControllerClient.SetConfig(*cfg)

	logger := logging.FromContext(ctx)

	var (
		cc ControllerAPIClient = spireControllerClient
	)
	defer cc.Close()

	tests := []struct {
		// description of test case
		desc string
		// annotations to set
		setAnnotations map[string]string
		// modify the status
		modifyStatus bool
		// modify the hash to match the new status but not the signature
		modifyHashToMatch bool
		// if test should pass
		verify bool
	}{
		{
			desc:   "tamper nothing",
			verify: true,
		},
		{
			desc: "tamper unrelated hash",
			setAnnotations: map[string]string{
				"unrelated-hash": "change",
			},
			verify: true,
		},
		{
			desc: "tamper status hash",
			setAnnotations: map[string]string{
				TaskRunStatusHashAnnotation: "change-hash",
			},
			verify: false,
		},
		{
			desc: "tamper sig",
			setAnnotations: map[string]string{
				taskRunStatusHashSigAnnotation: "change-sig",
			},
			verify: false,
		},
		{
			desc: "tamper SVID",
			setAnnotations: map[string]string{
				controllerSvidAnnotation: "change-svid",
			},
			verify: false,
		},
		{
			desc: "delete status hash",
			setAnnotations: map[string]string{
				TaskRunStatusHashAnnotation: "",
			},
			verify: false,
		},
		{
			desc: "delete sig",
			setAnnotations: map[string]string{
				taskRunStatusHashSigAnnotation: "",
			},
			verify: false,
		},
		{
			desc: "delete SVID",
			setAnnotations: map[string]string{
				controllerSvidAnnotation: "",
			},
			verify: false,
		},
		{
			desc:         "tamper status",
			modifyStatus: true,
			verify:       false,
		},
		{
			desc:              "tamper status and status hash",
			modifyStatus:      true,
			modifyHashToMatch: true,
			verify:            false,
		},
	}
	for _, tt := range tests {

		for _, tr := range testTaskRuns() {
			err := cc.AppendStatusInternalAnnotation(ctx, tr)
			if err != nil {
				t.Fatalf("failed to sign TaskRun: %v", err)
			}

			if tr.Status.Status.Annotations == nil {
				tr.Status.Status.Annotations = map[string]string{}
			}

			if tt.setAnnotations != nil {
				for k, v := range tt.setAnnotations {
					tr.Status.Status.Annotations[k] = v
				}
			}

			if tt.modifyStatus {
				tr.Status.TaskRunStatusFields.Steps = append(tr.Status.TaskRunStatusFields.Steps, v1beta1.StepState{
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{ExitCode: int32(54321)},
					}})
			}

			if tt.modifyHashToMatch {
				h, _ := hashTaskrunStatusInternal(tr)
				tr.Status.Status.Annotations[TaskRunStatusHashAnnotation] = h
			}

			verified := cc.VerifyStatusInternalAnnotation(ctx, tr, logger) == nil
			if verified != tt.verify {
				t.Fatalf("test %v expected verify %v, got %v", tt.desc, tt.verify, verified)
			}
		}

	}

}

func TestSpire_TaskRunResultsSign(t *testing.T) {
	ctx, _ := ttesting.SetupDefaultContext(t)

	ca := test.NewCA(t, td)

	wl := fakeworkloadapi.New(t)
	defer wl.Stop()

	wl.SetX509Bundles(ca.X509Bundle())

	cfg := &config.SpireConfig{}
	cfg.SocketPath = wl.Addr()
	cfg.TrustDomain = trustDomain
	spireEntryPointerClient := GetEntrypointerAPIClient(ctx)
	spireControllerClient := GetControllerAPIClient(ctx)
	spireEntryPointerClient.SetConfig(*cfg)
	spireControllerClient.SetConfig(*cfg)

	var (
		cc ControllerAPIClient   = spireControllerClient
		ec EntrypointerAPIClient = spireEntryPointerClient
	)
	defer cc.Close()
	defer ec.Close()

	testCases := []struct {
		// description of test
		desc string
		// skip entry creation of pod identity
		skipEntryCreate bool
		// set wrong pod identity for signer
		wrongPodIdentity bool
		// whether sign/verify procedure should succeed
		success bool
		// List of taskruns to test against
		taskRunList []*v1beta1.TaskRun
		// List of PipelineResourceResult to test against
		pipelineResourceResults [][]v1beta1.PipelineResourceResult
	}{
		{
			desc:            "sign/verify result when entry isn't created",
			skipEntryCreate: true,
			success:         false,
			// Using single taskrun and pipelineResourceResults as the unit test
			// times out with the default 30 second. getWorkloadSVID has a 20 second
			// time out before it throws an error for no svid found
			taskRunList:             testSingleTaskRun(),
			pipelineResourceResults: testSinglePipelineResourceResults(),
		},
		{
			desc:                    "regular sign/verify result",
			success:                 true,
			taskRunList:             testTaskRuns(),
			pipelineResourceResults: testPipelineResourceResults(),
		},
		{
			desc:                    "sign/verify result when signing with wrong pod identity",
			wrongPodIdentity:        true,
			success:                 false,
			taskRunList:             testTaskRuns(),
			pipelineResourceResults: testPipelineResourceResults(),
		},
	}

	for _, tt := range testCases {
		ctx := context.Background()
		for _, tr := range tt.taskRunList {
			if !tt.skipEntryCreate {
				if !tt.wrongPodIdentity {
					resp := &fakeworkloadapi.X509SVIDResponse{
						Bundle: ca.X509Bundle(),
						SVIDs:  makeX509SVIDs(ca, spiffeid.RequireFromPath(td, getTaskrunPath(tr))),
					}
					wl.SetX509SVIDResponse(resp)
				} else {
					resp := &fakeworkloadapi.X509SVIDResponse{
						Bundle: ca.X509Bundle(),
						SVIDs:  makeX509SVIDs(ca, fooID),
					}
					wl.SetX509SVIDResponse(resp)
				}
			}

			for _, results := range tt.pipelineResourceResults {
				success := func() bool {

					sigResults, err := ec.Sign(ctx, results)
					if err != nil {
						return false
					}

					results = append(results, sigResults...)

					err = cc.VerifyTaskRunResults(ctx, results, tr)
					if err != nil {
						return false
					}

					return true
				}()

				if success != tt.success {
					t.Fatalf("test %v expected verify %v, got %v", tt.desc, tt.success, success)
				}
			}
		}
	}
}

// Task result sign, modify signature/content and verify
func TestSpire_TaskRunResultsSignTamper(t *testing.T) {
	ctx, _ := ttesting.SetupDefaultContext(t)

	ca := test.NewCA(t, td)

	wl := fakeworkloadapi.New(t)
	defer wl.Stop()

	wl.SetX509Bundles(ca.X509Bundle())

	cfg := &config.SpireConfig{}
	cfg.SocketPath = wl.Addr()
	cfg.TrustDomain = trustDomain
	spireEntryPointerClient := GetEntrypointerAPIClient(ctx)
	spireControllerClient := GetControllerAPIClient(ctx)
	spireEntryPointerClient.SetConfig(*cfg)
	spireControllerClient.SetConfig(*cfg)

	var (
		cc ControllerAPIClient   = spireControllerClient
		ec EntrypointerAPIClient = spireEntryPointerClient
	)
	defer cc.Close()
	defer ec.Close()

	genPr := func() []v1beta1.PipelineResourceResult {
		return []v1beta1.PipelineResourceResult{
			{
				Key:          "foo",
				Value:        "foo-value",
				ResourceName: "source-image",
				ResultType:   v1beta1.TaskRunResultType,
			},
			{
				Key:          "bar",
				Value:        "bar-value",
				ResourceName: "source-image2",
				ResultType:   v1beta1.TaskRunResultType,
			},
		}
	}

	testCases := []struct {
		// description of test
		desc string
		// function to tamper
		tamperFn func([]v1beta1.PipelineResourceResult) []v1beta1.PipelineResourceResult
		// whether sign/verify procedure should succeed
		success bool
	}{
		{
			desc:    "no tamper",
			success: true,
		},
		{
			desc: "non-intrusive tamper",
			tamperFn: func(prs []v1beta1.PipelineResourceResult) []v1beta1.PipelineResourceResult {
				prs = append(prs, v1beta1.PipelineResourceResult{
					Key:   "not-taskrun-result-type-add",
					Value: "abc:12345",
				})
				return prs
			},
			success: true,
		},
		{
			desc: "tamper SVID",
			tamperFn: func(prs []v1beta1.PipelineResourceResult) []v1beta1.PipelineResourceResult {
				for i, pr := range prs {
					if pr.Key == KeySVID {
						prs[i].Value = "tamper-value"
					}
				}
				return prs
			},
			success: false,
		},
		{
			desc: "tamper result manifest",
			tamperFn: func(prs []v1beta1.PipelineResourceResult) []v1beta1.PipelineResourceResult {
				for i, pr := range prs {
					if pr.Key == KeyResultManifest {
						prs[i].Value = "tamper-value"
					}
				}
				return prs
			},
			success: false,
		},
		{
			desc: "tamper result manifest signature",
			tamperFn: func(prs []v1beta1.PipelineResourceResult) []v1beta1.PipelineResourceResult {
				for i, pr := range prs {
					if pr.Key == KeyResultManifest+KeySignatureSuffix {
						prs[i].Value = "tamper-value"
					}
				}
				return prs
			},
			success: false,
		},
		{
			desc: "tamper result field",
			tamperFn: func(prs []v1beta1.PipelineResourceResult) []v1beta1.PipelineResourceResult {
				for i, pr := range prs {
					if pr.Key == "foo" {
						prs[i].Value = "tamper-value"
					}
				}
				return prs
			},
			success: false,
		},
		{
			desc: "tamper result field signature",
			tamperFn: func(prs []v1beta1.PipelineResourceResult) []v1beta1.PipelineResourceResult {
				for i, pr := range prs {
					if pr.Key == "foo"+KeySignatureSuffix {
						prs[i].Value = "tamper-value"
					}
				}
				return prs
			},
			success: false,
		},
		{
			desc: "delete SVID",
			tamperFn: func(prs []v1beta1.PipelineResourceResult) []v1beta1.PipelineResourceResult {
				for i, pr := range prs {
					if pr.Key == KeySVID {
						return append(prs[:i], prs[i+1:]...)
					}
				}
				return prs
			},
			success: false,
		},
		{
			desc: "delete result manifest",
			tamperFn: func(prs []v1beta1.PipelineResourceResult) []v1beta1.PipelineResourceResult {
				for i, pr := range prs {
					if pr.Key == KeyResultManifest {
						return append(prs[:i], prs[i+1:]...)
					}
				}
				return prs
			},
			success: false,
		},
		{
			desc: "delete result manifest signature",
			tamperFn: func(prs []v1beta1.PipelineResourceResult) []v1beta1.PipelineResourceResult {
				for i, pr := range prs {
					if pr.Key == KeyResultManifest+KeySignatureSuffix {
						return append(prs[:i], prs[i+1:]...)
					}
				}
				return prs
			},
			success: false,
		},
		{
			desc: "delete result field",
			tamperFn: func(prs []v1beta1.PipelineResourceResult) []v1beta1.PipelineResourceResult {
				for i, pr := range prs {
					if pr.Key == "foo" {
						return append(prs[:i], prs[i+1:]...)
					}
				}
				return prs
			},
			success: false,
		},
		{
			desc: "delete result field signature",
			tamperFn: func(prs []v1beta1.PipelineResourceResult) []v1beta1.PipelineResourceResult {
				for i, pr := range prs {
					if pr.Key == "foo"+KeySignatureSuffix {
						return append(prs[:i], prs[i+1:]...)
					}
				}
				return prs
			},
			success: false,
		},
		{
			desc: "add to result manifest",
			tamperFn: func(prs []v1beta1.PipelineResourceResult) []v1beta1.PipelineResourceResult {
				for i, pr := range prs {
					if pr.Key == KeyResultManifest {
						prs[i].Value += ",xyz"
					}
				}
				return prs
			},
			success: false,
		},
	}

	for _, tt := range testCases {
		ctx := context.Background()
		for _, tr := range testTaskRuns() {

			results := genPr()
			success := func() bool {

				resp := &fakeworkloadapi.X509SVIDResponse{
					Bundle: ca.X509Bundle(),
					SVIDs:  makeX509SVIDs(ca, spiffeid.RequireFromPath(td, getTaskrunPath(tr))),
				}
				wl.SetX509SVIDResponse(resp)

				sigResults, err := ec.Sign(ctx, results)
				if err != nil {
					return false
				}

				results = append(results, sigResults...)
				if tt.tamperFn != nil {
					results = tt.tamperFn(results)
				}

				err = cc.VerifyTaskRunResults(ctx, results, tr)
				if err != nil {
					return false
				}

				return true
			}()

			if success != tt.success {
				t.Fatalf("test %v expected verify %v, got %v", tt.desc, tt.success, success)
			}
		}
	}
}

func makeX509SVIDs(ca *test.CA, ids ...spiffeid.ID) []*x509svid.SVID {
	svids := []*x509svid.SVID{}
	for _, id := range ids {
		svids = append(svids, ca.CreateX509SVID(id))
	}
	return svids
}

func getTaskrunPath(tr *v1beta1.TaskRun) string {
	return fmt.Sprintf("/ns/%v/taskrun/%v", tr.Namespace, tr.Name)
}

func testSingleTaskRun() []*v1beta1.TaskRun {
	return []*v1beta1.TaskRun{
		// taskRun 1
		{
			ObjectMeta: objectMeta("taskrun-example", "foo"),
			Spec: v1beta1.TaskRunSpec{
				TaskRef: &v1beta1.TaskRef{
					Name:       "taskname",
					APIVersion: "a1",
				},
				ServiceAccountName: "test-sa",
			},
		},
	}
}

func testSinglePipelineResourceResults() [][]v1beta1.PipelineResourceResult {
	return [][]v1beta1.PipelineResourceResult{
		// Single result
		{
			{
				Key:          "digest",
				Value:        "sha256:12345",
				ResourceName: "source-image",
				ResultType:   v1beta1.TaskRunResultType,
			},
		},
	}
}
