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

package entrypoint

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInlineBudgetTracker_BasicConsumption(t *testing.T) {
	f := filepath.Join(t.TempDir(), "budget")
	bt := NewInlineBudgetTracker(f, 2048)

	if bt.Remaining() != 2048 {
		t.Errorf("Remaining() = %d, want 2048", bt.Remaining())
	}
	if bt.Consumed() != 0 {
		t.Errorf("Consumed() = %d, want 0", bt.Consumed())
	}

	if !bt.TryConsume(1000) {
		t.Fatal("TryConsume(1000) should succeed")
	}
	if bt.Remaining() != 1048 {
		t.Errorf("Remaining() = %d, want 1048", bt.Remaining())
	}
	if bt.Consumed() != 1000 {
		t.Errorf("Consumed() = %d, want 1000", bt.Consumed())
	}
}

func TestInlineBudgetTracker_ExceedsLimit(t *testing.T) {
	f := filepath.Join(t.TempDir(), "budget")
	bt := NewInlineBudgetTracker(f, 100)

	if bt.TryConsume(101) {
		t.Fatal("TryConsume(101) should fail with limit 100")
	}
	if bt.Consumed() != 0 {
		t.Errorf("Consumed() should be 0 after failed consume, got %d", bt.Consumed())
	}
}

func TestInlineBudgetTracker_ExactLimit(t *testing.T) {
	f := filepath.Join(t.TempDir(), "budget")
	bt := NewInlineBudgetTracker(f, 100)

	if !bt.TryConsume(100) {
		t.Fatal("TryConsume(100) should succeed at exact limit")
	}
	if bt.TryConsume(1) {
		t.Fatal("TryConsume(1) should fail after budget exhausted")
	}
	if bt.Remaining() != 0 {
		t.Errorf("Remaining() = %d, want 0", bt.Remaining())
	}
}

func TestInlineBudgetTracker_PersistsAcrossInstances(t *testing.T) {
	f := filepath.Join(t.TempDir(), "budget")

	// Step 1 consumes 1000 bytes
	bt1 := NewInlineBudgetTracker(f, 2048)
	if !bt1.TryConsume(1000) {
		t.Fatal("step 1: TryConsume(1000) should succeed")
	}

	// Step 2 reads persisted state and tries to consume more
	bt2 := NewInlineBudgetTracker(f, 2048)
	if bt2.Consumed() != 1000 {
		t.Errorf("step 2: Consumed() = %d, want 1000", bt2.Consumed())
	}
	if bt2.Remaining() != 1048 {
		t.Errorf("step 2: Remaining() = %d, want 1048", bt2.Remaining())
	}

	// Should succeed within remaining budget
	if !bt2.TryConsume(1000) {
		t.Fatal("step 2: TryConsume(1000) should succeed")
	}

	// Should fail — only 48 bytes left
	if bt2.TryConsume(100) {
		t.Fatal("step 2: TryConsume(100) should fail with only 48 remaining")
	}

	// Step 3 verifies final state
	bt3 := NewInlineBudgetTracker(f, 2048)
	if bt3.Consumed() != 2000 {
		t.Errorf("step 3: Consumed() = %d, want 2000", bt3.Consumed())
	}
}

func TestInlineBudgetTracker_NoFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "nonexistent", "budget")
	bt := NewInlineBudgetTracker(f, 2048)

	if bt.Consumed() != 0 {
		t.Errorf("Consumed() = %d, want 0 when file doesn't exist", bt.Consumed())
	}
	if bt.Remaining() != 2048 {
		t.Errorf("Remaining() = %d, want 2048", bt.Remaining())
	}
}

func TestInlineBudgetTracker_CorruptFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "budget")
	if err := os.WriteFile(f, []byte("not-a-number"), 0o644); err != nil {
		t.Fatal(err)
	}

	bt := NewInlineBudgetTracker(f, 2048)
	if bt.Consumed() != 0 {
		t.Errorf("Consumed() = %d, want 0 for corrupt file", bt.Consumed())
	}
}

func TestInlineBudgetTracker_NegativeValueInFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "budget")
	if err := os.WriteFile(f, []byte("-100"), 0o644); err != nil {
		t.Fatal(err)
	}

	bt := NewInlineBudgetTracker(f, 2048)
	if bt.Consumed() != 0 {
		t.Errorf("Consumed() = %d, want 0 for negative value in file", bt.Consumed())
	}
}

func TestInlineBudgetTracker_Refund(t *testing.T) {
	f := filepath.Join(t.TempDir(), "budget")
	bt := NewInlineBudgetTracker(f, 2048)

	if !bt.TryConsume(1500) {
		t.Fatal("TryConsume(1500) should succeed")
	}
	bt.Refund(1500)
	if bt.Consumed() != 0 {
		t.Errorf("Consumed() = %d, want 0 after full refund", bt.Consumed())
	}
	if bt.Remaining() != 2048 {
		t.Errorf("Remaining() = %d, want 2048 after full refund", bt.Remaining())
	}

	// Refund should persist for the next step
	bt2 := NewInlineBudgetTracker(f, 2048)
	if bt2.Consumed() != 0 {
		t.Errorf("step 2: Consumed() = %d, want 0 after persisted refund", bt2.Consumed())
	}

	// Over-refund clamps to zero
	bt2.TryConsume(100)
	bt2.Refund(500)
	if bt2.Consumed() != 0 {
		t.Errorf("Consumed() = %d, want 0 after over-refund", bt2.Consumed())
	}
}

func TestInlineBudgetExceededError_Error(t *testing.T) {
	err := &InlineBudgetExceededError{
		ArtifactName: "sbom",
		ArtifactSize: 2048,
		Remaining:    500,
	}
	msg := err.Error()
	if msg == "" {
		t.Fatal("expected non-empty error message")
	}
}
