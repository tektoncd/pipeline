// Code generated from semantic convention specification. DO NOT EDIT.

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package goconv provides types and functionality for OpenTelemetry semantic
// conventions in the "go" namespace.
package goconv

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
)

var (
	addOptPool = &sync.Pool{New: func() any { return &[]metric.AddOption{} }}
	recOptPool = &sync.Pool{New: func() any { return &[]metric.RecordOption{} }}
)

// CPUStateAttr is an attribute conforming to the go.cpu.state semantic
// conventions. It represents the state of the CPU.
type CPUStateAttr string

var (
	// CPUStateUser is the CPU time spent running user Go code.
	CPUStateUser CPUStateAttr = "user"
	// CPUStateGC is the CPU time spent performing garbage collection tasks.
	CPUStateGC CPUStateAttr = "gc"
	// CPUStateScavenge is the CPU time spent returning unused memory to the
	// underlying platform.
	CPUStateScavenge CPUStateAttr = "scavenge"
	// CPUStateIdle is the available CPU time not spent executing any Go or Go
	// runtime code.
	CPUStateIdle CPUStateAttr = "idle"
)

// MemoryTypeAttr is an attribute conforming to the go.memory.type semantic
// conventions. It represents the type of memory.
type MemoryTypeAttr string

var (
	// MemoryTypeStack is the memory allocated from the heap that is reserved for
	// stack space, whether or not it is currently in-use.
	MemoryTypeStack MemoryTypeAttr = "stack"
	// MemoryTypeOther is the memory used by the Go runtime, excluding other
	// categories of memory usage described in this enumeration.
	MemoryTypeOther MemoryTypeAttr = "other"
)

// ConfigGogc is an instrument used to record metric values conforming to the
// "go.config.gogc" semantic conventions. It represents the heap size target
// percentage configured by the user, otherwise 100.
type ConfigGogc struct {
	metric.Int64ObservableUpDownCounter
}

var newConfigGogcOpts = []metric.Int64ObservableUpDownCounterOption{
	metric.WithDescription("Heap size target percentage configured by the user, otherwise 100."),
	metric.WithUnit("%"),
}

// NewConfigGogc returns a new ConfigGogc instrument.
func NewConfigGogc(
	m metric.Meter,
	opt ...metric.Int64ObservableUpDownCounterOption,
) (ConfigGogc, error) {
	// Check if the meter is nil.
	if m == nil {
		return ConfigGogc{noop.Int64ObservableUpDownCounter{}}, nil
	}

	if len(opt) == 0 {
		opt = newConfigGogcOpts
	} else {
		opt = append(opt, newConfigGogcOpts...)
	}

	i, err := m.Int64ObservableUpDownCounter(
		"go.config.gogc",
		opt...,
	)
	if err != nil {
		return ConfigGogc{noop.Int64ObservableUpDownCounter{}}, err
	}
	return ConfigGogc{i}, nil
}

// Inst returns the underlying metric instrument.
func (m ConfigGogc) Inst() metric.Int64ObservableUpDownCounter {
	return m.Int64ObservableUpDownCounter
}

// Name returns the semantic convention name of the instrument.
func (ConfigGogc) Name() string {
	return "go.config.gogc"
}

// Unit returns the semantic convention unit of the instrument
func (ConfigGogc) Unit() string {
	return "%"
}

// Description returns the semantic convention description of the instrument
func (ConfigGogc) Description() string {
	return "Heap size target percentage configured by the user, otherwise 100."
}

// CPUTime is an instrument used to record metric values conforming to the
// "go.cpu.time" semantic conventions. It represents the estimated CPU time spent
// by the Go runtime.
type CPUTime struct {
	metric.Float64Counter
}

var newCPUTimeOpts = []metric.Float64CounterOption{
	metric.WithDescription("Estimated CPU time spent by the Go runtime."),
	metric.WithUnit("s"),
}

// NewCPUTime returns a new CPUTime instrument.
func NewCPUTime(
	m metric.Meter,
	opt ...metric.Float64CounterOption,
) (CPUTime, error) {
	// Check if the meter is nil.
	if m == nil {
		return CPUTime{noop.Float64Counter{}}, nil
	}

	if len(opt) == 0 {
		opt = newCPUTimeOpts
	} else {
		opt = append(opt, newCPUTimeOpts...)
	}

	i, err := m.Float64Counter(
		"go.cpu.time",
		opt...,
	)
	if err != nil {
		return CPUTime{noop.Float64Counter{}}, err
	}
	return CPUTime{i}, nil
}

