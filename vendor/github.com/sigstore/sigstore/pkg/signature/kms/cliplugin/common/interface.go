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

const (
	// ProtocolVersion is the version of the schema and communication protocol for the plugin system.
	// Breaking changes to the PluginClient and this schema necessarily mean major version bumps of
	// this ProtocolVersion and the sigstore version.
	// Plugin authors may choose to be backwards compatible with older versions.
	ProtocolVersion = "v1"
	// DefaultAlgorithmMethodName is the MethodName for DefaultAlgorithsm().
	DefaultAlgorithmMethodName = "defaultAlgorithm"
	// SupportedAlgorithmsMethodName is the MethodName for SupportedAlgorithms().
	SupportedAlgorithmsMethodName = "supportedAlgorithms"
	// CreateKeyMethodName is the MethodName for CreateKey().
	CreateKeyMethodName = "createKey"
	// PublicKeyMethodName is the MethodName for PublicKey().
	PublicKeyMethodName = "publicKey"
	// SignMessageMethodName is the MethodName for SignMessage().
	SignMessageMethodName = "signMessage"
	// VerifySignatureMethodName is the MethodName for VerifySignature().
	VerifySignatureMethodName = "verifySignature"
	// CryptoSigner is not to be added to the protocol.
	// PluginClient.CryptoSigner() will instead return a wrapper around the plugin.
)

// PluginArgs contains all the initialization and method arguments to be sent to the plugin as a CLI argument.
type PluginArgs struct {
	InitOptions *InitOptions `json:"initOptions"`
	*MethodArgs
}

// InitOptions contains the initial arguments when calling cliplugin.LoadSignerVerifier().
type InitOptions struct {
	// CtxDeadline serializes to RFC 3339. See https://pkg.go.dev/time@go1.23.5#Time.MarshalJSON. e.g, 2025-04-01T02:47:00Z.
	CtxDeadline     *time.Time `json:"ctxDeadline,omitempty"`
	ProtocolVersion string     `json:"protocolVersion"`
	KeyResourceID   string     `json:"keyResourceID"`
	// HashFunc will serialize to ints according to https://pkg.go.dev/crypto@go1.23.5#Hash. e.g., crypto.SHA256 serializes to 5.
	HashFunc   crypto.Hash `json:"hashFunc"`
	RPCOptions *RPCOptions `json:"rpcOptions"`
}

// MethodArgs contains the method arguments. MethodName must be specified,
// while any one of the other fields describing method arguments must also be specified.
// Arguments that are io.Readers, like `message` in `SignMessage()` will be sent over stdin.
type MethodArgs struct {
	// MethodName specifies which method is intended to be called.
	MethodName          string                   `json:"methodName"`
	DefaultAlgorithm    *DefaultAlgorithmArgs    `json:"defaultAlgorithm,omitempty"`
	SupportedAlgorithms *SupportedAlgorithmsArgs `json:"supportedAlgorithms,omitempty"`
	CreateKey           *CreateKeyArgs           `json:"createKey,omitempty"`
	PublicKey           *PublicKeyArgs           `json:"publicKey,omitempty"`
	SignMessage         *SignMessageArgs         `json:"signMessage,omitempty"`
	VerifySignature     *VerifySignatureArgs     `json:"verifySignature,omitempty"`
}

// PluginResp contains the serialized plugin method return values.
type PluginResp struct {
	ErrorMessage        string                   `json:"errorMessage,omitempty"`
	DefaultAlgorithm    *DefaultAlgorithmResp    `json:"defaultAlgorithm,omitempty"`
	SupportedAlgorithms *SupportedAlgorithmsResp `json:"supportedAlgorithms,omitempty"`
	CreateKey           *CreateKeyResp           `json:"createKey,omitempty"`
	PublicKey           *PublicKeyResp           `json:"publicKey,omitempty"`
	SignMessage         *SignMessageResp         `json:"signMessage,omitempty"`
	VerifySignature     *VerifySignatureResp     `json:"verifySignature,omitempty"`
}

// DefaultAlgorithmArgs contains the serialized arguments for `DefaultAlgorithm()`.
type DefaultAlgorithmArgs struct{}

// DefaultAlgorithmResp contains the serialized response for `DefaultAlgorithm()`.
type DefaultAlgorithmResp struct {
	DefaultAlgorithm string `json:"defaultAlgorithm"`
}

// SupportedAlgorithmsArgs contains the serialized arguments for `SupportedAlgorithms()`.
type SupportedAlgorithmsArgs struct{}

// SupportedAlgorithmsResp contains the serialized response for `SupportedAlgorithms()`.
type SupportedAlgorithmsResp struct {
	SupportedAlgorithms []string `json:"supportedAlgorithms"`
}

// CreateKeyArgs contains the serialized arguments for `CreateKeyArgs()`.
type CreateKeyArgs struct {
	// CtxDeadline serializes to RFC 3339. See https://pkg.go.dev/time@go1.23.5#Time.MarshalJSON. e.g, 2025-04-01T02:47:00Z.
	CtxDeadline *time.Time `json:"ctxDeadline,omitempty"`
	Algorithm   string     `json:"algorithm"`
}

// CreateKeyResp contains the serialized response for `CreateKeyResp()`.
type CreateKeyResp struct {
	// PublicKeyPEM is a base64 encoding of the Public Key PEM bytes. e.g, []byte("mypem") serializes to "bXlwZW0=".
	PublicKeyPEM []byte `json:"publicKeyPEM"`
}

// PublicKeyArgs contains the serialized response for `PublicKey()`.
type PublicKeyArgs struct {
	PublicKeyOptions *PublicKeyOptions `json:"publicKeyOptions"`
}

// PublicKeyResp contains the serialized response for `PublicKey()`.
type PublicKeyResp struct {
	// PublicKeyPEM is a base64 encoding of the Public Key PEM bytes. e.g, []byte("mypem") serializes to "bXlwZW0=".
	PublicKeyPEM []byte `json:"publicKeyPEM"`
}

// SignMessageArgs contains the serialized arguments for `SignMessage()`.
type SignMessageArgs struct {
	SignOptions *SignOptions `json:"signOptions"`
}

// SignMessageResp contains the serialized response for `SignMessage()`.
type SignMessageResp struct {
	// Signature is a base64 encoding of the signature bytes. e.g, []byte("any-signature") serializes to "W55LXNpZ25hdHVyZQ==".
	Signature []byte `json:"signature"`
}

// VerifySignatureArgs contains the serialized arguments for `VerifySignature()`.
type VerifySignatureArgs struct {
	// Signature is a base64 encoding of the signature bytes. e.g, []byte("any-signature") serializes to "W55LXNpZ25hdHVyZQ==".
	Signature     []byte         `json:"signature"`
	VerifyOptions *VerifyOptions `json:"verifyOptions"`
}

// VerifySignatureResp contains the serialized response for `VerifySignature()`.
type VerifySignatureResp struct{}
