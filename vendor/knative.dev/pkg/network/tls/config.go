/*
Copyright 2026 The Knative Authors

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

package tls

import (
	cryptotls "crypto/tls"
	"fmt"
	"os"
	"strings"
)

// Environment variable name suffixes for TLS configuration.
// Use with a prefix to namespace them, e.g. "WEBHOOK_" + MinVersionEnvKey
// reads the WEBHOOK_TLS_MIN_VERSION variable.
const (
	MinVersionEnvKey       = "TLS_MIN_VERSION"
	MaxVersionEnvKey       = "TLS_MAX_VERSION"
	CipherSuitesEnvKey     = "TLS_CIPHER_SUITES"
	CurvePreferencesEnvKey = "TLS_CURVE_PREFERENCES"
)

// DefaultConfigFromEnv returns a tls.Config with secure defaults.
// The prefix is prepended to each standard env-var suffix;
// for example with prefix "WEBHOOK_" the function reads
// WEBHOOK_TLS_MIN_VERSION, WEBHOOK_TLS_MAX_VERSION, etc.
func DefaultConfigFromEnv(prefix string) (*cryptotls.Config, error) {
	cfg := &cryptotls.Config{
		MinVersion: cryptotls.VersionTLS13,
	}

	if v := os.Getenv(prefix + MinVersionEnvKey); v != "" {
		ver, err := parseVersion(v)
		if err != nil {
			return nil, fmt.Errorf("invalid %s%s %q: %w", prefix, MinVersionEnvKey, v, err)
		}
		cfg.MinVersion = ver
	}

	if v := os.Getenv(prefix + MaxVersionEnvKey); v != "" {
		ver, err := parseVersion(v)
		if err != nil {
			return nil, fmt.Errorf("invalid %s%s %q: %w", prefix, MaxVersionEnvKey, v, err)
		}
		cfg.MaxVersion = ver
	}

	if v := os.Getenv(prefix + CipherSuitesEnvKey); v != "" {
		suites, err := parseCipherSuites(v)
		if err != nil {
			return nil, fmt.Errorf("invalid %s%s: %w", prefix, CipherSuitesEnvKey, err)
		}
		cfg.CipherSuites = suites
	}

	if v := os.Getenv(prefix + CurvePreferencesEnvKey); v != "" {
		curves, err := parseCurvePreferences(v)
		if err != nil {
			return nil, fmt.Errorf("invalid %s%s: %w", prefix, CurvePreferencesEnvKey, err)
		}
		cfg.CurvePreferences = curves
	}

	return cfg, nil
}

// parseVersion converts a TLS version string to the corresponding
// crypto/tls constant. Accepted values are "1.2" and "1.3".
func parseVersion(v string) (uint16, error) {
	switch v {
	case "1.2":
		return cryptotls.VersionTLS12, nil
	case "1.3":
		return cryptotls.VersionTLS13, nil
	default:
		return 0, fmt.Errorf("unsupported TLS version %q: must be %q or %q", v, "1.2", "1.3")
	}
}

// parseCipherSuites parses a comma-separated list of TLS cipher-suite names
// (e.g. "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384")
// into a slice of cipher-suite IDs. Names must match those returned by
// crypto/tls.CipherSuiteName.
func parseCipherSuites(s string) ([]uint16, error) {
	lookup := cipherSuiteLookup()
	parts := strings.Split(s, ",")
	suites := make([]uint16, 0, len(parts))

	for _, name := range parts {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		id, ok := lookup[name]
		if !ok {
			return nil, fmt.Errorf("unknown cipher suite %q", name)
		}
		suites = append(suites, id)
	}

	return suites, nil
}

// parseCurvePreferences parses a comma-separated list of elliptic-curve names
// (e.g. "X25519,CurveP256") into a slice of crypto/tls.CurveID values.
// Both Go constant names (CurveP256) and standard names (P-256) are accepted.
func parseCurvePreferences(s string) ([]cryptotls.CurveID, error) {
	parts := strings.Split(s, ",")
	curves := make([]cryptotls.CurveID, 0, len(parts))

	for _, name := range parts {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		id, ok := curvesByName[name]
		if !ok {
			return nil, fmt.Errorf("unknown curve %q", name)
		}
		curves = append(curves, id)
	}

	return curves, nil
}

func cipherSuiteLookup() map[string]uint16 {
	m := make(map[string]uint16)
	for _, cs := range cryptotls.CipherSuites() {
		m[cs.Name] = cs.ID
	}
	return m
}

var curvesByName = map[string]cryptotls.CurveID{
	"CurveP256":      cryptotls.CurveP256,
	"CurveP384":      cryptotls.CurveP384,
	"CurveP521":      cryptotls.CurveP521,
	"X25519":         cryptotls.X25519,
	"X25519MLKEM768": cryptotls.X25519MLKEM768,
	"P-256":          cryptotls.CurveP256,
	"P-384":          cryptotls.CurveP384,
	"P-521":          cryptotls.CurveP521,
}
