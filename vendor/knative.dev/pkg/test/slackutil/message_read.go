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

// message_read.go includes functions to read messages from Slack.

package slackutil

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const conversationHistoryURL = "https://slack.com/api/conversations.history"

// ReadOperations defines the read operations that can be done to Slack
type ReadOperations interface {
	MessageHistory(channel string, startTime time.Time) ([]string, error)
}

// readClient contains Slack bot related information to perform read operations
type readClient struct {
	userName string
	tokenStr string
}

// NewReadClient reads token file and stores it for later authentication
func NewReadClient(userName, tokenPath string) (ReadOperations, error) {
	b, err := ioutil.ReadFile(tokenPath)
	if err != nil {
		return nil, err
	}
	return &readClient{
		userName: userName,
		tokenStr: strings.TrimSpace(string(b)),
	}, nil
}

func (c *readClient) MessageHistory(channel string, startTime time.Time) ([]string, error) {
	u, _ := url.Parse(conversationHistoryURL)
	q := u.Query()
	q.Add("username", c.userName)
	q.Add("token", c.tokenStr)
	q.Add("channel", channel)
	q.Add("oldest", strconv.FormatInt(startTime.Unix(), 10))
	u.RawQuery = q.Encode()

	content, err := get(u.String())
	if err != nil {
		return nil, err
	}

	// response code could also be 200 if channel doesn't exist, parse response body to find out
	type m struct {
		Text string `json:"text"`
	}
	var r struct {
		OK       bool `json:"ok"`
		Messages []m  `json:"messages"`
	}
	if err = json.Unmarshal(content, &r); nil != err || !r.OK {
		return nil, fmt.Errorf("response not ok '%s'", string(content))
	}

	res := make([]string, len(r.Messages))
	for i, message := range r.Messages {
		res[i] = message.Text
	}

	return res, nil
}
