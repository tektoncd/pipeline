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
	"fmt"
	"os"
	"strconv"
	"strings"
)

// InlineBudgetTracker tracks the pod-wide inline artifact budget using a
// shared file. Steps in a Pod run sequentially, so no locking is needed.
type InlineBudgetTracker struct {
	budgetFile string
	limit      int
	consumed   int
}

// NewInlineBudgetTracker creates a tracker that reads existing consumption
// from budgetFile (if present) and enforces a pod-wide limit on raw inline
// artifact bytes.
func NewInlineBudgetTracker(budgetFile string, limit int) *InlineBudgetTracker {
	t := &InlineBudgetTracker{
		budgetFile: budgetFile,
		limit:      limit,
	}
	data, err := os.ReadFile(budgetFile)
	if err == nil {
		n, parseErr := strconv.Atoi(strings.TrimSpace(string(data)))
		if parseErr == nil && n >= 0 {
			t.consumed = n
		}
	}
	return t
}

// TryConsume attempts to reserve size raw bytes from the pod budget.
// Returns true if the reservation succeeded, false if it would exceed the limit.
// On success the consumed total is persisted to the budget file so subsequent
// steps see the updated value.
func (t *InlineBudgetTracker) TryConsume(size int) bool {
	if t.consumed+size > t.limit {
		return false
	}
	t.consumed += size
	t.persist()
	return true
}

// Refund releases size raw bytes previously reserved by TryConsume.
// Used when an artifact was accepted into the budget but later falls back
// to OCI because the encoded termination message would overflow.
func (t *InlineBudgetTracker) Refund(size int) {
	if size <= 0 {
		return
	}
	t.consumed -= size
	if t.consumed < 0 {
		t.consumed = 0
	}
	t.persist()
}

func (t *InlineBudgetTracker) persist() {
	os.WriteFile(t.budgetFile, []byte(strconv.Itoa(t.consumed)), 0o600) //nolint:errcheck
}

// Remaining returns the number of raw bytes still available for inlining.
func (t *InlineBudgetTracker) Remaining() int {
	r := t.limit - t.consumed
	if r < 0 {
		return 0
	}
	return r
}

// Consumed returns the total raw bytes consumed so far.
func (t *InlineBudgetTracker) Consumed() int {
	return t.consumed
}

// InlineBudgetExceededError is returned when an artifact exceeds both the
// inline budget and has no backend to fall back to.
type InlineBudgetExceededError struct {
	ArtifactName string
	ArtifactSize int64
	Remaining    int
}

func (e *InlineBudgetExceededError) Error() string {
	return fmt.Sprintf("artifact %q (%d bytes) exceeds remaining pod inline budget (%d bytes) and no storage backend is configured",
		e.ArtifactName, e.ArtifactSize, e.Remaining)
}
