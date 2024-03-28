/*
Copyright 2020 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hash

// This file contains the implementation of the subsetting algorithm for
// choosing a subset of input values in a consistent manner.

import (
	"bytes"
	"hash"
	"hash/fnv"
	"sort"
	"strconv"

	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	startSalt = "start-angle-salt"
	stepSalt  = "step-angle-salt"

	// universe represents the possible range of angles [0, universe).
	// We want to have universe divide total range evenly to reduce bias.
	universe = (1 << 11)
)

// computeAngle returns a uint64 number which represents
// a hash built off the given `n` string for consistent selection
// algorithm.
// We return uint64 here and cast after computing modulo, since
// int might 32 bits on 32 platforms and that would trim result.
func computeHash(n []byte, h hash.Hash64) uint64 {
	h.Reset()
	h.Write(n)
	return h.Sum64()
}

type hashData struct {
	// The set of all hashes for fast lookup and to name mapping
	nameLookup map[int]string
	// Sorted set of hashes for selection algorithm.
	hashPool []int
	// start angle
	start int
	// step angle
	step int
}

func (hd *hashData) fromIndexSet(s sets.Set[int]) sets.Set[string] {
	ret := make(sets.Set[string], len(s))
	for v := range s {
		ret.Insert(hd.nameForHIndex(v))
	}
	return ret
}

func (hd *hashData) nameForHIndex(hi int) string {
	return hd.nameLookup[hd.hashPool[hi]]
}

func buildHashes(in sets.Set[string], target string) *hashData {
	// Any one changing this function must execute
	// `go test -run=TestOverlay -count=200`.
	// This is to ensure there is no regression in the selection
	// algorithm.

	// Sorted list to ensure consistent results every time.
	from := sets.List(in)
	// Write in two pieces, so we don't allocate temp string which is sum of both.
	buf := bytes.NewBufferString(target)
	buf.WriteString(startSalt)
	hasher := fnv.New64a()
	hd := &hashData{
		nameLookup: make(map[int]string, len(from)),
		hashPool:   make([]int, len(from)),
		start:      int(computeHash(buf.Bytes(), hasher) % universe),
	}
	buf.Truncate(len(target)) // Discard the angle salt.
	buf.WriteString(stepSalt)
	hd.step = int(computeHash(buf.Bytes(), hasher) % universe)

	for i, f := range from {
		buf.Reset() // This retains the storage.
		// Make unique sets for every target.
		buf.WriteString(f)
		buf.WriteString(target)
		h := computeHash(buf.Bytes(), hasher)
		hs := int(h % universe)
		// Two values slotted to the same bucket.
		// On average should happen with 1/universe probability.
		_, ok := hd.nameLookup[hs]
		for ok {
			// Feed the hash as salt.
			buf.WriteString(strconv.FormatUint(h, 16 /*append hex strings for shortness*/))
			h = computeHash(buf.Bytes(), hasher)
			hs = int(h % universe)
			_, ok = hd.nameLookup[hs]
		}

		hd.hashPool[i] = hs
		hd.nameLookup[hs] = f
	}
	// Sort for consistent mapping later.
	sort.Slice(hd.hashPool, func(i, j int) bool {
		return hd.hashPool[i] < hd.hashPool[j]
	})
	return hd
}

// ChooseSubset consistently chooses n items from `from`, using
// `target` as a seed value.
// ChooseSubset is an internal function and presumes sanitized inputs.
// TODO(vagababov): once initial impl is ready, think about how to cache
// the prepared data.
func ChooseSubset(from sets.Set[string], n int, target string) sets.Set[string] {
	if n >= len(from) {
		return from
	}

	hashData := buildHashes(from, target)

	// The algorithm for selection does the following:
	// 0. Select angle to be the start angle
	// 1. While n candidates are not selected
	// 2. Find the index for that angle.
	//    2.1. While that index is already selected pick next index
	// 3. Advance angle by `step`
	// 4. Goto 1.
	selection := sets.New[int]()
	angle := hashData.start
	hpl := len(hashData.hashPool)
	for len(selection) < n {
		root := sort.Search(hpl, func(i int) bool {
			return hashData.hashPool[i] >= angle
		})
		// Wrap around.
		if root == hpl {
			root = 0
		}
		// Already matched this one. Continue to the next index.
		for selection.Has(root) {
			root++
			if root == hpl {
				root = 0
			}
		}
		selection.Insert(root)
		angle = (angle + hashData.step) % universe
	}

	return hashData.fromIndexSet(selection)
}
