package apiserver_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned/fake"
	"github.com/tektoncd/pipeline/pkg/reconciler/apiserver"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ktesting "k8s.io/client-go/testing"
)

func TestDryRunCreate_Valid_DifferentGVKs(t *testing.T) {
	tcs := []struct {
		name    string
		obj     runtime.Object
		wantErr bool
		wantObj runtime.Object
	}{{
		name:    "v1 task",
		obj:     &v1.Task{},
		wantObj: &v1.Task{},
	}, {
		name:    "v1beta1 task",
		obj:     &v1beta1.Task{},
		wantObj: &v1beta1.Task{},
	}, {
		name:    "v1 pipeline",
		obj:     &v1.Pipeline{},
		wantObj: &v1.Pipeline{},
	}, {
		name:    "v1beta1 pipeline",
		obj:     &v1beta1.Pipeline{},
		wantObj: &v1beta1.Pipeline{},
	}, {
		name:    "v1alpha1 stepaction",
		obj:     &v1alpha1.StepAction{},
		wantObj: &v1alpha1.StepAction{},
	}, {
		name:    "v1beta1 stepaction",
		obj:     &v1beta1.StepAction{},
		wantObj: &v1beta1.StepAction{},
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			tektonclient := fake.NewSimpleClientset()
			mutatedObj, err := apiserver.DryRunValidate(context.Background(), "default", tc.obj, tektonclient)
			if (err != nil) != tc.wantErr {
				t.Errorf("wantErr was %t but got err %v", tc.wantErr, err)
			}
			if d := cmp.Diff(tc.wantObj, mutatedObj, cmpopts.IgnoreFields(metav1.ObjectMeta{}, "Name", "Namespace")); d != "" {
				t.Errorf("wrong object: %s", d)
			}
		})
	}
}

func TestDryRunCreate_Invalid_DifferentGVKs(t *testing.T) {
	tcs := []struct {
		name    string
		obj     runtime.Object
		wantErr error
	}{{
		name:    "v1 task",
		obj:     &v1.Task{},
		wantErr: apiserver.ErrReferencedObjectValidationFailed,
	}, {
		name:    "v1beta1 task",
		obj:     &v1beta1.Task{},
		wantErr: apiserver.ErrReferencedObjectValidationFailed,
	}, {
		name:    "v1 pipeline",
		obj:     &v1.Pipeline{},
		wantErr: apiserver.ErrReferencedObjectValidationFailed,
	}, {
		name:    "v1beta1 pipeline",
		obj:     &v1beta1.Pipeline{},
		wantErr: apiserver.ErrReferencedObjectValidationFailed,
	}, {
		name:    "v1alpha1 stepaction",
		obj:     &v1alpha1.StepAction{},
		wantErr: apiserver.ErrReferencedObjectValidationFailed,
	}, {
		name:    "v1beta1 stepaction",
		obj:     &v1beta1.StepAction{},
		wantErr: apiserver.ErrReferencedObjectValidationFailed,
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			tektonclient := fake.NewSimpleClientset()
			tektonclient.PrependReactor("create", "tasks", func(action ktesting.Action) (bool, runtime.Object, error) {
				return true, nil, apierrors.NewBadRequest("bad request")
			})
			tektonclient.PrependReactor("create", "pipelines", func(action ktesting.Action) (bool, runtime.Object, error) {
				return true, nil, apierrors.NewBadRequest("bad request")
			})
			tektonclient.PrependReactor("create", "stepactions", func(action ktesting.Action) (bool, runtime.Object, error) {
				return true, nil, apierrors.NewBadRequest("bad request")
			})
			_, err := apiserver.DryRunValidate(context.Background(), "default", tc.obj, tektonclient)
			if d := cmp.Diff(tc.wantErr, err, cmpopts.EquateErrors()); d != "" {
				t.Errorf("wrong error: %s", d)
			}
		})
	}
}

func TestDryRunCreate_DifferentErrTypes(t *testing.T) {
	tcs := []struct {
		name             string
		webhookErr       error
		wantErr          error
		wantTransientErr bool
	}{{
		name:    "no error",
		wantErr: nil,
	}, {
		name:             "bad request",
		webhookErr:       apierrors.NewBadRequest("bad request"),
		wantErr:          apiserver.ErrReferencedObjectValidationFailed,
		wantTransientErr: false,
	}, {
		name:             "invalid",
		webhookErr:       apierrors.NewInvalid(schema.GroupKind{Group: "tekton.dev/v1", Kind: "Task"}, "task", field.ErrorList{}),
		wantErr:          apiserver.ErrCouldntValidateObjectPermanent,
		wantTransientErr: false,
	}, {
		name:             "not supported",
		webhookErr:       apierrors.NewMethodNotSupported(schema.GroupResource{}, "create"),
		wantErr:          apiserver.ErrCouldntValidateObjectPermanent,
		wantTransientErr: false,
	}, {
		name:             "timeout",
		webhookErr:       apierrors.NewTimeoutError("timeout", 5),
		wantErr:          apiserver.ErrCouldntValidateObjectRetryable,
		wantTransientErr: true,
	}, {
		name:             "server timeout",
		webhookErr:       apierrors.NewServerTimeout(schema.GroupResource{}, "create", 5),
		wantErr:          apiserver.ErrCouldntValidateObjectRetryable,
		wantTransientErr: true,
	}, {
		name:             "too many requests",
		webhookErr:       apierrors.NewTooManyRequests("foo", 5),
		wantErr:          apiserver.ErrCouldntValidateObjectRetryable,
		wantTransientErr: true,
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			tektonclient := fake.NewSimpleClientset()
			tektonclient.PrependReactor("create", "tasks", func(action ktesting.Action) (bool, runtime.Object, error) {
				return true, nil, tc.webhookErr
			})
			_, err := apiserver.DryRunValidate(context.Background(), "default", &v1.Task{}, tektonclient)
			if d := cmp.Diff(tc.wantErr, err, cmpopts.EquateErrors()); d != "" {
				t.Errorf("wrong error: %s", d)
			}
			if tc.wantErr != nil {
				errIsTransient := resolutioncommon.IsErrTransient(err)
				if tc.wantTransientErr != errIsTransient {
					errMode := "permanent"
					if tc.wantTransientErr {
						errMode = "transient"
					}
					if tc.wantTransientErr {
						t.Errorf("expected %s error but received error was not %s: %v", errMode, errMode, err)
					}
				}
			}
		})
	}
}
