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

package metrics

import (
	"context"
	"log"
	"runtime"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

// NewMemStatsAll creates a new MemStatsProvider with stats for all of the
// supported Go runtime.MemStat fields.
func NewMemStatsAll() *MemStatsProvider {
	return &MemStatsProvider{
		Alloc: stats.Int64(
			"go_alloc",
			"The number of bytes of allocated heap objects.",
			stats.UnitNone,
		),
		TotalAlloc: stats.Int64(
			"go_total_alloc",
			"The cumulative bytes allocated for heap objects.",
			stats.UnitNone,
		),
		Sys: stats.Int64(
			"go_sys",
			"The total bytes of memory obtained from the OS.",
			stats.UnitNone,
		),
		Lookups: stats.Int64(
			"go_lookups",
			"The number of pointer lookups performed by the runtime.",
			stats.UnitNone,
		),
		Mallocs: stats.Int64(
			"go_mallocs",
			"The cumulative count of heap objects allocated.",
			stats.UnitNone,
		),
		Frees: stats.Int64(
			"go_frees",
			"The cumulative count of heap objects freed.",
			stats.UnitNone,
		),
		HeapAlloc: stats.Int64(
			"go_heap_alloc",
			"The number of bytes of allocated heap objects.",
			stats.UnitNone,
		),
		HeapSys: stats.Int64(
			"go_heap_sys",
			"The number of bytes of heap memory obtained from the OS.",
			stats.UnitNone,
		),
		HeapIdle: stats.Int64(
			"go_heap_idle",
			"The number of bytes in idle (unused) spans.",
			stats.UnitNone,
		),
		HeapInuse: stats.Int64(
			"go_heap_in_use",
			"The number of bytes in in-use spans.",
			stats.UnitNone,
		),
		HeapReleased: stats.Int64(
			"go_heap_released",
			"The number of bytes of physical memory returned to the OS.",
			stats.UnitNone,
		),
		HeapObjects: stats.Int64(
			"go_heap_objects",
			"The number of allocated heap objects.",
			stats.UnitNone,
		),
		StackInuse: stats.Int64(
			"go_stack_in_use",
			"The number of bytes in stack spans.",
			stats.UnitNone,
		),
		StackSys: stats.Int64(
			"go_stack_sys",
			"The number of bytes of stack memory obtained from the OS.",
			stats.UnitNone,
		),
		MSpanInuse: stats.Int64(
			"go_mspan_in_use",
			"The number of bytes of allocated mspan structures.",
			stats.UnitNone,
		),
		MSpanSys: stats.Int64(
			"go_mspan_sys",
			"The number of bytes of memory obtained from the OS for mspan structures.",
			stats.UnitNone,
		),
		MCacheInuse: stats.Int64(
			"go_mcache_in_use",
			"The number of bytes of allocated mcache structures.",
			stats.UnitNone,
		),
		MCacheSys: stats.Int64(
			"go_mcache_sys",
			"The number of bytes of memory obtained from the OS for mcache structures.",
			stats.UnitNone,
		),
		BuckHashSys: stats.Int64(
			"go_bucket_hash_sys",
			"The number of bytes of memory in profiling bucket hash tables.",
			stats.UnitNone,
		),
		GCSys: stats.Int64(
			"go_gc_sys",
			"The number of bytes of memory in garbage collection metadata.",
			stats.UnitNone,
		),
		OtherSys: stats.Int64(
			"go_other_sys",
			"The number of bytes of memory in miscellaneous off-heap runtime allocations.",
			stats.UnitNone,
		),
		NextGC: stats.Int64(
			"go_next_gc",
			"The target heap size of the next GC cycle.",
			stats.UnitNone,
		),
		LastGC: stats.Int64(
			"go_last_gc",
			"The time the last garbage collection finished, as nanoseconds since 1970 (the UNIX epoch).",
			"ns",
		),
		PauseTotalNs: stats.Int64(
			"go_total_gc_pause_ns",
			"The cumulative nanoseconds in GC stop-the-world pauses since the program started.",
			"ns",
		),
		NumGC: stats.Int64(
			"go_num_gc",
			"The number of completed GC cycles.",
			stats.UnitNone,
		),
		NumForcedGC: stats.Int64(
			"go_num_forced_gc",
			"The number of GC cycles that were forced by the application calling the GC function.",
			stats.UnitNone,
		),
		GCCPUFraction: stats.Float64(
			"go_gc_cpu_fraction",
			"The fraction of this program's available CPU time used by the GC since the program started.",
			stats.UnitNone,
		),
	}
}

// MemStatsOrDie sets up reporting on Go memory usage every 30 seconds or dies
// by calling log.Fatalf.
func MemStatsOrDie(ctx context.Context) {
	msp := NewMemStatsAll()
	msp.Start(ctx, 30*time.Second)

	if err := view.Register(msp.DefaultViews()...); err != nil {
		log.Fatal("Error exporting go memstats view: ", err)
	}
}

// MemStatsProvider is used to expose metrics based on Go's runtime.MemStats.
// The fields below (and their comments) are a filtered list taken from
// Go's runtime.MemStats.
type MemStatsProvider struct {
	// Alloc is bytes of allocated heap objects.
	//
	// This is the same as HeapAlloc (see below).
	Alloc *stats.Int64Measure

	// TotalAlloc is cumulative bytes allocated for heap objects.
	//
	// TotalAlloc increases as heap objects are allocated, but
	// unlike Alloc and HeapAlloc, it does not decrease when
	// objects are freed.
	TotalAlloc *stats.Int64Measure

	// Sys is the total bytes of memory obtained from the OS.
	//
	// Sys is the sum of the XSys fields below. Sys measures the
	// virtual address space reserved by the Go runtime for the
	// heap, stacks, and other internal data structures. It's
	// likely that not all of the virtual address space is backed
	// by physical memory at any given moment, though in general
	// it all was at some point.
	Sys *stats.Int64Measure

	// Lookups is the number of pointer lookups performed by the
	// runtime.
	//
	// This is primarily useful for debugging runtime internals.
	Lookups *stats.Int64Measure

	// Mallocs is the cumulative count of heap objects allocated.
	// The number of live objects is Mallocs - Frees.
	Mallocs *stats.Int64Measure

	// Frees is the cumulative count of heap objects freed.
	Frees *stats.Int64Measure

	// HeapAlloc is bytes of allocated heap objects.
	//
	// "Allocated" heap objects include all reachable objects, as
	// well as unreachable objects that the garbage collector has
	// not yet freed. Specifically, HeapAlloc increases as heap
	// objects are allocated and decreases as the heap is swept
	// and unreachable objects are freed. Sweeping occurs
	// incrementally between GC cycles, so these two processes
	// occur simultaneously, and as a result HeapAlloc tends to
	// change smoothly (in contrast with the sawtooth that is
	// typical of stop-the-world garbage collectors).
	HeapAlloc *stats.Int64Measure

	// HeapSys is bytes of heap memory obtained from the OS.
	//
	// HeapSys measures the amount of virtual address space
	// reserved for the heap. This includes virtual address space
	// that has been reserved but not yet used, which consumes no
	// physical memory, but tends to be small, as well as virtual
	// address space for which the physical memory has been
	// returned to the OS after it became unused (see HeapReleased
	// for a measure of the latter).
	//
	// HeapSys estimates the largest size the heap has had.
	HeapSys *stats.Int64Measure

	// HeapIdle is bytes in idle (unused) spans.
	//
	// Idle spans have no objects in them. These spans could be
	// (and may already have been) returned to the OS, or they can
	// be reused for heap allocations, or they can be reused as
	// stack memory.
	//
	// HeapIdle minus HeapReleased estimates the amount of memory
	// that could be returned to the OS, but is being retained by
	// the runtime so it can grow the heap without requesting more
	// memory from the OS. If this difference is significantly
	// larger than the heap size, it indicates there was a recent
	// transient spike in live heap size.
	HeapIdle *stats.Int64Measure

	// HeapInuse is bytes in in-use spans.
	//
	// In-use spans have at least one object in them. These spans
	// can only be used for other objects of roughly the same
	// size.
	//
	// HeapInuse minus HeapAlloc estimates the amount of memory
	// that has been dedicated to particular size classes, but is
	// not currently being used. This is an upper bound on
	// fragmentation, but in general this memory can be reused
	// efficiently.
	HeapInuse *stats.Int64Measure

	// HeapReleased is bytes of physical memory returned to the OS.
	//
	// This counts heap memory from idle spans that was returned
	// to the OS and has not yet been reacquired for the heap.
	HeapReleased *stats.Int64Measure

	// HeapObjects is the number of allocated heap objects.
	//
	// Like HeapAlloc, this increases as objects are allocated and
	// decreases as the heap is swept and unreachable objects are
	// freed.
	HeapObjects *stats.Int64Measure

	// StackInuse is bytes in stack spans.
	//
	// In-use stack spans have at least one stack in them. These
	// spans can only be used for other stacks of the same size.
	//
	// There is no StackIdle because unused stack spans are
	// returned to the heap (and hence counted toward HeapIdle).
	StackInuse *stats.Int64Measure

	// StackSys is bytes of stack memory obtained from the OS.
	//
	// StackSys is StackInuse, plus any memory obtained directly
	// from the OS for OS thread stacks (which should be minimal).
	StackSys *stats.Int64Measure

	// MSpanInuse is bytes of allocated mspan structures.
	MSpanInuse *stats.Int64Measure

	// MSpanSys is bytes of memory obtained from the OS for mspan
	// structures.
	MSpanSys *stats.Int64Measure

	// MCacheInuse is bytes of allocated mcache structures.
	MCacheInuse *stats.Int64Measure

	// MCacheSys is bytes of memory obtained from the OS for
	// mcache structures.
	MCacheSys *stats.Int64Measure

	// BuckHashSys is bytes of memory in profiling bucket hash tables.
	BuckHashSys *stats.Int64Measure

	// GCSys is bytes of memory in garbage collection metadata.
	GCSys *stats.Int64Measure

	// OtherSys is bytes of memory in miscellaneous off-heap
	// runtime allocations.
	OtherSys *stats.Int64Measure

	// NextGC is the target heap size of the next GC cycle.
	//
	// The garbage collector's goal is to keep HeapAlloc ≤ NextGC.
	// At the end of each GC cycle, the target for the next cycle
	// is computed based on the amount of reachable data and the
	// value of GOGC.
	NextGC *stats.Int64Measure

	// LastGC is the time the last garbage collection finished, as
	// nanoseconds since 1970 (the UNIX epoch).
	LastGC *stats.Int64Measure

	// PauseTotalNs is the cumulative nanoseconds in GC
	// stop-the-world pauses since the program started.
	//
	// During a stop-the-world pause, all goroutines are paused
	// and only the garbage collector can run.
	PauseTotalNs *stats.Int64Measure

	// NumGC is the number of completed GC cycles.
	NumGC *stats.Int64Measure

	// NumForcedGC is the number of GC cycles that were forced by
	// the application calling the GC function.
	NumForcedGC *stats.Int64Measure

	// GCCPUFraction is the fraction of this program's available
	// CPU time used by the GC since the program started.
	//
	// GCCPUFraction is expressed as a number between 0 and 1,
	// where 0 means GC has consumed none of this program's CPU. A
	// program's available CPU time is defined as the integral of
	// GOMAXPROCS since the program started. That is, if
	// GOMAXPROCS is 2 and a program has been running for 10
	// seconds, its "available CPU" is 20 seconds. GCCPUFraction
	// does not include CPU time used for write barrier activity.
	//
	// This is the same as the fraction of CPU reported by
	// GODEBUG=gctrace=1.
	GCCPUFraction *stats.Float64Measure
}

// Start initiates a Go routine that starts pushing metrics into
// the provided measures.
func (msp *MemStatsProvider) Start(ctx context.Context, period time.Duration) {
	go func() {
		ticker := time.NewTicker(period)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				ms := runtime.MemStats{}
				runtime.ReadMemStats(&ms)
				if msp.Alloc != nil {
					Record(ctx, msp.Alloc.M(int64(ms.Alloc)))
				}
				if msp.TotalAlloc != nil {
					Record(ctx, msp.TotalAlloc.M(int64(ms.TotalAlloc)))
				}
				if msp.Sys != nil {
					Record(ctx, msp.Sys.M(int64(ms.Sys)))
				}
				if msp.Lookups != nil {
					Record(ctx, msp.Lookups.M(int64(ms.Lookups)))
				}
				if msp.Mallocs != nil {
					Record(ctx, msp.Mallocs.M(int64(ms.Mallocs)))
				}
				if msp.Frees != nil {
					Record(ctx, msp.Frees.M(int64(ms.Frees)))
				}
				if msp.HeapAlloc != nil {
					Record(ctx, msp.HeapAlloc.M(int64(ms.HeapAlloc)))
				}
				if msp.HeapSys != nil {
					Record(ctx, msp.HeapSys.M(int64(ms.HeapSys)))
				}
				if msp.HeapIdle != nil {
					Record(ctx, msp.HeapIdle.M(int64(ms.HeapIdle)))
				}
				if msp.HeapInuse != nil {
					Record(ctx, msp.HeapInuse.M(int64(ms.HeapInuse)))
				}
				if msp.HeapReleased != nil {
					Record(ctx, msp.HeapReleased.M(int64(ms.HeapReleased)))
				}
				if msp.HeapObjects != nil {
					Record(ctx, msp.HeapObjects.M(int64(ms.HeapObjects)))
				}
				if msp.StackInuse != nil {
					Record(ctx, msp.StackInuse.M(int64(ms.StackInuse)))
				}
				if msp.StackSys != nil {
					Record(ctx, msp.StackSys.M(int64(ms.StackSys)))
				}
				if msp.MSpanInuse != nil {
					Record(ctx, msp.MSpanInuse.M(int64(ms.MSpanInuse)))
				}
				if msp.MSpanSys != nil {
					Record(ctx, msp.MSpanSys.M(int64(ms.MSpanSys)))
				}
				if msp.MCacheInuse != nil {
					Record(ctx, msp.MCacheInuse.M(int64(ms.MCacheInuse)))
				}
				if msp.MCacheSys != nil {
					Record(ctx, msp.MCacheSys.M(int64(ms.MCacheSys)))
				}
				if msp.BuckHashSys != nil {
					Record(ctx, msp.BuckHashSys.M(int64(ms.BuckHashSys)))
				}
				if msp.GCSys != nil {
					Record(ctx, msp.GCSys.M(int64(ms.GCSys)))
				}
				if msp.OtherSys != nil {
					Record(ctx, msp.OtherSys.M(int64(ms.OtherSys)))
				}
				if msp.NextGC != nil {
					Record(ctx, msp.NextGC.M(int64(ms.NextGC)))
				}
				if msp.LastGC != nil {
					Record(ctx, msp.LastGC.M(int64(ms.LastGC)))
				}
				if msp.PauseTotalNs != nil {
					Record(ctx, msp.PauseTotalNs.M(int64(ms.PauseTotalNs)))
				}
				if msp.NumGC != nil {
					Record(ctx, msp.NumGC.M(int64(ms.NumGC)))
				}
				if msp.NumForcedGC != nil {
					Record(ctx, msp.NumForcedGC.M(int64(ms.NumForcedGC)))
				}
				if msp.GCCPUFraction != nil {
					Record(ctx, msp.GCCPUFraction.M(ms.GCCPUFraction))
				}
			}
		}
	}()
}

