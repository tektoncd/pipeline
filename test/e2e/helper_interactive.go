package e2e

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"runtime/debug"
	"sync"
	"testing"
	"time"

	"github.com/Netflix/go-expect"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

type CapturingPassThroughWriter struct {
	m   sync.RWMutex
	buf bytes.Buffer
	w   io.Writer
}

// NewCapturingPassThroughWriter creates new CapturingPassThroughWriter
func NewCapturingPassThroughWriter(w io.Writer) *CapturingPassThroughWriter {
	return &CapturingPassThroughWriter{
		w: w,
	}
}

func (w *CapturingPassThroughWriter) Write(d []byte) (int, error) {
	w.m.Lock()
	defer w.m.Unlock()
	w.buf.Write(d)
	return w.w.Write(d)
}

// Bytes returns bytes written to the writer
func (w *CapturingPassThroughWriter) Bytes() []byte {
	w.m.RLock()
	defer w.m.RUnlock()
	return w.buf.Bytes()
}

func testCloser(t *testing.T, closer io.Closer) {
	if err := closer.Close(); err != nil {
		t.Errorf("Close failed: %s", err)
		debug.PrintStack()
	}
}

type InteractiveTestData struct {
	Cmd           []string
	OpsVsExpected []interface{}
	ExpectedLogs  []string
}

type Operation struct {
	Ops      string
	Expected string
}

// Helps to Run Interactive Session
func RunInteractivePipelineLogs(t *testing.T, namespace string, ops *InteractiveTestData) *expect.Console {

	c, err := expect.NewConsole(expect.WithStdout(os.Stdout))
	if err != nil {
		t.Fatal(err)
	}
	defer testCloser(t, c)

	cmd := exec.Command("./tkn", ops.Cmd[0:len(ops.Cmd)]...)
	cmd.Stdin = c.Tty()
	var errStdout, errStderr error
	stdoutIn, _ := cmd.StdoutPipe()
	stderrIn, _ := cmd.StderrPipe()
	stdout := NewCapturingPassThroughWriter(os.Stdout)
	stderr := NewCapturingPassThroughWriter(os.Stderr)

	err = cmd.Start()
	if err != nil {
		t.Error(err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := c.ExpectEOF()
		if err != nil {
			t.Error(err)
		}
	}()

	go func() {
		defer wg.Done()
		_, errStdout = io.Copy(stdout, stdoutIn)
		_, errStderr = io.Copy(stderr, stderrIn)

	}()

	for _, ops := range ops.OpsVsExpected {
		time.Sleep(1 * time.Second)

		assert.Assert(t, is.Regexp(".*(> "+ops.(*Operation).Expected+").*?", string(stdout.Bytes())))

		time.Sleep(1 * time.Second)
		if _, err := c.SendLine(ops.(*Operation).Ops); err != nil {
			t.Error(err)
		}

	}

	wg.Wait()

	for _, log := range ops.ExpectedLogs {
		time.Sleep(1 * time.Second)
		logs := string(stdout.Bytes())
		assert.Assert(t, is.Regexp(log, logs))
	}

	err = cmd.Wait()
	if err != nil {
		t.Errorf("cmd.Run() failed with %s\n", err)
	}
	if errStdout != nil || errStderr != nil {
		t.Errorf("failed to capture stdout or stderr\n")
	}

	return c

}
