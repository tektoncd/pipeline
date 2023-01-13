package buffer_test

import (
	"fmt"
	"testing"

	"github.com/tektoncd/pipeline/pkg/credentials/buffer"
)

func TestIsFull(t *testing.T) {
	type isFullTest struct {
		capacity     int
		bytesToAdd   []byte
		expectedFull bool
	}
	testCases := []isFullTest{
		{0, []byte{}, true},
		{1, []byte{}, false},
		{1, []byte{1}, true},
		{2, []byte{1}, false},
		{2, []byte{1, 2}, true},
		{3, []byte{}, false},
		{3, []byte{1, 2}, false},
		{3, []byte{1, 2, 3}, true},
	}

	for _, tc := range testCases {
		name := fmt.Sprintf("cap_%d_full_%t_withbytes_%v", tc.capacity, tc.expectedFull, tc.bytesToAdd)
		t.Run(name, func(t1 *testing.T) {
			buf := buffer.NewRingBuffer(tc.capacity)

			buf.AddMulti(tc.bytesToAdd)

			isFull := buf.IsFull()

			if isFull != tc.expectedFull {
				t1.Errorf("expected buffer full %t, but was not", tc.expectedFull)
			}
		})
	}
}

func TestIsEmpty(t *testing.T) {
	type isEmptyTest struct {
		capacity      int
		bytesToAdd    []byte
		expectedEmpty bool
	}
	testCases := []isEmptyTest{
		{0, []byte{}, true},
		{1, []byte{}, true},
		{2, []byte{}, true},
		{3, []byte{}, true},
		{1, []byte{1}, false},
		{2, []byte{1}, false},
		{2, []byte{1, 2}, false},
		{3, []byte{1, 2}, false},
		{3, []byte{1, 2, 3}, false},
	}

	for _, tc := range testCases {
		name := fmt.Sprintf("cap_%d_empty_%t_withbytes_%v", tc.capacity, tc.expectedEmpty, tc.bytesToAdd)
		t.Run(name, func(t1 *testing.T) {
			buf := buffer.NewRingBuffer(tc.capacity)

			buf.AddMulti(tc.bytesToAdd)

			isEmpty := buf.IsEmpty()

			if isEmpty != tc.expectedEmpty {
				t1.Errorf("expected buffer empty %t, but was not", tc.expectedEmpty)
			}
		})
	}
}

func TestNotIsEmptyAfterPoppingPartially(t *testing.T) {
	buf := buffer.NewRingBuffer(3)

	buf.Add(1)
	buf.Add(2)
	buf.Add(3)

	_, err := buf.Pop(2)
	if err != nil {
		t.Fatal(err)
	}
	empty := buf.IsEmpty()
	if empty {
		t.Error("Expected buffer to be not empty")
	}
}

func TestIsEmptyAfterPoppingAllBytes(t *testing.T) {
	buf := buffer.NewRingBuffer(2)

	buf.Add(1)
	buf.Add(2)

	_, err := buf.PopFirst()
	if err != nil {
		t.Fatal(err)
	}
	_, err = buf.PopFirst()
	if err != nil {
		t.Fatal(err)
	}

	isEmpty := buf.IsEmpty()
	if !isEmpty {
		t.Error("expected buffer to be empty at end")
	}
}

