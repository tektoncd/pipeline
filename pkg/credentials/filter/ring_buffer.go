package filter

import (
	"math"
)

// RingBuffer implements a simple ring buffer with a static maximal length.
type RingBuffer struct {
	start         int
	end           int
	currentLength int
	buffer        []byte
}

// NewRingBuffer creates a RingBuffer with the given length.
func NewRingBuffer(length int) *RingBuffer {
	return &RingBuffer{
		start:         0,
		end:           -1,
		currentLength: 0,
		buffer:        make([]byte, length),
	}
}

// IsFull checks if the buffer is full.
func (buffer *RingBuffer) IsFull() bool {
	return buffer.currentLength == len(buffer.buffer)
}

// IsEmpty checks if the buffer is empty.
func (buffer *RingBuffer) IsEmpty() bool {
	return buffer.currentLength == 0
}

// PopFirst removes the first byte from the buffer and returns it.
func (buffer *RingBuffer) PopFirst() byte {
	b := buffer.buffer[buffer.start]
	buffer.start = (buffer.start + 1) % len(buffer.buffer)
	buffer.currentLength--

	return b
}

// Pop removes the n first bytes from the buffer and returns them.
func (buffer *RingBuffer) Pop(n int) []byte {
	poppedBytes := make([]byte, 0, n)
	for i := 0; i < n; i++ {
		poppedBytes = append(poppedBytes, buffer.PopFirst())
	}

	return poppedBytes
}

// PopAll removes all bytes from the buffer and returns them.
func (buffer *RingBuffer) PopAll() []byte {
	return buffer.Pop(buffer.currentLength)
}

// Get retrieves the byte at index from the buffer.
// It does not pop it.
func (buffer *RingBuffer) Get(index int) byte {
	return buffer.buffer[(buffer.start+index)%len(buffer.buffer)]
}

// Add pushes a byte to the end of the buffer.
func (buffer *RingBuffer) Add(b byte) {
	buffer.end = (buffer.end + 1) % len(buffer.buffer)
	buffer.currentLength = int(math.Min(float64(buffer.currentLength+1), float64(len(buffer.buffer))))
	buffer.buffer[buffer.end] = b
}

// MatchStart tests if the start of the buffer contains the given byte sequence.
func (buffer *RingBuffer) MatchStart(bytes []byte) bool {
	if len(bytes) > buffer.currentLength {
		return false
	}
	for i := 0; i < len(bytes); i++ {
		b := buffer.Get(i)
		if b != bytes[i] {
			return false
		}
	}

	return true
}
