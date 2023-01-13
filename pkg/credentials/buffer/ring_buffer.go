// Package buffer implements a ring buffer for credential filtering.
//
// You can initialize a ring buffer with a predefined capacity and add some bytes to it.
//
//   rb := buffer.NewRingBuffer(2)
//   rb.Add(1)
//   rb.Add(2)
//   b, err := rb.Get(0)  // return 1, nil
//   b, err := rb.Get(1)  // return 2, nil
//
// When adding more bytes than the capacity the buffer will overflow and drop the oldest byte:
//
//   rb.Add(3)
//   b, err := rb.Get(0)  // return 2, nil
//   b, err := rb.Get(1)  // return 3, nil
//
// You can pop one or multiple bytes from the start of the buffer:
//
//   b, err := rb.Pop()  // return 2, nil
//   b, err := rb.Pop()  // return 3, nil
//   b, err := rb.Pop()  // return _, err because buffer is empty now
//
// Match a buffer for whole sequences of bytes:
// 
//   rb := buffer.NewRingBuffer(2) 
//   rb.Add(1)
//   rb.Add(2)
//   m, err := rb.MatchStart([]byte{1,2}) // return true, nil
//   m, err := rb.MatchStart([]byte{2,3}) // return false, nil
package buffer

import (
	"fmt"
	"math"
)

// RingBuffer implements a simple ring buffer with a static maximal length.
type RingBuffer struct {
	start         int
	currentLength int
	buffer        []byte
}

// NewRingBuffer creates a RingBuffer with the given length.
func NewRingBuffer(capacity int) *RingBuffer {
	return &RingBuffer{
		start:         0,
		currentLength: 0,
		buffer:        make([]byte, capacity),
	}
}

// IsFull checks if the buffer is full.
func (rb *RingBuffer) IsFull() bool {
	return rb.currentLength == len(rb.buffer)
}

// IsEmpty checks if the buffer is empty.
func (rb *RingBuffer) IsEmpty() bool {
	return rb.currentLength == 0
}

// PopFirst removes the first byte from the buffer and returns it.
// Returns an error when empty.
func (rb *RingBuffer) PopFirst() (byte, error) {
	if rb.currentLength == 0 {
		return 0, fmt.Errorf("can not pop byte, buffer is empty")
	}

	b := rb.buffer[rb.start]
	rb.incrementStart()
	rb.currentLength--

	return b, nil
}

// Pop removes the n first bytes from the buffer and returns them.
// Returns an error when more bytes are requested than contained in the buffer or n smaller than zero.
func (rb *RingBuffer) Pop(n int) ([]byte, error) {
	if n < 0 {
		return nil, fmt.Errorf("can not pop %d bytes, must be >=0 ", n)
	}
	if n > rb.currentLength {
		return nil, fmt.Errorf("can not pop %d bytes, buffer only contains %d bytes", n, rb.currentLength)
	}
	poppedBytes := make([]byte, 0, n)
	for i := 0; i < n; i++ {
		b, err := rb.PopFirst()
		if err != nil {
			return nil, err
		}
		poppedBytes = append(poppedBytes, b)
	}

	return poppedBytes, nil
}

// PopAll removes all bytes from the buffer and returns them.
func (rb *RingBuffer) PopAll() []byte {
	b, _ := rb.Pop(rb.currentLength)
	return b
}

// Get retrieves the byte at index from the buffer.
// It does not pop it.
// If the index is out of bounds an error will be returned.
func (rb *RingBuffer) Get(index int) (byte, error) {
	if index < 0 {
		return 0, fmt.Errorf("can not get byte with index %d, index must be >= 0", index)
	}
	if index >= rb.currentLength {
		return 0, fmt.Errorf("can not get byte at index %d, buffer only has %d bytes", index, rb.currentLength)
	}

	return rb.buffer[(rb.start+index)%len(rb.buffer)], nil
}

// Add pushes multiple bytes to the end of the buffer.
// If the ring buffer is full, the oldest byte will be dropped.
func (rb *RingBuffer) AddMulti(bs []byte) {
	for _, b := range bs {
		rb.Add(b)
	}
}

// Add pushes a byte to the end of the buffer.
// If the ring buffer is full, the oldest byte will be dropped.
func (rb *RingBuffer) Add(b byte) {
	next := (rb.start + rb.currentLength) % len(rb.buffer)
	if rb.IsFull() {
		next = rb.start
		rb.incrementStart()
	}
	rb.currentLength = minInt(rb.currentLength+1, len(rb.buffer))
	rb.buffer[next] = b
}

// MatchStart tests if the start of the buffer contains the given byte sequence.
// Will return false If the buffer does not contain enough bytes.
func (rb *RingBuffer) MatchStart(bytes []byte) (bool, error) {
	if len(bytes) > rb.currentLength {
		return false, nil
	}
	for i := 0; i < len(bytes); i++ {
		b, err := rb.Get(i)
		if err != nil {
			return false, err
		}
		if b != bytes[i] {
			return false, nil
		}
	}

	return true, nil
}

func (rb *RingBuffer) incrementStart() {	
	rb.start = (rb.start + 1) % len(rb.buffer)
}

func minInt(i1 int, i2 int) int {
	return int(math.Min(float64(i1), float64(i2)))
}