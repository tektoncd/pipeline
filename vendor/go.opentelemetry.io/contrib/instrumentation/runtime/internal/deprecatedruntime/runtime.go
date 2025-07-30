// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deprecatedruntime // import "go.opentelemetry.io/contrib/instrumentation/runtime/internal/deprecatedruntime"

import (
	"context"
	"math"
	goruntime "runtime"
	"sync"
	"time"

	"go.opentelemetry.io/otel/metric"
)

// Runtime reports the work-in-progress conventional runtime metrics specified by OpenTelemetry.
type runtime struct {
	minimumReadMemStatsInterval time.Duration
	meter                       metric.Meter
}

// Start initializes reporting of runtime metrics using the supplied config.
func Start(meter metric.Meter, minimumReadMemStatsInterval time.Duration) error {
	r := &runtime{
		meter:                       meter,
		minimumReadMemStatsInterval: minimumReadMemStatsInterval,
	}
	return r.register()
}

func (r *runtime) register() error {
	startTime := time.Now()
	uptime, err := r.meter.Int64ObservableCounter(
		"runtime.uptime",
		metric.WithUnit("ms"),
		metric.WithDescription("Milliseconds since application was initialized"),
	)
	if err != nil {
		return err
	}

	goroutines, err := r.meter.Int64ObservableUpDownCounter(
		"process.runtime.go.goroutines",
		metric.WithDescription("Number of goroutines that currently exist"),
	)
	if err != nil {
		return err
	}

	cgoCalls, err := r.meter.Int64ObservableUpDownCounter(
		"process.runtime.go.cgo.calls",
		metric.WithDescription("Number of cgo calls made by the current process"),
	)
	if err != nil {
		return err
	}

	_, err = r.meter.RegisterCallback(
		func(ctx context.Context, o metric.Observer) error {
			o.ObserveInt64(uptime, time.Since(startTime).Milliseconds())
			o.ObserveInt64(goroutines, int64(goruntime.NumGoroutine()))
			o.ObserveInt64(cgoCalls, goruntime.NumCgoCall())
			return nil
		},
		uptime,
		goroutines,
		cgoCalls,
	)
	if err != nil {
		return err
	}

	return r.registerMemStats()
}

