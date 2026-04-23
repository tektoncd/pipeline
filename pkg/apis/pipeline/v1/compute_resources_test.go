package v1

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestComputeResourceRequirements_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		want    ComputeResourceRequirements
		wantErr bool
	}{{
		name: "valid quantities",
		json: `{"requests":{"cpu":"100m","memory":"128Mi"},"limits":{"cpu":"200m","memory":"256Mi"}}`,
		want: ComputeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("200m"),
				corev1.ResourceMemory: resource.MustParse("256Mi"),
			},
		},
	}, {
		name: "param reference in requests",
		json: `{"requests":{"memory":"$(params.MEM)"},"limits":{"memory":"256Mi"}}`,
		want: ComputeResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("256Mi"),
			},
			RawRequests: map[corev1.ResourceName]string{
				corev1.ResourceMemory: "$(params.MEM)",
			},
		},
	}, {
		name: "all param references",
		json: `{"requests":{"cpu":"$(params.CPU)","memory":"$(params.MEM)"},"limits":{"cpu":"$(params.CPU_LIMIT)"}}`,
		want: ComputeResourceRequirements{
			RawRequests: map[corev1.ResourceName]string{
				corev1.ResourceCPU:    "$(params.CPU)",
				corev1.ResourceMemory: "$(params.MEM)",
			},
			RawLimits: map[corev1.ResourceName]string{
				corev1.ResourceCPU: "$(params.CPU_LIMIT)",
			},
		},
	}, {
		name: "task result reference",
		json: `{"requests":{"memory":"$(tasks.get-config.results.memory)"}}`,
		want: ComputeResourceRequirements{
			RawRequests: map[corev1.ResourceName]string{
				corev1.ResourceMemory: "$(tasks.get-config.results.memory)",
			},
		},
	}, {
		name: "empty",
		json: `{}`,
		want: ComputeResourceRequirements{},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got ComputeResourceRequirements
			err := json.Unmarshal([]byte(tt.json), &got)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Errorf("UnmarshalJSON() diff (-want +got):\n%s", d)
			}
		})
	}
}

func TestComputeResourceRequirements_MarshalJSON(t *testing.T) {
	tests := []struct {
		name  string
		input ComputeResourceRequirements
		want  map[string]interface{} // compare as maps to avoid ordering issues
	}{{
		name: "valid quantities roundtrip",
		input: ComputeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("100m"),
			},
		},
		want: map[string]interface{}{
			"requests": map[string]interface{}{"cpu": "100m"},
		},
	}, {
		name: "raw values serialize as strings",
		input: ComputeResourceRequirements{
			RawRequests: map[corev1.ResourceName]string{
				corev1.ResourceMemory: "$(params.MEM)",
			},
		},
		want: map[string]interface{}{
			"requests": map[string]interface{}{"memory": "$(params.MEM)"},
		},
	}, {
		name: "mixed parsed and raw",
		input: ComputeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("100m"),
			},
			RawRequests: map[corev1.ResourceName]string{
				corev1.ResourceMemory: "$(params.MEM)",
			},
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("256Mi"),
			},
		},
		want: map[string]interface{}{
			"requests": map[string]interface{}{
				"cpu":    "100m",
				"memory": "$(params.MEM)",
			},
			"limits": map[string]interface{}{
				"memory": "256Mi",
			},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("MarshalJSON() error: %v", err)
			}
			var gotMap map[string]interface{}
			if err := json.Unmarshal(got, &gotMap); err != nil {
				t.Fatalf("failed to parse marshaled JSON: %v", err)
			}
			if d := cmp.Diff(tt.want, gotMap); d != "" {
				t.Errorf("MarshalJSON() diff (-want +got):\n%s", d)
			}
		})
	}
}

func TestComputeResourceRequirements_ToK8s(t *testing.T) {
	tests := []struct {
		name    string
		input   ComputeResourceRequirements
		want    corev1.ResourceRequirements
		wantErr bool
	}{{
		name: "fully resolved",
		input: ComputeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("100m"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("256Mi"),
			},
		},
		want: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("100m"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("256Mi"),
			},
		},
	}, {
		name: "unresolved param reference",
		input: ComputeResourceRequirements{
			RawRequests: map[corev1.ResourceName]string{
				corev1.ResourceMemory: "$(params.MEM)",
			},
		},
		wantErr: true,
	}, {
		name:  "empty is valid",
		input: ComputeResourceRequirements{},
		want:  corev1.ResourceRequirements{},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.input.ToK8s()
			if (err != nil) != tt.wantErr {
				t.Errorf("ToK8s() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if d := cmp.Diff(tt.want, got); d != "" {
					t.Errorf("ToK8s() diff (-want +got):\n%s", d)
				}
			}
		})
	}
}

func TestComputeResourceRequirements_ApplyReplacements(t *testing.T) {
	input := ComputeResourceRequirements{
		RawRequests: map[corev1.ResourceName]string{
			corev1.ResourceMemory: "$(params.MEM)",
			corev1.ResourceCPU:    "$(params.CPU)",
		},
		RawLimits: map[corev1.ResourceName]string{
			corev1.ResourceMemory: "$(params.MEM_LIMIT)",
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse("500m"),
		},
	}

	replacements := map[string]string{
		"params.MEM":       "128Mi",
		"params.CPU":       "100m",
		"params.MEM_LIMIT": "256Mi",
	}

	got := input.ApplyReplacements(replacements)

	want := ComputeResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("128Mi"),
			corev1.ResourceCPU:    resource.MustParse("100m"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("500m"),
			corev1.ResourceMemory: resource.MustParse("256Mi"),
		},
	}

	if d := cmp.Diff(want, got); d != "" {
		t.Errorf("ApplyReplacements() diff (-want +got):\n%s", d)
	}
	if got.HasUnresolvedReferences() {
		t.Error("expected all references to be resolved")
	}
}

