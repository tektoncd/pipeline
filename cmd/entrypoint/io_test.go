package main

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestCopyAsyncEOF(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	defer close(stopCh)

	pr, pw := io.Pipe()
	defer pr.Close()

	buf := &bytes.Buffer{}
	copyCh := copyAsync(buf, pr, stopCh)

	expectedString := "hello world"
	pw.Write([]byte(expectedString))
	pw.Close()

	if c := <-copyCh; c.err != nil {
		t.Fatalf("Unexpected error: %v", c.err)
	}
	if buf.String() != expectedString {
		t.Errorf("got: %v, wanted: %v", buf.String(), expectedString)
	}
}

func TestCopyAsyncStop(t *testing.T) {
	stopCh := make(chan struct{}, 1)

	pr, pw := io.Pipe()
	defer pr.Close()
	defer pw.Close()

	buf := &bytes.Buffer{}
	copyCh := copyAsync(buf, pr, stopCh)

	expectedString := "hello world"
	pw.Write([]byte(expectedString))

	close(stopCh)

	if c := <-copyCh; c.err != nil {
		t.Fatalf("Unexpected error: %v", c.err)
	}
	if buf.String() != expectedString {
		t.Errorf("got: %v, wanted: %v", buf.String(), expectedString)
	}
}

func TestCopyAsyncError(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	defer close(stopCh)

	pr, pw := io.Pipe()
	defer pr.Close()

	buf := &bytes.Buffer{}
	copyCh := copyAsync(buf, pr, stopCh)

	expectedString := "hello world"
	expectedError := errors.New("test error")
	pw.Write([]byte(expectedString))
	pw.CloseWithError(expectedError)

	if c := <-copyCh; !errors.Is(c.err, expectedError) {
		t.Errorf("Expected error %v but got %v", expectedError, c.err)
	}
	if buf.String() != expectedString {
		t.Errorf("got: %v, wanted: %v", buf.String(), expectedString)
	}
}