func TestGet(t *testing.T) {
	type getTest struct {
		capacity        int
		bytesToAdd      []byte
		indexToRetrieve int
		expectedByte    byte
		expectedError   bool
	}
	testCases := []getTest{
		{0, []byte{}, -1, 0, true},
		{0, []byte{}, 0, 0, true},
		{1, []byte{}, 1, 0, true},
		{1, []byte{}, -1, 0, true},
		{3, []byte{1, 2}, 2, 0, true},
		{3, []byte{1, 2, 3}, 0, 1, false},
		{3, []byte{1, 2, 3}, 1, 2, false},
		{3, []byte{1, 2, 3}, 2, 3, false},
		{3, []byte{1, 2, 3, 4}, 2, 4, false},    // overflow buffer
		{3, []byte{1, 2, 3, 4, 5}, 2, 5, false}, // overflow buffer
	}

	for _, tc := range testCases {
		name := fmt.Sprintf("cap_%d_index_%d_byte_%b_err_%t_withbytes_%v", tc.capacity,
			tc.indexToRetrieve, tc.expectedByte, tc.expectedError, tc.bytesToAdd)
		t.Run(name, func(t1 *testing.T) {
			buf := buffer.NewRingBuffer(tc.capacity)

			buf.AddMulti(tc.bytesToAdd)

			b, err := buf.Get(tc.indexToRetrieve)
			if tc.expectedError && err == nil {
				t1.Error("expected error but got nil")
			} else if !tc.expectedError && err != nil {
				t1.Error(err)
			} else if tc.expectedByte != b {
				t1.Errorf("expected byte %b but got %b", tc.expectedByte, b)
			}

			if !tc.expectedError {
				b, err := buf.Get(tc.indexToRetrieve)
				if err != nil {
					t1.Error("when doing second get", err)
				}
				if tc.expectedByte != b {
					t1.Errorf("expected byte %b but got %b on second get", tc.expectedByte, b)
				}
			}
		})
	}
}

func TestPopFirst(t *testing.T) {
	type popFirstTest struct {
		capacity      int
		bytesToAdd    []byte
		expectedBytes []byte
		expectedError bool
	}
	testCases := []popFirstTest{
		{0, []byte{}, []byte{}, false},
		{0, []byte{}, []byte{0}, true},
		{3, []byte{}, []byte{0}, true},
		{3, []byte{}, []byte{0}, true},
		{5, []byte{1, 2, 3}, []byte{1}, false},
		{5, []byte{1, 2, 3}, []byte{1, 2}, false},
		{5, []byte{1, 2, 3}, []byte{1, 2, 3}, false},
		{5, []byte{1, 2, 3}, []byte{1, 2, 3, 0}, true},
		{3, []byte{1, 2, 3, 4}, []byte{2, 3, 4}, false},         // normal ring overflow
		{3, []byte{1, 2, 3, 4, 5, 6}, []byte{4, 5, 6}, false},   // normal ring overflow
		{3, []byte{1, 2, 3, 4, 5, 6}, []byte{4, 5, 6, 0}, true}, // normal ring overflow, but popping to much
	}

	for _, tc := range testCases {
		name := fmt.Sprintf("cap_%d_err_%t_withbytes_%v_popbytes_%v", tc.capacity,
			tc.expectedError, tc.bytesToAdd, tc.expectedBytes)
		t.Run(name, func(t1 *testing.T) {
			buf := buffer.NewRingBuffer(tc.capacity)

			buf.AddMulti(tc.bytesToAdd)

			var b byte
			var err error = nil
			for i, expectedByte := range tc.expectedBytes {
				b, err = buf.PopFirst()
				if err != nil {
					break
				}

				if b != expectedByte {
					t.Errorf("expected byte %b at index %d, but got %b", expectedByte, i, b)
				}
			}
			if tc.expectedError && err == nil {
				t1.Error("expected error but got nil")
			} else if !tc.expectedError && err != nil {
				t1.Error(err)
			}
		})
	}
}

func TestPop(t *testing.T) {
	type popTest struct {
		capacity      int
		bytesToAdd    []byte
		expectedBytes []byte
		expectedError bool
	}
	testCases := []popTest{
		{0, []byte{}, []byte{}, false},
		{0, []byte{}, []byte{0}, true}, // can not pop number of expected bytes from empty buffer
		{3, []byte{}, []byte{0}, true}, // can not pop number of expected bytes from empty buffer
		{3, []byte{}, []byte{0}, true}, // can not pop number of expected bytes from empty buffer
		{5, []byte{1, 2, 3}, []byte{1}, false},
		{5, []byte{1, 2, 3}, []byte{1, 2}, false},
		{5, []byte{1, 2, 3}, []byte{1, 2, 3}, false},
		{5, []byte{1, 2, 3}, []byte{1, 2, 3, 0}, true},
		{3, []byte{1, 2, 3, 4}, []byte{2, 3, 4}, false},    // overflow buffer
		{3, []byte{1, 2, 3, 4, 5}, []byte{3, 4, 5}, false}, // overflow buffer
	}

	for _, tc := range testCases {
		name := fmt.Sprintf("cap_%d_err_%t_withbytes_%v_popbytes_%v", tc.capacity,
			tc.expectedError, tc.bytesToAdd, tc.expectedBytes)
		t.Run(name, func(t1 *testing.T) {
			buf := buffer.NewRingBuffer(tc.capacity)

			buf.AddMulti(tc.bytesToAdd)

			bytes, err := buf.Pop(len(tc.expectedBytes))
			if tc.expectedError && err == nil {
				t1.Error("expected error but got nil")
			} else if !tc.expectedError && err != nil {
				t1.Error(err)
			} else {
				for i, b := range bytes {
					if b != tc.expectedBytes[i] {
						t.Errorf("expected byte %b at index %d, but got %b", tc.expectedBytes[i], i, b)
					}
				}
			}
		})
	}
}

