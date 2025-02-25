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

// Package cliplugin implements the plugin functionality.
package cliplugin

import (
	"bytes"
	"context"
	"crypto"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/sigstore/sigstore/pkg/signature/kms/cliplugin/common"
	"github.com/sigstore/sigstore/pkg/signature/kms/cliplugin/encoding"
	"github.com/sigstore/sigstore/pkg/signature/options"
)

var (
	// ErrorExecutingPlugin indicates a problem executing the plugin program.
	ErrorExecutingPlugin = errors.New("error executing plugin program")
	// ErrorResponseParse indicates a problem parsing the plugin response.
	ErrorResponseParse = errors.New("parsing plugin response")
	// ErrorPluginReturnError indicates that the plugin returned a praseable error.
	ErrorPluginReturnError = errors.New("plugin returned error")
)

// PluginClient implements signerverifier.SignerVerifier with calls to our plugin program.
type PluginClient struct {
	executable  string
	initOptions common.InitOptions
	makeCmdFunc makeCmdFunc
}

// newPluginClient creates a new PluginClient.
func newPluginClient(executable string, initOptions *common.InitOptions, makeCmd makeCmdFunc) *PluginClient {
	pluginClient := &PluginClient{
		executable:  executable,
		initOptions: *initOptions,
		makeCmdFunc: makeCmd,
	}
	return pluginClient
}

// invokePlugin invokes the plugin program and parses its response.
func (c PluginClient) invokePlugin(ctx context.Context, stdin io.Reader, methodArgs *common.MethodArgs) (*common.PluginResp, error) {
	pluginArgs := &common.PluginArgs{
		InitOptions: &c.initOptions,
		MethodArgs:  methodArgs,
	}
	argsEnc, err := json.Marshal(pluginArgs)
	if err != nil {
		return nil, err
	}
	cmd := c.makeCmdFunc(ctx, stdin, os.Stderr, c.executable, common.ProtocolVersion, string(argsEnc))
	// We won't look at the program's non-zero exit code, but we will respect any other
	// error, and cases when exec.ExitError.ExitCode() is 0 or -1:
	//   * (0) the program finished successfully or
	//   * (-1) there was some other problem not due to the program itself.
	// The only debugging is to either parse the the returned error in stdout,
	// or for the user to examine the sterr logs.
	// See https://pkg.go.dev/os#ProcessState.ExitCode.
	stdout, err := cmd.Output()
	var exitError cmdExitError
	if err != nil && (!errors.As(err, &exitError) || exitError.ExitCode() < 1) {
		return nil, fmt.Errorf("%w: %w", ErrorExecutingPlugin, err)
	}
	var resp common.PluginResp
	if unmarshallErr := json.Unmarshal(stdout, &resp); unmarshallErr != nil {
		return nil, fmt.Errorf("%w: %w", ErrorResponseParse, unmarshallErr)
	}
	if resp.ErrorMessage != "" {
		return nil, fmt.Errorf("%w: %s", ErrorPluginReturnError, resp.ErrorMessage)
	}
	return &resp, nil
}

// DefaultAlgorithm calls and returns the plugin's implementation of DefaultAlgorithm().
func (c PluginClient) DefaultAlgorithm() string {
	args := &common.MethodArgs{
		MethodName:       common.DefaultAlgorithmMethodName,
		DefaultAlgorithm: &common.DefaultAlgorithmArgs{},
	}
	resp, err := c.invokePlugin(context.Background(), nil, args)
	if err != nil {
		log.Fatal(err)
	}
	return resp.DefaultAlgorithm.DefaultAlgorithm
}

// SupportedAlgorithms calls and returns the plugin's implementation of SupportedAlgorithms().
func (c PluginClient) SupportedAlgorithms() []string {
	args := &common.MethodArgs{
		MethodName:          common.SupportedAlgorithmsMethodName,
		SupportedAlgorithms: &common.SupportedAlgorithmsArgs{},
	}
	resp, err := c.invokePlugin(context.Background(), nil, args)
	if err != nil {
		log.Fatal(err)
	}
	return resp.SupportedAlgorithms.SupportedAlgorithms
}

