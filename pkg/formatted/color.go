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

package formatted

import (
	"io"

	"github.com/fatih/color"
)

//Color formatter to print the colored output on streams
type Color struct {
	red  *color.Color
	blue *color.Color
}

//NewColor returns a new instance color formatter
func NewColor() *Color {
	return &Color{
		red:  color.New(color.FgRed),
		blue: color.New(color.FgBlue),
	}
}

//PrintBlue prints the formatted content to given destination in blue color
func (c *Color) PrintBlue(w io.Writer, format string, args ...interface{}) {
	c.blue.Fprintf(w, format, args...)
}

//Header prints the formatted content to given destination in blue color
func (c *Color) Header(w io.Writer, format string, args ...interface{}) {
	c.PrintBlue(w, format, args...)
}

//PrintBlue prints the formatted content to given destination in red color
func (c *Color) PrintRed(w io.Writer, format string, args ...interface{}) {
	c.red.Fprintf(w, format, args...)
}

//Error prints the formatted content to given destination in red color
func (c *Color) Error(w io.Writer, format string, args ...interface{}) {
	c.PrintRed(w, format, args...)
}
