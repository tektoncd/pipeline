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
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	pconf "github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"github.com/tektoncd/pipeline/pkg/result"
	"github.com/tektoncd/pipeline/pkg/spire/config"
	"github.com/tektoncd/pipeline/pkg/spire/test"
	"github.com/tektoncd/pipeline/pkg/spire/test/fakeworkloadapi"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/logging"
)

const (
	trustDomain    = "example.org"
	notImplemented = "notImplemented"
)

var (
	td           = spiffeid.RequireTrustDomainFromString(trustDomain)
	controllerID = spiffeid.RequireFromPath(td, "/controller")
)

func init() {
	// shorten timeout and backoff intervals for faster tests;
	// variables are declared in entrypointer.go
	timeout = time.Second
	backoff = 100 * time.Millisecond
}

func TestTaskRunSign(t *testing.T) {
	ctx, _ := ttesting.SetupDefaultContext(t)

	ca := test.NewCA(t, td)
	wl := fakeworkloadapi.New(t)
	defer wl.Stop()

	wl.SetX509Bundles(ca.X509Bundle())

	resp := &fakeworkloadapi.X509SVIDResponse{
		Bundle: ca.X509Bundle(),
		SVIDs:  x509svids(ca, controllerID),
	}
	wl.SetX509SVIDResponse(resp)

	cfg := &config.SpireConfig{}
	cfg.SocketPath = wl.Addr()
	cfg.ServerAddr = notImplemented
	cfg.TrustDomain = trustDomain

	cc := GetControllerAPIClient(ctx)
	cc.SetConfig(*cfg)
	defer cc.Close()

	logger := logging.FromContext(ctx)

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

func TestCheckSpireVerifiedFlag(t *testing.T) {
	ctx, _ := ttesting.SetupDefaultContext(t)

	ca := test.NewCA(t, td)
	wl := fakeworkloadapi.New(t)
	defer wl.Stop()

	wl.SetX509Bundles(ca.X509Bundle())

	resp := &fakeworkloadapi.X509SVIDResponse{
		Bundle: ca.X509Bundle(),
		SVIDs:  x509svids(ca, controllerID),
	}
	wl.SetX509SVIDResponse(resp)

	cfg := &config.SpireConfig{}
	cfg.SocketPath = wl.Addr()
	cfg.ServerAddr = notImplemented
	cfg.TrustDomain = trustDomain

	cc := GetControllerAPIClient(ctx)
	cc.SetConfig(*cfg)
	defer cc.Close()

	trs := testTaskRuns()
	tr := trs[0]

	if !cc.CheckSpireVerifiedFlag(tr) {
		t.Fatalf("verified flag should be unset")
	}

	if tr.Status.Status.Annotations == nil {
		tr.Status.Status.Annotations = map[string]string{}
	}
	tr.Status.Status.Annotations[VerifiedAnnotation] = "no"

	if cc.CheckSpireVerifiedFlag(tr) {
		t.Fatalf("verified flag should be set")
	}
}

func TestCheckHashSimilarities(t *testing.T) {
	ctx, _ := ttesting.SetupDefaultContext(t)

	ca := test.NewCA(t, td)
	wl := fakeworkloadapi.New(t)
	defer wl.Stop()

	wl.SetX509Bundles(ca.X509Bundle())

	resp := &fakeworkloadapi.X509SVIDResponse{
		Bundle: ca.X509Bundle(),
		SVIDs:  x509svids(ca, controllerID),
	}
	wl.SetX509SVIDResponse(resp)

	cfg := &config.SpireConfig{}
	cfg.SocketPath = wl.Addr()
	cfg.ServerAddr = notImplemented
	cfg.TrustDomain = trustDomain

	cc := GetControllerAPIClient(ctx)
	cc.SetConfig(*cfg)
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
func TestCheckTamper(t *testing.T) {
	ctx, _ := ttesting.SetupDefaultContext(t)

	ca := test.NewCA(t, td)
	wl := fakeworkloadapi.New(t)
	defer wl.Stop()

	wl.SetX509Bundles(ca.X509Bundle())

	resp := &fakeworkloadapi.X509SVIDResponse{
		Bundle: ca.X509Bundle(),
		SVIDs:  x509svids(ca, controllerID),
	}
	wl.SetX509SVIDResponse(resp)

	cfg := &config.SpireConfig{}
	cfg.SocketPath = wl.Addr()
	cfg.ServerAddr = notImplemented
	cfg.TrustDomain = trustDomain

	cc := GetControllerAPIClient(ctx)
	cc.SetConfig(*cfg)
	defer cc.Close()

	logger := logging.FromContext(ctx)

	tests := []struct {
		// description of test case
		desc string
		// annotations to set
		setAnnotations map[string]string
		// skip annotation set
		skipAnnotation bool
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
			desc: "set temper flag",
			setAnnotations: map[string]string{
				VerifiedAnnotation: "no",
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
		{
			desc:           "tamper status and status hash",
			skipAnnotation: true,
			verify:         false,
		},
	}
	for _, tt := range tests {
		for _, tr := range testTaskRuns() {
			if !tt.skipAnnotation {
				err := cc.AppendStatusInternalAnnotation(ctx, tr)
				if err != nil {
					t.Fatalf("failed to sign TaskRun: %v", err)
				}
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

func TestNoSVID(t *testing.T) {
	ctx := context.Background()
	cfg := &config.SpireConfig{SocketPath: "tcp://127.0.0.1:12345"} // bogus SocketPath
	ec := NewEntrypointerAPIClient(cfg)
	defer ec.Close()

	if _, err := ec.Sign(ctx, nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("got %v; want %v", err, context.DeadlineExceeded)
	}
}

func TestBadPodIdentity(t *testing.T) {
	ctx, _ := ttesting.SetupDefaultContext(t)

	ca := test.NewCA(t, td)

	wl := fakeworkloadapi.New(t)
	defer wl.Stop()

	wl.SetX509Bundles(ca.X509Bundle())

	cfg := &config.SpireConfig{}
	cfg.SocketPath = wl.Addr()
	cfg.ServerAddr = notImplemented
	cfg.TrustDomain = trustDomain

	ec := NewEntrypointerAPIClient(cfg)
	cc := GetControllerAPIClient(ctx)
	cc.SetConfig(*cfg)
	defer cc.Close()
	defer ec.Close()

	trList := testTaskRuns()
	rsrcResults := testRunResults()

	wl.SetX509SVIDResponse(&fakeworkloadapi.X509SVIDResponse{
		Bundle: ca.X509Bundle(),
		SVIDs:  x509svids(ca, spiffeid.RequireFromPath(td, "/bogus")),
	})

	for _, tr := range trList {
		for i, results := range rsrcResults {
			t.Run(fmt.Sprintf("%s: %d", tr.ObjectMeta.Name, i), func(t *testing.T) {
				sigResults, err := ec.Sign(ctx, results)
				if err != nil {
					t.Fatalf("ec.Sign error: %v", err)
				}

				results = append(results, sigResults...)
				// It would be nice to check the error type, but
				// VerifyTaskRunResults returns a generic errors.New().
				if cc.VerifyTaskRunResults(ctx, results, tr) == nil {
					t.Error("got nil; want error")
				}
			})
		}
	}
}

func TestSignTaskRunResults(t *testing.T) {
	ctx, _ := ttesting.SetupDefaultContext(t)

	wl := fakeworkloadapi.New(t)
	defer wl.Stop()

	ca := test.NewCA(t, td)
	wl.SetX509Bundles(ca.X509Bundle())

	cfg := &config.SpireConfig{
		SocketPath:  wl.Addr(),
		ServerAddr:  notImplemented,
		TrustDomain: trustDomain,
	}

	ec := NewEntrypointerAPIClient(cfg)
	cc := GetControllerAPIClient(ctx)
	cc.SetConfig(*cfg)
	defer cc.Close()
	defer ec.Close()

	trList := testTaskRuns()
	rsrcResults := testRunResults()

	for _, tr := range trList {
		for i, results := range rsrcResults {
			t.Run(fmt.Sprintf("%s: %d", tr.ObjectMeta.Name, i), func(t *testing.T) {
				wl.SetX509SVIDResponse(&fakeworkloadapi.X509SVIDResponse{
					Bundle: ca.X509Bundle(),
					SVIDs:  x509svids(ca, spiffeid.RequireFromPath(td, taskrunPath(tr))),
				})

				sigResults, err := ec.Sign(ctx, results)
				if err != nil {
					t.Fatalf("sign error: %v", err)
				}

				results = append(results, sigResults...)
				if err := cc.VerifyTaskRunResults(ctx, results, tr); err != nil {
					t.Errorf("verify error: %v", err)
				}
			})
		}
	}
}

// Task result sign, modify signature/content and verify
func TestTaskRunResultsSignTamper(t *testing.T) {
	ctx, _ := ttesting.SetupDefaultContext(t)

	ca := test.NewCA(t, td)

	wl := fakeworkloadapi.New(t)
	defer wl.Stop()

	wl.SetX509Bundles(ca.X509Bundle())

	cfg := &config.SpireConfig{}
	cfg.SocketPath = wl.Addr()
	cfg.ServerAddr = notImplemented
	cfg.TrustDomain = trustDomain

	ec := NewEntrypointerAPIClient(cfg)
	cc := GetControllerAPIClient(ctx)
	cc.SetConfig(*cfg)
	defer cc.Close()
	defer ec.Close()

	testCases := []struct {
		desc string
		// function to tamper
		tamperFn func([]result.RunResult) []result.RunResult
		wantErr  bool
	}{
		{
			desc:     "no tamper",
			wantErr:  false,
			tamperFn: func(prs []result.RunResult) []result.RunResult { return prs },
		},
		{
			desc: "non-intrusive tamper",
			tamperFn: func(prs []result.RunResult) []result.RunResult {
				prs = append(prs, result.RunResult{
					Key:   "not-taskrun-result-type-add",
					Value: "abc:12345",
				})
				return prs
			},
			wantErr: false,
		},
		{
			desc: "tamper SVID",
			tamperFn: func(prs []result.RunResult) []result.RunResult {
				for i, pr := range prs {
					if pr.Key == KeySVID {
						prs[i].Value = "tamper-value"
					}
				}
				return prs
			},
			wantErr: true,
		},
		{
			desc: "tamper result manifest",
			tamperFn: func(prs []result.RunResult) []result.RunResult {
				for i, pr := range prs {
					if pr.Key == KeyResultManifest {
						prs[i].Value = "tamper-value"
					}
				}
				return prs
			},
			wantErr: true,
		},
		{
			desc: "tamper result manifest signature",
			tamperFn: func(prs []result.RunResult) []result.RunResult {
				for i, pr := range prs {
					if pr.Key == KeyResultManifest+KeySignatureSuffix {
						prs[i].Value = "tamper-value"
					}
				}
				return prs
			},
			wantErr: true,
		},
		{
			desc: "tamper result field",
			tamperFn: func(prs []result.RunResult) []result.RunResult {
				for i, pr := range prs {
					if pr.Key == "foo" {
						prs[i].Value = "tamper-value"
					}
				}
				return prs
			},
			wantErr: true,
		},
		{
			desc: "tamper result field signature",
			tamperFn: func(prs []result.RunResult) []result.RunResult {
				for i, pr := range prs {
					if pr.Key == "foo"+KeySignatureSuffix {
						prs[i].Value = "tamper-value"
					}
				}
				return prs
			},
			wantErr: true,
		},
		{
			desc: "delete SVID",
			tamperFn: func(prs []result.RunResult) []result.RunResult {
				for i, pr := range prs {
					if pr.Key == KeySVID {
						return append(prs[:i], prs[i+1:]...)
					}
				}
				return prs
			},
			wantErr: true,
		},
		{
			desc: "delete result manifest",
			tamperFn: func(prs []result.RunResult) []result.RunResult {
				for i, pr := range prs {
					if pr.Key == KeyResultManifest {
						return append(prs[:i], prs[i+1:]...)
					}
				}
				return prs
			},
			wantErr: true,
		},
		{
			desc: "delete result manifest signature",
			tamperFn: func(prs []result.RunResult) []result.RunResult {
				for i, pr := range prs {
					if pr.Key == KeyResultManifest+KeySignatureSuffix {
						return append(prs[:i], prs[i+1:]...)
					}
				}
				return prs
			},
			wantErr: true,
		},
		{
			desc: "delete result field",
			tamperFn: func(prs []result.RunResult) []result.RunResult {
				for i, pr := range prs {
					if pr.Key == "foo" {
						return append(prs[:i], prs[i+1:]...)
					}
				}
				return prs
			},
			wantErr: true,
		},
		{
			desc: "delete result field signature",
			tamperFn: func(prs []result.RunResult) []result.RunResult {
				for i, pr := range prs {
					if pr.Key == "foo"+KeySignatureSuffix {
						return append(prs[:i], prs[i+1:]...)
					}
				}
				return prs
			},
			wantErr: true,
		},
		{
			desc: "add to result manifest",
			tamperFn: func(prs []result.RunResult) []result.RunResult {
				for i, pr := range prs {
					if pr.Key == KeyResultManifest {
						prs[i].Value += ",xyz"
					}
				}
				return prs
			},
			wantErr: true,
		},
	}

	for _, tt := range testCases {
		ctx := context.Background()
		for _, tr := range testTaskRuns() {
			t.Run(tt.desc+" "+tr.ObjectMeta.Name, func(t *testing.T) {
				results := []result.RunResult{{
					Key:          "foo",
					Value:        "foo-value",
					ResourceName: "source-image",
					ResultType:   result.TaskRunResultType,
				}, {
					Key:          "bar",
					Value:        "bar-value",
					ResourceName: "source-image2",
					ResultType:   result.TaskRunResultType,
				}}

				wl.SetX509SVIDResponse(&fakeworkloadapi.X509SVIDResponse{
					Bundle: ca.X509Bundle(),
					SVIDs:  x509svids(ca, spiffeid.RequireFromPath(td, taskrunPath(tr))),
				})

				sigResults, err := ec.Sign(ctx, results)
				if err != nil {
					t.Fatalf("ec.Sign(): %v", err)
				}

				results = append(results, sigResults...)
				results = tt.tamperFn(results)
				err = cc.VerifyTaskRunResults(ctx, results, tr)

				if gotErr := err != nil; gotErr != tt.wantErr {
					t.Errorf("got %v, wantErr == %t", err, tt.wantErr)
				}
			})
		}
	}
}

func TestOnStore(t *testing.T) {
	ctx, _ := ttesting.SetupDefaultContext(t)
	logger := logging.FromContext(ctx)
	ctx = context.WithValue(ctx, controllerKey{}, &spireControllerAPIClient{
		config: &config.SpireConfig{
			TrustDomain:     "before_test_domain",
			SocketPath:      "before_test_socket_path",
			ServerAddr:      "before_test_server_path",
			NodeAliasPrefix: "before_test_node_alias_prefix",
		},
	})
	want := config.SpireConfig{
		TrustDomain:     "after_test_domain",
		SocketPath:      "after_test_socket_path",
		ServerAddr:      "after_test_server_path",
		NodeAliasPrefix: "after_test_node_alias_prefix",
	}
	OnStore(ctx, logger)(pconf.GetSpireConfigName(), &want)
	got := *GetControllerAPIClient(ctx).(*spireControllerAPIClient).config
	if d := cmp.Diff(want, got); d != "" {
		t.Fatalf("Diff in TestOnStore, diff: %s", diff.PrintWantGot(d))
	}
}

func x509svids(ca *test.CA, ids ...spiffeid.ID) []*x509svid.SVID {
	svids := []*x509svid.SVID{}
	for _, id := range ids {
		svids = append(svids, ca.CreateX509SVID(id))
	}
	return svids
}

func taskrunPath(tr *v1beta1.TaskRun) string {
	return fmt.Sprintf("/ns/%v/taskrun/%v", tr.Namespace, tr.Name)
}