// CreateKey calls and returns the plugin's implementation of CreateKey().
func (c PluginClient) CreateKey(ctx context.Context, algorithm string) (crypto.PublicKey, error) {
	args := &common.MethodArgs{
		MethodName: common.CreateKeyMethodName,
		CreateKey: &common.CreateKeyArgs{
			Algorithm: algorithm,
		},
	}
	if deadline, ok := ctx.Deadline(); ok {
		args.CreateKey.CtxDeadline = &deadline
	}
	resp, err := c.invokePlugin(ctx, nil, args)
	if err != nil {
		return nil, err
	}
	return cryptoutils.UnmarshalPEMToPublicKey(resp.CreateKey.PublicKeyPEM)
}

// PublicKey calls and returns the plugin's implementation of PublicKey().
// If the opts contain a context, then it will be used with the Cmd.
func (c PluginClient) PublicKey(opts ...signature.PublicKeyOption) (crypto.PublicKey, error) {
	args := &common.MethodArgs{
		MethodName: common.PublicKeyMethodName,
		PublicKey: &common.PublicKeyArgs{
			PublicKeyOptions: encoding.PackPublicKeyOptions(opts),
		},
	}
	ctx := context.Background()
	for _, opt := range opts {
		opt.ApplyContext(&ctx)
	}
	resp, err := c.invokePlugin(ctx, nil, args)
	if err != nil {
		return nil, err
	}
	return cryptoutils.UnmarshalPEMToPublicKey(resp.PublicKey.PublicKeyPEM)
}

// SignMessage calls and returns the plugin's implementation of SignMessage().
// If the opts contain a context, then it will be used with the Cmd.
func (c PluginClient) SignMessage(message io.Reader, opts ...signature.SignOption) ([]byte, error) {
	args := &common.MethodArgs{
		MethodName: common.SignMessageMethodName,
		SignMessage: &common.SignMessageArgs{
			SignOptions: encoding.PackSignOptions(opts),
		},
	}
	ctx := context.Background()
	for _, opt := range opts {
		opt.ApplyContext(&ctx)
	}
	resp, err := c.invokePlugin(ctx, message, args)
	if err != nil {
		return nil, err
	}
	signature := resp.SignMessage.Signature
	return signature, nil
}

// VerifySignature calls and returns the plugin's implementation of VerifySignature().
// If the opts contain a context, then it will be used with the Cmd.
func (c PluginClient) VerifySignature(signature, message io.Reader, opts ...signature.VerifyOption) error {
	// signatures won't be larger than 1MB, so it's fine to read the entire content into memory.
	signatureBytes, err := io.ReadAll(signature)
	if err != nil {
		return err
	}
	args := &common.MethodArgs{
		MethodName: common.VerifySignatureMethodName,
		VerifySignature: &common.VerifySignatureArgs{
			Signature:     signatureBytes,
			VerifyOptions: encoding.PackVerifyOptions(opts),
		},
	}
	ctx := context.Background()
	for _, opt := range opts {
		opt.ApplyContext(&ctx)
	}
	_, err = c.invokePlugin(ctx, message, args)
	return err
}

// CryptoSigner is a wrapper around PluginClient.
type CryptoSigner struct {
	client  *PluginClient
	ctx     context.Context
	errFunc func(error)
}

// CryptoSigner returns a wrapper around PluginClient.
func (c PluginClient) CryptoSigner(ctx context.Context, errFunc func(error)) (crypto.Signer, crypto.SignerOpts, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	return &CryptoSigner{
		client:  &c,
		ctx:     ctx,
		errFunc: errFunc,
	}, c.initOptions.HashFunc, nil
}

// Sign is a wrapper around PluginClient.SignMessage(). The first argument for a rand source is not used.
func (c CryptoSigner) Sign(_ io.Reader, digest []byte, cryptoSignerOpts crypto.SignerOpts) (sig []byte, err error) {
	emptyMessage := bytes.NewReader([]byte(""))
	opts := []signature.SignOption{
		options.WithCryptoSignerOpts(cryptoSignerOpts.HashFunc()),
		options.WithDigest(digest),
		// the client's initializing ctx should not be used in calls to its methods.
	}
	sig, err = c.client.SignMessage(emptyMessage, opts...)
	if err != nil && c.errFunc != nil {
		c.errFunc(err)
	}
	return sig, err
}

// Public is a wrapper around PluginClient.PublicKey().
func (c CryptoSigner) Public() crypto.PublicKey {
	publicKey, err := c.client.PublicKey()
	if err != nil && c.errFunc != nil {
		c.errFunc(err)
		// we don't panic here.
	}
	return publicKey
}
