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

package bundle_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/registry"
	resolverconfig "github.com/tektoncd/pipeline/pkg/apis/config/resolver"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	pipelinev1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	bundle "github.com/tektoncd/pipeline/pkg/resolution/resolver/bundle"
	frtesting "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework/testing"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/internal"
	"github.com/tektoncd/pipeline/test"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/system"
	_ "knative.dev/pkg/system/testing" // Setup system.Namespace()
	"sigs.k8s.io/yaml"
)

const (
	disabledError = "cannot handle resolution request, enable-bundles-resolver feature flag not true"
)

func TestGetSelector(t *testing.T) {
	resolver := bundle.Resolver{}
	sel := resolver.GetSelector(context.Background())
	if typ, has := sel[resolutioncommon.LabelKeyResolverType]; !has {
		t.Fatalf("unexpected selector: %v", sel)
	} else if typ != bundle.LabelValueBundleResolverType {
		t.Fatalf("unexpected type: %q", typ)
	}
}

func TestValidateParams(t *testing.T) {
	resolver := bundle.Resolver{}

	paramsWithTask := []pipelinev1.Param{{
		Name:  bundle.ParamKind,
		Value: *pipelinev1.NewStructuredValues("task"),
	}, {
		Name:  bundle.ParamName,
		Value: *pipelinev1.NewStructuredValues("foo"),
	}, {
		Name:  bundle.ParamBundle,
		Value: *pipelinev1.NewStructuredValues("bar"),
	}, {
		Name:  bundle.ParamServiceAccount,
		Value: *pipelinev1.NewStructuredValues("baz"),
	}}

	if err := resolver.ValidateParams(context.Background(), paramsWithTask); err != nil {
		t.Fatalf("unexpected error validating params: %v", err)
	}

	paramsWithPipeline := []pipelinev1.Param{{
		Name:  bundle.ParamKind,
		Value: *pipelinev1.NewStructuredValues("pipeline"),
	}, {
		Name:  bundle.ParamName,
		Value: *pipelinev1.NewStructuredValues("foo"),
	}, {
		Name:  bundle.ParamBundle,
		Value: *pipelinev1.NewStructuredValues("bar"),
	}, {
		Name:  bundle.ParamServiceAccount,
		Value: *pipelinev1.NewStructuredValues("baz"),
	}}
	if err := resolver.ValidateParams(context.Background(), paramsWithPipeline); err != nil {
		t.Fatalf("unexpected error validating params: %v", err)
	}
}

func TestValidateParamsDisabled(t *testing.T) {
	resolver := bundle.Resolver{}

	var err error

	params := []pipelinev1.Param{{
		Name:  bundle.ParamKind,
		Value: *pipelinev1.NewStructuredValues("task"),
	}, {
		Name:  bundle.ParamName,
		Value: *pipelinev1.NewStructuredValues("foo"),
	}, {
		Name:  bundle.ParamBundle,
		Value: *pipelinev1.NewStructuredValues("bar"),
	}, {
		Name:  bundle.ParamServiceAccount,
		Value: *pipelinev1.NewStructuredValues("baz"),
	}}
	err = resolver.ValidateParams(resolverDisabledContext(), params)
	if err == nil {
		t.Fatalf("expected disabled err")
	}

	if d := cmp.Diff(disabledError, err.Error()); d != "" {
		t.Errorf("unexpected error: %s", diff.PrintWantGot(d))
	}
}

func TestValidateParamsMissing(t *testing.T) {
	resolver := bundle.Resolver{}

	var err error

	paramsMissingBundle := []pipelinev1.Param{{
		Name:  bundle.ParamKind,
		Value: *pipelinev1.NewStructuredValues("task"),
	}, {
		Name:  bundle.ParamName,
		Value: *pipelinev1.NewStructuredValues("foo"),
	}, {
		Name:  bundle.ParamServiceAccount,
		Value: *pipelinev1.NewStructuredValues("baz"),
	}}
	err = resolver.ValidateParams(context.Background(), paramsMissingBundle)
	if err == nil {
		t.Fatalf("expected missing kind err")
	}

	paramsMissingName := []pipelinev1.Param{{
		Name:  bundle.ParamKind,
		Value: *pipelinev1.NewStructuredValues("task"),
	}, {
		Name:  bundle.ParamBundle,
		Value: *pipelinev1.NewStructuredValues("bar"),
	}, {
		Name:  bundle.ParamServiceAccount,
		Value: *pipelinev1.NewStructuredValues("baz"),
	}}
	err = resolver.ValidateParams(context.Background(), paramsMissingName)
	if err == nil {
		t.Fatalf("expected missing name err")
	}
}