// Inst returns the underlying metric instrument.
func (m CPUTime) Inst() metric.Float64Counter {
	return m.Float64Counter
}

// Name returns the semantic convention name of the instrument.
func (CPUTime) Name() string {
	return "go.cpu.time"
}

// Unit returns the semantic convention unit of the instrument
func (CPUTime) Unit() string {
	return "s"
}

// Description returns the semantic convention description of the instrument
func (CPUTime) Description() string {
	return "Estimated CPU time spent by the Go runtime."
}

// Add adds incr to the existing count for attrs.
//
// The cpuState is the the state of the CPU.
//
// All additional attrs passed are included in the recorded value.
//
// Computed from `/cpu/classes/...` metrics. This metric is an overestimate, and
// not directly comparable to system CPU time measurements. Compare only with
// other `go.cpu.time` metrics.
func (m CPUTime) Add(
	ctx context.Context,
	incr float64,
	cpuState CPUStateAttr,
	attrs ...attribute.KeyValue,
) {
	if !m.Float64Counter.Enabled(ctx) {
		return
	}
	if len(attrs) == 0 {
		m.Float64Counter.Add(ctx, incr, metric.WithAttributes(
			attribute.String("go.cpu.state", string(cpuState)),
		))
		return
	}

	o := addOptPool.Get().(*[]metric.AddOption)
	defer func() {
		clear(*o)
		*o = (*o)[:0]
		addOptPool.Put(o)
	}()

	*o = append(
		*o,
		metric.WithAttributes(
			append(
				attrs[:len(attrs):len(attrs)],
				attribute.String("go.cpu.state", string(cpuState)),
			)...,
		),
	)

	m.Float64Counter.Add(ctx, incr, *o...)
}

// AddSet adds incr to the existing count for set.
//
// Computed from `/cpu/classes/...` metrics. This metric is an overestimate, and
// not directly comparable to system CPU time measurements. Compare only with
// other `go.cpu.time` metrics.
func (m CPUTime) AddSet(ctx context.Context, incr float64, set attribute.Set) {
	if !m.Float64Counter.Enabled(ctx) {
		return
	}
	if set.Len() == 0 {
		m.Float64Counter.Add(ctx, incr)
		return
	}

	o := addOptPool.Get().(*[]metric.AddOption)
	defer func() {
		clear(*o)
		*o = (*o)[:0]
		addOptPool.Put(o)
	}()

	*o = append(*o, metric.WithAttributeSet(set))
	m.Float64Counter.Add(ctx, incr, *o...)
}

// AttrCPUDetailedState returns an optional attribute for the
// "go.cpu.detailed_state" semantic convention. It represents the detailed state
// of the CPU.
func (CPUTime) AttrCPUDetailedState(val string) attribute.KeyValue {
	return attribute.String("go.cpu.detailed_state", val)
}

// CPUTimeObservable is an instrument used to record metric values conforming to
// the "go.cpu.time" semantic conventions. It represents the estimated CPU time
// spent by the Go runtime.
type CPUTimeObservable struct {
	metric.Float64ObservableCounter
}

var newCPUTimeObservableOpts = []metric.Float64ObservableCounterOption{
	metric.WithDescription("Estimated CPU time spent by the Go runtime."),
	metric.WithUnit("s"),
}

// NewCPUTimeObservable returns a new CPUTimeObservable instrument.
func NewCPUTimeObservable(
	m metric.Meter,
	opt ...metric.Float64ObservableCounterOption,
) (CPUTimeObservable, error) {
	// Check if the meter is nil.
	if m == nil {
		return CPUTimeObservable{noop.Float64ObservableCounter{}}, nil
	}

	if len(opt) == 0 {
		opt = newCPUTimeObservableOpts
	} else {
		opt = append(opt, newCPUTimeObservableOpts...)
	}

	i, err := m.Float64ObservableCounter(
		"go.cpu.time",
		opt...,
	)
	if err != nil {
		return CPUTimeObservable{noop.Float64ObservableCounter{}}, err
	}
	return CPUTimeObservable{i}, nil
}

// Inst returns the underlying metric instrument.
func (m CPUTimeObservable) Inst() metric.Float64ObservableCounter {
	return m.Float64ObservableCounter
}

