/*
Copyright 2020 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import "knative.dev/pkg/reconciler"

// Options is additional resources a Controller might want to use depending
// on implementation.
type Options struct {
	// ConfigStore is used to attach the frozen configuration to the context.
	ConfigStore reconciler.ConfigStore

	// FinalizerName is the name of the finalizer this reconciler uses. This
	// overrides a default finalizer name assigned by the generator if needed.
	FinalizerName string

	// AgentName is the name of the agent this reconciler uses. This overrides
	// the default controller's agent name.
	AgentName string

	// SkipStatusUpdates configures this reconciler to either do automated status
	// updates (default) or skip them if this is set to true.
	SkipStatusUpdates bool

	// DemoteFunc configures the demote function this reconciler uses
	DemoteFunc func(b reconciler.Bucket)

	// Concurrency - The number of workers to use when processing the controller's workqueue.
	Concurrency int

	// PromoteFilterFunc filters the objects that are enqueued when the reconciler is promoted to leader.
	// Objects that pass the filter (return true) will be reconciled when a new leader is promoted.
	// If no filter is specified, all objects will be reconciled.
	PromoteFilterFunc func(obj interface{}) bool

	// PromoteFunc is called when a reconciler is promoted for the given bucket
	// The provided function must not block execution.
	PromoteFunc func(bkt reconciler.Bucket)
}

// OptionsFn is a callback method signature that accepts an Impl and returns
// Options. Used for controllers that need access to the members of Options but
// to build Options, integrators need an Impl.
type OptionsFn func(impl *Impl) Options