func TestResolveDisabled(t *testing.T) {
	resolver := bundle.Resolver{}

	var err error

	params := []pipelinev1.Param{{
		Name:  bundle.ParamKind,
		Value: *pipelinev1.NewStructuredValues("task"),
	}, {
		Name:  bundle.ParamName,
		Value: *pipelinev1.NewStructuredValues("foo"),
	}, {
		Name:  bundle.ParamBundle,
		Value: *pipelinev1.NewStructuredValues("bar"),
	}, {
		Name:  bundle.ParamServiceAccount,
		Value: *pipelinev1.NewStructuredValues("baz"),
	}}
	_, err = resolver.Resolve(resolverDisabledContext(), params)
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

	// too many objects in bundle resolver test
	var tooManyObjs []runtime.Object
	for i := 0; i <= bundle.MaximumBundleObjects; i++ {
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

	// Set up a fake registry to push an image to.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	r := fmt.Sprintf("%s/%s", u.Host, "testbundleresolver")
	testImages := map[string]*imageRef{
		"single-task":                     pushToRegistry(t, r, "single-task", []runtime.Object{exampleTask}, test.DefaultObjectAnnotationMapper),
		"single-pipeline":                 pushToRegistry(t, r, "single-pipeline", []runtime.Object{examplePipeline}, test.DefaultObjectAnnotationMapper),
		"multiple-resources":              pushToRegistry(t, r, "multiple-resources", []runtime.Object{exampleTask, examplePipeline}, test.DefaultObjectAnnotationMapper),
		"too-many-objs":                   pushToRegistry(t, r, "too-many-objs", tooManyObjs, asIsMapper),
		"single-task-no-version":          pushToRegistry(t, r, "single-task-no-version", []runtime.Object{&pipelinev1beta1.Task{TypeMeta: metav1.TypeMeta{Kind: "task"}, ObjectMeta: metav1.ObjectMeta{Name: "foo"}}}, asIsMapper),
		"single-task-no-kind":             pushToRegistry(t, r, "single-task-no-kind", []runtime.Object{&pipelinev1beta1.Task{TypeMeta: metav1.TypeMeta{APIVersion: "tekton.dev/v1beta1"}, ObjectMeta: metav1.ObjectMeta{Name: "foo"}}}, asIsMapper),
		"single-task-no-name":             pushToRegistry(t, r, "single-task-no-name", []runtime.Object{&pipelinev1beta1.Task{TypeMeta: metav1.TypeMeta{APIVersion: "tekton.dev/v1beta1", Kind: "task"}}}, asIsMapper),
		"single-task-kind-incorrect-form": pushToRegistry(t, r, "single-task-kind-incorrect-form", []runtime.Object{&pipelinev1beta1.Task{TypeMeta: metav1.TypeMeta{APIVersion: "tekton.dev/v1beta1", Kind: "Task"}, ObjectMeta: metav1.ObjectMeta{Name: "foo"}}}, asIsMapper),
	}

	testcases := []struct {
		name               string
		args               *params
		imageName          string
		kindInBundle       string
		expectedStatus     *v1beta1.ResolutionRequestStatus
		expectedErrMessage string
	}{
		{
			name: "single task: digest is included in the bundle parameter",
			args: &params{
				bundle: fmt.Sprintf("%s@%s:%s", testImages["single-task"].uri, testImages["single-task"].algo, testImages["single-task"].hex),
				name:   "example-task",
				kind:   "task",
			},
			imageName:      "single-task",
			expectedStatus: internal.CreateResolutionRequestStatusWithData(taskAsYAML),
		}, {
			name: "single task: param kind is capitalized, but kind in bundle is not",
			args: &params{
				bundle: fmt.Sprintf("%s@%s:%s", testImages["single-task"].uri, testImages["single-task"].algo, testImages["single-task"].hex),
				name:   "example-task",
				kind:   "Task",
			},
			kindInBundle:   "task",
			imageName:      "single-task",
			expectedStatus: internal.CreateResolutionRequestStatusWithData(taskAsYAML),
		}, {
			name: "single task: tag is included in the bundle parameter",
			args: &params{
				bundle: testImages["single-task"].uri + ":latest",
				name:   "example-task",
				kind:   "task",
			},
			imageName:      "single-task",
			expectedStatus: internal.CreateResolutionRequestStatusWithData(taskAsYAML),
		}, {
			name: "single task: using default kind value from configmap",
			args: &params{
				bundle: testImages["single-task"].uri + ":latest",
				name:   "example-task",
			},
			imageName:      "single-task",
			expectedStatus: internal.CreateResolutionRequestStatusWithData(taskAsYAML),
		}, {
			name: "single pipeline",
			args: &params{
				bundle: testImages["single-pipeline"].uri + ":latest",
				name:   "example-pipeline",
				kind:   "pipeline",
			},
			imageName:      "single-pipeline",
			expectedStatus: internal.CreateResolutionRequestStatusWithData(pipelineAsYAML),
		}, {
			name: "multiple resources: an image has both task and pipeline resource",
			args: &params{
				bundle: testImages["multiple-resources"].uri + ":latest",
				name:   "example-pipeline",
				kind:   "pipeline",
			},
			imageName:      "multiple-resources",
			expectedStatus: internal.CreateResolutionRequestStatusWithData(pipelineAsYAML),
		}, {
			name: "too many objects in an image",
			args: &params{
				bundle: testImages["too-many-objs"].uri + ":latest",
				name:   "2-task",
				kind:   "task",
			},
			expectedStatus:     internal.CreateResolutionRequestFailureStatus(),
			expectedErrMessage: fmt.Sprintf("contained more than the maximum %d allow objects", bundle.MaximumBundleObjects),
		}, {
			name: "single task no version",
			args: &params{
				bundle: testImages["single-task-no-version"].uri + ":latest",
				name:   "foo",
				kind:   "task",
			},
			expectedStatus:     internal.CreateResolutionRequestFailureStatus(),
			expectedErrMessage: fmt.Sprintf("the layer 0 does not contain a %s annotation", bundle.BundleAnnotationAPIVersion),
		}, {
			name: "single task no kind",
			args: &params{
				bundle: testImages["single-task-no-kind"].uri + ":latest",
				name:   "foo",
				kind:   "task",
			},
			expectedStatus:     internal.CreateResolutionRequestFailureStatus(),
			expectedErrMessage: fmt.Sprintf("the layer 0 does not contain a %s annotation", bundle.BundleAnnotationKind),
		}, {
			name: "single task no name",
			args: &params{
				bundle: testImages["single-task-no-name"].uri + ":latest",
				name:   "foo",
				kind:   "task",
			},
			expectedStatus:     internal.CreateResolutionRequestFailureStatus(),
			expectedErrMessage: fmt.Sprintf("the layer 0 does not contain a %s annotation", bundle.BundleAnnotationName),
		}, {
			name: "single task kind incorrect form",
			args: &params{
				bundle: testImages["single-task-kind-incorrect-form"].uri + ":latest",
				name:   "foo",
				kind:   "task",
			},
			expectedStatus:     internal.CreateResolutionRequestFailureStatus(),
			expectedErrMessage: fmt.Sprintf("the layer 0 the annotation %s must be lowercased and singular, found %s", bundle.BundleAnnotationKind, "Task"),
		},
	}

	resolver := &bundle.Resolver{}
	confMap := map[string]string{
		bundle.ConfigKind: "task",
		// service account is not used in testing, but we have to set this since
		// param validation will check if the service account is set either from param or from configmap.
		bundle.ConfigServiceAccount: "placeholder",
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := ttesting.SetupFakeContext(t)

			request := createRequest(tc.args)

			d := test.Data{
				ResolutionRequests: []*v1beta1.ResolutionRequest{request},
				ConfigMaps: []*corev1.ConfigMap{{
					ObjectMeta: metav1.ObjectMeta{
						Name:      bundle.ConfigMapName,
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

					switch {
					case tc.kindInBundle != "":
						expectedStatus.Annotations[bundle.ResolverAnnotationKind] = tc.kindInBundle
					case tc.args.kind != "":
						expectedStatus.Annotations[bundle.ResolverAnnotationKind] = tc.args.kind
					default:
						expectedStatus.Annotations[bundle.ResolverAnnotationKind] = "task"
					}

					expectedStatus.Annotations[bundle.ResolverAnnotationName] = tc.args.name
					expectedStatus.Annotations[bundle.ResolverAnnotationAPIVersion] = "v1beta1"

					expectedStatus.RefSource = &pipelinev1.RefSource{
						URI: testImages[tc.imageName].uri,
						Digest: map[string]string{
							testImages[tc.imageName].algo: testImages[tc.imageName].hex,
						},
						EntryPoint: tc.args.name,
					}
					expectedStatus.Source = expectedStatus.RefSource
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
				resolutioncommon.LabelKeyResolverType: bundle.LabelValueBundleResolverType,
			},
		},
		Spec: v1beta1.ResolutionRequestSpec{
			Params: []pipelinev1.Param{{
				Name:  bundle.ParamBundle,
				Value: *pipelinev1.NewStructuredValues(p.bundle),
			}, {
				Name:  bundle.ParamName,
				Value: *pipelinev1.NewStructuredValues(p.name),
			}, {
				Name:  bundle.ParamKind,
				Value: *pipelinev1.NewStructuredValues(p.kind),
			}, {
				Name:  bundle.ParamServiceAccount,
				Value: *pipelinev1.NewStructuredValues(p.serviceAccount),
			}},
		},
	}
	return rr
}

func createError(image, msg string) error {
	return &resolutioncommon.GetResourceError{
		ResolverName: bundle.BundleResolverName,
		Key:          "foo/rr",
		Original:     fmt.Errorf("invalid tekton bundle %s, error: %s", image, msg),
	}
}

func asIsMapper(obj runtime.Object) map[string]string {
	annotations := map[string]string{}
	if test.GetObjectName(obj) != "" {
		annotations[bundle.BundleAnnotationName] = test.GetObjectName(obj)
	}

	if obj.GetObjectKind().GroupVersionKind().Kind != "" {
		annotations[bundle.BundleAnnotationKind] = obj.GetObjectKind().GroupVersionKind().Kind
	}
	if obj.GetObjectKind().GroupVersionKind().Version != "" {
		annotations[bundle.BundleAnnotationAPIVersion] = obj.GetObjectKind().GroupVersionKind().Version
	}
	return annotations
}

func resolverDisabledContext() context.Context {
	return frtesting.ContextWithBundlesResolverDisabled(context.Background())
}

type imageRef struct {
	// uri is the image repositry identifier i.e. "gcr.io/tekton-releases/catalog/upstream/golang-build"
	uri string
	// algo is the algorithm portion of a particular image digest i.e. "sha256".
	algo string
	// hex is hex encoded portion of a particular image digest i.e. "23293df97dc11957ec36a88c80101bb554039a76e8992a435112eea8283b30d4".
	hex string
}

// pushToRegistry pushes an image to the registry and returns an imageRef.
// It accepts a registry address, image name, the data and an ObjectAnnotationMapper
// to map an object to the annotations for it.
// NOTE: Every image pushed to the registry has a default tag named "latest".
func pushToRegistry(t *testing.T, registry, imageName string, data []runtime.Object, mapper test.ObjectAnnotationMapper) *imageRef {
	t.Helper()
	ref, err := test.CreateImageWithAnnotations(fmt.Sprintf("%s/%s:latest", registry, imageName), mapper, data...)
	if err != nil {
		t.Fatalf("couldn't push the image: %v", err)
	}

	refSplit := strings.Split(ref, "@")
	uri, digest := refSplit[0], refSplit[1]
	digSplits := strings.Split(digest, ":")
	algo, hex := digSplits[0], digSplits[1]

	return &imageRef{
		uri:  uri,
		algo: algo,
		hex:  hex,
	}
}
