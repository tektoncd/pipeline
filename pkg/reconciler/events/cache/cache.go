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
	"errors"
	"fmt"
	"hash/fnv"
	"strconv"
	"time"

	bc "github.com/allegro/bigcache/v3"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"knative.dev/pkg/apis"
)

// containsOrAddKey checks if the key exists in the cache.
// Returns true (already cached) if the key was found; returns false (new) and adds
// the key if it was not present. Either way, the current timestamp is appended as value.
func containsOrAddKey(cacheClient *bc.BigCache, key string) (bool, error) {
	if cacheClient == nil {
		return false, errors.New("cache client is nil")
	}
	// Set the new key
	timeNow, err := hash(time.Now().Format(time.RFC3339Nano))
	if err != nil {
		return false, err
	}
	err = cacheClient.Append(key, []byte(timeNow))
	if err != nil {
		return false, err
	}
	// Get the value - if it matches what we added, the key was new
	value, err := cacheClient.Get(key)
	if err != nil {
		return false, err
	}
	return (string(value) != timeNow), nil
}

// ContainsOrAddObject adds an object key (ObjectKey) to the cache and returns true if
// the key was already present, false if it was new.
// A cache miss requires a new object or change in the condition (including message-only changes)
func ContainsOrAddObject(cacheClient *bc.BigCache, object v1beta1.RunObject) (bool, error) {
	if cacheClient == nil {
		return false, errors.New("cache client is nil")
	}
	objectKey, err := ObjectKey(object)
	if err != nil {
		return false, err
	}
	return containsOrAddKey(cacheClient, objectKey)
}

// ContainsOrAddCloudEvent adds an event key (EventKey) to the cache and returns true if
// the key was already present, false if it was new.
// A cache miss requires a new object or a new event type for the same object
func ContainsOrAddCloudEvent(cacheClient *bc.BigCache, event *cloudevents.Event, object v1beta1.RunObject) (bool, error) {
	if cacheClient == nil {
		return false, errors.New("cache client is nil")
	}
	eventKey, err := EventKey(event, object)
	if err != nil {
		return false, err
	}
	return containsOrAddKey(cacheClient, eventKey)
}

// EventKey encodes the event type and object identity (GVK, namespace, name).
// Once an event type is recorded here it is never re-sent for the same object,
// regardless of condition changes.
func EventKey(event *cloudevents.Event, object v1beta1.RunObject) (string, error) {
	if object == nil || event == nil {
		return "", fmt.Errorf("both object (%v) and event (%v) must be not nil", object, event)
	}
	if object.GetObjectKind() == nil {
		return "", fmt.Errorf("object %v has nil object kind", object)
	}
	if object.GetObjectMeta() == nil {
		return "", fmt.Errorf("object %v has nil object meta", object)
	}
	return hash(fmt.Sprintf("%s/%s/%s/%s/%s/%s",
		event.Type(),
		object.GetObjectKind().GroupVersionKind().Group,
		object.GetObjectKind().GroupVersionKind().Version,
		object.GetObjectKind().GroupVersionKind().Kind,
		object.GetObjectMeta().GetNamespace(),
		object.GetObjectMeta().GetName()))
}

// ObjectKey encodes the full condition snapshot (Status, Reason, Message)
// plus object identity (GVK, namespace, name).
// Any condition change — including a message-only change — produces a new key.
func ObjectKey(object v1beta1.RunObject) (string, error) {
	var condition *apis.Condition
	if object == nil {
		return "", errors.New("object must be not nil")
	}
	// The condition may not be set, in that case return an empty condition
	if object.GetStatusCondition() != nil {
		condition = object.GetStatusCondition().GetCondition(apis.ConditionSucceeded)
	}
	if condition == nil {
		condition = &apis.Condition{}
	}
	if object.GetObjectKind() == nil {
		return "", fmt.Errorf("object %v has nil object kind", object)
	}
	if object.GetObjectMeta() == nil {
		return "", fmt.Errorf("object %v has nil object meta", object)
	}
	return hash(fmt.Sprintf("%s/%s/%s/%s/%s/%s/%s/%s",
		condition.Status,
		condition.Reason,
		condition.Message,
		object.GetObjectKind().GroupVersionKind().Group,
		object.GetObjectKind().GroupVersionKind().Version,
		object.GetObjectKind().GroupVersionKind().Kind,
		object.GetObjectMeta().GetNamespace(),
		object.GetObjectMeta().GetName()))
}

// hash provide fnv64 hash converted to string in base36
func hash(input string) (string, error) {
	hasher := fnv.New64a()
	_, err := hasher.Write([]byte(input))
	if err != nil {
		return "", err
	}
	return strconv.FormatUint(hasher.Sum64(), 36), nil
}
