package v1

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tektoncd/pipeline/pkg/substitution"
	"knative.dev/pkg/apis"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// ComputeResourceRequirements is a Tekton-owned wrapper around corev1.ResourceRequirements
// that supports variable substitutions (e.g. $(params.MEM)) in resource quantity values.
//
// CRD schema validation uses a relaxed pattern that accepts both valid quantities and
// $(…) variable references. An alternative would be to use x-kubernetes-validations (CEL)
// rules in the CRD for more expressive server-side validation (requires K8s 1.29+).
//
// When a value contains a variable reference, it is stored in RawRequests/RawLimits as a string.
// When a value is a valid quantity, it is stored in Requests/Limits as a parsed resource.Quantity.
// After variable substitution via ApplyReplacements(), raw values become parsed quantities.
type ComputeResourceRequirements struct {
	// Requests holds parsed resource quantities (no variable references).
	Requests corev1.ResourceList `json:"-"`
	// Limits holds parsed resource quantities (no variable references).
	Limits corev1.ResourceList `json:"-"`
	// RawRequests holds string values that contain variable references.
	RawRequests map[corev1.ResourceName]string `json:"-"`
	// RawLimits holds string values that contain variable references.
	RawLimits map[corev1.ResourceName]string `json:"-"`
}

// IsZero returns true if no resources are configured.
func (r ComputeResourceRequirements) IsZero() bool {
	return len(r.Requests) == 0 && len(r.Limits) == 0 &&
		len(r.RawRequests) == 0 && len(r.RawLimits) == 0
}

// HasUnresolvedReferences returns true if any values contain variable references
// that have not yet been substituted.
func (r ComputeResourceRequirements) HasUnresolvedReferences() bool {
	return len(r.RawRequests) > 0 || len(r.RawLimits) > 0
}

// resourceRequirementsJSON is used for JSON serialization.
type resourceRequirementsJSON struct {
	Requests map[corev1.ResourceName]string `json:"requests,omitempty"`
	Limits   map[corev1.ResourceName]string `json:"limits,omitempty"`
}

// MarshalJSON serializes ComputeResourceRequirements to JSON.
// Parsed quantities are serialized as their string representation,
// raw values (with variable references) are serialized as-is.
func (r ComputeResourceRequirements) MarshalJSON() ([]byte, error) {
	out := resourceRequirementsJSON{
		Requests: mergeResourceMaps(r.Requests, r.RawRequests),
		Limits:   mergeResourceMaps(r.Limits, r.RawLimits),
	}
	return json.Marshal(out)
}

// UnmarshalJSON deserializes JSON into ComputeResourceRequirements.
// Values that are valid resource.Quantity are parsed and stored in Requests/Limits.
// Values containing variable references are stored in RawRequests/RawLimits.
func (r *ComputeResourceRequirements) UnmarshalJSON(data []byte) error {
	var raw resourceRequirementsJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	r.Requests, r.RawRequests = splitResourceMap(raw.Requests)
	r.Limits, r.RawLimits = splitResourceMap(raw.Limits)
	return nil
}

// ToK8s converts to corev1.ResourceRequirements.
// Returns an error if any values contain unresolved variable references.
func (r ComputeResourceRequirements) ToK8s() (corev1.ResourceRequirements, error) {
	if r.HasUnresolvedReferences() {
		var unresolved []string
		for k, v := range r.RawRequests {
			unresolved = append(unresolved, fmt.Sprintf("requests.%s=%s", k, v))
		}
		for k, v := range r.RawLimits {
			unresolved = append(unresolved, fmt.Sprintf("limits.%s=%s", k, v))
		}
		return corev1.ResourceRequirements{}, fmt.Errorf("unresolved variable references in compute resources: %s", strings.Join(unresolved, ", "))
	}
	return corev1.ResourceRequirements{
		Requests: r.Requests,
		Limits:   r.Limits,
	}, nil
}

// mustToK8s converts to corev1.ResourceRequirements, ignoring unresolved references.
// Only the parsed Requests/Limits are returned. This is used internally for
// intermediate Container conversions where variable references haven't been
// resolved yet (e.g. during ToK8sContainer() calls before substitution).
func (r ComputeResourceRequirements) MustToK8s() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: r.Requests,
		Limits:   r.Limits,
	}
}

// FromK8sResourceRequirements creates a ComputeResourceRequirements from corev1.ResourceRequirements.
func FromK8sResourceRequirements(k8s corev1.ResourceRequirements) ComputeResourceRequirements {
	return ComputeResourceRequirements{
		Requests: k8s.Requests,
		Limits:   k8s.Limits,
	}
}