// Name returns the semantic convention name of the instrument.
func (CPUTimeObservable) Name() string {
	return "go.cpu.time"
}

// Unit returns the semantic convention unit of the instrument
func (CPUTimeObservable) Unit() string {
	return "s"
}

// Description returns the semantic convention description of the instrument
func (CPUTimeObservable) Description() string {
	return "Estimated CPU time spent by the Go runtime."
}

// AttrCPUState returns a required attribute for the "go.cpu.state" semantic
// convention. It represents the state of the CPU.
func (CPUTimeObservable) AttrCPUState(val CPUStateAttr) attribute.KeyValue {
	return attribute.String("go.cpu.state", string(val))
}

// AttrCPUDetailedState returns an optional attribute for the
// "go.cpu.detailed_state" semantic convention. It represents the detailed state
// of the CPU.
func (CPUTimeObservable) AttrCPUDetailedState(val string) attribute.KeyValue {
	return attribute.String("go.cpu.detailed_state", val)
}

// GoroutineCount is an instrument used to record metric values conforming to the
// "go.goroutine.count" semantic conventions. It represents the count of live
// goroutines.
type GoroutineCount struct {
	metric.Int64ObservableUpDownCounter
}

var newGoroutineCountOpts = []metric.Int64ObservableUpDownCounterOption{
	metric.WithDescription("Count of live goroutines."),
	metric.WithUnit("{goroutine}"),
}

// NewGoroutineCount returns a new GoroutineCount instrument.
func NewGoroutineCount(
	m metric.Meter,
	opt ...metric.Int64ObservableUpDownCounterOption,
) (GoroutineCount, error) {
	// Check if the meter is nil.
	if m == nil {
		return GoroutineCount{noop.Int64ObservableUpDownCounter{}}, nil
	}

	if len(opt) == 0 {
		opt = newGoroutineCountOpts
	} else {
		opt = append(opt, newGoroutineCountOpts...)
	}

	i, err := m.Int64ObservableUpDownCounter(
		"go.goroutine.count",
		opt...,
	)
	if err != nil {
		return GoroutineCount{noop.Int64ObservableUpDownCounter{}}, err
	}
	return GoroutineCount{i}, nil
}

// Inst returns the underlying metric instrument.
func (m GoroutineCount) Inst() metric.Int64ObservableUpDownCounter {
	return m.Int64ObservableUpDownCounter
}

// Name returns the semantic convention name of the instrument.
func (GoroutineCount) Name() string {
	return "go.goroutine.count"
}

// Unit returns the semantic convention unit of the instrument
func (GoroutineCount) Unit() string {
	return "{goroutine}"
}

// Description returns the semantic convention description of the instrument
func (GoroutineCount) Description() string {
	return "Count of live goroutines."
}

// MemoryAllocated is an instrument used to record metric values conforming to
// the "go.memory.allocated" semantic conventions. It represents the memory
// allocated to the heap by the application.
type MemoryAllocated struct {
	metric.Int64ObservableCounter
}

var newMemoryAllocatedOpts = []metric.Int64ObservableCounterOption{
	metric.WithDescription("Memory allocated to the heap by the application."),
	metric.WithUnit("By"),
}

// NewMemoryAllocated returns a new MemoryAllocated instrument.
func NewMemoryAllocated(
	m metric.Meter,
	opt ...metric.Int64ObservableCounterOption,
) (MemoryAllocated, error) {
	// Check if the meter is nil.
	if m == nil {
		return MemoryAllocated{noop.Int64ObservableCounter{}}, nil
	}

	if len(opt) == 0 {
		opt = newMemoryAllocatedOpts
	} else {
		opt = append(opt, newMemoryAllocatedOpts...)
	}

	i, err := m.Int64ObservableCounter(
		"go.memory.allocated",
		opt...,
	)
	if err != nil {
		return MemoryAllocated{noop.Int64ObservableCounter{}}, err
	}
	return MemoryAllocated{i}, nil
}

// Inst returns the underlying metric instrument.
func (m MemoryAllocated) Inst() metric.Int64ObservableCounter {
	return m.Int64ObservableCounter
}

// Name returns the semantic convention name of the instrument.
func (MemoryAllocated) Name() string {
	return "go.memory.allocated"
}

// Unit returns the semantic convention unit of the instrument
func (MemoryAllocated) Unit() string {
	return "By"
}

