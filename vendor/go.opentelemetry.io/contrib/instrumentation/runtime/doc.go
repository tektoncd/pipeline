// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package runtime implements the conventional runtime metrics specified by OpenTelemetry.
//
// The metric events produced are:
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
package runtime // import "go.opentelemetry.io/contrib/instrumentation/runtime"
