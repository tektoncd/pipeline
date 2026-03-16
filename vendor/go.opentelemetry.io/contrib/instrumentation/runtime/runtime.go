// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package runtime // import "go.opentelemetry.io/contrib/instrumentation/runtime"

import (
	"context"
	"math"
	"runtime/metrics"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/semconv/v1.39.0/goconv"

	"go.opentelemetry.io/contrib/instrumentation/runtime/internal/deprecatedruntime"
	"go.opentelemetry.io/contrib/instrumentation/runtime/internal/x"
)

// ScopeName is the instrumentation scope name.
const ScopeName = "go.opentelemetry.io/contrib/instrumentation/runtime"

const (
	goTotalMemory       = "/memory/classes/total:bytes"
	goMemoryReleased    = "/memory/classes/heap/released:bytes"
	goHeapMemory        = "/memory/classes/heap/stacks:bytes"
	goMemoryLimit       = "/gc/gomemlimit:bytes"
	goMemoryAllocated   = "/gc/heap/allocs:bytes"
	goMemoryAllocations = "/gc/heap/allocs:objects"
	goMemoryGoal        = "/gc/heap/goal:bytes"
	goGoroutines        = "/sched/goroutines:goroutines"
	goMaxProcs          = "/sched/gomaxprocs:threads"
	goConfigGC          = "/gc/gogc:percent"
	goSchedLatencies    = "/sched/latencies:seconds"
)

// Start initializes reporting of runtime metrics using the supplied config.
// For goroutine scheduling metrics, additionally see [NewProducer].
//
// Metrics emitted by Start includes:
//
//	go.memory.used          By            Memory used by the Go runtime.
//	go.memory.limit         By            Go runtime memory limit configured by the user, if a limit exists.
//	go.memory.allocated     By            Memory allocated to the heap by the application.
//	go.memory.allocations   {allocation}  Count of allocations to the heap by the application.
//	go.memory.gc.goal       By            Heap size target for the end of the GC cycle.
//	go.goroutine.count      {goroutine}   Count of live goroutines.
//	go.processor.limit      {thread}      The number of OS threads that can execute user-level Go code simultaneously.
//	go.config.gogc          %             Heap size target percentage configured by the user, otherwise 100.
//
// When the OTEL_GO_X_DEPRECATED_RUNTIME_METRICS environment variable is set to
// true, the following deprecated metrics are produced:
//
//	runtime.go.cgo.calls         -          Number of cgo calls made by the current process
//	runtime.go.gc.count          -          Number of completed garbage collection cycles
//	runtime.go.gc.pause_ns       (ns)       Amount of nanoseconds in GC stop-the-world pauses
//	runtime.go.gc.pause_total_ns (ns)       Cumulative nanoseconds in GC stop-the-world pauses since the program started
//	runtime.go.goroutines        -          Number of goroutines that currently exist
//	runtime.go.lookups           -          Number of pointer lookups performed by the runtime
//	runtime.go.mem.heap_alloc    (bytes)    Bytes of allocated heap objects
//	runtime.go.mem.heap_idle     (bytes)    Bytes in idle (unused) spans
//	runtime.go.mem.heap_inuse    (bytes)    Bytes in in-use spans
//	runtime.go.mem.heap_objects  -          Number of allocated heap objects
//	runtime.go.mem.heap_released (bytes)    Bytes of idle spans whose physical memory has been returned to the OS
//	runtime.go.mem.heap_sys      (bytes)    Bytes of heap memory obtained from the OS
//	runtime.go.mem.live_objects  -          Number of live objects is the number of cumulative Mallocs - Frees
//	runtime.uptime               (ms)       Milliseconds since application was initialized
func Start(opts ...Option) error {
	c := newConfig(opts...)
	meter := c.MeterProvider.Meter(
		ScopeName,
		metric.WithInstrumentationVersion(Version),
	)
	if x.DeprecatedRuntimeMetrics.Enabled() {
		if err := deprecatedruntime.Start(meter, c.MinimumReadMemStatsInterval); err != nil {
			return err
		}
	}
	memoryUsed, err := goconv.NewMemoryUsed(meter)
	if err != nil {
		return err
	}
	memoryLimit, err := goconv.NewMemoryLimit(meter)
	if err != nil {
		return err
	}
	memoryAllocated, err := goconv.NewMemoryAllocated(meter)
	if err != nil {
		return err
	}
	memoryAllocations, err := goconv.NewMemoryAllocations(meter)
	if err != nil {
		return err
	}
	memoryGCGoal, err := goconv.NewMemoryGCGoal(meter)
	if err != nil {
		return err
	}
	goroutineCount, err := goconv.NewGoroutineCount(meter)
	if err != nil {
		return err
	}
	processorLimit, err := goconv.NewProcessorLimit(meter)
	if err != nil {
		return err
	}
	configGogc, err := goconv.NewConfigGogc(meter)
	if err != nil {
		return err
	}

	otherMemoryOpt := metric.WithAttributeSet(
		attribute.NewSet(memoryUsed.AttrMemoryType(goconv.MemoryTypeOther)),
	)
	stackMemoryOpt := metric.WithAttributeSet(
		attribute.NewSet(memoryUsed.AttrMemoryType(goconv.MemoryTypeStack)),
	)
	collector := newCollector(c.MinimumReadMemStatsInterval, runtimeMetrics)
	var lock sync.Mutex
	_, err = meter.RegisterCallback(
		func(_ context.Context, o metric.Observer) error {
			lock.Lock()
			defer lock.Unlock()
			collector.refresh()
			stackMemory := collector.getInt(goHeapMemory)
			o.ObserveInt64(memoryUsed.Inst(), stackMemory, stackMemoryOpt)
			totalMemory := collector.getInt(goTotalMemory) - collector.getInt(goMemoryReleased)
			otherMemory := totalMemory - stackMemory
			o.ObserveInt64(memoryUsed.Inst(), otherMemory, otherMemoryOpt)
			// Only observe the limit metric if a limit exists
			if limit := collector.getInt(goMemoryLimit); limit != math.MaxInt64 {
				o.ObserveInt64(memoryLimit.Inst(), limit)
			}
			o.ObserveInt64(memoryAllocated.Inst(), collector.getInt(goMemoryAllocated))
			o.ObserveInt64(memoryAllocations.Inst(), collector.getInt(goMemoryAllocations))
			o.ObserveInt64(memoryGCGoal.Inst(), collector.getInt(goMemoryGoal))
			o.ObserveInt64(goroutineCount.Inst(), collector.getInt(goGoroutines))
			o.ObserveInt64(processorLimit.Inst(), collector.getInt(goMaxProcs))
			o.ObserveInt64(configGogc.Inst(), collector.getInt(goConfigGC))
			return nil
		},
		memoryUsed.Inst(),
		memoryLimit.Inst(),
		memoryAllocated.Inst(),
		memoryAllocations.Inst(),
		memoryGCGoal.Inst(),
		goroutineCount.Inst(),
		processorLimit.Inst(),
		configGogc.Inst(),
	)
	if err != nil {
		return err
	}
	return nil
}