// Description returns the semantic convention description of the instrument
func (MemoryAllocated) Description() string {
	return "Memory allocated to the heap by the application."
}

// MemoryAllocations is an instrument used to record metric values conforming to
// the "go.memory.allocations" semantic conventions. It represents the count of
// allocations to the heap by the application.
type MemoryAllocations struct {
	metric.Int64ObservableCounter
}

var newMemoryAllocationsOpts = []metric.Int64ObservableCounterOption{
	metric.WithDescription("Count of allocations to the heap by the application."),
	metric.WithUnit("{allocation}"),
}

// NewMemoryAllocations returns a new MemoryAllocations instrument.
func NewMemoryAllocations(
	m metric.Meter,
	opt ...metric.Int64ObservableCounterOption,
) (MemoryAllocations, error) {
	// Check if the meter is nil.
	if m == nil {
		return MemoryAllocations{noop.Int64ObservableCounter{}}, nil
	}

	if len(opt) == 0 {
		opt = newMemoryAllocationsOpts
	} else {
		opt = append(opt, newMemoryAllocationsOpts...)
	}

	i, err := m.Int64ObservableCounter(
		"go.memory.allocations",
		opt...,
	)
	if err != nil {
		return MemoryAllocations{noop.Int64ObservableCounter{}}, err
	}
	return MemoryAllocations{i}, nil
}

// Inst returns the underlying metric instrument.
func (m MemoryAllocations) Inst() metric.Int64ObservableCounter {
	return m.Int64ObservableCounter
}

// Name returns the semantic convention name of the instrument.
func (MemoryAllocations) Name() string {
	return "go.memory.allocations"
}

// Unit returns the semantic convention unit of the instrument
func (MemoryAllocations) Unit() string {
	return "{allocation}"
}

// Description returns the semantic convention description of the instrument
func (MemoryAllocations) Description() string {
	return "Count of allocations to the heap by the application."
}

// MemoryGCCycles is an instrument used to record metric values conforming to the
// "go.memory.gc.cycles" semantic conventions. It represents the number of
// completed GC cycles.
type MemoryGCCycles struct {
	metric.Int64Counter
}

var newMemoryGCCyclesOpts = []metric.Int64CounterOption{
	metric.WithDescription("Number of completed GC cycles."),
	metric.WithUnit("{gc_cycle}"),
}

// NewMemoryGCCycles returns a new MemoryGCCycles instrument.
func NewMemoryGCCycles(
	m metric.Meter,
	opt ...metric.Int64CounterOption,
) (MemoryGCCycles, error) {
	// Check if the meter is nil.
	if m == nil {
		return MemoryGCCycles{noop.Int64Counter{}}, nil
	}

	if len(opt) == 0 {
		opt = newMemoryGCCyclesOpts
	} else {
		opt = append(opt, newMemoryGCCyclesOpts...)
	}

	i, err := m.Int64Counter(
		"go.memory.gc.cycles",
		opt...,
	)
	if err != nil {
		return MemoryGCCycles{noop.Int64Counter{}}, err
	}
	return MemoryGCCycles{i}, nil
}

// Inst returns the underlying metric instrument.
func (m MemoryGCCycles) Inst() metric.Int64Counter {
	return m.Int64Counter
}

// Name returns the semantic convention name of the instrument.
func (MemoryGCCycles) Name() string {
	return "go.memory.gc.cycles"
}

// Unit returns the semantic convention unit of the instrument
func (MemoryGCCycles) Unit() string {
	return "{gc_cycle}"
}

// Description returns the semantic convention description of the instrument
func (MemoryGCCycles) Description() string {
	return "Number of completed GC cycles."
}

// Add adds incr to the existing count for attrs.
//
// Computed from `/gc/cycles/total:gc-cycles`.
func (m MemoryGCCycles) Add(ctx context.Context, incr int64, attrs ...attribute.KeyValue) {
	if !m.Int64Counter.Enabled(ctx) {
		return
	}
	if len(attrs) == 0 {
		m.Int64Counter.Add(ctx, incr)
		return
	}

	o := addOptPool.Get().(*[]metric.AddOption)
	defer func() {
		clear(*o)
		*o = (*o)[:0]
		addOptPool.Put(o)
	}()

	*o = append(*o, metric.WithAttributes(attrs...))
	m.Int64Counter.Add(ctx, incr, *o...)
}

