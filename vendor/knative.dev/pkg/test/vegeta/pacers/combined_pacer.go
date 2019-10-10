/*
Copyright 2019 The Knative Authors

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

package pacers

import (
	"errors"
	"fmt"
	"strings"
	"time"

	vegeta "github.com/tsenart/vegeta/lib"
)

// combinedPacer is a Pacer that combines multiple Pacers and runs them sequentially when being used for attack.
type combinedPacer struct {
	// pacers is a list of pacers that will be used sequentially for attack.
	// MUST have more than 1 pacer.
	pacers []vegeta.Pacer
	// durations is the list of durations for the given Pacers.
	// MUST have the same length as Pacers, and each duration MUST be longer than 1 second.
	durations []time.Duration

	// totalDuration is sum of the given Durations.
	totalDuration uint64
	// stepDurations is the accumulative duration of each step calculated from the given Durations.
	stepDurations []uint64

	curPacerIndex   uint
	prevElapsedHits uint64
	prevElapsedTime uint64
}

// NewCombined returns a new CombinedPacer with the given config.
func NewCombined(pacers []vegeta.Pacer, durations []time.Duration) (vegeta.Pacer, error) {
	if len(pacers) == 0 || len(durations) == 0 || len(pacers) != len(durations) || len(pacers) == 1 {
		return nil, errors.New("configuration for this CombinedPacer is invalid")
	}

	var totalDuration uint64
	var stepDurations = make([]uint64, len(pacers))
	for i, duration := range durations {
		if duration < 1*time.Second {
			return nil, errors.New("duration for each pacer must be longer than 1 second")
		}
		totalDuration += uint64(duration)
		if i == 0 {
			stepDurations[0] = uint64(duration)
		} else {
			stepDurations[i] = stepDurations[i-1] + uint64(duration)
		}
	}
	pacer := &combinedPacer{
		pacers:    pacers,
		durations: durations,

		totalDuration: totalDuration,
		stepDurations: stepDurations,
	}
	return pacer, nil
}

// combinedPacer satisfies the Pacer interface.
var _ vegeta.Pacer = &combinedPacer{}

// String returns a pretty-printed description of the combinedPacer's behaviour.
func (cp *combinedPacer) String() string {
	var sb strings.Builder
	for i := range cp.pacers {
		sb.WriteString(fmt.Sprintf("Pacer: %s, Duration: %s\n", cp.pacers[i], cp.durations[i]))
	}
	return sb.String()
}

func (cp *combinedPacer) Pace(elapsedTime time.Duration, elapsedHits uint64) (time.Duration, bool) {
	pacerTimeOffset := uint64(elapsedTime) % cp.totalDuration
	pacerIndex := cp.pacerIndex(pacerTimeOffset)

	// If it needs to switch to the next pacer, update prevElapsedTime, prevElapsedHits and curPacerIndex.
	if pacerIndex != cp.curPacerIndex {
		cp.prevElapsedTime = uint64(elapsedTime)
		cp.prevElapsedHits = elapsedHits
		cp.curPacerIndex = pacerIndex
	}

	// Use the adjusted elapsedTime and elapsedHits to get the time to wait for the next hit.
	curPacer := cp.pacers[cp.curPacerIndex]
	curElapsedTime := time.Duration(uint64(elapsedTime) - cp.prevElapsedTime)
	curElapsedHits := elapsedHits - cp.prevElapsedHits
	return curPacer.Pace(curElapsedTime, curElapsedHits)
}

// pacerIndex returns the index of pacer that pacerTimeOffset falls into
func (cp *combinedPacer) pacerIndex(pacerTimeOffset uint64) uint {
	i, j := 0, len(cp.stepDurations)
	for i < j {
		m := i + (j-i)/2
		if pacerTimeOffset >= cp.stepDurations[m] {
			i = m + 1
		} else {
			j = m
		}
	}
	return uint(i)
}
