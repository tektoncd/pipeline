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
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"k8s.io/client-go/tools/cache"
)

// ReflectorProvider implements reflector.MetricsProvider and may be used with
// reflector.SetProvider to have metrics exported to the provided metrics.
type ReflectorProvider struct {
	ItemsInList *stats.Float64Measure
	// TODO(mattmoor): This is not in the latest version, so it will
	// be removed in a future version.
	ItemsInMatch        *stats.Float64Measure
	ItemsInWatch        *stats.Float64Measure
	LastResourceVersion *stats.Float64Measure
	ListDuration        *stats.Float64Measure
	Lists               *stats.Int64Measure
	ShortWatches        *stats.Int64Measure
	WatchDuration       *stats.Float64Measure
	Watches             *stats.Int64Measure
}

var (
	_ cache.MetricsProvider = (*ReflectorProvider)(nil)
)

// NewItemsInListMetric implements MetricsProvider
func (rp *ReflectorProvider) NewItemsInListMetric(name string) cache.SummaryMetric {
	return floatMetric{
		mutators: []tag.Mutator{tag.Insert(tagName, name)},
		measure:  rp.ItemsInList,
	}
}

// ItemsInListView returns a view of the ItemsInList metric.
func (rp *ReflectorProvider) ItemsInListView() *view.View {
	return measureView(rp.ItemsInList, view.Distribution(BucketsNBy10(0.1, 6)...))
}

// NewItemsInMatchMetric implements MetricsProvider
func (rp *ReflectorProvider) NewItemsInMatchMetric(name string) cache.SummaryMetric {
	return floatMetric{
		mutators: []tag.Mutator{tag.Insert(tagName, name)},
		measure:  rp.ItemsInMatch,
	}
}

// ItemsInMatchView returns a view of the ItemsInMatch metric.
func (rp *ReflectorProvider) ItemsInMatchView() *view.View {
	return measureView(rp.ItemsInMatch, view.Distribution(BucketsNBy10(0.1, 6)...))
}

// NewItemsInWatchMetric implements MetricsProvider
func (rp *ReflectorProvider) NewItemsInWatchMetric(name string) cache.SummaryMetric {
	return floatMetric{
		mutators: []tag.Mutator{tag.Insert(tagName, name)},
		measure:  rp.ItemsInWatch,
	}
}

// ItemsInWatchView returns a view of the ItemsInWatch metric.
func (rp *ReflectorProvider) ItemsInWatchView() *view.View {
	return measureView(rp.ItemsInWatch, view.Distribution(BucketsNBy10(0.1, 6)...))
}

// NewLastResourceVersionMetric implements MetricsProvider
func (rp *ReflectorProvider) NewLastResourceVersionMetric(name string) cache.GaugeMetric {
	return floatMetric{
		mutators: []tag.Mutator{tag.Insert(tagName, name)},
		measure:  rp.LastResourceVersion,
	}
}

// LastResourceVersionView returns a view of the LastResourceVersion metric.
func (rp *ReflectorProvider) LastResourceVersionView() *view.View {
	return measureView(rp.LastResourceVersion, view.LastValue())
}

// NewListDurationMetric implements MetricsProvider
func (rp *ReflectorProvider) NewListDurationMetric(name string) cache.SummaryMetric {
	return floatMetric{
		mutators: []tag.Mutator{tag.Insert(tagName, name)},
		measure:  rp.ListDuration,
	}
}

// ListDurationView returns a view of the ListDuration metric.
func (rp *ReflectorProvider) ListDurationView() *view.View {
	return measureView(rp.ListDuration, view.Distribution(BucketsNBy10(0.1, 6)...))
}

// NewListsMetric implements MetricsProvider
func (rp *ReflectorProvider) NewListsMetric(name string) cache.CounterMetric {
	return counterMetric{
		mutators: []tag.Mutator{tag.Insert(tagName, name)},
		measure:  rp.Lists,
	}
}

// ListsView returns a view of the Lists metric.
func (rp *ReflectorProvider) ListsView() *view.View {
	return measureView(rp.Lists, view.Count())
}

// NewShortWatchesMetric implements MetricsProvider
func (rp *ReflectorProvider) NewShortWatchesMetric(name string) cache.CounterMetric {
	return counterMetric{
		mutators: []tag.Mutator{tag.Insert(tagName, name)},
		measure:  rp.ShortWatches,
	}
}

// ShortWatchesView returns a view of the ShortWatches metric.
func (rp *ReflectorProvider) ShortWatchesView() *view.View {
	return measureView(rp.ShortWatches, view.Count())
}

// NewWatchDurationMetric implements MetricsProvider
func (rp *ReflectorProvider) NewWatchDurationMetric(name string) cache.SummaryMetric {
	return floatMetric{
		mutators: []tag.Mutator{tag.Insert(tagName, name)},
		measure:  rp.WatchDuration,
	}
}

// WatchDurationView returns a view of the WatchDuration metric.
func (rp *ReflectorProvider) WatchDurationView() *view.View {
	return measureView(rp.WatchDuration, view.Distribution(BucketsNBy10(0.1, 6)...))
}

// NewWatchesMetric implements MetricsProvider
func (rp *ReflectorProvider) NewWatchesMetric(name string) cache.CounterMetric {
	return counterMetric{
		mutators: []tag.Mutator{tag.Insert(tagName, name)},
		measure:  rp.Watches,
	}
}

// WatchesView returns a view of the Watches metric.
func (rp *ReflectorProvider) WatchesView() *view.View {
	return measureView(rp.Watches, view.Count())
}

// DefaultViews returns a list of views suitable for passing to view.Register
func (rp *ReflectorProvider) DefaultViews() []*view.View {
	return []*view.View{
		rp.ItemsInListView(),
		rp.ItemsInMatchView(),
		rp.ItemsInWatchView(),
		rp.LastResourceVersionView(),
		rp.ListDurationView(),
		rp.ListsView(),
		rp.ShortWatchesView(),
		rp.WatchDurationView(),
		rp.WatchesView(),
	}
}