// AddSet adds incr to the existing count for set.
//
// Computed from `/gc/cycles/total:gc-cycles`.
func (m MemoryGCCycles) AddSet(ctx context.Context, incr int64, set attribute.Set) {
	if !m.Int64Counter.Enabled(ctx) {
		return
	}
	if set.Len() == 0 {
		m.Int64Counter.Add(ctx, incr)
		return
	}

	o := addOptPool.Get().(*[]metric.AddOption)
	defer func() {
		clear(*o)
		*o = (*o)[:0]
		addOptPool.Put(o)
	}()

	*o = append(*o, metric.WithAttributeSet(set))
	m.Int64Counter.Add(ctx, incr, *o...)
}

// MemoryGCCyclesObservable is an instrument used to record metric values
// conforming to the "go.memory.gc.cycles" semantic conventions. It represents
// the number of completed GC cycles.
type MemoryGCCyclesObservable struct {
	metric.Int64ObservableCounter
}

var newMemoryGCCyclesObservableOpts = []metric.Int64ObservableCounterOption{
	metric.WithDescription("Number of completed GC cycles."),
	metric.WithUnit("{gc_cycle}"),
}

// NewMemoryGCCyclesObservable returns a new MemoryGCCyclesObservable instrument.
func NewMemoryGCCyclesObservable(
	m metric.Meter,
	opt ...metric.Int64ObservableCounterOption,
) (MemoryGCCyclesObservable, error) {
	// Check if the meter is nil.
	if m == nil {
		return MemoryGCCyclesObservable{noop.Int64ObservableCounter{}}, nil
	}

	if len(opt) == 0 {
		opt = newMemoryGCCyclesObservableOpts
	} else {
		opt = append(opt, newMemoryGCCyclesObservableOpts...)
	}

	i, err := m.Int64ObservableCounter(
		"go.memory.gc.cycles",
		opt...,
	)
	if err != nil {
		return MemoryGCCyclesObservable{noop.Int64ObservableCounter{}}, err
	}
	return MemoryGCCyclesObservable{i}, nil
}

// Inst returns the underlying metric instrument.
func (m MemoryGCCyclesObservable) Inst() metric.Int64ObservableCounter {
	return m.Int64ObservableCounter
}

// Name returns the semantic convention name of the instrument.
func (MemoryGCCyclesObservable) Name() string {
	return "go.memory.gc.cycles"
}

// Unit returns the semantic convention unit of the instrument
func (MemoryGCCyclesObservable) Unit() string {
	return "{gc_cycle}"
}

// Description returns the semantic convention description of the instrument
func (MemoryGCCyclesObservable) Description() string {
	return "Number of completed GC cycles."
}

// MemoryGCGoal is an instrument used to record metric values conforming to the
// "go.memory.gc.goal" semantic conventions. It represents the heap size target
// for the end of the GC cycle.
type MemoryGCGoal struct {
	metric.Int64ObservableUpDownCounter
}

var newMemoryGCGoalOpts = []metric.Int64ObservableUpDownCounterOption{
	metric.WithDescription("Heap size target for the end of the GC cycle."),
	metric.WithUnit("By"),
}

// NewMemoryGCGoal returns a new MemoryGCGoal instrument.
func NewMemoryGCGoal(
	m metric.Meter,
	opt ...metric.Int64ObservableUpDownCounterOption,
) (MemoryGCGoal, error) {
	// Check if the meter is nil.
	if m == nil {
		return MemoryGCGoal{noop.Int64ObservableUpDownCounter{}}, nil
	}

	if len(opt) == 0 {
		opt = newMemoryGCGoalOpts
	} else {
		opt = append(opt, newMemoryGCGoalOpts...)
	}

	i, err := m.Int64ObservableUpDownCounter(
		"go.memory.gc.goal",
		opt...,
	)
	if err != nil {
		return MemoryGCGoal{noop.Int64ObservableUpDownCounter{}}, err
	}
	return MemoryGCGoal{i}, nil
}

// Inst returns the underlying metric instrument.
func (m MemoryGCGoal) Inst() metric.Int64ObservableUpDownCounter {
	return m.Int64ObservableUpDownCounter
}

// Name returns the semantic convention name of the instrument.
func (MemoryGCGoal) Name() string {
	return "go.memory.gc.goal"
}

// Unit returns the semantic convention unit of the instrument
func (MemoryGCGoal) Unit() string {
	return "By"
}

