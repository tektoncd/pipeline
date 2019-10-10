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
	"flag"
	"fmt"
	"sync"
	"time"

	"knative.dev/pkg/test/helpers"
	"knative.dev/pkg/test/mako/config"
	"knative.dev/pkg/test/slackutil"
)

var minInterval = flag.Duration("min-alert-interval", 24*time.Hour, "The minimum interval of sending Slack alerts.")

const (
	messageTemplate = `
As of %s, there is a new performance regression detected from automation test:
%s`
)

// MessageHandler handles methods for slack messages
type MessageHandler struct {
	readClient  slackutil.ReadOperations
	writeClient slackutil.WriteOperations
	channels    []config.Channel
	dryrun      bool
}

// Setup creates the necessary setup to make calls to work with slack
func Setup(userName, readTokenPath, writeTokenPath string, channels []config.Channel, dryrun bool) (*MessageHandler, error) {
	readClient, err := slackutil.NewReadClient(userName, readTokenPath)
	if err != nil {
		return nil, fmt.Errorf("cannot authenticate to slack read client: %v", err)
	}
	writeClient, err := slackutil.NewWriteClient(userName, writeTokenPath)
	if err != nil {
		return nil, fmt.Errorf("cannot authenticate to slack write client: %v", err)
	}
	return &MessageHandler{
		readClient:  readClient,
		writeClient: writeClient,
		channels:    channels,
		dryrun:      dryrun,
	}, nil
}

// SendAlert will send the alert text to the slack channel(s)
func (smh *MessageHandler) SendAlert(text string) error {
	dryrun := smh.dryrun
	errCh := make(chan error)
	var wg sync.WaitGroup
	for i := range smh.channels {
		channel := smh.channels[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			// get the recent message history in the channel for this user
			startTime := time.Now().Add(-1 * *minInterval)
			var messageHistory []string
			if err := helpers.Run(
				fmt.Sprintf("retrieving message history in channel %q", channel.Name),
				func() error {
					var err error
					messageHistory, err = smh.readClient.MessageHistory(channel.Identity, startTime)
					return err
				},
				dryrun,
			); err != nil {
				errCh <- fmt.Errorf("failed to retrieve message history in channel %q", channel.Name)
			}
			// do not send message again if messages were sent on the same channel a short while ago
			if len(messageHistory) != 0 {
				return
			}
			// send the alert message to the channel
			message := fmt.Sprintf(messageTemplate, time.Now(), text)
			if err := helpers.Run(
				fmt.Sprintf("sending message %q to channel %q", message, channel.Name),
				func() error {
					return smh.writeClient.Post(message, channel.Identity)
				},
				dryrun,
			); err != nil {
				errCh <- fmt.Errorf("failed to send message to channel %q", channel.Name)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(errCh)
	}()

	errs := make([]error, 0)
	for err := range errCh {
		errs = append(errs, err)
	}

	return helpers.CombineErrors(errs)
}
