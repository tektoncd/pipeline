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

// messaging.go includes functions to send message to Slack channel.

package slackutil

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"

	"net/http"
	"net/url"
)

const postMessageURL = "https://slack.com/api/chat.postMessage"

// Operations defines the operations that can be done to Slack
type Operations interface {
	Post(text, channel string) error
}

// client contains Slack bot related information
type client struct {
	userName  string
	tokenStr  string
	iconEmoji *string
}

// NewClient reads token file and stores it for later authentication
func NewClient(userName, tokenPath string) (Operations, error) {
	b, err := ioutil.ReadFile(tokenPath)
	if err != nil {
		return nil, err
	}
	return &client{
		userName: userName,
		tokenStr: strings.TrimSpace(string(b)),
	}, nil
}

// Post posts the given text to channel
func (c *client) Post(text, channel string) error {
	uv := url.Values{}
	uv.Add("username", c.userName)
	uv.Add("token", c.tokenStr)
	if nil != c.iconEmoji {
		uv.Add("icon_emoji", *c.iconEmoji)
	}
	uv.Add("channel", channel)
	uv.Add("text", text)

	return c.postMessage(uv)
}

// postMessage does http post
func (c *client) postMessage(uv url.Values) error {
	resp, err := http.PostForm(postMessageURL, uv)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	t, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http response code is not '%d': '%s'", http.StatusOK, string(t))
	}
	// response code could also be 200 if channel doesn't exist, parse response body to find out
	var b struct {
		OK bool `json:"ok"`
	}
	if err = json.Unmarshal(t, &b); nil != err || !b.OK {
		return fmt.Errorf("response not ok '%s'", string(t))
	}
	return nil
}
