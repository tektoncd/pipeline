//go:build !windows

package main

import (
	"bytes"
	"testing"
)

const fakeJWT = "eyJhbGciOiJSUzI1NiIsImtpZCI6InRlc3QifQ.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dGVzdHNpZ25hdHVyZQ"

var jwtPatterns = []string{`eyJ[A-Za-z0-9_-]+\.eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`}

func newJWTMaskingWriter(w *bytes.Buffer) *maskingWriter {
	return newMaskingWriterWithConfig(w, jwtPatterns, nil)
}

func TestMaskingWriter(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no JWT present",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "single JWT",
			input:    "TOKEN=" + fakeJWT + "!",
			expected: "TOKEN=***!",
		},
		{
			name:     "multiple JWTs",
			input:    "a=" + fakeJWT + " b=" + fakeJWT,
			expected: "a=*** b=***",
		},
		{
			name:     "set -x style output",
			input:    "+ TOKEN=" + fakeJWT + "\n+ echo done\n",
			expected: "+ TOKEN=***\n+ echo done\n",
		},
		{
			name:     "no false positive on partial eyJ",
			input:    "eyJust some text",
			expected: "eyJust some text",
		},
		{
			name:     "JWT at end of input (no trailing char)",
			input:    "TOKEN=" + fakeJWT,
			expected: "TOKEN=***",
		},
		{
			name:     "empty input",
			input:    "",
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			w := newJWTMaskingWriter(&buf)
			if _, err := w.Write([]byte(tc.input)); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if err := w.Flush(); err != nil {
				t.Fatalf("unexpected flush error: %v", err)
			}
			if got := buf.String(); got != tc.expected {
				t.Errorf("got %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestMaskingWriterSplitAcrossWrites(t *testing.T) {
	var buf bytes.Buffer
	w := newJWTMaskingWriter(&buf)

	// Split the JWT across two writes — the "eyJ" start is in write #1,
	// the rest comes in write #2
	jwt := fakeJWT
	splitPoint := 10 // splits in the middle of the header
	if _, err := w.Write([]byte("prefix " + jwt[:splitPoint])); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := w.Write([]byte(jwt[splitPoint:] + " suffix")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("unexpected flush error: %v", err)
	}

	if got, want := buf.String(), "prefix *** suffix"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMaskingWriterPartialPrefixHeldBack(t *testing.T) {
	var buf bytes.Buffer
	w := newJWTMaskingWriter(&buf)

	// Write ends with "ey" — should be held back in carry
	if _, err := w.Write([]byte("some text ey")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Next write completes it as NOT a JWT
	if _, err := w.Write([]byte("es are blue")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("unexpected flush error: %v", err)
	}

	if got, want := buf.String(), "some text eyes are blue"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMaskingWriterMultipleWritesNoJWT(t *testing.T) {
	var buf bytes.Buffer
	w := newMaskingWriter(&buf)

	writes := []string{"hello ", "world ", "no secrets here\n"}
	for _, s := range writes {
		if _, err := w.Write([]byte(s)); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("unexpected flush error: %v", err)
	}

	if got, want := buf.String(), "hello world no secrets here\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMaskingWriterFlushWithNoData(t *testing.T) {
	var buf bytes.Buffer
	w := newMaskingWriter(&buf)

	if err := w.Flush(); err != nil {
		t.Fatalf("unexpected flush error: %v", err)
	}
	if got := buf.String(); got != "" {
		t.Errorf("expected empty output, got %q", got)
	}
}

// --- Secrets-only tests (no regex patterns) ---

func TestMaskingWriterSecretsOnly(t *testing.T) {
	testCases := []struct {
		name     string
		secrets  []string
		input    string
		expected string
	}{
		{
			name:     "single secret redacted",
			secrets:  []string{"SuperSecret123!"},
			input:    "password is SuperSecret123! ok",
			expected: "password is *** ok",
		},
		{
			name:     "multiple secrets redacted",
			secrets:  []string{"password1", "api-key-xyz"},
			input:    "creds: password1 and api-key-xyz end",
			expected: "creds: *** and *** end",
		},
		{
			name:     "secret appears multiple times",
			secrets:  []string{"tok"},
			input:    "tok and tok again",
			expected: "*** and *** again",
		},
		{
			name:     "no secret present passes through",
			secrets:  []string{"missing"},
			input:    "nothing to see here",
			expected: "nothing to see here",
		},
		{
			name:     "empty secrets list is pass-through",
			secrets:  []string{},
			input:    "plain text",
			expected: "plain text",
		},
		{
			name:     "whitespace-only secret is ignored",
			secrets:  []string{"  ", "real-secret"},
			input:    "has real-secret inside",
			expected: "has *** inside",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			w := newMaskingWriterWithConfig(&buf, nil, tc.secrets)
			if _, err := w.Write([]byte(tc.input)); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if err := w.Flush(); err != nil {
				t.Fatalf("unexpected flush error: %v", err)
			}
			if got := buf.String(); got != tc.expected {
				t.Errorf("got %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestMaskingWriterSecretSplitAcrossWrites(t *testing.T) {
	secret := "LongSecretValue123"
	var buf bytes.Buffer
	w := newMaskingWriterWithConfig(&buf, nil, []string{secret})

	// Split the secret across two writes
	splitPoint := 8
	if _, err := w.Write([]byte("prefix " + secret[:splitPoint])); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := w.Write([]byte(secret[splitPoint:] + " suffix")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("unexpected flush error: %v", err)
	}

	if got, want := buf.String(), "prefix *** suffix"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// --- Regex-only tests (no exact secrets) ---

func TestMaskingWriterRegexOnly(t *testing.T) {
	ghPatterns := []string{`ghp_[A-Za-z0-9]{36}`}
	fakeGHP := "ghp_abcdefghij0123456789ABCDEFGHIJ012345"

	testCases := []struct {
		name     string
		patterns []string
		input    string
		expected string
	}{
		{
			name:     "GitHub PAT redacted",
			patterns: ghPatterns,
			input:    "token=" + fakeGHP + " done",
			expected: "token=*** done",
		},
		{
			name:     "no match passes through",
			patterns: ghPatterns,
			input:    "no tokens here",
			expected: "no tokens here",
		},
		{
			name:     "multiple regex patterns",
			patterns: []string{`ghp_[A-Za-z0-9]{36}`, `slack_tok-[0-9]{10}-[A-Za-z0-9]{24}`},
			input:    "gh=" + fakeGHP + " slack=slack_tok-0000000000-FAKE0TOKEN0FOR0TESTING00 end",
			expected: "gh=*** slack=*** end",
		},
		{
			name:     "invalid regex is silently skipped",
			patterns: []string{`[invalid`, `ghp_[A-Za-z0-9]{36}`},
			input:    "token=" + fakeGHP,
			expected: "token=***",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			w := newMaskingWriterWithConfig(&buf, tc.patterns, nil)
			if _, err := w.Write([]byte(tc.input)); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if err := w.Flush(); err != nil {
				t.Fatalf("unexpected flush error: %v", err)
			}
			if got := buf.String(); got != tc.expected {
				t.Errorf("got %q, want %q", got, tc.expected)
			}
		})
	}
}

// --- Both regex + secrets tests ---

func TestMaskingWriterBothRegexAndSecrets(t *testing.T) {
	patterns := jwtPatterns
	secrets := []string{"SuperSecret123!"}

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "both JWT and secret redacted",
			input:    "jwt=" + fakeJWT + " pass=SuperSecret123!",
			expected: "jwt=*** pass=***",
		},
		{
			name:     "only JWT present",
			input:    "jwt=" + fakeJWT + " end",
			expected: "jwt=*** end",
		},
		{
			name:     "only secret present",
			input:    "pass=SuperSecret123! end",
			expected: "pass=*** end",
		},
		{
			name:     "neither present",
			input:    "clean log line",
			expected: "clean log line",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			w := newMaskingWriterWithConfig(&buf, patterns, secrets)
			if _, err := w.Write([]byte(tc.input)); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if err := w.Flush(); err != nil {
				t.Fatalf("unexpected flush error: %v", err)
			}
			if got := buf.String(); got != tc.expected {
				t.Errorf("got %q, want %q", got, tc.expected)
			}
		})
	}
}

// --- Pass-through tests (feature flag off — no config) ---

func TestMaskingWriterPassThrough(t *testing.T) {
	testCases := []struct {
		name  string
		input string
	}{
		{
			name:  "JWT is NOT redacted when no config",
			input: "token=" + fakeJWT + " end",
		},
		{
			name:  "plain text passes through unchanged",
			input: "hello world\n",
		},
		{
			name:  "secret-like string passes through when no secrets configured",
			input: "password=SuperSecret123!",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			w := newMaskingWriter(&buf)
			if _, err := w.Write([]byte(tc.input)); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if err := w.Flush(); err != nil {
				t.Fatalf("unexpected flush error: %v", err)
			}
			if got := buf.String(); got != tc.input {
				t.Errorf("pass-through failed: got %q, want %q", got, tc.input)
			}
		})
	}
}

// --- Helper function tests ---

func TestBuildCombinedPatternWithNoPatterns(t *testing.T) {
	pat, prefixes := buildCombinedPattern(nil)
	if pat != nil {
		t.Errorf("expected nil pattern, got %v", pat)
	}
	if prefixes != nil {
		t.Errorf("expected nil prefixes, got %v", prefixes)
	}
}

func TestBuildCombinedPatternSkipsInvalid(t *testing.T) {
	pat, _ := buildCombinedPattern([]string{`[invalid`, `abc`})
	if pat == nil {
		t.Fatal("expected non-nil pattern")
	}
	if !pat.MatchString("abc") {
		t.Error("expected pattern to match 'abc'")
	}
}

func TestExtractLiteralPrefix(t *testing.T) {
	tests := []struct {
		pattern string
		want    string
	}{
		{`eyJ[A-Za-z0-9_-]+`, "eyJ"},
		{`ghp_[A-Za-z0-9]{36}`, "ghp_"},
		{`[A-Z]something`, ""},
		{`plain`, "plain"},
		{`a.b`, "a"},
	}

	for _, tc := range tests {
		t.Run(tc.pattern, func(t *testing.T) {
			if got := extractLiteralPrefix(tc.pattern); got != tc.want {
				t.Errorf("extractLiteralPrefix(%q) = %q, want %q", tc.pattern, got, tc.want)
			}
		})
	}
}

func BenchmarkMaskingWriterNoJWT(b *testing.B) {
	data := []byte("some normal log line without any secrets\n")
	for b.Loop() {
		var buf bytes.Buffer
		w := newMaskingWriterWithConfig(&buf, jwtPatterns, nil)
		w.Write(data)
		w.Flush()
	}
}

func BenchmarkMaskingWriterWithJWT(b *testing.B) {
	data := []byte("TOKEN=" + fakeJWT + "rfergeg" + fakeJWT + "\n")
	for b.Loop() {
		var buf bytes.Buffer
		w := newMaskingWriterWithConfig(&buf, jwtPatterns, nil)
		w.Write(data)
		w.Flush()
	}
}
