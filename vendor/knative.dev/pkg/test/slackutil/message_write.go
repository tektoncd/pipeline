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

// message_write.go includes functions to send messages to Slack.

package slackutil

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"

	"net/url"
)

const postMessageURL = "https://slack.com/api/chat.postMessage"

// WriteOperations defines the write operations that can be done to Slack
type WriteOperations interface {
	Post(text, channel string) error
}

// writeClient contains Slack bot related information to perform write operations
type writeClient struct {
	userName string
	tokenStr string
}

// NewWriteClient reads token file and stores it for later authentication
func NewWriteClient(userName, tokenPath string) (WriteOperations, error) {
	b, err := ioutil.ReadFile(tokenPath)
	if err != nil {
		return nil, err
	}
	return &writeClient{
		userName: userName,
		tokenStr: strings.TrimSpace(string(b)),
	}, nil
}

// Post posts the given text to channel
func (c *writeClient) Post(text, channel string) error {
	uv := url.Values{}
	uv.Add("username", c.userName)
	uv.Add("token", c.tokenStr)
	uv.Add("channel", channel)
	uv.Add("text", text)

	content, err := post(postMessageURL, uv)
	if err != nil {
		return err
	}

	// response code could also be 200 if channel doesn't exist, parse response body to find out
	var b struct {
		OK bool `json:"ok"`
	}
	if err = json.Unmarshal(content, &b); nil != err || !b.OK {
		return fmt.Errorf("response not ok '%s'", string(content))
	}

	return nil
}
