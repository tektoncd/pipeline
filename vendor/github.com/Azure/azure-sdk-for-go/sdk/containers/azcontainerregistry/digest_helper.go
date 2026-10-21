//go:build go1.18
// +build go1.18

// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License. See License.txt in the project root for license information.

package azcontainerregistry

import (
	"crypto/sha256"
	"encoding"
	"errors"
	"fmt"
	"hash"
	"io"
	"strings"
)

var (
	validatorCtors           = map[string]func() digestValidator{"sha256": newSha256Validator}
	ErrMismatchedHash        = errors.New("mismatched hash")
	ErrDigestAlgNotSupported = errors.New("digest algorithm not supported")
)

type digestValidator interface {
	io.Writer
	validate(digest string) error
}

func parseDigestValidator(digest string) (digestValidator, error) {
	i := strings.Index(digest, ":")
	if i < 0 {
		return nil, ErrDigestAlgNotSupported
	}
	alg := digest[:i]
	if v, ok := validatorCtors[alg]; ok {
		return v(), nil
	}
	return nil, ErrDigestAlgNotSupported
}

type sha256Validator struct {
	hash.Hash
}

func newSha256Validator() digestValidator {
	return &sha256Validator{sha256.New()}
}

func (s *sha256Validator) validate(digest string) error {
	if fmt.Sprintf("sha256:%x", s.Sum(nil)) != digest {
		return ErrMismatchedHash
	}
	return nil
}

// DigestValidationReader help to validate digest when fetching manifest or blob.
// Don't use this type directly, use NewDigestValidationReader() instead.
type DigestValidationReader struct {
	digest          string
	digestValidator digestValidator
	reader          io.Reader
}

// NewDigestValidationReader creates a new reader that help you to validate digest when you read manifest or blob data.
func NewDigestValidationReader(digest string, reader io.Reader) (*DigestValidationReader, error) {
	validator, err := parseDigestValidator(digest)
	if err != nil {
		return nil, err
	}
	return &DigestValidationReader{
		digest:          digest,
		digestValidator: validator,
		reader:          reader,
	}, nil
}

// Read write to digest validator while read and validate digest when reach EOF.
func (d *DigestValidationReader) Read(p []byte) (int, error) {
	n, err := d.reader.Read(p)
	if err == nil || err == io.EOF {
		wn, werr := d.digestValidator.Write(p[:n])
		if werr != nil {
			return wn, werr
		}
	}
	if err == io.EOF {
		if err := d.digestValidator.validate(d.digest); err != nil {
			return n, err
		}
	}
	return n, err
}

// BlobDigestCalculator help to calculate blob digest when uploading blob.
// Don't use this type directly, use NewBlobDigestCalculator() instead.
type BlobDigestCalculator struct {
	h     hash.Hash
	state []byte
}

// NewBlobDigestCalculator creates a new calculator to help to calculate blob digest when uploading blob.
// You should use a new BlobDigestCalculator each time you upload a blob.
func NewBlobDigestCalculator() *BlobDigestCalculator {
	return &BlobDigestCalculator{
		h: sha256.New(),
	}
}

func (b *BlobDigestCalculator) saveState() {
	b.state, _ = b.h.(encoding.BinaryMarshaler).MarshalBinary()
}

func (b *BlobDigestCalculator) restoreState() {
	if b.state == nil {
		return
	}
	_ = b.h.(encoding.BinaryUnmarshaler).UnmarshalBinary(b.state)
}

func (b *BlobDigestCalculator) getDigest() string {
	return fmt.Sprintf("sha256:%x", b.h.Sum(nil))
}

func (b *BlobDigestCalculator) wrapReader(reader io.ReadSeeker) (io.Reader, error) {
	size, err := reader.Seek(0, io.SeekEnd) // Seek to the end to get the stream's size
	if err != nil {
		return nil, err
	}
	_, err = reader.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}
	return newLimitTeeReader(reader, b.h, size), nil
}

type wrappedReadSeeker struct {
	io.Reader
	io.Seeker
}

// newLimitTeeReader returns a Reader that writes to w what it reads from r with n bytes limit.
func newLimitTeeReader(r io.Reader, w io.Writer, n int64) io.Reader {
	return &limitTeeReader{r, w, n}
}

type limitTeeReader struct {
	r io.Reader
	w io.Writer
	n int64
}

func (lt *limitTeeReader) Read(p []byte) (int, error) {
	n, err := lt.r.Read(p)
	if n > 0 && lt.n > 0 {
		wn, werr := lt.w.Write(p[:n])
		if werr != nil {
			return wn, werr
		}
		lt.n -= int64(wn)
	}
	return n, err
}
