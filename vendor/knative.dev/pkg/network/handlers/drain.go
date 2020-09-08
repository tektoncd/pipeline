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
	"net/http"
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
// probes with a "200 OK" until the handler is told to Drain.
// When the Drainer is told to Drain, it will immediately start to fail
// probes with a "500 shutting down", and the call will block until no
// requests have been received for QuietPeriod (defaults to
// network.DefaultDrainTimeout).
type Drainer struct {
	// Mutex guards the initialization and resets of the timer
	sync.RWMutex

	// Inner is the http.Handler to which we delegate actual requests.
	Inner http.Handler

	// QuietPeriod is the duration that must elapse without any requests
	// after Drain is called before it may return.
	QuietPeriod time.Duration

	// once is used to initialize timer
	once sync.Once

	// timer is used to orchestrate the drain.
	timer timer
}

// Ensure Drainer implements http.Handler
var _ http.Handler = (*Drainer)(nil)

// ServeHTTP implements http.Handler
func (d *Drainer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if network.IsKubeletProbe(r) { // Respond to probes regardless of path.
		if d.draining() {
			http.Error(w, "shutting down", http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		return
	}

	d.reset()
	d.Inner.ServeHTTP(w, r)
}

// Drain blocks until QuietPeriod has elapsed since the last request,
// starting when this is invoked.
func (d *Drainer) Drain() {
	// Note: until the first caller exits, the others
	// will wait blocked as well.
	d.once.Do(func() {
		t := func() timer {
			d.Lock()
			defer d.Unlock()
			if d.QuietPeriod <= 0 {
				d.QuietPeriod = network.DefaultDrainTimeout
			}
			d.timer = newTimer(d.QuietPeriod)
			return d.timer
		}()

		<-t.tickChan()
	})
}

// reset resets the drain timer to the full amount of time.
func (d *Drainer) reset() {
	if func() bool {
		d.RLock()
		defer d.RUnlock()
		return d.timer == nil
	}() {
		return
	}

	d.Lock()
	defer d.Unlock()
	if d.timer.Stop() {
		d.timer.Reset(d.QuietPeriod)
	}
}

// draining returns whether we are draining the handler.
func (d *Drainer) draining() bool {
	d.RLock()
	defer d.RUnlock()
	return d.timer != nil
}
