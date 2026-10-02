/*
Copyright 2025 The Tekton Authors

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
package v1

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/tektoncd/pipeline/pkg/substitution"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"knative.dev/pkg/apis"
)

// ComputeResourceRequirements is a Tekton-owned wrapper around corev1.ResourceRequirements
// that supports variable substitutions (e.g. $(params.MEM)) in resource quantity values.
//
// CRD schema uses x-kubernetes-preserve-unknown-fields: true so the apiserver won't
// prune unresolved $(…) variable references at storage time. Validation of these values
// is performed by the admission webhook via Validate(). A future enhancement could add
// x-kubernetes-validations (CEL) rules for richer server-side validation (requires K8s 1.29+).
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
// Values use json.RawMessage to accept both strings ("128Mi", "$(params.MEM)")
// and numbers (500) which are valid in Kubernetes resource quantities.
type resourceRequirementsJSON struct {
	Requests map[corev1.ResourceName]json.RawMessage `json:"requests,omitempty"`
	Limits   map[corev1.ResourceName]json.RawMessage `json:"limits,omitempty"`
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

	r.Requests, r.RawRequests = splitResourceMap(rawMessageMapToStringMap(raw.Requests))
	r.Limits, r.RawLimits = splitResourceMap(rawMessageMapToStringMap(raw.Limits))
	return nil
}

// rawMessageMapToStringMap converts a map of json.RawMessage values to strings,
// handling both quoted strings and bare numbers (e.g. cpu: 500).
func rawMessageMapToStringMap(m map[corev1.ResourceName]json.RawMessage) map[corev1.ResourceName]string {
	if len(m) == 0 {
		return nil
	}
	result := make(map[corev1.ResourceName]string, len(m))
	for k, v := range m {
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			result[k] = s
		} else {
			// Bare number (e.g. 500) — use raw JSON as the string value
			result[k] = string(v)
		}
	}
	return result
}

// ToK8s converts to corev1.ResourceRequirements.
// Returns an error if any values are not valid resource quantities — either because
// they still contain variable references, or because a substituted value did not
// parse as a quantity.
func (r ComputeResourceRequirements) ToK8s() (corev1.ResourceRequirements, error) {
	if r.HasUnresolvedReferences() {
		var unresolved []string
		for k, v := range r.RawRequests {
			unresolved = append(unresolved, fmt.Sprintf("requests.%s=%s", k, v))
		}
		for k, v := range r.RawLimits {
			unresolved = append(unresolved, fmt.Sprintf("limits.%s=%s", k, v))
		}
		// Sort for a deterministic error message: map iteration order is random.
		sort.Strings(unresolved)
		return corev1.ResourceRequirements{}, fmt.Errorf("invalid compute resources, values are not valid quantities: %s", strings.Join(unresolved, ", "))
	}
	return corev1.ResourceRequirements{
		Requests: r.Requests,
		Limits:   r.Limits,
	}, nil
}

// MustToK8s converts to corev1.ResourceRequirements, dropping any values that are
// not valid quantities. Only the parsed Requests/Limits are returned.
//
// This is lossy by design and must only be used for intermediate conversions where
// the caller does not care about unresolved variable references (e.g. rendering a
// Container for a strategic merge patch, or down-converting to v1beta1 where the
// raw values are preserved separately). Anything that builds a real Pod must use
// ToK8s() so that invalid values surface as an error instead of silently becoming
// "no resource requirements at all".
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
	for _, k := range sortedResourceNames(r.RawRequests) {
		if !isVariableReference(r.RawRequests[k]) {
			errs = errs.Also(apis.ErrInvalidValue(r.RawRequests[k], fmt.Sprintf("%s.requests.%s", fieldPath, k),
				"must be a valid quantity or a variable reference like $(params.name)"))
		}
	}
	for _, k := range sortedResourceNames(r.RawLimits) {
		if !isVariableReference(r.RawLimits[k]) {
			errs = errs.Also(apis.ErrInvalidValue(r.RawLimits[k], fmt.Sprintf("%s.limits.%s", fieldPath, k),
				"must be a valid quantity or a variable reference like $(params.name)"))
		}
	}
	return errs
}

// variableReferenceRegex matches a well-formed Tekton variable reference such as
// $(params.MEM), $(params["my.mem"]) or $(tasks.a.results.b).
var variableReferenceRegex = regexp.MustCompile(`\$\([-._a-zA-Z0-9\[\]"'*]+\)`)

// isVariableReference reports whether s is a well-formed value containing variable
// references. A value is accepted when it holds at least one complete reference and
// no leftover "$(" once the references are removed — so "$(params.SIZE)Mi" is valid
// (a reference concatenated with a quantity suffix) while "$(params.SIZE" is not.
func isVariableReference(s string) bool {
	if !variableReferenceRegex.MatchString(s) {
		return false
	}
	return !strings.Contains(variableReferenceRegex.ReplaceAllString(s, ""), "$(")
}

// Merge returns the result of applying override on top of r. Values are merged per
// resource name, with override winning, and raw (unresolved) values are preserved on
// both sides. A resource name set in override replaces the value from r regardless of
// whether either side holds a parsed quantity or a raw variable reference.
func (r ComputeResourceRequirements) Merge(override ComputeResourceRequirements) ComputeResourceRequirements {
	out := ComputeResourceRequirements{}
	out.Requests, out.RawRequests = mergeInto(r.Requests, r.RawRequests, override.Requests, override.RawRequests)
	out.Limits, out.RawLimits = mergeInto(r.Limits, r.RawLimits, override.Limits, override.RawLimits)
	return out
}

// mergeInto merges a base parsed/raw pair with an override parsed/raw pair.
func mergeInto(baseParsed corev1.ResourceList, baseRaw map[corev1.ResourceName]string, overrideParsed corev1.ResourceList, overrideRaw map[corev1.ResourceName]string) (corev1.ResourceList, map[corev1.ResourceName]string) {
	parsed := baseParsed.DeepCopy()
	var raw map[corev1.ResourceName]string
	for k, v := range baseRaw {
		if raw == nil {
			raw = map[corev1.ResourceName]string{}
		}
		raw[k] = v
	}
	// An override value replaces the base value for that resource name, whichever
	// side of the parsed/raw split it currently lives on.
	for k, v := range overrideParsed {
		if parsed == nil {
			parsed = corev1.ResourceList{}
		}
		parsed[k] = v
		delete(raw, k)
	}
	for k, v := range overrideRaw {
		if raw == nil {
			raw = map[corev1.ResourceName]string{}
		}
		raw[k] = v
		delete(parsed, k)
	}
	if len(parsed) == 0 {
		parsed = nil
	}
	if len(raw) == 0 {
		raw = nil
	}
	return parsed, raw
}

// sortedResourceNames returns the keys of m in a stable order.
func sortedResourceNames(m map[corev1.ResourceName]string) []corev1.ResourceName {
	keys := make([]corev1.ResourceName, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
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
func mergeResourceMaps(parsed corev1.ResourceList, raw map[corev1.ResourceName]string) map[corev1.ResourceName]json.RawMessage {
	if len(parsed) == 0 && len(raw) == 0 {
		return nil
	}
	out := make(map[corev1.ResourceName]json.RawMessage, len(parsed)+len(raw))
	for k, v := range parsed {
		// Marshal quantity as a quoted string
		b, err := json.Marshal(v.String())
		if err != nil {
			return nil
		}
		out[k] = b
	}
	for k, v := range raw {
		b, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		out[k] = b
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
