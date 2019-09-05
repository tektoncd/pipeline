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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRainbowsColours(t *testing.T) {
	rb := newRainbow()
	assert.Equal(t, rb.counter.value, uint32(0)) // nothing

	c := rb.get("a") // get a label
	assert.NotNil(t, c)
	assert.Equal(t, rb.counter.value, uint32(1))

	_ = rb.get("b") // incremented
	assert.Equal(t, rb.counter.value, uint32(2))

	_ = rb.get("a") // no increment (cached)
	assert.Equal(t, rb.counter.value, uint32(2))

	rb = newRainbow()
	for c := range palette {
		rb.get(string(c))
	}
	assert.Equal(t, rb.counter.value, uint32(0)) // Looped back to 0
}
