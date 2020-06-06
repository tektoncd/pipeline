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

// The chaosduck binary is an e2e testing tool for leader election, which loads
// the leader election configuration within the system namespace and
// periodically kills one of the leader pods for each HA component.
package main

import (
	"context"
	"log"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/system"
)

// quack will periodically kill a leader shard of a component reconciler's leader set.
func quack(ctx context.Context, component string) error {
	log.Printf("Quacking at %q", component)

	kc := kubeclient.Get(ctx)
	for {
		select {
		case <-time.After(30 * time.Second):
			leases, err := kc.CoordinationV1().Leases(system.Namespace()).List(metav1.ListOptions{})
			if err != nil {
				return err
			}

			shards := make(map[string]sets.String, 1)
			for _, lease := range leases.Items {
				if lease.Spec.HolderIdentity == nil {
					log.Printf("Found lease %q held by nobody!", lease.Name)
					continue
				}
				pod := strings.Split(*lease.Spec.HolderIdentity, "_")[0]

				var reconciler string
				if lease.Name == component {
					// for back-compat
					reconciler = "all"
				} else if parts := strings.Split(lease.Name, "."); len(parts) < 3 || parts[0] != component {
					continue
				} else {
					// Shave off the prefix and the sharding
					reconciler = strings.Join(parts[1:len(parts)-1], ".")
				}

				set := shards[reconciler]
				if set == nil {
					set = sets.NewString()
				}
				set.Insert(pod)
				shards[reconciler] = set
			}

			killem := sets.NewString()

			for _, v := range shards {
				if v.HasAny(killem.UnsortedList()...) {
					// This is covered by killem
					continue
				}

				tribute, _ := v.PopAny()
				killem.Insert(tribute)
			}

			for _, name := range killem.List() {
				err := kc.CoreV1().Pods(system.Namespace()).Delete(name, &metav1.DeleteOptions{})
				if err != nil {
					return err
				}
			}

		case <-ctx.Done():
			return nil
		}
	}
}

func main() {
	ctx := signals.NewContext()

	ctx, informers := injection.Default.SetupInformers(ctx, sharedmain.ParseAndGetConfigOrDie())
	if err := controller.StartInformers(ctx.Done(), informers...); err != nil {
		log.Fatalf("Failed to start informers %v", err)
	}

	lec, err := sharedmain.GetLeaderElectionConfig(ctx)
	if err != nil {
		log.Fatalf("Unable to load leader election configuration: %v", err)
	}
	if lec.ResourceLock != "leases" {
		log.Fatalf("chaosduck only supports leases, but got %q", lec.ResourceLock)
	}

	eg, ctx := errgroup.WithContext(ctx)
	for _, x := range lec.EnabledComponents.List() {
		x := x
		eg.Go(func() error {
			return quack(ctx, x)
		})
	}

	if err := eg.Wait(); err != nil {
		log.Fatalf("Ended with err: %v", err)
	}
}