func (r *runtime) registerMemStats() error {
	var (
		err error

		heapAlloc    metric.Int64ObservableUpDownCounter
		heapIdle     metric.Int64ObservableUpDownCounter
		heapInuse    metric.Int64ObservableUpDownCounter
		heapObjects  metric.Int64ObservableUpDownCounter
		heapReleased metric.Int64ObservableUpDownCounter
		heapSys      metric.Int64ObservableUpDownCounter
		liveObjects  metric.Int64ObservableUpDownCounter

		// TODO: is ptrLookups useful? I've not seen a value
		// other than zero.
		ptrLookups metric.Int64ObservableCounter

		gcCount      metric.Int64ObservableCounter
		pauseTotalNs metric.Int64ObservableCounter
		gcPauseNs    metric.Int64Histogram

		lastNumGC    uint32
		lastMemStats time.Time
		memStats     goruntime.MemStats

		// lock prevents a race between batch observer and instrument registration.
		lock sync.Mutex
	)

	lock.Lock()
	defer lock.Unlock()

	if heapAlloc, err = r.meter.Int64ObservableUpDownCounter(
		"process.runtime.go.mem.heap_alloc",
		metric.WithUnit("By"),
		metric.WithDescription("Bytes of allocated heap objects"),
	); err != nil {
		return err
	}

	if heapIdle, err = r.meter.Int64ObservableUpDownCounter(
		"process.runtime.go.mem.heap_idle",
		metric.WithUnit("By"),
		metric.WithDescription("Bytes in idle (unused) spans"),
	); err != nil {
		return err
	}

	if heapInuse, err = r.meter.Int64ObservableUpDownCounter(
		"process.runtime.go.mem.heap_inuse",
		metric.WithUnit("By"),
		metric.WithDescription("Bytes in in-use spans"),
	); err != nil {
		return err
	}

	if heapObjects, err = r.meter.Int64ObservableUpDownCounter(
		"process.runtime.go.mem.heap_objects",
		metric.WithDescription("Number of allocated heap objects"),
	); err != nil {
		return err
	}

	// FYI see https://github.com/golang/go/issues/32284 to help
	// understand the meaning of this value.
	if heapReleased, err = r.meter.Int64ObservableUpDownCounter(
		"process.runtime.go.mem.heap_released",
		metric.WithUnit("By"),
		metric.WithDescription("Bytes of idle spans whose physical memory has been returned to the OS"),
	); err != nil {
		return err
	}

	if heapSys, err = r.meter.Int64ObservableUpDownCounter(
		"process.runtime.go.mem.heap_sys",
		metric.WithUnit("By"),
		metric.WithDescription("Bytes of heap memory obtained from the OS"),
	); err != nil {
		return err
	}

	if ptrLookups, err = r.meter.Int64ObservableCounter(
		"process.runtime.go.mem.lookups",
		metric.WithDescription("Number of pointer lookups performed by the runtime"),
	); err != nil {
		return err
	}

	if liveObjects, err = r.meter.Int64ObservableUpDownCounter(
		"process.runtime.go.mem.live_objects",
		metric.WithDescription("Number of live objects is the number of cumulative Mallocs - Frees"),
	); err != nil {
		return err
	}

	if gcCount, err = r.meter.Int64ObservableCounter(
		"process.runtime.go.gc.count",
		metric.WithDescription("Number of completed garbage collection cycles"),
	); err != nil {
		return err
	}

	// Note that the following could be derived as a sum of
	// individual pauses, but we may lose individual pauses if the
	// observation interval is too slow.
	if pauseTotalNs, err = r.meter.Int64ObservableCounter(
		"process.runtime.go.gc.pause_total_ns",
		// TODO: nanoseconds units
		metric.WithDescription("Cumulative nanoseconds in GC stop-the-world pauses since the program started"),
	); err != nil {
		return err
	}

	if gcPauseNs, err = r.meter.Int64Histogram(
		"process.runtime.go.gc.pause_ns",
		// TODO: nanoseconds units
		metric.WithDescription("Amount of nanoseconds in GC stop-the-world pauses"),
	); err != nil {
		return err
	}

	_, err = r.meter.RegisterCallback(
		func(ctx context.Context, o metric.Observer) error {
			lock.Lock()
			defer lock.Unlock()

			now := time.Now()
			if now.Sub(lastMemStats) >= r.minimumReadMemStatsInterval {
				goruntime.ReadMemStats(&memStats)
				lastMemStats = now
			}

			o.ObserveInt64(heapAlloc, clampUint64(memStats.HeapAlloc))
			o.ObserveInt64(heapIdle, clampUint64(memStats.HeapIdle))
			o.ObserveInt64(heapInuse, clampUint64(memStats.HeapInuse))
			o.ObserveInt64(heapObjects, clampUint64(memStats.HeapObjects))
			o.ObserveInt64(heapReleased, clampUint64(memStats.HeapReleased))
			o.ObserveInt64(heapSys, clampUint64(memStats.HeapSys))
			o.ObserveInt64(liveObjects, clampUint64(memStats.Mallocs-memStats.Frees))
			o.ObserveInt64(ptrLookups, clampUint64(memStats.Lookups))
			o.ObserveInt64(gcCount, int64(memStats.NumGC))
			o.ObserveInt64(pauseTotalNs, clampUint64(memStats.PauseTotalNs))

			computeGCPauses(ctx, gcPauseNs, memStats.PauseNs[:], lastNumGC, memStats.NumGC)

			lastNumGC = memStats.NumGC

			return nil
		},
		heapAlloc,
		heapIdle,
		heapInuse,
		heapObjects,
		heapReleased,
		heapSys,
		liveObjects,

		ptrLookups,

		gcCount,
		pauseTotalNs,
	)
	if err != nil {
		return err
	}
	return nil
}

func clampUint64(v uint64) int64 {
	if v > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(v) // nolint: gosec  // Overflow checked above.
}

func computeGCPauses(
	ctx context.Context,
	recorder metric.Int64Histogram,
	circular []uint64,
	lastNumGC, currentNumGC uint32,
) {
	delta := int(int64(currentNumGC) - int64(lastNumGC))

	if delta == 0 {
		return
	}

	if delta >= len(circular) {
		// There were > 256 collections, some may have been lost.
		recordGCPauses(ctx, recorder, circular)
		return
	}

	n := len(circular)
	if n < 0 {
		// Only the case in error situations.
		return
	}

	length := uint64(n) // nolint: gosec  // n >= 0

	i := uint64(lastNumGC) % length
	j := uint64(currentNumGC) % length

	if j < i { // wrap around the circular buffer
		recordGCPauses(ctx, recorder, circular[i:])
		recordGCPauses(ctx, recorder, circular[:j])
		return
	}

	recordGCPauses(ctx, recorder, circular[i:j])
}

func recordGCPauses(
	ctx context.Context,
	recorder metric.Int64Histogram,
	pauses []uint64,
) {
	for _, pause := range pauses {
		recorder.Record(ctx, clampUint64(pause))
	}
}
