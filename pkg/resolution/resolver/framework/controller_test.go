/*
Copyright 2026 The Tekton Authors

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

package framework_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	rrlister "github.com/tektoncd/pipeline/pkg/client/resolution/listers/resolution/v1beta1"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	"github.com/tektoncd/pipeline/test/diff"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	pkgreconciler "knative.dev/pkg/reconciler"
)

func resolutionRequest(namespace, name string, lbls map[string]string) *v1beta1.ResolutionRequest {
	return &v1beta1.ResolutionRequest{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			Labels:    lbls,
		},
	}
}

// TestLeaderAwareFuncsPromoteFiltersBySelector checks that a resolver only
// re-enqueues the ResolutionRequests addressed to it when it is promoted to
// leader. Enqueueing every request into every resolver's workqueue makes the
// wrong resolver fail requests it does not own, and because a failed request
// is already done the correct resolver then skips it.
func TestLeaderAwareFuncsPromoteFiltersBySelector(t *testing.T) {
	requests := []*v1beta1.ResolutionRequest{
		resolutionRequest("foo", "git-request", map[string]string{resolutioncommon.LabelKeyResolverType: "git"}),
		resolutionRequest("foo", "http-request", map[string]string{resolutioncommon.LabelKeyResolverType: "http"}),
		resolutionRequest("foo", "unlabeled-request", nil),
		resolutionRequest("bar", "another-git-request", map[string]string{
			resolutioncommon.LabelKeyResolverType: "git",
			"custom-label":                        "some-value",
		}),
	}

	for _, tc := range []struct {
		name     string
		selector map[string]string
		want     []types.NamespacedName
	}{{
		name:     "only requests carrying the resolver's type are enqueued",
		selector: map[string]string{resolutioncommon.LabelKeyResolverType: "git"},
		want: []types.NamespacedName{
			{Namespace: "bar", Name: "another-git-request"},
			{Namespace: "foo", Name: "git-request"},
		},
	}, {
		name:     "a request must match every label in the selector",
		selector: map[string]string{resolutioncommon.LabelKeyResolverType: "git", "custom-label": "some-value"},
		want: []types.NamespacedName{
			{Namespace: "bar", Name: "another-git-request"},
		},
	}, {
		name:     "a resolver with no matching requests enqueues nothing",
		selector: map[string]string{resolutioncommon.LabelKeyResolverType: "bundles"},
		want:     nil,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			for _, rr := range requests {
				if err := indexer.Add(rr); err != nil {
					t.Fatalf("failed to seed indexer: %v", err)
				}
			}

			var got []types.NamespacedName
			enq := func(_ pkgreconciler.Bucket, key types.NamespacedName) {
				got = append(got, key)
			}

			laf := framework.LeaderAwareFuncs(rrlister.NewResolutionRequestLister(indexer), tc.selector)
			if err := laf.PromoteFunc(pkgreconciler.UniversalBucket(), enq); err != nil {
				t.Fatalf("PromoteFunc returned an error: %v", err)
			}

			sortKeys := cmpopts.SortSlices(func(a, b types.NamespacedName) bool { return a.String() < b.String() })
			if d := cmp.Diff(tc.want, got, sortKeys, cmpopts.EquateEmpty()); d != "" {
				t.Errorf("wrong requests enqueued on promotion: %s", diff.PrintWantGot(d))
			}
		})
	}
}