// DefaultViews returns a list of views suitable for passing to view.Register
func (msp *MemStatsProvider) DefaultViews() (views []*view.View) {
	if m := msp.Alloc; m != nil {
		views = append(views, measureView(m, view.LastValue()))
	}
	if m := msp.TotalAlloc; m != nil {
		views = append(views, measureView(m, view.LastValue()))
	}
	if m := msp.Sys; m != nil {
		views = append(views, measureView(m, view.LastValue()))
	}
	if m := msp.Lookups; m != nil {
		views = append(views, measureView(m, view.LastValue()))
	}
	if m := msp.Mallocs; m != nil {
		views = append(views, measureView(m, view.LastValue()))
	}
	if m := msp.Frees; m != nil {
		views = append(views, measureView(m, view.LastValue()))
	}
	if m := msp.HeapAlloc; m != nil {
		views = append(views, measureView(m, view.LastValue()))
	}
	if m := msp.HeapSys; m != nil {
		views = append(views, measureView(m, view.LastValue()))
	}
	if m := msp.HeapIdle; m != nil {
		views = append(views, measureView(m, view.LastValue()))
	}
	if m := msp.HeapInuse; m != nil {
		views = append(views, measureView(m, view.LastValue()))
	}
	if m := msp.HeapReleased; m != nil {
		views = append(views, measureView(m, view.LastValue()))
	}
	if m := msp.HeapObjects; m != nil {
		views = append(views, measureView(m, view.LastValue()))
	}
	if m := msp.StackInuse; m != nil {
		views = append(views, measureView(m, view.LastValue()))
	}
	if m := msp.StackSys; m != nil {
		views = append(views, measureView(m, view.LastValue()))
	}
	if m := msp.MSpanInuse; m != nil {
		views = append(views, measureView(m, view.LastValue()))
	}
	if m := msp.MSpanSys; m != nil {
		views = append(views, measureView(m, view.LastValue()))
	}
	if m := msp.MCacheInuse; m != nil {
		views = append(views, measureView(m, view.LastValue()))
	}
	if m := msp.MCacheSys; m != nil {
		views = append(views, measureView(m, view.LastValue()))
	}
	if m := msp.BuckHashSys; m != nil {
		views = append(views, measureView(m, view.LastValue()))
	}
	if m := msp.GCSys; m != nil {
		views = append(views, measureView(m, view.LastValue()))
	}
	if m := msp.OtherSys; m != nil {
		views = append(views, measureView(m, view.LastValue()))
	}
	if m := msp.NextGC; m != nil {
		views = append(views, measureView(m, view.LastValue()))
	}
	if m := msp.LastGC; m != nil {
		views = append(views, measureView(m, view.LastValue()))
	}
	if m := msp.PauseTotalNs; m != nil {
		views = append(views, measureView(m, view.LastValue()))
	}
	if m := msp.NumGC; m != nil {
		views = append(views, measureView(m, view.LastValue()))
	}
	if m := msp.NumForcedGC; m != nil {
		views = append(views, measureView(m, view.LastValue()))
	}
	if m := msp.GCCPUFraction; m != nil {
		views = append(views, measureView(m, view.LastValue()))
	}
	return
}
