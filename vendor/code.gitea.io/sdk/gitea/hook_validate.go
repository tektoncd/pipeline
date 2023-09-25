// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
)

// VerifyWebhookSignature verifies that a payload matches the X-Gitea-Signature based on a secret
func VerifyWebhookSignature(secret, expected string, payload []byte) (bool, error) {
	hash := hmac.New(sha256.New, []byte(secret))
	if _, err := hash.Write(payload); err != nil {
		return false, err
	}
	expectedSum, err := hex.DecodeString(expected)
	if err != nil {
		return false, err
	}
	return hmac.Equal(hash.Sum(nil), expectedSum), nil
}

// VerifyWebhookSignatureMiddleware is a http.Handler for verifying X-Gitea-Signature on incoming webhooks
func VerifyWebhookSignatureMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var b bytes.Buffer
			if _, err := io.Copy(&b, r.Body); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			expected := r.Header.Get("X-Gitea-Signature")
			if expected == "" {
				http.Error(w, "no signature found", http.StatusBadRequest)
				return
			}

			ok, err := VerifyWebhookSignature(secret, expected, b.Bytes())
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}
			if !ok {
				http.Error(w, "invalid payload", http.StatusUnauthorized)
				return
			}

			r.Body = io.NopCloser(&b)
			next.ServeHTTP(w, r)
		})
	}
}
