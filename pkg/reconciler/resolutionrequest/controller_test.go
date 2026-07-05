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

package resolutionrequest

import (
	"errors"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// panicMessage captures the message a zap SugaredLogger emits when Panicf is
// called, so we can assert on how the format verb renders the wrapped error.
func panicMessage(t *testing.T, template string, args ...interface{}) (msg string) {
	t.Helper()
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(zap.NewProductionEncoderConfig()),
		zapcore.AddSync(discard{}),
		zapcore.DebugLevel,
	)
	logger := zap.New(core).Sugar()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected SugaredLogger.Panicf to panic, but it did not")
		}
		s, ok := r.(string)
		if !ok {
			t.Fatalf("expected panic value to be the rendered message string, got %T: %v", r, r)
		}
		msg = s
	}()

	logger.Panicf(template, args...)
	return ""
}

// TestPanicfDoesNotUseErrorWrapVerb guards the informer event handler
// registration panics in this package (and the sibling reconciler controllers),
// which log through logging.FromContext(ctx), a *zap.SugaredLogger. That logger
// formats with fmt.Sprintf, which does not implement the %w error-wrapping verb.
// Using %w there renders the literal "%!w(...)" instead of the error, so these
// call sites must use %v. This test fails if the verb regresses back to %w.
func TestPanicfDoesNotUseErrorWrapVerb(t *testing.T) {
	sentinel := errors.New("boom")
	const template = "Couldn't register ResolutionRequest informer event handler: %v"

	msg := panicMessage(t, template, sentinel)

	if strings.Contains(msg, "%!w(") {
		t.Errorf("panic message contains a malformed verb marker %q, want the error text; message: %q", "%!w(", msg)
	}
	if !strings.Contains(msg, sentinel.Error()) {
		t.Errorf("panic message %q does not contain the wrapped error text %q", msg, sentinel.Error())
	}

	// Sanity check that this test would catch a regression to %w: the wrapping
	// verb is not understood by fmt.Sprintf and renders as a malformed verb.
	buggy := panicMessage(t, "Couldn't register ResolutionRequest informer event handler: %w", sentinel)
	if !strings.Contains(buggy, "%!w(") {
		t.Errorf("expected %%w to render a malformed verb marker %q through SugaredLogger.Panicf, got %q", "%!w(", buggy)
	}
}

// discard is an io.Writer that drops everything written to it, keeping the test
// logger silent while still exercising the real panic path.
type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }
