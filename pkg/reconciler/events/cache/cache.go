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

package cache

import (
	"encoding/json"
	"errors"
	"fmt"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	lru "github.com/hashicorp/golang-lru"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
)

// Struct to unmarshal the event data
type eventData struct {
	Run *v1alpha1.Run `json:"run,omitempty"`
}

// AddEventSentToCache adds the particular object to cache marking it as sent
func AddEventSentToCache(cacheClient *lru.Cache, event *cloudevents.Event) error {
	if cacheClient == nil {
		return errors.New("cache client is nil")
	}
	eventKey, err := EventKey(event)
	if err != nil {
		return err
	}
	cacheClient.Add(eventKey, nil)
	return nil
}

// IsCloudEventSent checks if the event exists in the cache
func IsCloudEventSent(cacheClient *lru.Cache, event *cloudevents.Event) (bool, error) {
	if cacheClient == nil {
		return false, errors.New("cache client is nil")
	}
	eventKey, err := EventKey(event)
	if err != nil {
		return false, err
	}
	return cacheClient.Contains(eventKey), nil
}

// EventKey defines whether an event is considered different from another
// in future we might want to let specific event types override this
func EventKey(event *cloudevents.Event) (string, error) {
	var (
		data              eventData
		resourceName      string
		resourceNamespace string
	)
	err := json.Unmarshal(event.Data(), &data)
	if err != nil {
		return "", err
	}
	if data.Run == nil {
		return "", fmt.Errorf("Invalid Run data in %v", event)
	}
	resourceName = data.Run.Name
	resourceNamespace = data.Run.Namespace
	eventType := event.Type()
	return fmt.Sprintf("%s/run/%s/%s", eventType, resourceNamespace, resourceName), nil
}
