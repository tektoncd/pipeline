// Copyright © 2019 The Tekton Authors.
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

package test

import (
	"bytes"

	"github.com/spf13/cobra"
)

// ExecuteCommand executes the root command passing the args and returns
// the output as a string and error
func ExecuteCommand(root *cobra.Command, args ...string) (string, error) {
	_, output, err := ExecuteCommandC(root, args...)
	return output, err
}

// ExecuteCommandC executes the root command passing the args and returns
// the root command, output as a string and error if any
func ExecuteCommandC(c *cobra.Command, args ...string) (*cobra.Command, string, error) {
	buf := new(bytes.Buffer)
	c.SetOutput(buf)
	c.SetArgs(args)
	c.SilenceUsage = true

	root, err := c.ExecuteC()

	return root, buf.String(), err
}
