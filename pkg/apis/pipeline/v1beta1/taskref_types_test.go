package v1beta1

import "testing"

func TestTaskRef_IsCustomTask(t *testing.T) {
	tests := []struct {
		name string
		tr   *TaskRef
		want bool
	}{{
		name: "not a custom task - apiVersion and Kind are not set",
		tr: &TaskRef{
			Name: "foo",
		},
		want: false,
	}, {
		// related issue: https://github.com/tektoncd/pipeline/issues/6459
		name: "not a custom task - apiVersion is not set",
		tr: &TaskRef{
			Name: "foo",
			Kind: "Example",
		},
		want: false,
	}, {
		name: "not a custom task - kind is not set",
		tr: &TaskRef{
			Name:       "foo",
			APIVersion: "example/v0",
		},
		want: false,
	}, {
		name: "custom task with name",
		tr: &TaskRef{
			Name:       "foo",
			Kind:       "Example",
			APIVersion: "example/v0",
		},
		want: true,
	}, {
		name: "custom task without name",
		tr: &TaskRef{
			Kind:       "Example",
			APIVersion: "example/v0",
		},
		want: true,
	},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tr.IsCustomTask(); got != tt.want {
				t.Errorf("IsCustomTask() = %v, want %v", got, tt.want)
			}
		})
	}
}
