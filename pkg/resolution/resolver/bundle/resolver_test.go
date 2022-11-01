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

package bundle

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"sigs.k8s.io/yaml"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/registry"
	resolverconfig "github.com/tektoncd/pipeline/pkg/apis/config/resolver"
	pipelinev1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	frtesting "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework/testing"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/internal"
	"github.com/tektoncd/pipeline/test"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/system"
	_ "knative.dev/pkg/system/testing" // Setup system.Namespace()
)

func TestGetSelector(t *testing.T) {
	resolver := Resolver{}
	sel := resolver.GetSelector(context.Background())
	if typ, has := sel[resolutioncommon.LabelKeyResolverType]; !has {
		t.Fatalf("unexpected selector: %v", sel)
	} else if typ != LabelValueBundleResolverType {
		t.Fatalf("unexpected type: %q", typ)
	}
}

func TestValidateParams(t *testing.T) {
	resolver := Resolver{}

	paramsWithTask := []pipelinev1beta1.Param{{
		Name:  ParamKind,
		Value: *pipelinev1beta1.NewStructuredValues("task"),
	}, {
		Name:  ParamName,
		Value: *pipelinev1beta1.NewStructuredValues("foo"),
	}, {
		Name:  ParamBundle,
		Value: *pipelinev1beta1.NewStructuredValues("bar"),
	}, {
		Name:  ParamServiceAccount,
		Value: *pipelinev1beta1.NewStructuredValues("baz"),
	}}

	if err := resolver.ValidateParams(resolverContext(), paramsWithTask); err != nil {
		t.Fatalf("unexpected error validating params: %v", err)
	}

	paramsWithPipeline := []pipelinev1beta1.Param{{
		Name:  ParamKind,
		Value: *pipelinev1beta1.NewStructuredValues("pipeline"),
	}, {
		Name:  ParamName,
		Value: *pipelinev1beta1.NewStructuredValues("foo"),
	}, {
		Name:  ParamBundle,
		Value: *pipelinev1beta1.NewStructuredValues("bar"),
	}, {
		Name:  ParamServiceAccount,
		Value: *pipelinev1beta1.NewStructuredValues("baz"),
	}}
	if err := resolver.ValidateParams(resolverContext(), paramsWithPipeline); err != nil {
		t.Fatalf("unexpected error validating params: %v", err)
	}
}

func TestValidateParamsDisabled(t *testing.T) {
	resolver := Resolver{}

	var err error

	params := []pipelinev1beta1.Param{{
		Name:  ParamKind,
		Value: *pipelinev1beta1.NewStructuredValues("task"),
	}, {
		Name:  ParamName,
		Value: *pipelinev1beta1.NewStructuredValues("foo"),
	}, {
		Name:  ParamBundle,
		Value: *pipelinev1beta1.NewStructuredValues("bar"),
	}, {
		Name:  ParamServiceAccount,
		Value: *pipelinev1beta1.NewStructuredValues("baz"),
	}}
	err = resolver.ValidateParams(context.Background(), params)
	if err == nil {
		t.Fatalf("expected disabled err")
	}

	if d := cmp.Diff(disabledError, err.Error()); d != "" {
		t.Errorf("unexpected error: %s", diff.PrintWantGot(d))
	}
}

func TestValidateParamsMissing(t *testing.T) {
	resolver := Resolver{}

	var err error

	paramsMissingBundle := []pipelinev1beta1.Param{{
		Name:  ParamKind,
		Value: *pipelinev1beta1.NewStructuredValues("task"),
	}, {
		Name:  ParamName,
		Value: *pipelinev1beta1.NewStructuredValues("foo"),
	}, {
		Name:  ParamServiceAccount,
		Value: *pipelinev1beta1.NewStructuredValues("baz"),
	}}
	err = resolver.ValidateParams(resolverContext(), paramsMissingBundle)
	if err == nil {
		t.Fatalf("expected missing kind err")
	}

	paramsMissingName := []pipelinev1beta1.Param{{
		Name:  ParamKind,
		Value: *pipelinev1beta1.NewStructuredValues("task"),
	}, {
		Name:  ParamBundle,
		Value: *pipelinev1beta1.NewStructuredValues("bar"),
	}, {
		Name:  ParamServiceAccount,
		Value: *pipelinev1beta1.NewStructuredValues("baz"),
	}}
	err = resolver.ValidateParams(resolverContext(), paramsMissingName)
	if err == nil {
		t.Fatalf("expected missing name err")
	}

}

