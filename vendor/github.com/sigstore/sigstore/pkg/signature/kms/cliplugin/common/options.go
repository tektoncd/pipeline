//
// Copyright 2024 The Sigstore Authors.
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

// Package common defines the JSON schema for plugin arguments and return values.
package common

import (
	"crypto"
	"time"
)

// PublicKeyOptions contains the values for signature.PublicKeyOptions.
type PublicKeyOptions struct {
	RPCOptions RPCOptions `json:"rpcOptions"`
}

// SignOptions contains the values for signature.SignOption.
type SignOptions struct {
	RPCOptions     RPCOptions     `json:"rpcOptions"`
	MessageOptions MessageOptions `json:"messageOptions"`
}

// VerifyOptions contains the values for signature.VerifyOption.
type VerifyOptions struct {
	RPCOptions     RPCOptions     `json:"rpcOptions"`
	MessageOptions MessageOptions `json:"messageOptions"`
}

// RPCOptions contains the values for signature.RPCOption.
// We do not use RPCOptions.RPCAuth to avoid sending secrets over CLI to the plugin program.
// The plugin program should instead read secrets with env variables.
type RPCOptions struct {
	// CtxDeadline serializes to RFC 3339. See https://pkg.go.dev/time@go1.23.5#Time.MarshalJSON. e.g, 2025-04-01T02:47:00Z.
	CtxDeadline        *time.Time `json:"ctxDeadline,omitempty"`
	KeyVersion         *string    `json:"keyVersion,omitempty"`
	RemoteVerification *bool      `json:"remoteVerification,omitempty"`
}

// MessageOptions contains the values for signature.MessageOption.
type MessageOptions struct {
	// Digest is a base64 encoding of the digest bytes. e.g, []byte("anyDigest") serializes to "YW55RGlnZXN0".
	Digest *[]byte `json:"digest,omitempty"`
	// HashFunc will serialize to ints according to https://pkg.go.dev/crypto@go1.23.5#Hash. e.g., crypto.SHA256 serializes to 5.
	HashFunc *crypto.Hash `json:"hashFunc,omitempty"`
}
