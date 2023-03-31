/*
Copyright 2023 The Tekton Authors

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

package test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Methods to generate private keys. If generation starts slowing down test
// execution then switch over to pre-generated keys.

// NewEC256Key returns an ECDSA key over the P256 curve
func NewEC256Key(tb testing.TB) *ecdsa.PrivateKey {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(tb, err)
	return key
}

// NewKeyID returns a random id useful for identifying keys
func NewKeyID(tb testing.TB) string {
	choices := make([]byte, 32)
	_, err := rand.Read(choices)
	require.NoError(tb, err)
	return keyIDFromBytes(choices)
}

func keyIDFromBytes(choices []byte) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var builder strings.Builder
	for _, choice := range choices {
		builder.WriteByte(alphabet[int(choice)%len(alphabet)])
	}
	return builder.String()
}
