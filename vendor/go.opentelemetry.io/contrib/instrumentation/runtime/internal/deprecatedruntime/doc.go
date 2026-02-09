// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package deprecatedruntime implements the deprecated runtime metrics for OpenTelemetry.
//
// The metric events produced are:
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
package deprecatedruntime // import "go.opentelemetry.io/contrib/instrumentation/runtime/internal/deprecatedruntime"