// These are the metrics we actually fetch from the go runtime.
var runtimeMetrics = []string{
	goTotalMemory,
	goMemoryReleased,
	goHeapMemory,
	goMemoryLimit,
	goMemoryAllocated,
	goMemoryAllocations,
	goMemoryGoal,
	goGoroutines,
	goMaxProcs,
	goConfigGC,
}

type goCollector struct {
	// now is used to replace the implementation of time.Now for testing
	now func() time.Time
	// lastCollect tracks the last time metrics were refreshed
	lastCollect time.Time
	// minimumInterval is the minimum amount of time between calls to metrics.Read
	minimumInterval time.Duration
	// sampleBuffer is populated by runtime/metrics
	sampleBuffer []metrics.Sample
	// sampleMap allows us to easily get the value of a single metric
	sampleMap map[string]*metrics.Sample
}

func newCollector(minimumInterval time.Duration, metricNames []string) *goCollector {
	g := &goCollector{
		sampleBuffer:    make([]metrics.Sample, 0, len(metricNames)),
		sampleMap:       make(map[string]*metrics.Sample, len(metricNames)),
		minimumInterval: minimumInterval,
		now:             time.Now,
	}
	for _, metricName := range metricNames {
		g.sampleBuffer = append(g.sampleBuffer, metrics.Sample{Name: metricName})
		// sampleMap references a position in the sampleBuffer slice. If an
		// element is appended to sampleBuffer, it must be added to sampleMap
		// for the sample to be accessible in sampleMap.
		g.sampleMap[metricName] = &g.sampleBuffer[len(g.sampleBuffer)-1]
	}
	return g
}

func (g *goCollector) refresh() {
	now := g.now()
	if now.Sub(g.lastCollect) < g.minimumInterval {
		// refresh was invoked more frequently than allowed by the minimum
		// interval. Do nothing.
		return
	}
	metrics.Read(g.sampleBuffer)
	g.lastCollect = now
}

func (g *goCollector) getInt(name string) int64 {
	if s, ok := g.sampleMap[name]; ok && s.Value.Kind() == metrics.KindUint64 {
		v := s.Value.Uint64()
		if v > math.MaxInt64 {
			return math.MaxInt64
		}
		return int64(v)
	}
	return 0
}

func (g *goCollector) getHistogram(name string) *metrics.Float64Histogram {
	if s, ok := g.sampleMap[name]; ok && s.Value.Kind() == metrics.KindFloat64Histogram {
		return s.Value.Float64Histogram()
	}
	return nil
}