// Description returns the semantic convention description of the instrument
func (MemoryGCGoal) Description() string {
	return "Heap size target for the end of the GC cycle."
}

// MemoryGCPauseDuration is an instrument used to record metric values conforming
// to the "go.memory.gc.pause.duration" semantic conventions. It represents the
// distribution of individual GC-related stop-the-world pause latencies. This is
// the time from deciding to stop the world until the world is started again.
type MemoryGCPauseDuration struct {
	metric.Float64Histogram
}

var newMemoryGCPauseDurationOpts = []metric.Float64HistogramOption{
	metric.WithDescription("Distribution of individual GC-related stop-the-world pause latencies. This is the time from deciding to stop the world until the world is started again."),
	metric.WithUnit("s"),
}

// NewMemoryGCPauseDuration returns a new MemoryGCPauseDuration instrument.
func NewMemoryGCPauseDuration(
	m metric.Meter,
	opt ...metric.Float64HistogramOption,
) (MemoryGCPauseDuration, error) {
	// Check if the meter is nil.
	if m == nil {
		return MemoryGCPauseDuration{noop.Float64Histogram{}}, nil
	}

	if len(opt) == 0 {
		opt = newMemoryGCPauseDurationOpts
	} else {
		opt = append(opt, newMemoryGCPauseDurationOpts...)
	}

	i, err := m.Float64Histogram(
		"go.memory.gc.pause.duration",
		opt...,
	)
	if err != nil {
		return MemoryGCPauseDuration{noop.Float64Histogram{}}, err
	}
	return MemoryGCPauseDuration{i}, nil
}

// Inst returns the underlying metric instrument.
func (m MemoryGCPauseDuration) Inst() metric.Float64Histogram {
	return m.Float64Histogram
}

// Name returns the semantic convention name of the instrument.
func (MemoryGCPauseDuration) Name() string {
	return "go.memory.gc.pause.duration"
}

// Unit returns the semantic convention unit of the instrument
func (MemoryGCPauseDuration) Unit() string {
	return "s"
}

// Description returns the semantic convention description of the instrument
func (MemoryGCPauseDuration) Description() string {
	return "Distribution of individual GC-related stop-the-world pause latencies. This is the time from deciding to stop the world until the world is started again."
}

// Record records val to the current distribution for attrs.
//
// Computed from `/sched/pauses/total/gc:seconds`. Bucket boundaries are provided
// by the runtime, and are subject to change.
func (m MemoryGCPauseDuration) Record(ctx context.Context, val float64, attrs ...attribute.KeyValue) {
	if !m.Float64Histogram.Enabled(ctx) {
		return
	}
	if len(attrs) == 0 {
		m.Float64Histogram.Record(ctx, val)
		return
	}

	o := recOptPool.Get().(*[]metric.RecordOption)
	defer func() {
		clear(*o)
		*o = (*o)[:0]
		recOptPool.Put(o)
	}()

	*o = append(*o, metric.WithAttributes(attrs...))
	m.Float64Histogram.Record(ctx, val, *o...)
}

// RecordSet records val to the current distribution for set.
//
// Computed from `/sched/pauses/total/gc:seconds`. Bucket boundaries are provided
// by the runtime, and are subject to change.
func (m MemoryGCPauseDuration) RecordSet(ctx context.Context, val float64, set attribute.Set) {
	if !m.Float64Histogram.Enabled(ctx) {
		return
	}
	if set.Len() == 0 {
		m.Float64Histogram.Record(ctx, val)
		return
	}

	o := recOptPool.Get().(*[]metric.RecordOption)
	defer func() {
		clear(*o)
		*o = (*o)[:0]
		recOptPool.Put(o)
	}()

	*o = append(*o, metric.WithAttributeSet(set))
	m.Float64Histogram.Record(ctx, val, *o...)
}

// MemoryLimit is an instrument used to record metric values conforming to the
// "go.memory.limit" semantic conventions. It represents the go runtime memory
// limit configured by the user, if a limit exists.
type MemoryLimit struct {
	metric.Int64ObservableUpDownCounter
}

var newMemoryLimitOpts = []metric.Int64ObservableUpDownCounterOption{
	metric.WithDescription("Go runtime memory limit configured by the user, if a limit exists."),
	metric.WithUnit("By"),
}

