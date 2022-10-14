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

package metrics

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"go.opencensus.io/metric/metricdata"
	"go.opencensus.io/resource"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"k8s.io/utils/clock"
)

type storedViews struct {
	views []*view.View
	lock  sync.Mutex
}

type meterExporter struct {
	m view.Meter    // NOTE: DO NOT RETURN THIS DIRECTLY; the view.Meter will not work for the empty Resource
	o stats.Options // Cache the option to reduce allocations
	e view.Exporter
	t time.Time // Time when last access occurred
}

// ResourceExporterFactory provides a hook for producing separate view.Exporters
// for each observed Resource. This is needed because OpenCensus support for
// Resources is a bit tacked-on rather than being a first-class component like
// Tags are.
type ResourceExporterFactory func(*resource.Resource) (view.Exporter, error)
type meters struct {
	meters         map[string]*meterExporter
	factory        ResourceExporterFactory
	lock           sync.Mutex
	clock          clock.WithTicker
	ticker         clock.Ticker
	tickerStopChan chan struct{}
}

// Lock regime: lock allMeters before resourceViews. The critical path is in
// optionForResource, which must lock allMeters, but only needs to lock
// resourceViews if a new meter needs to be created.
var (
	resourceViews = storedViews{}
	allMeters     = meters{
		meters: map[string]*meterExporter{"": &defaultMeter},
		clock:  clock.RealClock{},
	}

	cleanupOnce                = new(sync.Once)
	meterExporterScanFrequency = 1 * time.Minute
	maxMeterExporterAge        = 10 * time.Minute
)

// cleanup looks through allMeter's meterExporter and prunes any that were last visited
// more than maxMeterExporterAge ago. This will clean up any exporters for resources
// that no longer exist on the system (a replaced revision, e.g.).  All meterExporter
// lookups (and creations) run meterExporterForResource, which updates the timestamp
// on each access.
func cleanup() {
	expiryCutoff := allMeters.clock.Now().Add(-1 * maxMeterExporterAge)
	allMeters.lock.Lock()
	defer allMeters.lock.Unlock()
	resourceViews.lock.Lock()
	defer resourceViews.lock.Unlock()
	for key, meter := range allMeters.meters {
		if key != "" && meter.t.Before(expiryCutoff) {
			flushGivenExporter(meter.e)
			// Make a copy of views to avoid data races
			viewsCopy := copyViews(resourceViews.views)
			meter.m.Unregister(viewsCopy...)
			delete(allMeters.meters, key)
			meter.m.Stop()
		}
	}
}

// startCleanup is configured to only run once in production.  For testing,
// calling startCleanup again will terminate old cleanup threads,
// and restart anew.
func startCleanup() {
	if allMeters.tickerStopChan != nil {
		close(allMeters.tickerStopChan)
	}

	curClock := allMeters.clock
	newTicker := curClock.NewTicker(meterExporterScanFrequency)
	newStopChan := make(chan struct{})
	allMeters.tickerStopChan = newStopChan
	allMeters.ticker = newTicker
	go func() {
		defer newTicker.Stop()
		for {
			select {
			case <-newTicker.C():
				cleanup()
			case <-newStopChan:
				return
			}
		}
	}()
}

// RegisterResourceView is similar to view.Register(), except that it will
// register the view across all Resources tracked by the system, rather than
// simply the default view.
func RegisterResourceView(views ...*view.View) error {
	var err error
	allMeters.lock.Lock()
	defer allMeters.lock.Unlock()
	resourceViews.lock.Lock()
	defer resourceViews.lock.Unlock()
	for _, meter := range allMeters.meters {
		// make a copy of views to avoid data races
		viewCopy := copyViews(views)
		if e := meter.m.Register(viewCopy...); e != nil {
			err = e
		}
	}
	if err != nil {
		return err
	}
	resourceViews.views = append(resourceViews.views, views...)
	return nil
}

