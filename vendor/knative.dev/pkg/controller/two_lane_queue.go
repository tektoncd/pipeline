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

package controller

import (
	"time"

	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/clock"
)

// twoLaneQueue is a rate limited queue that wraps around two queues
// -- fast queue (anonymously aliased), whose contents are processed with priority.
// -- slow queue (slowLane queue), whose contents are processed if fast queue has no items.
// All the default methods operate on the fast queue, unless noted otherwise.
type twoLaneQueue struct {
	fastLane workqueue.TypedInterface[any]
	slowLane workqueue.TypedInterface[any]
	// consumerQueue is necessary to ensure that we're not reconciling
	// the same object at the exact same time (e.g. if it had been enqueued
	// in both fast and slow and is the only object there).
	consumerQueue workqueue.TypedInterface[any]

	name string

	fastChan chan any
	slowChan chan any

	metrics *queueMetrics
}

type twoLaneRateLimitingQueue struct {
	q *twoLaneQueue
	workqueue.TypedRateLimitingInterface[any]
}

var _ workqueue.TypedInterface[any] = (*twoLaneQueue)(nil)

// Creates a new twoLaneQueue.
func newTwoLaneWorkQueue(name string, rl workqueue.TypedRateLimiter[any]) *twoLaneRateLimitingQueue {
	mp := globalMetricsProvider

	tlq := &twoLaneQueue{
		name:          name,
		fastLane:      workqueue.NewTyped[any](),
		slowLane:      workqueue.NewTyped[any](),
		consumerQueue: workqueue.NewTyped[any](),
		fastChan:      make(chan any),
		slowChan:      make(chan any),
	}

	tlq.metrics = createMetrics(tlq, mp, name)

	// Run consumer thread.
	go tlq.runConsumer()
	// Run producer threads.
	go process(tlq.fastLane, tlq.fastChan)
	go process(tlq.slowLane, tlq.slowChan)

	q := &twoLaneRateLimitingQueue{
		q: tlq,
		TypedRateLimitingInterface: workqueue.NewTypedRateLimitingQueueWithConfig(
			rl,
			workqueue.TypedRateLimitingQueueConfig[any]{
				DelayingQueue: workqueue.NewTypedDelayingQueueWithConfig(
					workqueue.TypedDelayingQueueConfig[any]{
						Name:            name, // Name needs to be set for retry metrics
						Queue:           tlq,
						MetricsProvider: mp,
					},
				),
			},
		),
	}
	return q
}

func createMetrics(q *twoLaneQueue, mp workqueue.MetricsProvider, name string) *queueMetrics {
	if mp == noopProvider {
		return nil
	}

	m := &queueMetrics{
		clock:                   clock.RealClock{},
		depth:                   mp.NewDepthMetric(name),
		adds:                    mp.NewAddsMetric(name),
		latency:                 mp.NewLatencyMetric(name),
		workDuration:            mp.NewWorkDurationMetric(name),
		unfinishedWorkSeconds:   mp.NewUnfinishedWorkSecondsMetric(name),
		longestRunningProcessor: mp.NewUnfinishedWorkSecondsMetric(name),
		addTimes:                make(map[any]time.Time),
		processingStartTimes:    make(map[any]time.Time),
	}

	go updateUnfinishedWorkLoop(q)

	return m
}

func updateUnfinishedWorkLoop(q *twoLaneQueue) {
	t := time.NewTicker(time.Second)
	defer t.Stop()

	for range t.C {
		if q.ShuttingDown() {
			return
		}

		q.metrics.updateUnfinishedWork()
	}
}

func process(q workqueue.TypedInterface[any], ch chan any) {
	// Sender closes the channel
	defer close(ch)
	for {
		i, d := q.Get()
		// If the queue is empty and we're shutting down — stop the loop.
		if d {
			break
		}
		q.Done(i)
		ch <- i
	}
}

func (tlq *twoLaneQueue) runConsumer() {
	// Shutdown flags.
	fast, slow := true, true
	// When both producer queues are shutdown stop the consumerQueue.
	defer tlq.consumerQueue.ShutDown()
	// While any of the queues is still running, try to read off of them.
	for fast || slow {
		// By default drain the fast lane.
		// Channels in select are picked random, so first
		// we have a select that only looks at the fast lane queue.
		if fast {
			select {
			case item, ok := <-tlq.fastChan:
				if !ok {
					// This queue is shutdown and drained. Stop looking at it.
					fast = false
					continue
				}
				tlq.consumerQueue.Add(item)
				continue
			default:
				// This immediately exits the wait if the fast chan is empty.
			}
		}

		// If the fast lane queue had no items, we can select from both.
		// Obviously if suddenly both are populated at the same time there's a
		// 50% chance that the slow would be picked first, but this should be
		// a rare occasion not to really worry about it.
		select {
		case item, ok := <-tlq.fastChan:
			if !ok {
				// This queue is shutdown and drained. Stop looking at it.
				fast = false
				continue
			}
			tlq.consumerQueue.Add(item)
		case item, ok := <-tlq.slowChan:
			if !ok {
				// This queue is shutdown and drained. Stop looking at it.
				slow = false
				continue
			}
			tlq.consumerQueue.Add(item)
		}
	}
}

// Shutdown implements workqueue.Interface.
// Shutdown shuts down both queues.
func (tlq *twoLaneQueue) ShutDown() {
	tlq.fastLane.ShutDown()
	tlq.slowLane.ShutDown()
}

// Done implements workqueue.Interface.
// Done marks the item as completed in all the queues.
// NB: this will just re-enqueue the object on the queue that didn't originate the object.
func (tlq *twoLaneQueue) Done(item any) {
	tlq.consumerQueue.Done(item)
	tlq.metrics.done(item)
}

func (tlq *twoLaneQueue) Add(item any) {
	tlq.metrics.add(item)
	tlq.fastLane.Add(item)
}

func (q *twoLaneRateLimitingQueue) AddSlow(item any) {
	q.q.metrics.add(item)
	q.q.slowLane.Add(item)
}

func (q *twoLaneRateLimitingQueue) SlowLen() int {
	return q.q.slowLane.Len()
}

func (q *twoLaneRateLimitingQueue) slowLane() workqueue.TypedInterface[any] {
	return q.q.slowLane
}

// Get implements workqueue.Interface.
// It gets the item from fast lane if it has anything, alternatively
// the slow lane.
func (tlq *twoLaneQueue) Get() (any, bool) {
	item, ok := tlq.consumerQueue.Get()
	tlq.metrics.get(item)
	return item, ok
}

// Len returns the sum of lengths.
// NB: actual _number_ of unique object might be less than this sum.
func (tlq *twoLaneQueue) Len() int {
	return tlq.fastLane.Len() + tlq.slowLane.Len() + tlq.consumerQueue.Len()
}

func (tlq *twoLaneQueue) ShutDownWithDrain() {
	tlq.fastLane.ShutDownWithDrain()
	tlq.slowLane.ShutDownWithDrain()
}

func (tlq *twoLaneQueue) ShuttingDown() bool {
	return tlq.fastLane.ShuttingDown() || tlq.slowLane.ShuttingDown()
}
