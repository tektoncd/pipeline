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
	"sync"
	"sync/atomic"

	"github.com/fatih/color"
)

var (
	// Red is avoided as keeping it for errors
	palette = []color.Attribute{
		color.FgHiGreen,
		color.FgHiYellow,
		color.FgHiBlue,
		color.FgHiMagenta,
		color.FgHiCyan,
	}
)

type atomicCounter struct {
	value     uint32
	threshold int
}

func (c *atomicCounter) next() int {
	v := atomic.AddUint32(&c.value, 1)
	next := int(v-1) % c.threshold
	atomic.CompareAndSwapUint32(&c.value, uint32(c.threshold), 0)
	return next

}

type rainbow struct {
	cache   sync.Map
	counter atomicCounter
}

func newRainbow() *rainbow {
	return &rainbow{
		counter: atomicCounter{threshold: len(palette)},
	}
}

func (r *rainbow) get(x string) color.Attribute {
	if value, ok := r.cache.Load(x); ok {
		return value.(color.Attribute)
	}

	clr := palette[r.counter.next()]
	r.cache.Store(x, clr)
	return clr
}

// Fprintf formats according to a format specifier and writes to w.
// the first argument is a label to keep the same colour on.
func (r *rainbow) Fprintf(label string, w io.Writer, format string, args ...interface{}) {
	attribute := r.get(label)
	crainbow := color.Set(attribute).Add(color.Bold)
	crainbow.Fprintf(w, format, args...)
}

//Color formatter to print the colored output on streams
type Color struct {
	Rainbow *rainbow

	red  *color.Color
	blue *color.Color
}

//NewColor returns a new instance color formatter
func NewColor() *Color {
	return &Color{
		Rainbow: newRainbow(),

		red:  color.New(color.FgRed),
		blue: color.New(color.FgBlue),
	}
}

//PrintBlue prints the formatted content to given destination in blue color
func (c *Color) PrintBlue(w io.Writer, format string, args ...interface{}) {
	c.blue.Fprintf(w, format, args...)
}

//PrintBlue prints the formatted content to given destination in red color
func (c *Color) PrintRed(w io.Writer, format string, args ...interface{}) {
	c.red.Fprintf(w, format, args...)
}

//Error prints the formatted content to given destination in red color
func (c *Color) Error(w io.Writer, format string, args ...interface{}) {
	c.PrintRed(w, format, args...)
}
