//go:build go1.18
// +build go1.18

// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License. See License.txt in the project root for license information.

package azcontainerregistry

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/streaming"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
)

// BlobClientOptions contains the optional parameters for the NewBlobClient method.
type BlobClientOptions struct {
	azcore.ClientOptions
}

// NewBlobClient creates a new instance of BlobClient with the specified values.
//   - endpoint - registry login URL
//   - credential - used to authorize requests. Usually a credential from azidentity.
//   - options - client options, pass nil to accept the default values.
func NewBlobClient(endpoint string, credential azcore.TokenCredential, options *BlobClientOptions) (*BlobClient, error) {
	if options == nil {
		options = &BlobClientOptions{}
	}

	if reflect.ValueOf(options.Cloud).IsZero() {
		options.Cloud = defaultCloud
	}
	c, ok := options.Cloud.Services[ServiceName]
	if !ok || c.Audience == "" {
		return nil, errors.New("provided Cloud field is missing Azure Container Registry configuration")
	}

	authClient, err := NewAuthenticationClient(endpoint, &AuthenticationClientOptions{
		options.ClientOptions,
	})
	if err != nil {
		return nil, err
	}

	authPolicy := newAuthenticationPolicy(
		credential,
		[]string{c.Audience + "/.default"},
		authClient,
		nil,
	)

	azcoreClient, err := azcore.NewClient(moduleName, moduleVersion, runtime.PipelineOptions{PerRetry: []policy.Policy{authPolicy}}, &options.ClientOptions)
	if err != nil {
		return nil, err
	}

	return &BlobClient{
		azcoreClient,
		endpoint,
	}, nil
}

// BlobClientUploadChunkOptions contains the optional parameters for the BlobClient.UploadChunk method.
type BlobClientUploadChunkOptions struct {
	// Start of range for the blob to be uploaded.
	RangeStart *int32
	// End of range for the blob to be uploaded, inclusive.
	RangeEnd *int32
}

// UploadChunk - Upload a stream of data without completing the upload.
//
//   - location - Link acquired from upload start or previous chunk
//   - chunkData - Raw data of blob
//   - blobDigestCalculator - Calculator that help to calculate blob digest
//   - options - BlobClientUploadChunkOptions contains the optional parameters for the BlobClient.UploadChunk method.
func (client *BlobClient) UploadChunk(ctx context.Context, location string, chunkData io.ReadSeeker, blobDigestCalculator *BlobDigestCalculator, options *BlobClientUploadChunkOptions) (BlobClientUploadChunkResponse, error) {
	blobDigestCalculator.saveState()
	reader, err := blobDigestCalculator.wrapReader(chunkData)
	if err != nil {
		return BlobClientUploadChunkResponse{}, err
	}
	wrappedChunkData := &wrappedReadSeeker{Reader: reader, Seeker: chunkData}
	var requestOptions *blobClientUploadChunkOptions
	if options != nil && options.RangeStart != nil && options.RangeEnd != nil {
		requestOptions = &blobClientUploadChunkOptions{ContentRange: to.Ptr(fmt.Sprintf("%d-%d", *options.RangeStart, *options.RangeEnd))}
	}
	resp, err := client.uploadChunk(ctx, location, streaming.NopCloser(wrappedChunkData), requestOptions)
	if err != nil {
		blobDigestCalculator.restoreState()
	}
	return resp, err
}

// CompleteUpload - Complete the upload with previously uploaded content.
//
//   - digest - Digest of a BLOB
//   - location - Link acquired from upload start or previous chunk
//   - blobDigestCalculator - Calculator that help to calculate blob digest
//   - options - BlobClientCompleteUploadOptions contains the optional parameters for the BlobClient.CompleteUpload method.
func (client *BlobClient) CompleteUpload(ctx context.Context, location string, blobDigestCalculator *BlobDigestCalculator, options *BlobClientCompleteUploadOptions) (BlobClientCompleteUploadResponse, error) {
	return client.completeUpload(ctx, blobDigestCalculator.getDigest(), location, options)
}
