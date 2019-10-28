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
	"math"
	"time"

	vegeta "github.com/tsenart/vegeta/lib"
)

// steadyUpPacer is a Pacer that describes attack request rates that increases in the beginning then becomes steady.
//  Max  |     ,----------------
//       |    /
//       |   /
//       |  /
//       | /
//  Min -+------------------------------> t
//       |<-Up->|
type steadyUpPacer struct {
	// upDuration is the duration that attack request rates increase from Min to Max.
	// MUST be larger than 0.
	upDuration time.Duration
	// min is the attack request rates from the beginning.
	// MUST be larger than 0.
	min vegeta.Rate
	// max is the maximum and final steady attack request rates.
	// MUST be larger than Min.
	max vegeta.Rate

	slope        float64
	minHitsPerNs float64
	maxHitsPerNs float64
}

// NewSteadyUp returns a new SteadyUpPacer with the given config.
func NewSteadyUp(min vegeta.Rate, max vegeta.Rate, upDuration time.Duration) (vegeta.Pacer, error) {
	if upDuration <= 0 || min.Freq <= 0 || min.Per <= 0 || max.Freq <= 0 || max.Per <= 0 {
		return nil, errors.New("configuration for this SteadyUpPacer is invalid")
	}
	minHitsPerNs := hitsPerNs(min)
	maxHitsPerNs := hitsPerNs(max)
	if minHitsPerNs >= maxHitsPerNs {
		return nil, errors.New("min rate must be smaller than max rate for SteadyUpPacer")
	}

	pacer := &steadyUpPacer{
		min:          min,
		max:          max,
		upDuration:   upDuration,
		slope:        (maxHitsPerNs - minHitsPerNs) / float64(upDuration),
		minHitsPerNs: minHitsPerNs,
		maxHitsPerNs: maxHitsPerNs,
	}
	return pacer, nil
}

// steadyUpPacer satisfies the Pacer interface.
var _ vegeta.Pacer = &steadyUpPacer{}

// String returns a pretty-printed description of the steadyUpPacer's behaviour.
func (sup *steadyUpPacer) String() string {
	return fmt.Sprintf("Up{%s + %s / %s}, then Steady{%s}", sup.min, sup.max, sup.upDuration, sup.max)
}

// Pace determines the length of time to sleep until the next hit is sent.
func (sup *steadyUpPacer) Pace(elapsedTime time.Duration, elapsedHits uint64) (time.Duration, bool) {
	expectedHits := sup.hits(elapsedTime)
	if elapsedHits < uint64(expectedHits) {
		// Running behind, send next hit immediately.
		return 0, false
	}

	// Re-arranging our hits equation to provide a duration given the number of
	// requests sent is non-trivial, so we must solve for the duration numerically.
	nsPerHit := 1 / sup.hitsPerNs(elapsedTime)
	hitsToWait := float64(elapsedHits+1) - expectedHits
	nextHitIn := time.Duration(nsPerHit * hitsToWait)

	// If we can't converge to an error of <1e-3 within 10 iterations, bail.
	// This rarely even loops for any large Period if hitsToWait is small.
	for i := 0; i < 10; i++ {
		hitsAtGuess := sup.hits(elapsedTime + nextHitIn)
		err := float64(elapsedHits+1) - hitsAtGuess
		if math.Abs(err) < 1e-3 {
			return nextHitIn, false
		}
		nextHitIn = time.Duration(float64(nextHitIn) / (hitsAtGuess - float64(elapsedHits)))
	}

	return nextHitIn, false
}

// hits returns the number of expected hits for this pacer during the given time.
func (sup *steadyUpPacer) hits(t time.Duration) float64 {
	// If t is smaller than the upDuration, calculate the hits as a trapezoid.
	if t <= sup.upDuration {
		curtHitsPerNs := sup.hitsPerNs(t)
		return (curtHitsPerNs + sup.minHitsPerNs) / 2.0 * float64(t)
	}

	// If t is larger than the upDuration, calculate the hits as a trapezoid + a rectangle.
	upHits := (sup.maxHitsPerNs + sup.minHitsPerNs) / 2.0 * float64(sup.upDuration)
	steadyHits := sup.maxHitsPerNs * float64(t-sup.upDuration)
	return upHits + steadyHits
}

// hitsPerNs returns the attack rate for this pacer at a given time.
func (sup *steadyUpPacer) hitsPerNs(t time.Duration) float64 {
	if t <= sup.upDuration {
		return sup.minHitsPerNs + float64(t)*sup.slope
	}

	return sup.maxHitsPerNs
}

// hitsPerNs returns the attack rate this ConstantPacer represents, in
// fractional hits per nanosecond.
func hitsPerNs(cp vegeta.ConstantPacer) float64 {
	return float64(cp.Freq) / float64(cp.Per)
}
