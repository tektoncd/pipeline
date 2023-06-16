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

package resource

import v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"

var _ Request = &BasicRequest{}

// BasicRequest holds the fields needed to submit a new resource request.
type BasicRequest struct {
	name      string
	namespace string
	params    v1.Params
}

// NewRequest returns an instance of a BasicRequest with the given name,
// namespace and params.
func NewRequest(name, namespace string, params v1.Params) Request {
	return &BasicRequest{name, namespace, params}
}

var _ Request = &BasicRequest{}

// Name returns the name attached to the request
func (req *BasicRequest) Name() string {
	return req.name
}

// Namespace returns the namespace that the request is associated with
func (req *BasicRequest) Namespace() string {
	return req.namespace
}

// Params are the map of parameters associated with this request
func (req *BasicRequest) Params() v1.Params {
	return req.params
}
