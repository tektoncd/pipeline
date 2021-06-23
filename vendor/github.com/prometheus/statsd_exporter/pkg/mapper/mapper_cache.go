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

type CacheMetrics struct {
	CacheLength    prometheus.Gauge
	CacheGetsTotal prometheus.Counter
	CacheHitsTotal prometheus.Counter
}

func NewCacheMetrics(reg prometheus.Registerer) *CacheMetrics {
	var m CacheMetrics

	m.CacheLength = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "statsd_metric_mapper_cache_length",
			Help: "The count of unique metrics currently cached.",
		},
	)
	m.CacheGetsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "statsd_metric_mapper_cache_gets_total",
			Help: "The count of total metric cache gets.",
		},
	)
	m.CacheHitsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "statsd_metric_mapper_cache_hits_total",
			Help: "The count of total metric cache hits.",
		},
	)

	if reg != nil {
		reg.MustRegister(m.CacheLength)
		reg.MustRegister(m.CacheGetsTotal)
		reg.MustRegister(m.CacheHitsTotal)
	}
	return &m
}

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
	cache   *lru.Cache
	metrics *CacheMetrics
}

type MetricMapperNoopCache struct {
	MetricMapperCache
	metrics *CacheMetrics
}

func NewMetricMapperCache(reg prometheus.Registerer, size int) (*MetricMapperLRUCache, error) {
	metrics := NewCacheMetrics(reg)
	cache, err := lru.New(size)
	if err != nil {
		return &MetricMapperLRUCache{}, err
	}
	return &MetricMapperLRUCache{metrics: metrics, cache: cache}, nil
}

func (m *MetricMapperLRUCache) Get(metricString string, metricType MetricType) (*MetricMapperCacheResult, bool) {
	m.metrics.CacheGetsTotal.Inc()
	if result, ok := m.cache.Get(formatKey(metricString, metricType)); ok {
		m.metrics.CacheHitsTotal.Inc()
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
	m.metrics.CacheLength.Set(float64(m.cache.Len()))
}

func formatKey(metricString string, metricType MetricType) string {
	return string(metricType) + "." + metricString
}

func NewMetricMapperNoopCache(reg prometheus.Registerer) *MetricMapperNoopCache {
	return &MetricMapperNoopCache{metrics: NewCacheMetrics(reg)}
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
	lock    sync.RWMutex
	size    int
	items   map[string]*MetricMapperCacheResult
	metrics *CacheMetrics
}

func NewMetricMapperRRCache(reg prometheus.Registerer, size int) (*MetricMapperRRCache, error) {
	metrics := NewCacheMetrics(reg)
	c := &MetricMapperRRCache{
		items:   make(map[string]*MetricMapperCacheResult, size+1),
		size:    size,
		metrics: metrics,
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
	m.metrics.CacheLength.Set(float64(length))
}