func TestComputeResourceRequirements_ApplyReplacements_Partial(t *testing.T) {
	input := ComputeResourceRequirements{
		RawRequests: map[corev1.ResourceName]string{
			corev1.ResourceMemory: "$(params.MEM)",
			corev1.ResourceCPU:    "$(params.CPU)",
		},
	}

	// Only resolve one of the two
	replacements := map[string]string{
		"params.MEM": "128Mi",
	}

	got := input.ApplyReplacements(replacements)

	if !got.HasUnresolvedReferences() {
		t.Error("expected unresolved references to remain")
	}
	wantRequests := corev1.ResourceList{
		corev1.ResourceMemory: resource.MustParse("128Mi"),
	}
	if d := cmp.Diff(wantRequests, got.Requests); d != "" {
		t.Errorf("Requests diff (-want +got):\n%s", d)
	}
	if got.RawRequests[corev1.ResourceCPU] != "$(params.CPU)" {
		t.Errorf("expected CPU to remain unresolved, got %q", got.RawRequests[corev1.ResourceCPU])
	}
}

func TestComputeResourceRequirements_IsZero(t *testing.T) {
	if !(ComputeResourceRequirements{}).IsZero() {
		t.Error("empty should be zero")
	}
	if (ComputeResourceRequirements{
		Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
	}).IsZero() {
		t.Error("with requests should not be zero")
	}
	if (ComputeResourceRequirements{
		RawRequests: map[corev1.ResourceName]string{corev1.ResourceCPU: "$(params.CPU)"},
	}).IsZero() {
		t.Error("with raw requests should not be zero")
	}
}

func TestComputeResourceRequirements_JSONRoundTrip(t *testing.T) {
	// Verify that marshal → unmarshal produces identical results
	original := ComputeResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse("100m"),
		},
		RawRequests: map[corev1.ResourceName]string{
			corev1.ResourceMemory: "$(params.MEM)",
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("256Mi"),
		},
		RawLimits: map[corev1.ResourceName]string{
			corev1.ResourceCPU: "$(params.CPU_LIMIT)",
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var roundTripped ComputeResourceRequirements
	if err := json.Unmarshal(data, &roundTripped); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if d := cmp.Diff(original, roundTripped); d != "" {
		t.Errorf("Round trip diff (-original +roundTripped):\n%s", d)
	}
}

func TestFromK8sResourceRequirements(t *testing.T) {
	k8s := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse("100m"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("256Mi"),
		},
	}
	got := FromK8sResourceRequirements(k8s)
	if d := cmp.Diff(k8s.Requests, got.Requests); d != "" {
		t.Errorf("Requests diff: %s", d)
	}
	if d := cmp.Diff(k8s.Limits, got.Limits); d != "" {
		t.Errorf("Limits diff: %s", d)
	}
	if got.HasUnresolvedReferences() {
		t.Error("k8s conversion should not have unresolved references")
	}
}

func TestComputeResourceRequirements_ApplyReplacements_InvalidQuantity(t *testing.T) {
	input := ComputeResourceRequirements{
		RawRequests: map[corev1.ResourceName]string{
			corev1.ResourceMemory: "$(params.MEM)",
		},
	}

	// Substituting an invalid quantity leaves it in RawRequests
	replacements := map[string]string{
		"params.MEM": "notanumber",
	}

	got := input.ApplyReplacements(replacements)

	if !got.HasUnresolvedReferences() {
		t.Error("expected unresolved references for invalid quantity")
	}
	if got.RawRequests[corev1.ResourceMemory] != "notanumber" {
		t.Errorf("expected raw value 'notanumber', got %q", got.RawRequests[corev1.ResourceMemory])
	}

	// ToK8s should fail with a clear error
	_, err := got.ToK8s()
	if err == nil {
		t.Fatal("expected error from ToK8s with unresolved references")
	}
	if !strings.Contains(err.Error(), "unresolved variable references") {
		t.Errorf("expected 'unresolved variable references' error, got: %v", err)
	}
}

func TestComputeResourceRequirements_ApplyReplacements_PartialSubstitution(t *testing.T) {
	// Partial substitution like "$(params.prefix)-100m" produces an invalid quantity
	input := ComputeResourceRequirements{
		RawRequests: map[corev1.ResourceName]string{
			corev1.ResourceCPU: "$(params.prefix)-100m",
		},
	}

	replacements := map[string]string{
		"params.prefix": "some",
	}

	got := input.ApplyReplacements(replacements)

	// "some-100m" is not a valid quantity, so it stays in RawRequests
	if !got.HasUnresolvedReferences() {
		t.Error("expected unresolved references for partial substitution result")
	}
	if got.RawRequests[corev1.ResourceCPU] != "some-100m" {
		t.Errorf("expected raw value 'some-100m', got %q", got.RawRequests[corev1.ResourceCPU])
	}
}

func TestComputeResourceRequirements_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   ComputeResourceRequirements
		wantErr bool
	}{{
		name: "valid variable reference",
		input: ComputeResourceRequirements{
			RawRequests: map[corev1.ResourceName]string{
				corev1.ResourceMemory: "$(params.MEM)",
			},
		},
		wantErr: false,
	}, {
		name: "invalid raw value - not a variable reference",
		input: ComputeResourceRequirements{
			RawRequests: map[corev1.ResourceName]string{
				corev1.ResourceMemory: "notaref",
			},
		},
		wantErr: true,
	}, {
		name:    "no raw values - valid",
		input:   ComputeResourceRequirements{},
		wantErr: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := tt.input.Validate("computeResources")
			if (errs != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", errs, tt.wantErr)
			}
		})
	}
}