func TestResolveDisabled(t *testing.T) {
	resolver := Resolver{}

	var err error

	params := []pipelinev1beta1.Param{{
		Name:  ParamKind,
		Value: *pipelinev1beta1.NewStructuredValues("task"),
	}, {
		Name:  ParamName,
		Value: *pipelinev1beta1.NewStructuredValues("foo"),
	}, {
		Name:  ParamBundle,
		Value: *pipelinev1beta1.NewStructuredValues("bar"),
	}, {
		Name:  ParamServiceAccount,
		Value: *pipelinev1beta1.NewStructuredValues("baz"),
	}}
	_, err = resolver.Resolve(context.Background(), params)
	if err == nil {
		t.Fatalf("expected disabled err")
	}

	if d := cmp.Diff(disabledError, err.Error()); d != "" {
		t.Errorf("unexpected error: %s", diff.PrintWantGot(d))
	}
}

type params struct {
	serviceAccount string
	bundle         string
	name           string
	kind           string
}

func TestResolve(t *testing.T) {
	// example task resource
	exampleTask := &pipelinev1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "example-task",
			Namespace:       "task-ns",
			ResourceVersion: "00002",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       string(pipelinev1beta1.NamespacedTaskKind),
			APIVersion: "tekton.dev/v1beta1",
		},
		Spec: pipelinev1beta1.TaskSpec{
			Steps: []pipelinev1beta1.Step{{
				Name:    "some-step",
				Image:   "some-image",
				Command: []string{"something"},
			}},
		},
	}
	taskAsYAML, err := yaml.Marshal(exampleTask)
	if err != nil {
		t.Fatalf("couldn't marshal task: %v", err)
	}

	// example pipeline resource
	examplePipeline := &pipelinev1beta1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "example-pipeline",
			Namespace:       "pipeline-ns",
			ResourceVersion: "00001",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pipeline",
			APIVersion: "tekton.dev/v1beta1",
		},
		Spec: pipelinev1beta1.PipelineSpec{
			Tasks: []pipelinev1beta1.PipelineTask{{
				Name: "some-pipeline-task",
				TaskRef: &pipelinev1beta1.TaskRef{
					Name: "some-task",
					Kind: pipelinev1beta1.NamespacedTaskKind,
				},
			}},
		},
	}
	pipelineAsYAML, err := yaml.Marshal(examplePipeline)
	if err != nil {
		t.Fatalf("couldn't marshal pipeline: %v", err)
	}

	validObjects := map[string][]runtime.Object{
		"single-task":        {exampleTask},
		"single-pipeline":    {examplePipeline},
		"multiple-resources": {exampleTask, examplePipeline},
	}

	// too many objects in bundle resolver test
	var tooManyObjs []runtime.Object
	for i := 0; i <= MaximumBundleObjects; i++ {
		name := fmt.Sprintf("%d-task", i)
		obj := pipelinev1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			TypeMeta: metav1.TypeMeta{
				APIVersion: "tekton.dev/v1beta1",
				Kind:       "Task",
			},
		}
		tooManyObjs = append(tooManyObjs, &obj)
	}

	invalidObjects := map[string][]runtime.Object{
		"too-many-objs":                   tooManyObjs,
		"single-task-no-version":          {&pipelinev1beta1.Task{TypeMeta: metav1.TypeMeta{Kind: "task"}, ObjectMeta: metav1.ObjectMeta{Name: "foo"}}},
		"single-task-no-kind":             {&pipelinev1beta1.Task{TypeMeta: metav1.TypeMeta{APIVersion: "tekton.dev/v1beta1"}, ObjectMeta: metav1.ObjectMeta{Name: "foo"}}},
		"single-task-no-name":             {&pipelinev1beta1.Task{TypeMeta: metav1.TypeMeta{APIVersion: "tekton.dev/v1beta1", Kind: "task"}}},
		"single-task-kind-incorrect-form": {&pipelinev1beta1.Task{TypeMeta: metav1.TypeMeta{APIVersion: "tekton.dev/v1beta1", Kind: "Task"}, ObjectMeta: metav1.ObjectMeta{Name: "foo"}}},
	}

	// Set up a fake registry to push an image to.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	validImageRefs := pushImagesToRegistry(t, u.Host, validObjects, test.DefaultObjectAnnotationMapper)
	invalidImageRefs := pushImagesToRegistry(t, u.Host, invalidObjects, asIsMapper)

	testcases := []struct {
		name               string
		args               *params
		expectedStatus     *v1beta1.ResolutionRequestStatus
		expectedErrMessage string
	}{
		{
			name: "single task",
			args: &params{
				bundle: validImageRefs["single-task"],
				name:   "example-task",
				kind:   "task",
			},
			expectedStatus: internal.CreateResolutionRequestStatusWithData(taskAsYAML),
		}, {
			name: "single task using default kind value from configmap",
			args: &params{
				bundle: validImageRefs["single-task"],
				name:   "example-task",
			},
			expectedStatus: internal.CreateResolutionRequestStatusWithData(taskAsYAML),
		}, {
			name: "single pipeline",
			args: &params{
				bundle: validImageRefs["single-pipeline"],
				name:   "example-pipeline",
				kind:   "pipeline",
			},
			expectedStatus: internal.CreateResolutionRequestStatusWithData(pipelineAsYAML),
		}, {
			name: "multiple resources",
			args: &params{
				bundle: validImageRefs["multiple-resources"],
				name:   "example-pipeline",
				kind:   "pipeline",
			},
			expectedStatus: internal.CreateResolutionRequestStatusWithData(pipelineAsYAML),
		},
		{
			name: "too many objects in an image",
			args: &params{
				bundle: invalidImageRefs["too-many-objs"],
				name:   "2-task",
				kind:   "task",
			},
			expectedStatus:     internal.CreateResolutionRequestFailureStatus(),
			expectedErrMessage: fmt.Sprintf("contained more than the maximum %d allow objects", MaximumBundleObjects),
		}, {
			name: "single task no version",
			args: &params{
				bundle: invalidImageRefs["single-task-no-version"],
				name:   "foo",
				kind:   "task",
			},
			expectedStatus:     internal.CreateResolutionRequestFailureStatus(),
			expectedErrMessage: fmt.Sprintf("the layer 0 does not contain a %s annotation", BundleAnnotationAPIVersion),
		}, {
			name: "single task no kind",
			args: &params{
				bundle: invalidImageRefs["single-task-no-kind"],
				name:   "foo",
				kind:   "task",
			},
			expectedStatus:     internal.CreateResolutionRequestFailureStatus(),
			expectedErrMessage: fmt.Sprintf("the layer 0 does not contain a %s annotation", BundleAnnotationKind),
		}, {
			name: "single task no name",
			args: &params{
				bundle: invalidImageRefs["single-task-no-name"],
				name:   "foo",
				kind:   "task",
			},
			expectedStatus:     internal.CreateResolutionRequestFailureStatus(),
			expectedErrMessage: fmt.Sprintf("the layer 0 does not contain a %s annotation", BundleAnnotationName),
		}, {
			name: "single task kind incorrect form",
			args: &params{
				bundle: invalidImageRefs["single-task-kind-incorrect-form"],
				name:   "foo",
				kind:   "task",
			},
			expectedStatus:     internal.CreateResolutionRequestFailureStatus(),
			expectedErrMessage: fmt.Sprintf("the layer 0 the annotation %s must be lowercased and singular, found %s", BundleAnnotationKind, "Task"),
		},
	}

	resolver := &Resolver{}
	confMap := map[string]string{
		ConfigKind: "task",
		// service account is not used in testing, but we have to set this since
		// param validation will check if the service account is set either from param or from configmap.
		ConfigServiceAccount: "placeholder",
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := ttesting.SetupFakeContext(t)

			request := createRequest(tc.args)

			d := test.Data{
				ResolutionRequests: []*v1beta1.ResolutionRequest{request},
				ConfigMaps: []*corev1.ConfigMap{{
					ObjectMeta: metav1.ObjectMeta{
						Name:      ConfigMapName,
						Namespace: resolverconfig.ResolversNamespace(system.Namespace()),
					},
					Data: confMap,
				}, {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: resolverconfig.ResolversNamespace(system.Namespace()),
						Name:      resolverconfig.GetFeatureFlagsConfigName(),
					},
					Data: map[string]string{
						"enable-bundles-resolver": "true",
					},
				}},
			}
			var expectedStatus *v1beta1.ResolutionRequestStatus
			var expectedError error
			if tc.expectedStatus != nil {
				expectedStatus = tc.expectedStatus.DeepCopy()
				if tc.expectedErrMessage == "" {
					if expectedStatus.Annotations == nil {
						expectedStatus.Annotations = make(map[string]string)
					}

					if tc.args.kind != "" {
						expectedStatus.Annotations[ResolverAnnotationKind] = tc.args.kind
					} else {
						expectedStatus.Annotations[ResolverAnnotationKind] = "task"
					}

					expectedStatus.Annotations[ResolverAnnotationName] = tc.args.name
					expectedStatus.Annotations[ResolverAnnotationAPIVersion] = "v1beta1"
				} else {
					expectedError = createError(tc.args.bundle, tc.expectedErrMessage)
					expectedStatus.Status.Conditions[0].Message = expectedError.Error()
				}
			}

			frtesting.RunResolverReconcileTest(ctx, t, d, resolver, request, expectedStatus, expectedError)

		})
	}
}