// NewMemoryLimit returns a new MemoryLimit instrument.
func NewMemoryLimit(
	m metric.Meter,
	opt ...metric.Int64ObservableUpDownCounterOption,
) (MemoryLimit, error) {
	// Check if the meter is nil.
	if m == nil {
		return MemoryLimit{noop.Int64ObservableUpDownCounter{}}, nil
	}

	if len(opt) == 0 {
		opt = newMemoryLimitOpts
	} else {
		opt = append(opt, newMemoryLimitOpts...)
	}

	i, err := m.Int64ObservableUpDownCounter(
		"go.memory.limit",
		opt...,
	)
	if err != nil {
		return MemoryLimit{noop.Int64ObservableUpDownCounter{}}, err
	}
	return MemoryLimit{i}, nil
}

// Inst returns the underlying metric instrument.
func (m MemoryLimit) Inst() metric.Int64ObservableUpDownCounter {
	return m.Int64ObservableUpDownCounter
}

// Name returns the semantic convention name of the instrument.
func (MemoryLimit) Name() string {
	return "go.memory.limit"
}

// Unit returns the semantic convention unit of the instrument
func (MemoryLimit) Unit() string {
	return "By"
}

// Description returns the semantic convention description of the instrument
func (MemoryLimit) Description() string {
	return "Go runtime memory limit configured by the user, if a limit exists."
}

// MemoryUsed is an instrument used to record metric values conforming to the
// "go.memory.used" semantic conventions. It represents the memory used by the Go
// runtime.
type MemoryUsed struct {
	metric.Int64ObservableUpDownCounter
}

var newMemoryUsedOpts = []metric.Int64ObservableUpDownCounterOption{
	metric.WithDescription("Memory used by the Go runtime."),
	metric.WithUnit("By"),
}

// NewMemoryUsed returns a new MemoryUsed instrument.
func NewMemoryUsed(
	m metric.Meter,
	opt ...metric.Int64ObservableUpDownCounterOption,
) (MemoryUsed, error) {
	// Check if the meter is nil.
	if m == nil {
		return MemoryUsed{noop.Int64ObservableUpDownCounter{}}, nil
	}

	if len(opt) == 0 {
		opt = newMemoryUsedOpts
	} else {
		opt = append(opt, newMemoryUsedOpts...)
	}

	i, err := m.Int64ObservableUpDownCounter(
		"go.memory.used",
		opt...,
	)
	if err != nil {
		return MemoryUsed{noop.Int64ObservableUpDownCounter{}}, err
	}
	return MemoryUsed{i}, nil
}

// Inst returns the underlying metric instrument.
func (m MemoryUsed) Inst() metric.Int64ObservableUpDownCounter {
	return m.Int64ObservableUpDownCounter
}

// Name returns the semantic convention name of the instrument.
func (MemoryUsed) Name() string {
	return "go.memory.used"
}

// Unit returns the semantic convention unit of the instrument
func (MemoryUsed) Unit() string {
	return "By"
}

// Description returns the semantic convention description of the instrument
func (MemoryUsed) Description() string {
	return "Memory used by the Go runtime."
}

// AttrMemoryType returns an optional attribute for the "go.memory.type" semantic
// convention. It represents the type of memory.
func (MemoryUsed) AttrMemoryType(val MemoryTypeAttr) attribute.KeyValue {
	return attribute.String("go.memory.type", string(val))
}

// AttrMemoryDetailedType returns an optional attribute for the
// "go.memory.detailed_type" semantic convention. It represents the detailed type
// of memory.
func (MemoryUsed) AttrMemoryDetailedType(val string) attribute.KeyValue {
	return attribute.String("go.memory.detailed_type", val)
}

// ProcessorLimit is an instrument used to record metric values conforming to the
// "go.processor.limit" semantic conventions. It represents the number of OS
// threads that can execute user-level Go code simultaneously.
type ProcessorLimit struct {
	metric.Int64ObservableUpDownCounter
}

var newProcessorLimitOpts = []metric.Int64ObservableUpDownCounterOption{
	metric.WithDescription("The number of OS threads that can execute user-level Go code simultaneously."),
	metric.WithUnit("{thread}"),
}

