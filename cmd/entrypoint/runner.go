package main

import (
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/tektoncd/pipeline/pkg/entrypoint"
)

// TODO(jasonhall): Test that original exit code is propagated and that
// stdout/stderr are collected -- needs e2e tests.

// realRunner actually runs commands.
type realRunner struct{}

var _ entrypoint.Runner = (*realRunner)(nil)

func (*realRunner) Run(args ...string) error {
	var systemSignals = make(chan os.Signal, 1)
	defer close(systemSignals)
	signal.Notify(systemSignals)
	defer signal.Reset()

	if len(args) == 0 {
		return nil
	}
	name, args := args[0], args[1:]

	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Create dedicated pgidgroup for forwarding
	// signals to main process and all children
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	go func() {
		for s := range systemSignals {
			if s != syscall.SIGCHLD {
				// Forward signal to main process and all children
				syscall.Kill(-cmd.Process.Pid, s.(syscall.Signal))
			}
		}
	}()

	return nil
}