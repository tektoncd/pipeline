// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package hmac

import (
	"crypto/hmac"
	"crypto/sha1" // #nosec
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"strings"
)

// Validate checks the hmac signature of the mssasge
// using a hex encoded signature.
func Validate(h func() hash.Hash, message, key []byte, signature string) bool {
	decoded, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}
	return validate(h, message, key, decoded)
}

// ValidatePrefix checks the hmac signature of the message
// using the message prefix to determine the signing algorithm.
func ValidatePrefix(message, key []byte, signature string) bool {
	parts := strings.Split(signature, "=")
	if len(parts) != 2 {
		return false
	}
	switch parts[0] {
	case "sha1":
		return Validate(sha1.New, message, key, parts[1])
	case "sha256":
		return Validate(sha256.New, message, key, parts[1])
	default:
		return false
	}
}

func validate(h func() hash.Hash, message, key, signature []byte) bool {
	mac := hmac.New(h, key)
	mac.Write(message) // #nosec
	sum := mac.Sum(nil)
	return hmac.Equal(signature, sum)
}
