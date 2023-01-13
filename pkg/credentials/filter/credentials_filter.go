package filter

import (
	"fmt"
	"io"
	"strings"
)

// FilteredCredentialsWriter is a writer implementation that filters a set of credentials from the written bytes.
type FilteredCredentialsWriter struct {
	detectedSecrets []*DetectedSecret
	ringBuffer      *RingBuffer
	out             io.WriteCloser
}

// NewFilteredCredentialsWriter creates a FilterCredentialsWriter with a set of credentials to filter.
// It will write the filtered output to the given writer.
func NewFilteredCredentialsWriter(detectedSecrets []*DetectedSecret, out io.WriteCloser) *FilteredCredentialsWriter {
	longestSecretLength := 0

	for _, secret := range detectedSecrets {
		if len(secret.Value) > longestSecretLength {
			longestSecretLength = len(secret.Value)
		}
	}

	var ringBuffer *RingBuffer
	if len(detectedSecrets) > 0 && longestSecretLength > 0 {
		ringBuffer = NewRingBuffer(longestSecretLength)
	}

	return &FilteredCredentialsWriter{
		detectedSecrets: detectedSecrets,
		ringBuffer:      ringBuffer,
		out:             out,
	}
}

// Write write a single byte to the output stream.
// It keeps a buffer and only flushes bytes that are guaranteed to not be part of a secret.
func (w *FilteredCredentialsWriter) Write(p []byte) (n int, err error) {
	bytesToOut := make([]byte, 0)
	if w.ringBuffer == nil {
		bytesToOut = append(bytesToOut, p...)
	} else {
		for _, b := range p {
			if w.ringBuffer.IsFull() {
				firstByte := w.ringBuffer.PopFirst()
				bytesToOut = append(bytesToOut, firstByte)
			}

			w.ringBuffer.Add(b)

			for _, secret := range w.detectedSecrets {
				match := w.ringBuffer.MatchStart(secret.Value)
				if match {
					popped := w.ringBuffer.Pop(len(secret.Value))
					redactedString := w.getRedactedSecretString(secret.Name)
					redacted := strings.ReplaceAll(string(popped), string(secret.Value), redactedString)
					bytesToOut = append(bytesToOut, []byte(redacted)...)
				}
			}
		}
	}

	_, err = w.out.Write(bytesToOut)
	if err != nil {
		return 0, err
	}

	return len(p), nil
}

// Close will write remaining bytes from the internal buffer to the output stream.
// The remaining bytes will be filtered for any remaining secrets.
func (w *FilteredCredentialsWriter) Close() error {
	if w.ringBuffer == nil || w.ringBuffer.IsEmpty() {
		return nil
	}

	remainingBytes := string(w.ringBuffer.PopAll())
	for _, secret := range w.detectedSecrets {
		redactedString := w.getRedactedSecretString(secret.Name)
		remainingBytes = strings.ReplaceAll(string(remainingBytes), string(secret.Value), redactedString)
	}

	_, err := w.out.Write([]byte(remainingBytes))
	if err != nil {
		return err
	}

	return w.out.Close()
}

func (w *FilteredCredentialsWriter) getRedactedSecretString(name string) string {
	return fmt.Sprintf("[REDACTED:%s]", name)
}