func createRequest(p *params) *v1beta1.ResolutionRequest {
	rr := &v1beta1.ResolutionRequest{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "resolution.tekton.dev/v1beta1",
			Kind:       "ResolutionRequest",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              "rr",
			Namespace:         "foo",
			CreationTimestamp: metav1.Time{Time: time.Now()},
			Labels: map[string]string{
				resolutioncommon.LabelKeyResolverType: LabelValueBundleResolverType,
			},
		},
		Spec: v1beta1.ResolutionRequestSpec{
			Params: []pipelinev1beta1.Param{{
				Name:  ParamBundle,
				Value: *pipelinev1beta1.NewStructuredValues(p.bundle),
			}, {
				Name:  ParamName,
				Value: *pipelinev1beta1.NewStructuredValues(p.name),
			}, {
				Name:  ParamKind,
				Value: *pipelinev1beta1.NewStructuredValues(p.kind),
			}, {
				Name:  ParamServiceAccount,
				Value: *pipelinev1beta1.NewStructuredValues(p.serviceAccount),
			}},
		},
	}
	return rr
}

func createError(image, msg string) error {
	return &resolutioncommon.ErrorGettingResource{
		ResolverName: BundleResolverName,
		Key:          "foo/rr",
		Original:     errors.New(fmt.Sprintf("invalid tekton bundle %s, error: %s", image, msg)),
	}
}

func asIsMapper(obj runtime.Object) map[string]string {
	annotations := map[string]string{}
	if test.GetObjectName(obj) != "" {
		annotations[BundleAnnotationName] = test.GetObjectName(obj)
	}

	if obj.GetObjectKind().GroupVersionKind().Kind != "" {
		annotations[BundleAnnotationKind] = obj.GetObjectKind().GroupVersionKind().Kind
	}
	if obj.GetObjectKind().GroupVersionKind().Version != "" {
		annotations[BundleAnnotationAPIVersion] = obj.GetObjectKind().GroupVersionKind().Version
	}
	return annotations
}

func resolverContext() context.Context {
	return frtesting.ContextWithBundlesResolverEnabled(context.Background())
}

func pushImagesToRegistry(t *testing.T, host string, objects map[string][]runtime.Object, mapper test.ObjectAnnotationMapper) map[string]string {
	refs := map[string]string{}

	for name, objs := range objects {
		ref, err := test.CreateImageWithAnnotations(fmt.Sprintf("%s/testbundleresolver/%s", host, name), mapper, objs...)
		if err != nil {
			t.Fatalf("couldn't push the image: %v", err)
		}
		refs[name] = ref
	}

	return refs
}
