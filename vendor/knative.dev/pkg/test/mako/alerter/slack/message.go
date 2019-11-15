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

package slack

import (
	"fmt"
	"sync"
	"time"

	"knative.dev/pkg/test/mako/alerter"
	"knative.dev/pkg/test/slackutil"
)

const messageTemplate = `
As of %s, there is a new performance regression detected from automation test:
%s`

// messageHandler handles methods for slack messages
type messageHandler struct {
	client slackutil.Operations
	config repoConfig
	dryrun bool
}

// Setup creates the necessary setup to make calls to work with slack
func Setup(userName, tokenPath, repo string, dryrun bool) (*messageHandler, error) {
	client, err := slackutil.NewClient(userName, tokenPath)
	if err != nil {
		return nil, fmt.Errorf("cannot authenticate to slack: %v", err)
	}
	var config *repoConfig
	for _, repoConfig := range repoConfigs {
		if repoConfig.repo == repo {
			config = &repoConfig
			break
		}
	}
	if config == nil {
		return nil, fmt.Errorf("no channel configuration found for repo %v", repo)
	}
	return &messageHandler{client: client, config: *config, dryrun: dryrun}, nil
}

// Post will post the given text to the slack channel(s)
func (smh *messageHandler) Post(text string) error {
	// TODO(Fredy-Z): add deduplication logic, maybe do not send more than one alert within 24 hours?
	errs := make([]error, 0)
	channels := smh.config.channels
	mux := &sync.Mutex{}
	var wg sync.WaitGroup
	for i := range channels {
		channel := channels[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			message := fmt.Sprintf(messageTemplate, time.Now(), text)
			if err := smh.client.Post(message, channel.identity); err != nil {
				mux.Lock()
				errs = append(errs, fmt.Errorf("failed to send message to channel %v", channel))
				mux.Unlock()
			}
		}()
	}
	wg.Wait()

	return alerter.CombineErrors(errs)
}
