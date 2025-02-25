//
// Copyright 2025 The Sigstore Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package cliplugin implements the plugin functionality.
package cliplugin

import (
	"context"
	"io"
	"os/exec"
)

// cmd is an interface for os/exec.Cmd.
type cmd interface {
	Output() ([]byte, error)
}

// makeCmdFunc is a type for a function that can create a cmd.
type makeCmdFunc func(ctx context.Context, stdin io.Reader, stderr io.Writer, name string, args ...string) cmd

// makeCmd is an implementation of makeCmdFunc.
func makeCmd(ctx context.Context, stdin io.Reader, stderr io.Writer, name string, args ...string) cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = stdin
	cmd.Stderr = stderr
	return cmd
}

// cmdExitError is an interface for os/exec.ExitError.
type cmdExitError interface {
	Error() string
	ExitCode() int
}
