package trustedresources

import (
	"errors"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
)

func TestGetMatchedPolicies_RegexBypass(t *testing.T) {
	policy := &v1alpha1.VerificationPolicy{
		Spec: v1alpha1.VerificationPolicySpec{
			Resources: []v1alpha1.ResourcePattern{{
				Pattern: "https://github.com/tektoncd/catalog.git",
			}},
		},
	}
	policies := []*v1alpha1.VerificationPolicy{policy}

	tests := []struct {
		name        string
		source      string
		wantMatch   bool
		description string
	}{{
		name:        "malicious source with trusted URL as query param is rejected",
		source:      "https://evil.com/?x=https://github.com/tektoncd/catalog.git",
		wantMatch:   false,
		description: "substring bypass via query parameter",
	}, {
		name:        "malicious source with trusted URL as path component is rejected",
		source:      "https://evil.com/https://github.com/tektoncd/catalog.git/foo",
		wantMatch:   false,
		description: "substring bypass via path embedding",
	}, {
		name:        "exact match without prefix succeeds",
		source:      "https://github.com/tektoncd/catalog.git",
		wantMatch:   true,
		description: "exact source matches unanchored pattern",
	}, {
		name:        "git+ prefixed source matches after prefix stripping",
		source:      "git+https://github.com/tektoncd/catalog.git",
		wantMatch:   true,
		description: "git resolver spdx prefix is stripped before matching",
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			matched, err := getMatchedPolicies("test-resource", tc.source, policies)
			if tc.wantMatch {
				if err != nil {
					t.Fatalf("expected match for source %q, got err: %v", tc.source, err)
				}
				if len(matched) != 1 {
					t.Fatalf("expected 1 matched policy, got %d", len(matched))
				}
			} else {
				if err == nil {
					t.Fatalf("expected no match for source %q, but got a match", tc.source)
				}
				if !errors.Is(err, ErrNoMatchedPolicies) {
					t.Fatalf("expected ErrNoMatchedPolicies, got: %v", err)
				}
			}
		})
	}
}

func TestGetMatchedPolicies_AlreadyAnchoredPatterns(t *testing.T) {
	tests := []struct {
		name      string
		pattern   string
		source    string
		wantMatch bool
	}{{
		name:      "fully anchored pattern still works",
		pattern:   "^https://github\\.com/tektoncd/catalog\\.git$",
		source:    "https://github.com/tektoncd/catalog.git",
		wantMatch: true,
	}, {
		name:      "start-anchored pattern gets end-anchored",
		pattern:   "^https://github.com/tektoncd/.*",
		source:    "https://github.com/tektoncd/catalog.git",
		wantMatch: true,
	}, {
		name:      "end-anchored pattern gets start-anchored",
		pattern:   ".*catalog.git$",
		source:    "https://github.com/tektoncd/catalog.git",
		wantMatch: true,
	}, {
		name:      "wildcard pattern matches everything",
		pattern:   ".*",
		source:    "https://anything.example.com",
		wantMatch: true,
	}, {
		name:      "git+ prefix stripped before matching anchored pattern",
		pattern:   "^https://github\\.com/tektoncd/catalog\\.git$",
		source:    "git+https://github.com/tektoncd/catalog.git",
		wantMatch: true,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			policy := &v1alpha1.VerificationPolicy{
				Spec: v1alpha1.VerificationPolicySpec{
					Resources: []v1alpha1.ResourcePattern{{
						Pattern: tc.pattern,
					}},
				},
			}
			matched, err := getMatchedPolicies("test-resource", tc.source, []*v1alpha1.VerificationPolicy{policy})
			if tc.wantMatch {
				if err != nil {
					t.Fatalf("expected match for pattern %q against source %q, got err: %v", tc.pattern, tc.source, err)
				}
				if len(matched) != 1 {
					t.Fatalf("expected 1 matched policy, got %d", len(matched))
				}
			} else {
				if err == nil {
					t.Fatalf("expected no match for pattern %q against source %q, but got a match", tc.pattern, tc.source)
				}
			}
		})
	}
}

func TestAnchorPattern(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{{
		input: "https://github.com/tektoncd/catalog.git",
		want:  "^(?:https://github.com/tektoncd/catalog.git)$",
	}, {
		input: "^https://github.com/tektoncd/.*",
		want:  "^(?:https://github.com/tektoncd/.*)$",
	}, {
		input: ".*catalog.git$",
		want:  "^(?:.*catalog.git)$",
	}, {
		input: "^https://github\\.com/tektoncd/catalog\\.git$",
		want:  "^https://github\\.com/tektoncd/catalog\\.git$",
	}, {
		input: ".*",
		want:  "^(?:.*)$",
	}}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := anchorPattern(tc.input)
			if got != tc.want {
				t.Errorf("anchorPattern(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestStripResolverPrefix(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{{
		input: "git+https://github.com/tektoncd/catalog.git",
		want:  "https://github.com/tektoncd/catalog.git",
	}, {
		input: "https://github.com/tektoncd/catalog.git",
		want:  "https://github.com/tektoncd/catalog.git",
	}, {
		input: "gcr.io/tekton-releases/catalog/upstream/git-clone",
		want:  "gcr.io/tekton-releases/catalog/upstream/git-clone",
	}, {
		input: "",
		want:  "",
	}}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := stripResolverPrefix(tc.input)
			if got != tc.want {
				t.Errorf("stripResolverPrefix(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
