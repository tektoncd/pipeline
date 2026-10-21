/*
Copyright 2022 The Tekton Authors

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

package cache_test

import (
	"context"
	"fmt"
	"hash/fnv"
	"net/url"
	"strconv"
	"testing"
	"time"

	bc "github.com/allegro/bigcache/v3"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/event"
	cetypes "github.com/cloudevents/sdk-go/v2/types"
	"github.com/google/go-cmp/cmp"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	cache "github.com/tektoncd/pipeline/pkg/reconciler/events/cache"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/parse"
	corev1 "k8s.io/api/core/v1"
)

const (
	META_TEMPLATE = `
metadata:
  name: %s
  namespace: %s`
	CONDITION_BASE = `
status:
  conditions:
  - lastTransitionTime: "2023-07-04T09:40:03Z"
    type: Succeeded`
	STATUS_TEMPLATE = `
    status: "%s"`
	REASON_TEMPLATE = `
    reason: %s`
	MESSAGE_TEMPLATE = `
    message: %s`
)

func strptr(s string) *string { return &s }

// hash provide fnv64 hash converted to string in base36
func hash(input string) string {
	hasher := fnv.New64a()
	_, err := hasher.Write([]byte(input))
	if err != nil {
		return ""
	}
	return strconv.FormatUint(hasher.Sum64(), 36)
}

func getEventData(t *testing.T, run interface{}) map[string]interface{} {
	t.Helper()
	cloudEventData := map[string]interface{}{}
	if v, ok := run.(*v1beta1.CustomRun); ok {
		cloudEventData["customRun"] = v
	}
	return cloudEventData
}

func getEventToTest(t *testing.T, eventtype string, run interface{}) *event.Event {
	t.Helper()
	e := event.Event{
		Context: event.EventContextV1{
			Type:    eventtype,
			Source:  cetypes.URIRef{URL: url.URL{Path: "/foo/bar/source"}},
			ID:      "test-event",
			Time:    &cetypes.Timestamp{Time: time.Now()},
			Subject: strptr("topic"),
		}.AsV1(),
	}
	if err := e.SetData(cloudevents.ApplicationJSON, getEventData(t, run)); err != nil {
		panic(err)
	}
	return &e
}

func getRunYamlByMeta(t *testing.T, name, namespace, message, reason, status string) string {
	t.Helper()
	yaml := fmt.Sprintf(META_TEMPLATE, name, namespace)
	// If the status is not set, ignore the rest
	if status != "" {
		yaml += CONDITION_BASE
		yaml += fmt.Sprintf(STATUS_TEMPLATE, status)
		if message != "" {
			yaml += fmt.Sprintf(MESSAGE_TEMPLATE, message)
		}
		if reason != "" {
			yaml += fmt.Sprintf(REASON_TEMPLATE, reason)
		}
	}
	return yaml
}

func getCustomRunByMeta(t *testing.T, name, namespace, message, reason, status string) *v1beta1.CustomRun {
	t.Helper()
	return parse.MustParseCustomRun(t,
		getRunYamlByMeta(t, name, namespace, message, reason, status),
	)
}

func getTaskRunByMeta(t *testing.T, name, namespace, message, reason, status string) *v1.TaskRun {
	t.Helper()
	return parse.MustParseV1TaskRun(t,
		getRunYamlByMeta(t, name, namespace, message, reason, status),
	)
}

func getTaskRunBetaByMeta(t *testing.T, name, namespace, message, reason, status string) *v1beta1.TaskRun {
	t.Helper()
	return parse.MustParseV1beta1TaskRun(t,
		getRunYamlByMeta(t, name, namespace, message, reason, status),
	)
}

func getPipelineRunByMeta(t *testing.T, name, namespace, message, reason, status string) *v1.PipelineRun {
	t.Helper()
	return parse.MustParseV1PipelineRun(t,
		getRunYamlByMeta(t, name, namespace, message, reason, status),
	)
}

func getPipelineRunBetaByMeta(t *testing.T, name, namespace, message, reason, status string) *v1beta1.PipelineRun {
	t.Helper()
	return parse.MustParseV1beta1PipelineRun(t,
		getRunYamlByMeta(t, name, namespace, message, reason, status),
	)
}

// TestEventsKey verifies that keys are extracted correctly from events and runs
func TestEventsKey(t *testing.T) {
	testcases := []struct {
		name      string
		eventtype string
		run       v1beta1.RunObject
		wantKey   string
		wantErr   bool
	}{{
		name:      "customrun event",
		eventtype: "my.test.run.event",
		run:       getCustomRunByMeta(t, "myrun", "mynamespace", "", "", ""),
		wantKey:   "my.test.run.event/tekton.dev/v1beta1/CustomRun/mynamespace/myrun",
		wantErr:   false,
	}, {
		name:      "taskrun v1beta1 event",
		eventtype: "my.test.run.event",
		run:       getTaskRunBetaByMeta(t, "mytaskrun", "mynamespace", "", "", ""),
		wantKey:   "my.test.run.event/tekton.dev/v1beta1/TaskRun/mynamespace/mytaskrun",
		wantErr:   false,
	}, {
		name:      "taskrun event",
		eventtype: "my.test.run.event",
		run:       getTaskRunByMeta(t, "mytaskrun", "mynamespace", "", "", ""),
		wantKey:   "my.test.run.event/tekton.dev/v1/TaskRun/mynamespace/mytaskrun",
		wantErr:   false,
	}, {
		name:      "pipelinerun v1beta1 event",
		eventtype: "my.test.run.event",
		run:       getPipelineRunBetaByMeta(t, "mypipelinerun", "mynamespace", "", "", ""),
		wantKey:   "my.test.run.event/tekton.dev/v1beta1/PipelineRun/mynamespace/mypipelinerun",
		wantErr:   false,
	}, {
		name:      "pipelinerun event",
		eventtype: "my.test.run.event",
		run:       getPipelineRunByMeta(t, "mypipelinerun", "mynamespace", "", "", ""),
		wantKey:   "my.test.run.event/tekton.dev/v1/PipelineRun/mynamespace/mypipelinerun",
		wantErr:   false,
	}, {
		name:      "run event missing run",
		eventtype: "my.test.run.event",
		run:       nil,
		wantKey:   "",
		wantErr:   true,
	}}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			gotEvent := getEventToTest(t, tc.eventtype, tc.run)
			gotKey, err := cache.EventKey(gotEvent, tc.run)
			if err != nil {
				if !tc.wantErr {
					t.Errorf("Expecting an error, got none")
				}
			}
			// Compare the key, use the hash if don't expect an error
			wantKey := hash(tc.wantKey)
			if tc.wantErr {
				wantKey = tc.wantKey
			}
			if d := cmp.Diff(wantKey, gotKey); d != "" {
				t.Errorf("Wrong Event key %s", diff.PrintWantGot(d))
			}
		})
	}
}

// TestObjectKey verifies that keys are extracted correctly from objects
func TestObjectKey(t *testing.T) {
	testcases := []struct {
		name    string
		run     v1beta1.RunObject
		wantKey string
		wantErr bool
	}{{
		name:    "customrun no condition",
		run:     getCustomRunByMeta(t, "myrun", "mynamespace", "", "", ""),
		wantKey: "///tekton.dev/v1beta1/CustomRun/mynamespace/myrun",
		wantErr: false,
	}, {
		name:    "customrun only status",
		run:     getCustomRunByMeta(t, "myrun", "mynamespace", "", "", string(corev1.ConditionUnknown)),
		wantKey: "Unknown///tekton.dev/v1beta1/CustomRun/mynamespace/myrun",
		wantErr: false,
	}, {
		name:    "customrun all",
		run:     getCustomRunByMeta(t, "myrun", "mynamespace", "mymessage", "myreason", string(corev1.ConditionUnknown)),
		wantKey: "Unknown/myreason/mymessage/tekton.dev/v1beta1/CustomRun/mynamespace/myrun",
		wantErr: false,
	}, {
		name:    "taskrun v1beta1",
		run:     getTaskRunBetaByMeta(t, "mytaskrun", "mynamespace", "mymessage", "myreason", string(corev1.ConditionTrue)),
		wantKey: "True/myreason/mymessage/tekton.dev/v1beta1/TaskRun/mynamespace/mytaskrun",
		wantErr: false,
	}, {
		name:    "taskrun",
		run:     getTaskRunByMeta(t, "mytaskrun", "mynamespace", "mymessage", "myreason", string(corev1.ConditionFalse)),
		wantKey: "False/myreason/mymessage/tekton.dev/v1/TaskRun/mynamespace/mytaskrun",
		wantErr: false,
	}, {
		name:    "pipelinerun v1beta1",
		run:     getPipelineRunBetaByMeta(t, "mypipelinerun", "mynamespace", "mymessage", "myreason", string(corev1.ConditionUnknown)),
		wantKey: "Unknown/myreason/mymessage/tekton.dev/v1beta1/PipelineRun/mynamespace/mypipelinerun",
		wantErr: false,
	}, {
		name:    "pipelinerun",
		run:     getPipelineRunByMeta(t, "mypipelinerun", "mynamespace", "mymessage", "myreason", string(corev1.ConditionUnknown)),
		wantKey: "Unknown/myreason/mymessage/tekton.dev/v1/PipelineRun/mynamespace/mypipelinerun",
		wantErr: false,
	}, {
		name:    "run event missing run",
		run:     nil,
		wantKey: "",
		wantErr: true,
	}}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			gotKey, err := cache.ObjectKey(tc.run)
			if err != nil {
				if !tc.wantErr {
					t.Errorf("Expecting an error, got none")
				}
			}
			// Compare the key, use the hash if don't expect an error
			wantKey := hash(tc.wantKey)
			if tc.wantErr {
				wantKey = tc.wantKey
			}
			if d := cmp.Diff(wantKey, gotKey); d != "" {
				t.Errorf("Wrong Event key %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestContainsOrAddCloudEvent(t *testing.T) {
	customRun1Cond1 := getCustomRunByMeta(t, "arun", "anamespace", "", "", "")
	customRun1Cond2 := getCustomRunByMeta(t, "arun", "bnamespace", "mymessage", "myreason", string(corev1.ConditionUnknown))
	customRun1event1 := getEventToTest(t, "some.event.type", customRun1Cond1)
	customRun1event2 := getEventToTest(t, "some.other.event.type", customRun1Cond2)
	customRun2Cond1 := getCustomRunByMeta(t, "anotherun", "anamespace", "", "", "")
	customRun2Cond2 := getCustomRunByMeta(t, "anotherun", "anamespace", "mymessage", "myreason", string(corev1.ConditionUnknown))
	customRun2event1 := getEventToTest(t, "some.event.type", customRun2Cond1)
	customRun2event2 := getEventToTest(t, "some.other.event.type", customRun2Cond2)

	testCache, err := bc.New(context.Background(), bc.Config{
		Shards:       16,
		MaxEntrySize: 16,
	})
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	check := func(tc string, expected, found bool, err error) {
		if err != nil {
			t.Fatalf("%s: unexpected error adding the event %v", tc, err)
		}
		if d := cmp.Diff(expected, found); d != "" {
			t.Fatalf("%s: unexpected result from cache %s", tc, diff.PrintWantGot(d))
		}
	}
	// Add the first event for the first run
	found, err := cache.ContainsOrAddCloudEvent(testCache, customRun1event1, customRun1Cond1)
	check("run1, event1", false, found, err)
	// Add the first event for the second run
	found, err = cache.ContainsOrAddCloudEvent(testCache, customRun2event1, customRun2Cond1)
	check("run2, event1", false, found, err)
	// Add the first event for the first run (again)
	found, err = cache.ContainsOrAddCloudEvent(testCache, customRun1event1, customRun1Cond1)
	check("run1, event1 again", true, found, err)
	// Add the second event for the second run
	found, err = cache.ContainsOrAddCloudEvent(testCache, customRun2event2, customRun2Cond2)
	check("run2, event2", false, found, err)
	// Add the second event for the first run
	found, err = cache.ContainsOrAddCloudEvent(testCache, customRun1event2, customRun1Cond2)
	check("run1, event2", false, found, err)
	// Add the second event for the first run (again)
	found, err = cache.ContainsOrAddCloudEvent(testCache, customRun1event2, customRun1Cond2)
	check("run1, event2 again", true, found, err)
}

// TestTwoLevelCacheLifecycle verifies the two-level cache contract:
//   - Level 1 (ObjectKey): gate on full condition state; a message-only change opens
//     the gate again.
//   - Level 2 (EventKey):  gate on event type; blocks re-send of terminal events even
//     after a message change; exempt for "unknown" events.
func TestTwoLevelCacheLifecycle(t *testing.T) {
	// Simulate a TaskRun lifecycle:
	//   1. running, message ""         → L1 miss, L2 miss  → send unknown
	//   2. running, message "step 2/3" → L1 miss, L2 exempt → send unknown
	//   3. running, message "step 2/3" → L1 hit             → skip
	//   4. succeeded                   → L1 miss, L2 miss  → send successful
	//   5. succeeded, message changed  → L1 miss, L2 hit   → skip (terminal)
	runRunning1 := getTaskRunByMeta(t, "tr", "ns", "", "Running", string(corev1.ConditionUnknown))
	runRunning2 := getTaskRunByMeta(t, "tr", "ns", "step 2/3", "Running", string(corev1.ConditionUnknown))
	runSucceeded1 := getTaskRunByMeta(t, "tr", "ns", "", "Succeeded", string(corev1.ConditionTrue))
	runSucceeded2 := getTaskRunByMeta(t, "tr", "ns", "all done", "Succeeded", string(corev1.ConditionTrue))

	unknownEvent := getEventToTest(t, "dev.tekton.event.taskrun.unknown.v1", runRunning1)
	successEvent := getEventToTest(t, "dev.tekton.event.taskrun.successful.v1", runSucceeded1)

	testCache, err := bc.New(context.Background(), bc.DefaultConfig(2*time.Hour))
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	check := func(label string, wantFound, gotFound bool, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: unexpected error %v", label, err)
		}
		if d := cmp.Diff(wantFound, gotFound); d != "" {
			t.Errorf("%s: %s", label, diff.PrintWantGot(d))
		}
	}

	// Step 1: running, no message — L1 miss
	found, err := cache.ContainsOrAddObject(testCache, runRunning1)
	check("step1 L1 running/empty", false, found, err)
	// unknown is exempt from L2 — no L2 check needed; would send

	// Step 2: running, message "step 2/3" — L1 miss (new condition state)
	found, err = cache.ContainsOrAddObject(testCache, runRunning2)
	check("step2 L1 running/step2", false, found, err)
	// unknown is exempt from L2 — would send again

	// Step 3: same state repeated — L1 hit → skip
	found, err = cache.ContainsOrAddObject(testCache, runRunning2)
	check("step3 L1 running/step2 again", true, found, err)

	// Step 4: succeeded — L1 miss
	found, err = cache.ContainsOrAddObject(testCache, runSucceeded1)
	check("step4 L1 succeeded", false, found, err)
	// L2 miss for successful event — would send
	found, err = cache.ContainsOrAddCloudEvent(testCache, successEvent, runSucceeded1)
	check("step4 L2 successful", false, found, err)

	// Step 5: succeeded with different message — L1 miss (message changed)
	found, err = cache.ContainsOrAddObject(testCache, runSucceeded2)
	check("step5 L1 succeeded/new msg", false, found, err)
	// L2 hit — successful already sent; must NOT re-send
	found, err = cache.ContainsOrAddCloudEvent(testCache, successEvent, runSucceeded2)
	check("step5 L2 successful already sent", true, found, err)

	// Unknown event is never blocked by L2 — verify ContainsOrAddCloudEvent
	// would return false for it (i.e. callers must skip calling L2 for unknown).
	// We verify this by confirming the unknown EventKey is distinct and not yet cached.
	found, err = cache.ContainsOrAddCloudEvent(testCache, unknownEvent, runRunning1)
	check("unknown L2 first call", false, found, err)
	// Second call would return true — confirming callers must not call L2 for unknown.
	found, err = cache.ContainsOrAddCloudEvent(testCache, unknownEvent, runRunning1)
	check("unknown L2 second call (proves caller must skip L2 for unknown)", true, found, err)
}

func TestContainsOrAddObject(t *testing.T) {
	customRun1Cond1 := getCustomRunByMeta(t, "arun", "anamespace", "", "", "")
	customRun1Cond2 := getCustomRunByMeta(t, "arun", "bnamespace", "mymessage", "myreason", string(corev1.ConditionUnknown))
	customRun2Cond1 := getCustomRunByMeta(t, "anotherun", "anamespace", "", "", "")
	customRun2Cond2 := getCustomRunByMeta(t, "anotherun", "anamespace", "mymessage", "myreason", string(corev1.ConditionTrue))

	testCache, err := bc.New(context.Background(), bc.DefaultConfig(2*time.Hour))
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	check := func(tc string, expected, found bool, err error) {
		if err != nil {
			t.Fatalf("%s: unexpected error adding the event %v", tc, err)
		}
		if d := cmp.Diff(expected, found); d != "" {
			t.Fatalf("%s: unexpected result from cache %s", tc, diff.PrintWantGot(d))
		}
	}
	// Add the first condition for the first run
	found, err := cache.ContainsOrAddObject(testCache, customRun1Cond1)
	check("run1, cond1", false, found, err)
	// Add the first condition for the second run
	found, err = cache.ContainsOrAddObject(testCache, customRun2Cond1)
	check("run2, cond1", false, found, err)
	// Add the first condition for the first run (again)
	found, err = cache.ContainsOrAddObject(testCache, customRun1Cond1)
	check("run1, cond1 again", true, found, err)
	// Add the second condition for the second run
	found, err = cache.ContainsOrAddObject(testCache, customRun2Cond2)
	check("run2, cond2", false, found, err)
	// Add the second condition for the first run
	found, err = cache.ContainsOrAddObject(testCache, customRun1Cond2)
	check("run1, cond2", false, found, err)
	// Add the second condition for the first run (again)
	found, err = cache.ContainsOrAddObject(testCache, customRun1Cond2)
	check("run1, cond2 again", true, found, err)
}
