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

package config

import (
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

const (
	patternsKey = "patterns"
)

var (
	DefaultRedactPatterns, _ = NewRedactPatternsFromMap(map[string]string{})
)

// RedactPatterns holds the redaction pattern configuration.
// +k8s:deepcopy-gen=true
type RedactPatterns struct {
	Patterns []string
}

// GetRedactPatternsConfigName returns the name of the configmap containing
// redact patterns.
func GetRedactPatternsConfigName() string {
	if e := os.Getenv("CONFIG_REDACT_PATTERNS_NAME"); e != "" {
		return e
	}
	return "config-redact-patterns"
}

// NewRedactPatternsFromMap returns a RedactPatterns given a map corresponding
// to a ConfigMap.
func NewRedactPatternsFromMap(cfgMap map[string]string) (*RedactPatterns, error) {
	rp := &RedactPatterns{}
	if raw, ok := cfgMap[patternsKey]; ok {
		for _, line := range strings.Split(raw, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			rp.Patterns = append(rp.Patterns, line)
		}
	}
	return rp, nil
}

// NewRedactPatternsFromConfigMap returns a RedactPatterns for the given configmap.
func NewRedactPatternsFromConfigMap(config *corev1.ConfigMap) (*RedactPatterns, error) {
	return NewRedactPatternsFromMap(config.Data)
}

// DeepCopy returns a deep copy of RedactPatterns.
func (rp *RedactPatterns) DeepCopy() *RedactPatterns {
	if rp == nil {
		return nil
	}
	out := &RedactPatterns{}
	if rp.Patterns != nil {
		out.Patterns = make([]string, len(rp.Patterns))
		copy(out.Patterns, rp.Patterns)
	}
	return out
}

// Equals returns true if two RedactPatterns are identical.
func (rp *RedactPatterns) Equals(other *RedactPatterns) bool {
	if rp == nil && other == nil {
		return true
	}
	if rp == nil || other == nil {
		return false
	}
	if len(rp.Patterns) != len(other.Patterns) {
		return false
	}
	for i := range rp.Patterns {
		if rp.Patterns[i] != other.Patterns[i] {
			return false
		}
	}
	return true
}