// NewProcessorLimit returns a new ProcessorLimit instrument.
func NewProcessorLimit(
	m metric.Meter,
	opt ...metric.Int64ObservableUpDownCounterOption,
) (ProcessorLimit, error) {
	// Check if the meter is nil.
	if m == nil {
		return ProcessorLimit{noop.Int64ObservableUpDownCounter{}}, nil
	}

	if len(opt) == 0 {
		opt = newProcessorLimitOpts
	} else {
		opt = append(opt, newProcessorLimitOpts...)
	}

	i, err := m.Int64ObservableUpDownCounter(
		"go.processor.limit",
		opt...,
	)
	if err != nil {
		return ProcessorLimit{noop.Int64ObservableUpDownCounter{}}, err
	}
	return ProcessorLimit{i}, nil
}

// Inst returns the underlying metric instrument.
func (m ProcessorLimit) Inst() metric.Int64ObservableUpDownCounter {
	return m.Int64ObservableUpDownCounter
}

// Name returns the semantic convention name of the instrument.
func (ProcessorLimit) Name() string {
	return "go.processor.limit"
}

// Unit returns the semantic convention unit of the instrument
func (ProcessorLimit) Unit() string {
	return "{thread}"
}

// Description returns the semantic convention description of the instrument
func (ProcessorLimit) Description() string {
	return "The number of OS threads that can execute user-level Go code simultaneously."
}

// ScheduleDuration is an instrument used to record metric values conforming to
// the "go.schedule.duration" semantic conventions. It represents the time
// goroutines have spent in the scheduler in a runnable state before actually
// running.
type ScheduleDuration struct {
	metric.Float64Histogram
}

var newScheduleDurationOpts = []metric.Float64HistogramOption{
	metric.WithDescription("The time goroutines have spent in the scheduler in a runnable state before actually running."),
	metric.WithUnit("s"),
}

// NewScheduleDuration returns a new ScheduleDuration instrument.
func NewScheduleDuration(
	m metric.Meter,
	opt ...metric.Float64HistogramOption,
) (ScheduleDuration, error) {
	// Check if the meter is nil.
	if m == nil {
		return ScheduleDuration{noop.Float64Histogram{}}, nil
	}

	if len(opt) == 0 {
		opt = newScheduleDurationOpts
	} else {
		opt = append(opt, newScheduleDurationOpts...)
	}

	i, err := m.Float64Histogram(
		"go.schedule.duration",
		opt...,
	)
	if err != nil {
		return ScheduleDuration{noop.Float64Histogram{}}, err
	}
	return ScheduleDuration{i}, nil
}

// Inst returns the underlying metric instrument.
func (m ScheduleDuration) Inst() metric.Float64Histogram {
	return m.Float64Histogram
}

// Name returns the semantic convention name of the instrument.
func (ScheduleDuration) Name() string {
	return "go.schedule.duration"
}

// Unit returns the semantic convention unit of the instrument
func (ScheduleDuration) Unit() string {
	return "s"
}

// Description returns the semantic convention description of the instrument
func (ScheduleDuration) Description() string {
	return "The time goroutines have spent in the scheduler in a runnable state before actually running."
}

// Record records val to the current distribution for attrs.
//
// Computed from `/sched/latencies:seconds`. Bucket boundaries are provided by
// the runtime, and are subject to change.
func (m ScheduleDuration) Record(ctx context.Context, val float64, attrs ...attribute.KeyValue) {
	if !m.Float64Histogram.Enabled(ctx) {
		return
	}
	if len(attrs) == 0 {
		m.Float64Histogram.Record(ctx, val)
		return
	}

	o := recOptPool.Get().(*[]metric.RecordOption)
	defer func() {
		clear(*o)
		*o = (*o)[:0]
		recOptPool.Put(o)
	}()

	*o = append(*o, metric.WithAttributes(attrs...))
	m.Float64Histogram.Record(ctx, val, *o...)
}

// RecordSet records val to the current distribution for set.
//
// Computed from `/sched/latencies:seconds`. Bucket boundaries are provided by
// the runtime, and are subject to change.
func (m ScheduleDuration) RecordSet(ctx context.Context, val float64, set attribute.Set) {
	if !m.Float64Histogram.Enabled(ctx) {
		return
	}
	if set.Len() == 0 {
		m.Float64Histogram.Record(ctx, val)
		return
	}

	o := recOptPool.Get().(*[]metric.RecordOption)
	defer func() {
		clear(*o)
		*o = (*o)[:0]
		recOptPool.Put(o)
	}()

	*o = append(*o, metric.WithAttributeSet(set))
	m.Float64Histogram.Record(ctx, val, *o...)
}
