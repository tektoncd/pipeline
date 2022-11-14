/*
Copyright 2021 The Tekton Authors

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
package events

import (
	"fmt"
	"regexp"
	"testing"
)

// CheckEventsOrdered checks that the events received via the given chan are the same as wantEvents,
// in the same order.
func (f *FakeRecorder)CheckEventsOrdered(t *testing.T, eventChan chan string, testName string, wantEvents []string) error {
	t.Helper()
	f.waitGroup.Wait()
	err := eventsFromChannel(eventChan, wantEvents)
	if err != nil {
		return fmt.Errorf("error in test %s: %v", testName, err)
	}
	return nil
}

// eventsFromChannel takes a chan of string, a test name, and a list of events that a test
// expects to receive. The events must be received in the same order they appear in the
// wantEvents list. Any extra or too few received events are considered errors.
func eventsFromChannel(c chan string, wantEvents []string) error {
	// we loop the channel to collect all events, if the collected events are not
	// expected events we will return error

	foundEvents := []string{}
	channelEvents := len(c)
	for ii := 0; ii < channelEvents; ii++ {
		event := <-c
		foundEvents = append(foundEvents, event)
		wantEvent := wantEvents[ii]
		matching, err := regexp.MatchString(wantEvent, event)
		if err == nil {
			if !matching {
				return fmt.Errorf("expected event \"%s\" but got \"%s\" instead", wantEvent, event)
			}
		} else {
			return fmt.Errorf("something went wrong matching the event: %s", err)
		}
	}
	if len(foundEvents) != len(wantEvents) {
		return fmt.Errorf("received %d events but %d expected. Found events: %#v", len(foundEvents), len(wantEvents), foundEvents)
	}
	return nil
}
