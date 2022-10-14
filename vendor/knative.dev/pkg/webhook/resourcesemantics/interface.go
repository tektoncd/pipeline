/*
Copyright 2019 The Knative Authors

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

package resourcesemantics

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"
)

// GenericCRD is the interface definition that allows us to perform the generic
// CRD actions like deciding whether to increment generation and so forth.
type GenericCRD interface {
	apis.Defaultable
	apis.Validatable
	runtime.Object
}

// VerbLimited defines which Verbs you want to have the webhook invoked on.
type VerbLimited interface {
	// SupportedVerbs define which operations (verbs) webhook is called on.
	SupportedVerbs() []admissionregistrationv1.OperationType
}

// SubResourceLimited defines which subresources you want to have the webhook
// invoked on. For example "status", "scale", etc.
type SubResourceLimited interface {
	// SupportedSubResources are the subresources that will be registered
	// for the resource validation.
	// If you wanted to add for example scale validation for Deployments, you'd
	// do:
	// []string{"", "/status", "/scale"}
	// And to get just the main resource, you would do:
	// []string{""}
	SupportedSubResources() []string
}
