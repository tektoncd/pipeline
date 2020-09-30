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
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"knative.dev/pkg/hash"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/network"
	"knative.dev/pkg/reconciler"
	"knative.dev/pkg/system"
)

// WithDynamicLeaderElectorBuilder sets up the statefulset elector based on environment,
// falling back on the standard elector.
func WithDynamicLeaderElectorBuilder(ctx context.Context, kc kubernetes.Interface, cc ComponentConfig) context.Context {
	logger := logging.FromContext(ctx)
	b, _, err := NewStatefulSetBucketAndSet(int(cc.Buckets))
	if err == nil {
		logger.Info("Running with StatefulSet leader election")
		return withStatefulSetElectorBuilder(ctx, cc, b)
	}
	logger.Info("Running with Standard leader election")
	return WithStandardLeaderElectorBuilder(ctx, kc, cc)
}

// WithStandardLeaderElectorBuilder infuses a context with the ability to build
// LeaderElectors with the provided component configuration acquiring resource
// locks via the provided kubernetes client.
func WithStandardLeaderElectorBuilder(ctx context.Context, kc kubernetes.Interface, cc ComponentConfig) context.Context {
	return context.WithValue(ctx, builderKey{}, &standardBuilder{
		kc:  kc,
		lec: cc,
	})
}

// withStatefulSetElectorBuilder infuses a context with the ability to build
// Electors which are assigned leadership based on the StatefulSet ordinal from
// the provided component configuration.
func withStatefulSetElectorBuilder(ctx context.Context, cc ComponentConfig, bkt reconciler.Bucket) context.Context {
	return context.WithValue(ctx, builderKey{}, &statefulSetBuilder{
		lec: cc,
		bkt: bkt,
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
// reconciler using a builder added to the context via WithStandardLeaderElectorBuilder.
func BuildElector(ctx context.Context, la reconciler.LeaderAware, queueName string, enq func(reconciler.Bucket, types.NamespacedName)) (Elector, error) {
	if val := ctx.Value(builderKey{}); val != nil {
		switch builder := val.(type) {
		case *standardBuilder:
			return builder.buildElector(ctx, la, queueName, enq)
		case *statefulSetBuilder:
			return builder.buildElector(ctx, la, enq)
		}
	}

	return &unopposedElector{
		la:  la,
		bkt: reconciler.UniversalBucket(),
		enq: enq,
	}, nil
}

type builderKey struct{}

type standardBuilder struct {
	kc  kubernetes.Interface
	lec ComponentConfig
}

func (b *standardBuilder) buildElector(ctx context.Context, la reconciler.LeaderAware,
	queueName string, enq func(reconciler.Bucket, types.NamespacedName)) (Elector, error) {
	logger := logging.FromContext(ctx)

	id := b.lec.Identity
	if id == "" {
		uid, err := UniqueID()
		if err != nil {
			return nil, err
		}
		id = uid
	}

	bkts := newStandardBuckets(queueName, b.lec)
	electors := make([]Elector, 0, b.lec.Buckets)
	for _, bkt := range bkts {
		// Use a local var which won't change across the for loop since it is
		// used in a callback asynchronously.
		bkt := bkt
		rl, err := resourcelock.New(KnativeResourceLock,
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
					if err := la.Promote(bkt, enq); err != nil {
						// TODO(mattmoor): We expect this to effectively never happen,
						// but if it does, we should support wrapping `le` in an elector
						// we can cancel here.
						logger.Fatalf("%q failed to Promote: %v", rl.Identity(), err)
					}
				},
				OnStoppedLeading: func() {
					logger.Infof("%q has stopped leading %q", rl.Identity(), bkt.Name())
					la.Demote(bkt)
				},
			},
			ReleaseOnCancel: true,
			Name:            rl.Identity(),
		})
		if err != nil {
			return nil, err
		}
		// TODO: use health check watchdog, knative/pkg#1048
		// if lec.WatchDog != nil {
		// 	lec.WatchDog.SetLeaderElection(le)
		// }
		electors = append(electors, &runUntilCancelled{Elector: le})
	}
	return &runAll{les: electors}, nil
}

func newStandardBuckets(queueName string, cc ComponentConfig) []reconciler.Bucket {
	ln := cc.LeaseName
	if ln == nil {
		ln = func(i uint32) string {
			return standardBucketName(i, queueName, cc)
		}
	}
	names := make(sets.String, cc.Buckets)
	for i := uint32(0); i < cc.Buckets; i++ {
		names.Insert(ln(i))
	}

	return hash.NewBucketSet(names).Buckets()
}

func standardBucketName(ordinal uint32, queueName string, cc ComponentConfig) string {
	return strings.ToLower(fmt.Sprintf("%s.%s.%02d-of-%02d", cc.Component, queueName, ordinal, cc.Buckets))
}

type statefulSetBuilder struct {
	lec ComponentConfig
	bkt reconciler.Bucket
}

func (b *statefulSetBuilder) buildElector(ctx context.Context, la reconciler.LeaderAware, enq func(reconciler.Bucket, types.NamespacedName)) (Elector, error) {
	logger := logging.FromContext(ctx)
	logger.Infof("%s will run in StatefulSet ordinal assignement mode with bucket name %s",
		b.lec.Component, b.bkt.Name())

	return &unopposedElector{
		bkt: b.bkt,
		la:  la,
		enq: enq,
	}, nil
}

// NewStatefulSetBucketAndSet creates a BucketSet for StatefulSet controller with
// the given bucket size and the information from environment variables. Then uses
// the created BucketSet to create a Bucket for this StatefulSet Pod.
func NewStatefulSetBucketAndSet(buckets int) (reconciler.Bucket, *hash.BucketSet, error) {
	ssc, err := newStatefulSetConfig()
	if err != nil {
		return nil, nil, err
	}

	if ssc.StatefulSetID.ordinal >= buckets {
		return nil, nil, fmt.Errorf("ordinal %d is out of range [0, %d)",
			ssc.StatefulSetID.ordinal, buckets)
	}

	names := make(sets.String, buckets)
	for i := 0; i < buckets; i++ {
		names.Insert(statefulSetPodDNS(i, ssc))
	}

	bs := hash.NewBucketSet(names)
	// Buckets is sorted in order of names so we can use ordinal to
	// get the correct Bucket for this binary.
	return bs.Buckets()[ssc.StatefulSetID.ordinal], bs, nil
}

func statefulSetPodDNS(ordinal int, ssc *statefulSetConfig) string {
	return fmt.Sprintf("%s://%s-%d.%s.%s.svc.%s:%s", ssc.Protocol,
		ssc.StatefulSetID.ssName, ordinal, ssc.ServiceName,
		system.Namespace(), network.GetClusterDomainName(), ssc.Port)
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
	// Elector is a single-term elector as we get from K8s leaderelection package.
	Elector
}

// Run implements Elector
func (ruc *runUntilCancelled) Run(ctx context.Context) {
	// Turn the single-term elector into a continuous election cycle.
	for {
		ruc.Elector.Run(ctx)
		select {
		case <-ctx.Done():
			return // Run quit because context was cancelled, we are done!
		default:
			// Context wasn't cancelled, start over.
		}
	}
}
