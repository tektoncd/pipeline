// Copyright 2025 The Sigstore Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package gcp defines options for KMS clients
package gcp

import (
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/sigstore/sigstore/pkg/signature/options"
	googleoption "google.golang.org/api/option"
)

// WithGoogleAPIClientOption specifies that the given context should be used in RPC to external services
func WithGoogleAPIClientOption(opt googleoption.ClientOption) signature.RPCOption {
	return &requestGoogleAPIClientOption{opt: opt}
}

type requestGoogleAPIClientOption struct {
	options.NoOpOptionImpl
	opt googleoption.ClientOption
}

func (r *requestGoogleAPIClientOption) ApplyGoogleAPIClientOption(opt *googleoption.ClientOption) {
	*opt = r.opt
}
