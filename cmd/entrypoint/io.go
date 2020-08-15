package main

import (
	"errors"
	"io"
	"math"
	"os"
	"time"
)

type ioResult struct {
	numBytes int
	err      error
}

// readAsync implements a non-blocking read.
func readAsync(r io.Reader, p []byte) <-chan ioResult {
	resultCh := make(chan ioResult, 1)
	go func() {
		defer close(resultCh)
		n, err := r.Read(p)
		resultCh <- ioResult{n, err}
	}()
	return resultCh
}

// copyAsync performs a non-blocking copy from src to dst.
func copyAsync(dst io.Writer, src io.Reader, stopCh <-chan struct{}) <-chan ioResult {
	resultCh := make(chan ioResult, 1)
	go func() {
		defer close(resultCh)

		buf := make([]byte, 1024)
		result := ioResult{}
		readCh := readAsync(src, buf)
		done := false
		timer := time.NewTimer(time.Duration(math.MaxInt64))
		defer timer.Stop()

		for !done {
			// If the stop channel is signalled, continue the loop to read the rest of the available
			// data with a short timeout instead of a non-blocking read to mitigate the race between
			// this loop and Read() running in another goroutine.
			if stopCh == nil {
				if !timer.Stop() {
					<-timer.C
				}
				timer.Reset(100 * time.Millisecond)
			}
			select {
			case r := <-readCh:
				if r.numBytes != 0 {
					nw, err := dst.Write(buf[:r.numBytes])
					result.numBytes += nw
					if err != nil {
						result.err = err
						done = true
					} else if nw < r.numBytes {
						result.err = io.ErrShortWrite
						done = true
					}
				}
				if r.err != nil {
					if !errors.Is(r.err, io.EOF) {
						result.err = r.err
					}
					done = true
				}
				if !done {
					readCh = readAsync(src, buf)
				}
			case <-stopCh:
				stopCh = nil
			case <-timer.C:
				done = true
			}
		}

		resultCh <- result
	}()
	return resultCh
}

type asyncWriter struct {
	w     io.Writer
	errCh <-chan error
}

func (w *asyncWriter) Write(p []byte) (int, error) {
	select {
	case err := <-w.errCh:
		return 0, err
	default:
		return w.w.Write(p)
	}
}

// AsyncWriter creates a write that duplicates its writes to the provided writer asynchronously.
func AsyncWriter(w io.Writer, stopCh <-chan struct{}) (io.Writer, error) {
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)

		errCh <- (<-copyAsync(w, pr, stopCh)).err
		pr.Close()
		pw.Close()
	}()

	return &asyncWriter{pw, errCh}, nil
}
