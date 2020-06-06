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

package leaderelection

import (
	"context"
	"fmt"
	"hash/fnv"
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"
	"knative.dev/pkg/system"
)

// WithLeaderElectorBuilder infuses a context with the ability to build
// LeaderElectors with the provided component configuration acquiring resource
// locks via the provided kubernetes client.
func WithLeaderElectorBuilder(ctx context.Context, kc kubernetes.Interface, cc ComponentConfig) context.Context {
	return context.WithValue(ctx, builderKey{}, &standardBuilder{
		kc:  kc,
		lec: cc,
	})
}

// HasLeaderElection returns whether there is leader election configuration
// associated with the context
func HasLeaderElection(ctx context.Context) bool {
	val := ctx.Value(builderKey{})
	return val != nil
}

// Elector is the interface for running a leader elector.
type Elector interface {
	Run(context.Context)
}

// BuildElector builds a leaderelection.LeaderElector for the named LeaderAware
// reconciler using a builder added to the context via WithLeaderElectorBuilder.
func BuildElector(ctx context.Context, la reconciler.LeaderAware, name string, enq func(reconciler.Bucket, types.NamespacedName)) (Elector, error) {
	if val := ctx.Value(builderKey{}); val != nil {
		switch builder := val.(type) {
		case *standardBuilder:
			return builder.BuildElector(ctx, la, name, enq)
		}
		// TODO(mattmoor): Add a flavor of builder that relies on StatefulSet to partition the key space.
	}

	return &unopposedElector{
		la:  la,
		bkt: reconciler.AllBuckets(),
		enq: enq,
	}, nil
}

type builderKey struct{}

type standardBuilder struct {
	kc  kubernetes.Interface
	lec ComponentConfig
}

func (b *standardBuilder) BuildElector(ctx context.Context, la reconciler.LeaderAware, name string, enq func(reconciler.Bucket, types.NamespacedName)) (Elector, error) {
	logger := logging.FromContext(ctx)

	id, err := UniqueID()
	if err != nil {
		return nil, err
	}

	buckets := make([]Elector, 0, b.lec.Buckets)
	for i := uint32(0); i < b.lec.Buckets; i++ {
		bkt := &bucket{
			component: b.lec.Component,
			name:      name,
			index:     i,
			total:     b.lec.Buckets,
		}

		rl, err := resourcelock.New(b.lec.ResourceLock,
			system.Namespace(), // use namespace we are running in
			bkt.Name(),
			b.kc.CoreV1(),
			b.kc.CoordinationV1(),
			resourcelock.ResourceLockConfig{
				Identity: id,
			})
		if err != nil {
			return nil, err
		}
		logger.Infof("%s will run in leader-elected mode with id %q", bkt.Name(), rl.Identity())

		le, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
			Lock:          rl,
			LeaseDuration: b.lec.LeaseDuration,
			RenewDeadline: b.lec.RenewDeadline,
			RetryPeriod:   b.lec.RetryPeriod,
			Callbacks: leaderelection.LeaderCallbacks{
				OnStartedLeading: func(context.Context) {
					logger.Infof("%q has started leading %q", rl.Identity(), bkt.Name())
					la.Promote(bkt, enq)
				},
				OnStoppedLeading: func() {
					logger.Infof("%q has stopped leading %q", rl.Identity(), bkt.Name())
					la.Demote(bkt)
				},
			},
			ReleaseOnCancel: true,

			// TODO: use health check watchdog, knative/pkg#1048
			Name: b.lec.Component,
		})
		if err != nil {
			return nil, err
		}
		// TODO: use health check watchdog, knative/pkg#1048
		// if lec.WatchDog != nil {
		// 	lec.WatchDog.SetLeaderElection(le)
		// }
		buckets = append(buckets, &runUntilCancelled{le: le})
	}
	return &runAll{les: buckets}, nil
}

// unopposedElector promotes when run without needing to be elected.
type unopposedElector struct {
	bkt reconciler.Bucket
	la  reconciler.LeaderAware
	enq func(reconciler.Bucket, types.NamespacedName)
}

// Run implements Elector
func (ue *unopposedElector) Run(ctx context.Context) {
	ue.la.Promote(ue.bkt, ue.enq)
}

type runAll struct {
	les []Elector
}

// Run implements Elector
func (ra *runAll) Run(ctx context.Context) {
	sg := sync.WaitGroup{}
	defer sg.Wait()

	for _, le := range ra.les {
		sg.Add(1)
		go func(le Elector) {
			defer sg.Done()
			le.Run(ctx)
		}(le)
	}
}

// runUntilCancelled wraps a single-term Elector into one that runs until
// the passed context is cancelled.
type runUntilCancelled struct {
	le Elector
}

// Run implements Elector
func (ruc *runUntilCancelled) Run(ctx context.Context) {
	// Run executes a single iteration of an election:
	// 1. Wait to become leader
	// 2. Call OnStartedLeading
	// 3. Wait for ctx cancel or loss of leader.
	// 4. Call OnStoppedLeading
	//
	// We effectively want this in a loop, where the termination
	// condition is cancellation of our context.
	for {
		ruc.le.Run(ctx)
		select {
		case <-ctx.Done():
			return // Run quit because context was cancelled, we are done!
		default:
			// Context wasn't cancelled, start over.
		}
	}
}

type bucket struct {
	component string
	name      string

	// We are bucket {index} of {total}
	index uint32
	total uint32
}

var _ reconciler.Bucket = (*bucket)(nil)

// Name implements reconciler.Bucket
func (b *bucket) Name() string {
	// The resource name is the lowercase:
	//   {component}.{workqueue}.{index}-of-{total}
	return strings.ToLower(fmt.Sprintf("%s.%s.%02d-of-%02d", b.component, b.name, b.index, b.total))
}

// Has implements reconciler.Bucket
func (b *bucket) Has(nn types.NamespacedName) bool {
	h := fnv.New32a()
	h.Write([]byte(nn.Namespace + "." + nn.Name))
	ii := h.Sum32() % b.total
	return b.index == ii
}
