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

package v1alpha1

import (
	"crypto"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// +genclient
// +genclient:noStatus
// +genreconciler:krshapedlogic=false
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VerificationPolicy defines the rules to verify Tekton resources.
// VerificationPolicy can config the mapping from resources to a list of public
// keys, so when verifying the resources we can use the corresponding public keys.
// +k8s:openapi-gen=true
type VerificationPolicy struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata"`

	// Spec holds the desired state of the VerificationPolicy.
	Spec VerificationPolicySpec `json:"spec"`
}

// VerificationPolicyList contains a list of VerificationPolicy
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type VerificationPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VerificationPolicy `json:"items"`
}

// GetGroupVersionKind implements kmeta.OwnerRefable.
func (*VerificationPolicy) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("VerificationPolicy")
}

// VerificationPolicySpec defines the patterns and authorities.
type VerificationPolicySpec struct {
	// Resources defines the patterns of resources sources that should be subject to this policy.
	// For example, we may want to apply this Policy from a certain GitHub repo.
	// Then the ResourcesPattern should be valid regex. E.g. If using gitresolver, and we want to config keys from a certain git repo.
	// `ResourcesPattern` can be `https://github.com/tektoncd/catalog.git`, we will use regex to filter out those resources.
	Resources []ResourcePattern `json:"resources"`
	// Authorities defines the rules for validating signatures.
	Authorities []Authority `json:"authorities"`
	// Mode controls whether a failing policy will fail the taskrun/pipelinerun, or only log the warnings
	// enforce - fail the taskrun/pipelinerun if verification fails (default)
	// warn - don't fail the taskrun/pipelinerun if verification fails but log warnings
	// +optional
	Mode ModeType `json:"mode,omitempty"`
}

// ResourcePattern defines the pattern of the resource source
type ResourcePattern struct {
	// Pattern defines a resource pattern. Regex is created to filter resources based on `Pattern`
	// Example patterns:
	// GitHub resource: https://github.com/tektoncd/catalog.git, https://github.com/tektoncd/*
	// Bundle resource: gcr.io/tekton-releases/catalog/upstream/git-clone, gcr.io/tekton-releases/catalog/upstream/*
	// Hub resource: https://artifacthub.io/*,
	Pattern string `json:"pattern"`
}

// The Authority block defines the keys for validating signatures.
type Authority struct {
	// Name is the name for this authority.
	Name string `json:"name"`
	// Key contains the public key to validate the resource.
	Key *KeyRef `json:"key,omitempty"`
}

// ModeType indicates the type of a mode for VerificationPolicy
type ModeType string

// Valid ModeType:
const (
	ModeWarn    ModeType = "warn"
	ModeEnforce ModeType = "enforce"
)

// KeyRef defines the reference to a public key
type KeyRef struct {
	// SecretRef sets a reference to a secret with the key.
	// +optional
	SecretRef *v1.SecretReference `json:"secretRef,omitempty"`
	// Data contains the inline public key.
	// +optional
	Data string `json:"data,omitempty"`
	// KMS contains the KMS url of the public key
	// Supported formats differ based on the KMS system used.
	// One example of a KMS url could be:
	// gcpkms://projects/[PROJECT]/locations/[LOCATION]>/keyRings/[KEYRING]/cryptoKeys/[KEY]/cryptoKeyVersions/[KEY_VERSION]
	// For more examples please refer https://docs.sigstore.dev/cosign/kms_support.
	// Note that the KMS is not supported yet.
	// +optional
	KMS string `json:"kms,omitempty"`
	// HashAlgorithm always defaults to sha256 if the algorithm hasn't been explicitly set
	// +optional
	HashAlgorithm HashAlgorithm `json:"hashAlgorithm,omitempty"`
}

// HashAlgorithm defines the hash algorithm used for the public key
type HashAlgorithm string

const (
	sha224 HashAlgorithm = "sha224"
	sha256 HashAlgorithm = "sha256"
	sha384 HashAlgorithm = "sha384"
	sha512 HashAlgorithm = "sha512"
	empty  HashAlgorithm = ""
)

// SupportedSignatureAlgorithms sets a list of support signature algorithms that is similar to the list supported by cosign.
// empty HashAlgorithm is allowed and will be set to SHA256.
var SupportedSignatureAlgorithms = map[HashAlgorithm]crypto.Hash{
	sha224: crypto.SHA224,
	sha256: crypto.SHA256,
	sha384: crypto.SHA384,
	sha512: crypto.SHA512,
	empty:  crypto.SHA256,
}
