/*
Copyright 2025 The Tekton Authors

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

package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
)

const (
	waitExponentialBackoffDurationKey = "duration"
	waitExponentialBackoffFactorKey   = "factor"
	waitExponentialBackoffJitterKey   = "jitter"
	waitExponentialBackoffStepsKey    = "steps"
	waitExponentialBackoffCapKey      = "cap"

	DefaultWaitExponentialBackoffDuration = "1s"
	DefaultWaitExponentialBackoffFactor   = 2.0
	DefaultWaitExponentialBackoffJitter   = 0.0
	DefaultWaitExponentialBackoffSteps    = 10
	DefaultWaitExponentialBackoffCap      = "60s"
)

// DefaultWaitExponentialBackoff holds all the default configurations for wait-exponential-backoff
var DefaultWaitExponentialBackoff, _ = newWaitExponentialBackoffFromMap(map[string]string{})

// WaitExponentialBackoff holds the configurations for exponential backoff
// +k8s:deepcopy-gen=true
type WaitExponentialBackoff struct {
	Duration time.Duration
	Factor   float64
	Jitter   float64
	Steps    int
	Cap      time.Duration
}

// Equals returns true if two Configs are identical
func (cfg *WaitExponentialBackoff) Equals(other *WaitExponentialBackoff) bool {
	if cfg == nil && other == nil {
		return true
	}
	if cfg == nil || other == nil {
		return false
	}
	return other.Duration == cfg.Duration &&
		other.Factor == cfg.Factor &&
		other.Jitter == cfg.Jitter &&
		other.Steps == cfg.Steps &&
		other.Cap == cfg.Cap
}

// GetWaitExponentialBackoffConfigName returns the name of the configmap containing all customizations for wait-exponential-backoff
func GetWaitExponentialBackoffConfigName() string {
	if e := os.Getenv("CONFIG_WAIT_EXPONENTIAL_BACKOFF_NAME"); e != "" {
		return e
	}
	return "config-wait-exponential-backoff"
}

// newWaitExponentialBackoffFromMap returns a Config given a map from ConfigMap
func newWaitExponentialBackoffFromMap(config map[string]string) (*WaitExponentialBackoff, error) {
	w := WaitExponentialBackoff{}

	durationStr := DefaultWaitExponentialBackoffDuration
	if v, ok := config[waitExponentialBackoffDurationKey]; ok {
		durationStr = v
	}
	dur, err := time.ParseDuration(durationStr)
	if err != nil {
		return nil, fmt.Errorf("failed parsing duration %q: %w", durationStr, err)
	}
	w.Duration = dur

	factor := DefaultWaitExponentialBackoffFactor
	if v, ok := config[waitExponentialBackoffFactorKey]; ok {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, fmt.Errorf("failed parsing factor %q: %w", v, err)
		}
		factor = f
	}
	w.Factor = factor

	jitter := DefaultWaitExponentialBackoffJitter
	if v, ok := config[waitExponentialBackoffJitterKey]; ok {
		j, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, fmt.Errorf("failed parsing jitter %q: %w", v, err)
		}
		jitter = j
	}
	w.Jitter = jitter

	steps := DefaultWaitExponentialBackoffSteps
	if v, ok := config[waitExponentialBackoffStepsKey]; ok {
		s, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("failed parsing steps %q: %w", v, err)
		}
		steps = s
	}
	w.Steps = steps

	capStr := DefaultWaitExponentialBackoffCap
	if v, ok := config[waitExponentialBackoffCapKey]; ok {
		capStr = v
	}
	capDur, err := time.ParseDuration(capStr)
	if err != nil {
		return nil, fmt.Errorf("failed parsing cap %q: %w", capStr, err)
	}
	w.Cap = capDur

	return &w, nil
}

// NewWaitExponentialBackoffFromConfigMap returns a Config given a ConfigMap
func NewWaitExponentialBackoffFromConfigMap(config *corev1.ConfigMap) (*WaitExponentialBackoff, error) {
	return newWaitExponentialBackoffFromMap(config.Data)
}
