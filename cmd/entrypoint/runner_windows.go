//go:build windows

/*
Copyright 2021 The Tekton Authors

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

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/tektoncd/pipeline/pkg/entrypoint"
)

// TODO(jasonhall): Test that original exit code is propagated and that
// stdout/stderr are collected -- needs e2e tests.

// realRunner actually runs commands.
type realRunner struct {
	stdoutPath string
	stderrPath string
}

var _ entrypoint.Runner = (*realRunner)(nil)

func (rr *realRunner) Run(ctx context.Context, args ...string) error {
	if rr.stdoutPath != "" || rr.stderrPath != "" {
		return errors.New("step.StdoutPath and step.StderrPath not supported on Windows")
	}
	if len(args) == 0 {
		return nil
	}
	name, args := args[0], args[1:]

	// Resolve the executable via LookPath to guard against non-static /
	// unverified command names being passed directly to exec.CommandContext.
	resolvedName, err := exec.LookPath(name)
	if err != nil {
		return err
	}

	// Enforce that LookPath returned an absolute path before using it as the
	// command to exec.CommandContext, preventing any path-relative injection.
	if !filepath.IsAbs(resolvedName) {
		return fmt.Errorf("resolved command path %q is not absolute, refusing to execute", resolvedName)
	}

	cmd := exec.CommandContext(ctx, resolvedName, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Run the defined command
	if err := cmd.Run(); err != nil {
		return err
	}
	return ctx.Err()
}
