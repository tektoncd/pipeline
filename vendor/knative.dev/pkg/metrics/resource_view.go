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
)

type storedViews struct {
	views []*view.View
	lock  sync.Mutex
}

type meterExporter struct {
	m view.Meter    // NOTE: DO NOT RETURN THIS DIRECTLY; the view.Meter will not work for the empty Resource
	o stats.Options // Cache the option to reduce allocations
	e view.Exporter
}

// ResourceExporterFactory provides a hook for producing separate view.Exporters
// for each observed Resource. This is needed because OpenCensus support for
// Resources is a bit tacked-on rather than being a first-class component like
// Tags are.
type ResourceExporterFactory func(*resource.Resource) (view.Exporter, error)
type meters struct {
	meters  map[string]*meterExporter
	factory ResourceExporterFactory
	lock    sync.Mutex
}

// Lock regime: lock allMeters before resourceViews. The critical path is in
// optionForResource, which must lock allMeters, but only needs to lock
// resourceViews if a new meter needs to be created.
var resourceViews = storedViews{}
var allMeters = meters{
	meters: map[string]*meterExporter{"": &defaultMeter},
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
		viewCopy := make([]*view.View, 0, len(views))
		for _, v := range views {
			c := *v
			viewCopy = append(viewCopy, &c)
		}
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

// UnregisterResourceView is similar to view.Unregiste(), except that it will
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
// including cleaning up background threads.
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
}

func meterExporterForResource(r *resource.Resource) *meterExporter {
	key := resourceToKey(r)
	mE := allMeters.meters[key]
	if mE == nil {
		mE = &meterExporter{}
		allMeters.meters[key] = mE
	}
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
	viewsCopy := make([]*view.View, 0, len(resourceViews.views))
	for _, v := range resourceViews.views {
		c := *v
		viewsCopy = append(viewsCopy, &c)
	}
	mE.m.Register(viewsCopy...)
	mE.o = stats.WithRecorder(mE.m)
	allMeters.meters[key] = mE
	return mE
}

// optionForResource finds or creates a stats exporter for the resource, and
// returns a stats.Option indicating which meter to record to.
func optionForResource(r *resource.Resource) (stats.Options, error) {
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
	var s strings.Builder
	l := len(r.Type)
	kvs := make([]string, 0, len(r.Labels))
	for k, v := range r.Labels {
		l += len(k) + len(v) + 2
		// We use byte values 1 and 2 to avoid colliding with valid resource labels
		// and to make unpacking easy
		kvs = append(kvs, fmt.Sprintf("\x01%s\x02%s", k, v))
	}
	s.Grow(l)
	s.WriteString(r.Type)

	sort.Strings(kvs) // Go maps are unsorted, so sort by key to produce stable output.
	for _, kv := range kvs {
		s.WriteString(kv)
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

// defaultMeterImpl is a pass-through to the default worker in OpenCensus.
type defaultMeterImpl struct{}

var _ view.Meter = (*defaultMeterImpl)(nil)

// defaultMeter is a custom meterExporter which passes through to the default
// worker in OpenCensus. This allows legacy code that uses OpenCensus and does
// not store a Resource in the context to continue to interoperate.
var defaultMeter = meterExporter{
	m: &defaultMeterImpl{},
	o: stats.WithRecorder(nil),
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