func TestPopReturnsErrorWhenLessThanZeroBytesRequested(t *testing.T) {
	buf := buffer.NewRingBuffer(3)

	_, err := buf.Pop(-1)
	if err == nil {
		t.Error("expected error to returned but got nil")
	}
}

func TestPopAll(t *testing.T) {
	type popTest struct {
		capacity   int
		bytesToAdd []byte
	}
	testCases := []popTest{
		{0, []byte{}},
		{5, []byte{1}},
		{5, []byte{1, 2}},
		{5, []byte{1, 2, 3}},
		{5, []byte{1, 2, 3, 4}},
		{5, []byte{1, 2, 3, 4, 5}},
	}

	for _, tc := range testCases {
		name := fmt.Sprintf("cap_%d_withbytes_%v", tc.capacity, tc.bytesToAdd)
		t.Run(name, func(t1 *testing.T) {
			buf := buffer.NewRingBuffer(tc.capacity)

			buf.AddMulti(tc.bytesToAdd)

			bytes := buf.PopAll()
			for i, b := range bytes {
				if b != tc.bytesToAdd[i] {
					t.Errorf("expected byte %b at index %d, but got %b", tc.bytesToAdd[i], i, b)
				}
			}
		})
	}
}

func TestMatchStart(t *testing.T) {
	type matchStartTest struct {
		capacity     int
		bytesToAdd   []byte
		bytesToMatch []byte
		expectMatch  bool
		expectError  bool
	}
	testCases := []matchStartTest{
		{0, []byte{}, []byte{0}, false, false},
		{5, []byte{1, 2, 3}, []byte{1, 2, 3, 4}, false, false},
		{0, []byte{}, []byte{}, true, false},
		{5, []byte{1, 2, 3}, []byte{1}, true, false},
		{5, []byte{1, 2, 3}, []byte{1}, true, false},
		{5, []byte{1, 2, 3}, []byte{1, 2}, true, false},
		{5, []byte{1, 2, 3}, []byte{1, 2, 3}, true, false},
		{5, []byte{1, 2, 3}, []byte{2}, false, false},
		{5, []byte{1, 2, 3}, []byte{2, 3}, false, false},
		{3, []byte{1, 2, 3, 4, 5, 6}, []byte{4}, true, false},       // normal buffer overflow
		{3, []byte{1, 2, 3, 4, 5, 6}, []byte{4, 5}, true, false},    // normal buffer overflow
		{3, []byte{1, 2, 3, 4, 5, 6}, []byte{4, 5, 6}, true, false}, // normal buffer overflow
	}

	for _, tc := range testCases {
		name := fmt.Sprintf("cap_%d_matchbytes_%v_match_%t_err_%t_withbytes_%v", tc.capacity,
			tc.bytesToMatch, tc.expectMatch, tc.expectError, tc.bytesToAdd)
		t.Run(name, func(t1 *testing.T) {
			buf := buffer.NewRingBuffer(tc.capacity)

			buf.AddMulti(tc.bytesToAdd)

			isMatch, err := buf.MatchStart(tc.bytesToMatch)
			if tc.expectError && err == nil {
				t1.Error("expected error but got nil")
			} else if !tc.expectError && err != nil {
				t1.Error(err)
			} else {
				if isMatch != tc.expectMatch {
					t.Errorf("expected match %t, but got opposite", tc.expectMatch)
				}
			}
		})
	}
}
