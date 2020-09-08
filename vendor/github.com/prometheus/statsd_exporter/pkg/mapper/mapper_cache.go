// Copyright 2019 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mapper

import (
	"sync"

	lru "github.com/hashicorp/golang-lru"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	cacheLength = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "statsd_metric_mapper_cache_length",
			Help: "The count of unique metrics currently cached.",
		},
	)
	cacheGetsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "statsd_metric_mapper_cache_gets_total",
			Help: "The count of total metric cache gets.",
		},
	)
	cacheHitsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "statsd_metric_mapper_cache_hits_total",
			Help: "The count of total metric cache hits.",
		},
	)
)

type cacheOptions struct {
	cacheType string
}

type CacheOption func(*cacheOptions)

func WithCacheType(cacheType string) CacheOption {
	return func(o *cacheOptions) {
		o.cacheType = cacheType
	}
}

type MetricMapperCacheResult struct {
	Mapping *MetricMapping
	Matched bool
	Labels  prometheus.Labels
}

type MetricMapperCache interface {
	Get(metricString string, metricType MetricType) (*MetricMapperCacheResult, bool)
	AddMatch(metricString string, metricType MetricType, mapping *MetricMapping, labels prometheus.Labels)
	AddMiss(metricString string, metricType MetricType)
}

type MetricMapperLRUCache struct {
	MetricMapperCache
	cache *lru.Cache
}

type MetricMapperNoopCache struct {
	MetricMapperCache
}

func NewMetricMapperCache(size int) (*MetricMapperLRUCache, error) {
	cacheLength.Set(0)
	cache, err := lru.New(size)
	if err != nil {
		return &MetricMapperLRUCache{}, err
	}
	return &MetricMapperLRUCache{cache: cache}, nil
}

func (m *MetricMapperLRUCache) Get(metricString string, metricType MetricType) (*MetricMapperCacheResult, bool) {
	cacheGetsTotal.Inc()
	if result, ok := m.cache.Get(formatKey(metricString, metricType)); ok {
		cacheHitsTotal.Inc()
		return result.(*MetricMapperCacheResult), true
	} else {
		return nil, false
	}
}

func (m *MetricMapperLRUCache) AddMatch(metricString string, metricType MetricType, mapping *MetricMapping, labels prometheus.Labels) {
	go m.trackCacheLength()
	m.cache.Add(formatKey(metricString, metricType), &MetricMapperCacheResult{Mapping: mapping, Matched: true, Labels: labels})
}

func (m *MetricMapperLRUCache) AddMiss(metricString string, metricType MetricType) {
	go m.trackCacheLength()
	m.cache.Add(formatKey(metricString, metricType), &MetricMapperCacheResult{Matched: false})
}

func (m *MetricMapperLRUCache) trackCacheLength() {
	cacheLength.Set(float64(m.cache.Len()))
}

func formatKey(metricString string, metricType MetricType) string {
	return string(metricType) + "." + metricString
}

func NewMetricMapperNoopCache() *MetricMapperNoopCache {
	cacheLength.Set(0)
	return &MetricMapperNoopCache{}
}

func (m *MetricMapperNoopCache) Get(metricString string, metricType MetricType) (*MetricMapperCacheResult, bool) {
	return nil, false
}

func (m *MetricMapperNoopCache) AddMatch(metricString string, metricType MetricType, mapping *MetricMapping, labels prometheus.Labels) {
	return
}

func (m *MetricMapperNoopCache) AddMiss(metricString string, metricType MetricType) {
	return
}

type MetricMapperRRCache struct {
	MetricMapperCache
	lock  sync.RWMutex
	size  int
	items map[string]*MetricMapperCacheResult
}

func NewMetricMapperRRCache(size int) (*MetricMapperRRCache, error) {
	cacheLength.Set(0)
	c := &MetricMapperRRCache{
		items: make(map[string]*MetricMapperCacheResult, size+1),
		size:  size,
	}
	return c, nil
}

func (m *MetricMapperRRCache) Get(metricString string, metricType MetricType) (*MetricMapperCacheResult, bool) {
	key := formatKey(metricString, metricType)

	m.lock.RLock()
	result, ok := m.items[key]
	m.lock.RUnlock()

	return result, ok
}

func (m *MetricMapperRRCache) addItem(metricString string, metricType MetricType, result *MetricMapperCacheResult) {
	go m.trackCacheLength()

	key := formatKey(metricString, metricType)

	m.lock.Lock()

	m.items[key] = result

	// evict an item if needed
	if len(m.items) > m.size {
		for k := range m.items {
			delete(m.items, k)
			break
		}
	}

	m.lock.Unlock()
}

func (m *MetricMapperRRCache) AddMatch(metricString string, metricType MetricType, mapping *MetricMapping, labels prometheus.Labels) {
	e := &MetricMapperCacheResult{Mapping: mapping, Matched: true, Labels: labels}
	m.addItem(metricString, metricType, e)
}

func (m *MetricMapperRRCache) AddMiss(metricString string, metricType MetricType) {
	e := &MetricMapperCacheResult{Matched: false}
	m.addItem(metricString, metricType, e)
}

func (m *MetricMapperRRCache) trackCacheLength() {
	m.lock.RLock()
	length := len(m.items)
	m.lock.RUnlock()
	cacheLength.Set(float64(length))
}

func init() {
	prometheus.MustRegister(cacheLength)
	prometheus.MustRegister(cacheGetsTotal)
	prometheus.MustRegister(cacheHitsTotal)
}
