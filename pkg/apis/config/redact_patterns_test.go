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

package config_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/test/diff"
)

func TestNewRedactPatternsFromMap(t *testing.T) {
	tests := []struct {
		name string
		data map[string]string
		want []string
	}{
		{
			name: "empty map returns no patterns",
			data: map[string]string{},
			want: nil,
		},
		{
			name: "missing patterns key returns no patterns",
			data: map[string]string{"other-key": "value"},
			want: nil,
		},
		{
			name: "single pattern",
			data: map[string]string{
				"patterns": "eyJ[A-Za-z0-9_-]+",
			},
			want: []string{"eyJ[A-Za-z0-9_-]+"},
		},
		{
			name: "multiple patterns",
			data: map[string]string{
				"patterns": "eyJ[A-Za-z0-9_-]+\nghp_[A-Za-z0-9]{36}\nxoxb-[0-9]{10,13}",
			},
			want: []string{"eyJ[A-Za-z0-9_-]+", "ghp_[A-Za-z0-9]{36}", "xoxb-[0-9]{10,13}"},
		},
		{
			name: "comments and blank lines are skipped",
			data: map[string]string{
				"patterns": "pattern-one\n# this is a comment\n\n  \npattern-two",
			},
			want: []string{"pattern-one", "pattern-two"},
		},
		{
			name: "whitespace is trimmed",
			data: map[string]string{
				"patterns": "  pattern-one  \n  pattern-two  ",
			},
			want: []string{"pattern-one", "pattern-two"},
		},
		{
			name: "all comments returns no patterns",
			data: map[string]string{
				"patterns": "# comment one\n# comment two\n",
			},
			want: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rp, err := config.NewRedactPatternsFromMap(tc.data)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if d := cmp.Diff(tc.want, rp.Patterns); d != "" {
				t.Errorf("patterns %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestRedactPatternsDeepCopy(t *testing.T) {
	original := &config.RedactPatterns{Patterns: []string{"abc", "def"}}
	copied := original.DeepCopy()

	if d := cmp.Diff(original.Patterns, copied.Patterns); d != "" {
		t.Errorf("deep copy mismatch %s", diff.PrintWantGot(d))
	}

	copied.Patterns[0] = "modified"
	if original.Patterns[0] == "modified" {
		t.Error("deep copy is not independent — modifying copy affected original")
	}
}

func TestRedactPatternsDeepCopyNil(t *testing.T) {
	var rp *config.RedactPatterns
	if got := rp.DeepCopy(); got != nil {
		t.Errorf("DeepCopy of nil should return nil, got %v", got)
	}
}

func TestRedactPatternsEquals(t *testing.T) {
	tests := []struct {
		name string
		a, b *config.RedactPatterns
		want bool
	}{
		{
			name: "both nil",
			a:    nil,
			b:    nil,
			want: true,
		},
		{
			name: "one nil",
			a:    &config.RedactPatterns{Patterns: []string{"a"}},
			b:    nil,
			want: false,
		},
		{
			name: "equal patterns",
			a:    &config.RedactPatterns{Patterns: []string{"a", "b"}},
			b:    &config.RedactPatterns{Patterns: []string{"a", "b"}},
			want: true,
		},
		{
			name: "different patterns",
			a:    &config.RedactPatterns{Patterns: []string{"a"}},
			b:    &config.RedactPatterns{Patterns: []string{"b"}},
			want: false,
		},
		{
			name: "different lengths",
			a:    &config.RedactPatterns{Patterns: []string{"a"}},
			b:    &config.RedactPatterns{Patterns: []string{"a", "b"}},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.Equals(tc.b); got != tc.want {
				t.Errorf("Equals() = %v, want %v", got, tc.want)
			}
		})
	}
}