// ApplyReplacements performs variable substitution on raw (unresolved) values.
// Successfully resolved values are moved from RawRequests/RawLimits to Requests/Limits.
// Values that still contain variable references after substitution remain in Raw*.
// This method does not mutate the receiver — it returns a new ComputeResourceRequirements.
func (r ComputeResourceRequirements) ApplyReplacements(replacements map[string]string) ComputeResourceRequirements {
	result := ComputeResourceRequirements{
		Requests: r.Requests.DeepCopy(),
		Limits:   r.Limits.DeepCopy(),
	}

	result.Requests, result.RawRequests = resolveRawMap(r.RawRequests, replacements, result.Requests)
	result.Limits, result.RawLimits = resolveRawMap(r.RawLimits, replacements, result.Limits)

	return result
}

// Validate checks that any raw (non-quantity) values are well-formed variable references.
// Returns errors for values that are neither valid quantities nor variable references.
func (r ComputeResourceRequirements) Validate(fieldPath string) *apis.FieldError {
	var errs *apis.FieldError
	for k, v := range r.RawRequests {
		if !looksLikeVariableReference(v) {
			errs = errs.Also(apis.ErrInvalidValue(v, fmt.Sprintf("%s.requests.%s", fieldPath, k),
				"must be a valid quantity or a variable reference like $(params.name)"))
		}
	}
	for k, v := range r.RawLimits {
		if !looksLikeVariableReference(v) {
			errs = errs.Also(apis.ErrInvalidValue(v, fmt.Sprintf("%s.limits.%s", fieldPath, k),
				"must be a valid quantity or a variable reference like $(params.name)"))
		}
	}
	return errs
}

// looksLikeVariableReference checks if a string looks like a Tekton variable reference.
func looksLikeVariableReference(s string) bool {
	return strings.HasPrefix(s, "$(") && strings.HasSuffix(s, ")")
}

// DeepCopy returns a deep copy of ComputeResourceRequirements.
func (r ComputeResourceRequirements) DeepCopy() ComputeResourceRequirements {
	out := ComputeResourceRequirements{
		Requests: r.Requests.DeepCopy(),
		Limits:   r.Limits.DeepCopy(),
	}
	if r.RawRequests != nil {
		out.RawRequests = make(map[corev1.ResourceName]string, len(r.RawRequests))
		for k, v := range r.RawRequests {
			out.RawRequests[k] = v
		}
	}
	if r.RawLimits != nil {
		out.RawLimits = make(map[corev1.ResourceName]string, len(r.RawLimits))
		for k, v := range r.RawLimits {
			out.RawLimits[k] = v
		}
	}
	return out
}

// DeepCopyInto copies all properties into another ComputeResourceRequirements.
func (r ComputeResourceRequirements) DeepCopyInto(out *ComputeResourceRequirements) {
	*out = r.DeepCopy()
}

// splitResourceMap separates a map of string values into parsed quantities and raw strings.
func splitResourceMap(m map[corev1.ResourceName]string) (corev1.ResourceList, map[corev1.ResourceName]string) {
	if len(m) == 0 {
		return nil, nil
	}
	var parsed corev1.ResourceList
	var raw map[corev1.ResourceName]string

	for k, v := range m {
		q, err := resource.ParseQuantity(v)
		if err == nil {
			if parsed == nil {
				parsed = corev1.ResourceList{}
			}
			parsed[k] = q
		} else {
			if raw == nil {
				raw = map[corev1.ResourceName]string{}
			}
			raw[k] = v
		}
	}
	return parsed, raw
}

// mergeResourceMaps merges parsed quantities and raw strings into a single string map for serialization.
func mergeResourceMaps(parsed corev1.ResourceList, raw map[corev1.ResourceName]string) map[corev1.ResourceName]string {
	if len(parsed) == 0 && len(raw) == 0 {
		return nil
	}
	out := make(map[corev1.ResourceName]string, len(parsed)+len(raw))
	for k, v := range parsed {
		out[k] = v.String()
	}
	for k, v := range raw {
		out[k] = v
	}
	return out
}

// resolveRawMap applies replacements to raw values and merges successfully parsed results into the existing list.
func resolveRawMap(raw map[corev1.ResourceName]string, replacements map[string]string, existing corev1.ResourceList) (corev1.ResourceList, map[corev1.ResourceName]string) {
	if len(raw) == 0 {
		return existing, nil
	}
	var remaining map[corev1.ResourceName]string
	for k, v := range raw {
		resolved := substitution.ApplyReplacements(v, replacements)
		q, err := resource.ParseQuantity(resolved)
		if err == nil {
			if existing == nil {
				existing = corev1.ResourceList{}
			}
			existing[k] = q
		} else {
			if remaining == nil {
				remaining = map[corev1.ResourceName]string{}
			}
			remaining[k] = resolved
		}
	}
	return existing, remaining
}