// UnregisterResourceView is similar to view.Unregister(), except that it will
// unregister the view across all Resources tracked byt he system, rather than
// simply the default view.
func UnregisterResourceView(views ...*view.View) {
	allMeters.lock.Lock()
	defer allMeters.lock.Unlock()

	resourceViews.lock.Lock()
	defer resourceViews.lock.Unlock()

	for _, meter := range allMeters.meters {
		// Since we make a defensive copy of all views in RegisterResourceView,
		// the caller might not have the same view pointer that was registered.
		// Use Meter.Find() to find the view with a matching name.
		for _, v := range views {
			name := v.Name
			if v.Name == "" {
				name = v.Measure.Name()
			}
			if v := meter.m.Find(name); v != nil {
				meter.m.Unregister(v)
			}
		}
	}

	j := 0
	for _, view := range resourceViews.views {
		toRemove := false
		for _, viewToRemove := range views {
			if view == viewToRemove {
				toRemove = true
				break
			}
		}
		if !toRemove {
			resourceViews.views[j] = view
			j++
		}
	}
	resourceViews.views = resourceViews.views[:j]
}

func setFactory(f ResourceExporterFactory) error {
	if f == nil {
		return errors.New("do not setFactory(nil)")
	}

	allMeters.lock.Lock()
	defer allMeters.lock.Unlock()

	allMeters.factory = f

	var retErr error

	for r, meter := range allMeters.meters {
		e, err := f(resourceFromKey(r))
		if err != nil {
			retErr = err
			continue // Keep trying to clean up remaining Meters.
		}
		meter.m.UnregisterExporter(meter.e)
		meter.m.RegisterExporter(e)
		meter.e = e
	}
	return retErr
}

func setReportingPeriod(mc *metricsConfig) {
	allMeters.lock.Lock()
	defer allMeters.lock.Unlock()

	rp := time.Duration(0)
	if mc != nil {
		rp = mc.reportingPeriod
	}
	for _, meter := range allMeters.meters {
		meter.m.SetReportingPeriod(rp)
	}
}

func flushResourceExporters() {
	allMeters.lock.Lock()
	defer allMeters.lock.Unlock()

	for _, meter := range allMeters.meters {
		flushGivenExporter(meter.e)
	}
}

// ClearMetersForTest clears the internal set of metrics being exported,
// including cleaning up background threads, and restarts cleanup thread.
//
// If a replacement clock is desired, it should be in allMeters.clock before calling.
func ClearMetersForTest() {
	allMeters.lock.Lock()
	defer allMeters.lock.Unlock()

	for k, meter := range allMeters.meters {
		if k == "" {
			continue
		}
		meter.m.Stop()
		delete(allMeters.meters, k)
	}
	startCleanup()
}

func meterExporterForResource(r *resource.Resource) *meterExporter {
	key := resourceToKey(r)
	mE := allMeters.meters[key]
	if mE == nil {
		mE = &meterExporter{}
		allMeters.meters[key] = mE
	}
	mE.t = allMeters.clock.Now()
	if mE.o != nil {
		return mE
	}
	mE.m = view.NewMeter()
	mE.m.SetResource(r)
	mE.m.Start()

	mc := getCurMetricsConfig()
	if mc != nil {
		mE.m.SetReportingPeriod(mc.reportingPeriod)
	}
	resourceViews.lock.Lock()
	defer resourceViews.lock.Unlock()
	// make a copy of views to avoid data races
	viewsCopy := copyViews(resourceViews.views)
	mE.m.Register(viewsCopy...)
	mE.o = stats.WithRecorder(mE.m)
	allMeters.meters[key] = mE
	return mE
}

// optionForResource finds or creates a stats exporter for the resource, and
// returns a stats.Option indicating which meter to record to.
func optionForResource(r *resource.Resource) (stats.Options, error) {
	// Start the allMeters cleanup thread, if not already started
	cleanupOnce.Do(startCleanup)

	allMeters.lock.Lock()
	defer allMeters.lock.Unlock()

	mE := meterExporterForResource(r)
	if mE == nil {
		return nil, fmt.Errorf("failed lookup for resource %v", r)
	}

	if mE.e != nil {
		// Assume the exporter is already started.
		return mE.o, nil
	}

	if allMeters.factory == nil {
		if mE.o != nil {
			// If we can't create exporters but we have a Meter, return that.
			return mE.o, nil
		}
		return nil, fmt.Errorf("whoops, allMeters.factory is nil")
	}
	exporter, err := allMeters.factory(r)
	if err != nil {
		return nil, err
	}

	mE.m.RegisterExporter(exporter)
	mE.e = exporter
	return mE.o, nil
}

