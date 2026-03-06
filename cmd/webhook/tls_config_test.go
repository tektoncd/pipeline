/*
Copyright 2026 The Tekton Authors

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

package main

import (
	"crypto/tls"
	"testing"
)

func TestParseTLSMinVersion(t *testing.T) {
	tests := []struct {
		name      string
		envValue  string
		want      uint16
		wantError bool
	}{
		{
			name:      "TLS 1.2",
			envValue:  "1.2",
			want:      tls.VersionTLS12,
			wantError: false,
		},
		{
			name:      "TLS 1.3",
			envValue:  "1.3",
			want:      tls.VersionTLS13,
			wantError: false,
		},
		{
			name:      "empty string (use default)",
			envValue:  "",
			want:      0,
			wantError: false,
		},
		{
			name:      "invalid version",
			envValue:  "1.1",
			want:      0,
			wantError: true,
		},
		{
			name:      "invalid format",
			envValue:  "invalid",
			want:      0,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTLSMinVersion(tt.envValue)
			if (err != nil) != tt.wantError {
				t.Errorf("parseTLSMinVersion() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if got != tt.want {
				t.Errorf("parseTLSMinVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseCipherSuites(t *testing.T) {
	tests := []struct {
		name      string
		envValue  string
		want      []uint16
		wantError bool
	}{
		{
			name:      "single cipher",
			envValue:  "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
			want:      []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
			wantError: false,
		},
		{
			name:      "multiple ciphers",
			envValue:  "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
			want:      []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384},
			wantError: false,
		},
		{
			name:      "ciphers with spaces",
			envValue:  "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256 , TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
			want:      []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384},
			wantError: false,
		},
		{
			name:      "TLS 1.3 ciphers (accepted but ignored by Go)",
			envValue:  "TLS_AES_128_GCM_SHA256,TLS_AES_256_GCM_SHA384,TLS_CHACHA20_POLY1305_SHA256",
			want:      []uint16{0x1301, 0x1302, 0x1303},
			wantError: false,
		},
		{
			name:      "mixed TLS 1.2 and 1.3 ciphers (Intermediate profile)",
			envValue:  "TLS_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
			want:      []uint16{0x1301, tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256},
			wantError: false,
		},
		{
			name:      "CBC mode ciphers",
			envValue:  "TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA",
			want:      []uint16{tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256, tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA},
			wantError: false,
		},
		{
			name:      "legacy ciphers (Old profile)",
			envValue:  "TLS_RSA_WITH_AES_128_CBC_SHA,TLS_RSA_WITH_3DES_EDE_CBC_SHA",
			want:      []uint16{tls.TLS_RSA_WITH_AES_128_CBC_SHA, tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA},
			wantError: false,
		},
		{
			name:      "empty string (use default)",
			envValue:  "",
			want:      nil,
			wantError: false,
		},
		{
			name:      "unknown cipher",
			envValue:  "UNKNOWN_CIPHER",
			want:      nil,
			wantError: true,
		},
		{
			name:      "mixed valid and invalid",
			envValue:  "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,UNKNOWN_CIPHER",
			want:      nil,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCipherSuites(tt.envValue)
			if (err != nil) != tt.wantError {
				t.Errorf("parseCipherSuites() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError {
				if len(got) != len(tt.want) {
					t.Errorf("parseCipherSuites() length = %v, want %v", len(got), len(tt.want))
					return
				}
				for i := range got {
					if got[i] != tt.want[i] {
						t.Errorf("parseCipherSuites()[%d] = %v, want %v", i, got[i], tt.want[i])
					}
				}
			}
		})
	}
}

func TestParseCurvePreferences(t *testing.T) {
	tests := []struct {
		name      string
		envValue  string
		want      []tls.CurveID
		wantError bool
	}{
		{
			name:      "single curve",
			envValue:  "X25519",
			want:      []tls.CurveID{tls.X25519},
			wantError: false,
		},
		{
			name:      "multiple curves",
			envValue:  "X25519,CurveP256,CurveP384",
			want:      []tls.CurveID{tls.X25519, tls.CurveP256, tls.CurveP384},
			wantError: false,
		},
		{
			name:      "curves with spaces",
			envValue:  "X25519 , CurveP256 , CurveP384",
			want:      []tls.CurveID{tls.X25519, tls.CurveP256, tls.CurveP384},
			wantError: false,
		},
		{
			name:      "empty string (use default)",
			envValue:  "",
			want:      nil,
			wantError: false,
		},
		{
			name:      "unknown curve",
			envValue:  "UnknownCurve",
			want:      nil,
			wantError: true,
		},
		{
			name:      "mixed valid and invalid",
			envValue:  "X25519,UnknownCurve",
			want:      nil,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCurvePreferences(tt.envValue)
			if (err != nil) != tt.wantError {
				t.Errorf("parseCurvePreferences() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError {
				if len(got) != len(tt.want) {
					t.Errorf("parseCurvePreferences() length = %v, want %v", len(got), len(tt.want))
					return
				}
				for i := range got {
					if got[i] != tt.want[i] {
						t.Errorf("parseCurvePreferences()[%d] = %v, want %v", i, got[i], tt.want[i])
					}
				}
			}
		})
	}
}
