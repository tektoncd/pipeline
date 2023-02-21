/*
Copyright 2020 The Knative Authors

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

package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"knative.dev/pkg/network"
)

type timer interface {
	Stop() bool
	Reset(time.Duration) bool
	tickChan() <-chan time.Time
}

type sysTimer struct {
	*time.Timer
}

func (s *sysTimer) tickChan() <-chan time.Time {
	return s.Timer.C
}

// This constructor is overridden in tests to control the progress
// of time in the test.
var newTimer = func(d time.Duration) timer {
	return &sysTimer{
		time.NewTimer(d),
	}
}

// Drainer wraps an inner http.Handler to support responding to kubelet
// probes and KProbes with a "200 OK" until the handler is told to Drain,
// or Drainer will optionally run the HealthCheck if it is defined.
// When the Drainer is told to Drain, it will immediately start to fail
// probes with a "500 shutting down", and the call will block until no
// requests have been received for QuietPeriod (defaults to
// network.DefaultDrainTimeout).
type Drainer struct {
	// Mutex guards the initialization and resets of the timer
	sync.RWMutex

	// HealthCheck is an optional health check that is performed until the drain signal is received.
	// When unspecified, a "200 OK" is returned, otherwise this function is invoked.
	HealthCheck http.HandlerFunc

	// Inner is the http.Handler to which we delegate actual requests.
	Inner http.Handler

	// QuietPeriod is the duration that must elapse without any requests
	// after Drain is called before it may return.
	QuietPeriod time.Duration

	// timer is used to orchestrate the drain.
	timer timer

	// used to synchronize callers of Drain
	drainCh chan struct{}

	// used to synchronize Drain and Reset
	resetCh chan struct{}

	// HealthCheckUAPrefixes are the additional user agent prefixes that trigger the
	// drainer's health check
	HealthCheckUAPrefixes []string
}

// Ensure Drainer implements http.Handler
var _ http.Handler = (*Drainer)(nil)

// ServeHTTP implements http.Handler
func (d *Drainer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Respond to probes regardless of path.
	if d.isHealthCheckRequest(r) {
		if d.draining() {
			http.Error(w, "shutting down", http.StatusServiceUnavailable)
		} else if d.HealthCheck != nil {
			d.HealthCheck(w, r)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		return
	}
	if isKProbe(r) {
		if d.draining() {
			http.Error(w, "shutting down", http.StatusServiceUnavailable)
		} else {
			serveKProbe(w, r)
		}
		return
	}

	d.resetTimer()
	d.Inner.ServeHTTP(w, r)
}

// Drain blocks until QuietPeriod has elapsed since the last request,
// starting when this is invoked.
func (d *Drainer) Drain() {
	// Note: until the first caller exits, the others
	// will wait blocked as well.
	ch := func() chan struct{} {
		d.Lock()
		defer d.Unlock()
		if d.drainCh != nil {
			return d.drainCh
		}

		if d.QuietPeriod <= 0 {
			d.QuietPeriod = network.DefaultDrainTimeout
		}

		timer := newTimer(d.QuietPeriod)
		drainCh := make(chan struct{})
		resetCh := make(chan struct{})

		go func() {
			select {
			case <-resetCh:
			case <-timer.tickChan():
			}
			close(drainCh)
		}()

		d.drainCh = drainCh
		d.resetCh = resetCh
		d.timer = timer
		return drainCh
	}()

	<-ch
}

// isHealthcheckRequest validates if the request has a user agent that is for healthcheck
func (d *Drainer) isHealthCheckRequest(r *http.Request) bool {
	if network.IsKubeletProbe(r) {
		return true
	}

	for _, ua := range d.HealthCheckUAPrefixes {
		if strings.HasPrefix(r.Header.Get(network.UserAgentKey), ua) {
			return true
		}
	}

	return false
}

// Reset interrupts Drain and clears the drainers internal state
// Thus further calls to Drain will block and wait for the entire QuietPeriod
func (d *Drainer) Reset() {
	d.Lock()
	defer d.Unlock()

	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}

	if d.resetCh != nil {
		close(d.resetCh)
		d.resetCh = nil
	}
	if d.drainCh != nil {
		d.drainCh = nil
	}
}

func (d *Drainer) resetTimer() {
	if func() bool {
		d.RLock()
		defer d.RUnlock()
		return d.timer == nil
	}() {
		return
	}

	d.Lock()
	defer d.Unlock()
	if d.timer != nil && d.timer.Stop() {
		d.timer.Reset(d.QuietPeriod)
	}
}

// draining returns whether we are draining the handler.
func (d *Drainer) draining() bool {
	d.RLock()
	defer d.RUnlock()
	return d.timer != nil
}

// isKProbe returns true if the request is a knatvie probe.
func isKProbe(r *http.Request) bool {
	return r.Header.Get(network.ProbeHeaderName) == network.ProbeHeaderValue
}

// serveKProbe serve KProbe requests.
func serveKProbe(w http.ResponseWriter, r *http.Request) {
	hh := r.Header.Get(network.HashHeaderName)
	if hh == "" {
		http.Error(w,
			fmt.Sprintf("a probe request must contain a non-empty %q header", network.HashHeaderName),
			http.StatusBadRequest)
		return
	}
	w.Header().Set(network.HashHeaderName, hh)
	w.WriteHeader(http.StatusOK)
}