func resourceToKey(r *resource.Resource) string {
	if r == nil {
		return ""
	}

	// If there are no labels, we have just the Type.
	if len(r.Labels) == 0 {
		return r.Type
	}

	var s strings.Builder
	writeKV := func(key, value string) { // This lambda doesn't force an allocation.
		// We use byte values 1 and 2 to avoid colliding with valid resource labels
		// and to make unpacking easy
		s.WriteByte('\x01')
		s.WriteString(key)
		s.WriteByte('\x02')
		s.WriteString(value)
	}

	// If there's only one label, we can skip building and sorting a slice of keys.
	if len(r.Labels) == 1 {
		for k, v := range r.Labels {
			// We know this only executes once.
			s.Grow(len(r.Type) + len(k) + len(v) + 2)
			s.WriteString(r.Type)
			writeKV(k, v)
			return s.String()
		}
	}

	l := len(r.Type)
	keys := make([]string, 0, len(r.Labels))
	for k, v := range r.Labels {
		l += len(k) + len(v) + 2
		keys = append(keys, k)
	}
	s.Grow(l)

	s.WriteString(r.Type)
	sort.Strings(keys) // Go maps are unsorted, so sort by key to produce stable output.
	for _, key := range keys {
		writeKV(key, r.Labels[key])
	}

	return s.String()
}

func resourceFromKey(s string) *resource.Resource {
	if s == "" {
		return nil
	}
	r := &resource.Resource{Labels: map[string]string{}}
	parts := strings.Split(s, "\x01")
	r.Type = parts[0]
	for _, label := range parts[1:] {
		keyValue := strings.SplitN(label, "\x02", 2)
		r.Labels[keyValue[0]] = keyValue[1]
	}
	return r
}

func copyViews(views []*view.View) []*view.View {
	viewsCopy := make([]*view.View, 0, len(resourceViews.views))
	for _, v := range views {
		c := *v
		c.TagKeys = make([]tag.Key, len(v.TagKeys))
		copy(c.TagKeys, v.TagKeys)
		agg := *v.Aggregation
		c.Aggregation = &agg
		c.Aggregation.Buckets = make([]float64, len(v.Aggregation.Buckets))
		copy(c.Aggregation.Buckets, v.Aggregation.Buckets)
		viewsCopy = append(viewsCopy, &c)
	}
	return viewsCopy
}

// defaultMeterImpl is a pass-through to the default worker in OpenCensus.
type defaultMeterImpl struct{}

var _ view.Meter = (*defaultMeterImpl)(nil)

// defaultMeter is a custom meterExporter which passes through to the default
// worker in OpenCensus. This allows legacy code that uses OpenCensus and does
// not store a Resource in the context to continue to interoperate.
var defaultMeter = meterExporter{
	m: &defaultMeterImpl{},
	o: stats.WithRecorder(nil),
	t: time.Now(), // time.Now() here is ok - defaultMeter never expires, so this won't be checked
}

func (*defaultMeterImpl) Record(*tag.Map, interface{}, map[string]interface{}) {
	// using an empty option prevents this from being called
}

// Find calls view.Find
func (*defaultMeterImpl) Find(name string) *view.View {
	return view.Find(name)
}

// Register calls view.Register
func (*defaultMeterImpl) Register(views ...*view.View) error {
	return view.Register(views...)
}
func (*defaultMeterImpl) Unregister(views ...*view.View) {
	view.Unregister(views...)
}
func (*defaultMeterImpl) SetReportingPeriod(t time.Duration) {
	view.SetReportingPeriod(t)
}
func (*defaultMeterImpl) RegisterExporter(e view.Exporter) {
	view.RegisterExporter(e)
}
func (*defaultMeterImpl) UnregisterExporter(e view.Exporter) {
	view.UnregisterExporter(e)
}
func (*defaultMeterImpl) Start() {}
func (*defaultMeterImpl) Stop()  {}
func (*defaultMeterImpl) RetrieveData(viewName string) ([]*view.Row, error) {
	return view.RetrieveData(viewName)
}
func (*defaultMeterImpl) SetResource(*resource.Resource) {
}

// Read is implemented to support casting defaultMeterImpl to a metricproducer.Producer,
// but returns no values because the prometheus exporter (which is the only consumer)
// already has a built in path which collects these metrics via metricexport, which calls
// concat(x.Read() for x in metricproducer.GlobalManager.GetAll()).
func (*defaultMeterImpl) Read() []*metricdata.Metric {
	return []*metricdata.Metric{}
}
