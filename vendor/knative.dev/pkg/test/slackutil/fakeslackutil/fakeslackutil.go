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

// fakeslackutil.go fakes SlackClient for testing purpose

package fakeslackutil

import (
	"sync"
	"time"
)

type messageEntry struct {
	text     string
	sentTime time.Time
}

// FakeSlackClient is a faked client, implements all functions of slackutil.ReadOperations and slackutil.WriteOperations
type FakeSlackClient struct {
	History map[string][]messageEntry
	mutex   sync.RWMutex
}

// NewFakeSlackClient creates a FakeSlackClient and initialize it's maps
func NewFakeSlackClient() *FakeSlackClient {
	return &FakeSlackClient{
		History: make(map[string][]messageEntry), // map of channel name: slice of messages sent to the channel
		mutex:   sync.RWMutex{},
	}
}

// MessageHistory returns the messages to the channel from the given startTime
func (c *FakeSlackClient) MessageHistory(channel string, startTime time.Time) ([]string, error) {
	c.mutex.Lock()
	messages := make([]string, 0)
	if history, ok := c.History[channel]; ok {
		for _, msg := range history {
			if time.Now().After(startTime) {
				messages = append(messages, msg.text)
			}
		}
	}
	c.mutex.Unlock()
	return messages, nil
}

// Post sends the text as a message to the given channel
func (c *FakeSlackClient) Post(text, channel string) error {
	c.mutex.Lock()
	messages := make([]messageEntry, 0)
	if history, ok := c.History[channel]; ok {
		messages = history
	}
	messages = append(messages, messageEntry{text: text, sentTime: time.Now()})
	c.History[channel] = messages
	c.mutex.Unlock()
	return nil
}
