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
	"io/ioutil"
	"log"

	yaml "gopkg.in/yaml.v2"
)

// configFile saves all information we need to send slack message to channel(s)
// when performance regression happens in the automation tests.
const configFile = "config.yaml"

var repoConfigs []repoConfig

// config contains all repo configs for performance regression alerting.
type config struct {
	repoConfigs []repoConfig `yaml:"repoConfigs"`
}

// repoConfig is initial configuration for a given repo, defines which channel(s) to alert
type repoConfig struct {
	repo     string    `yaml:"repo"` // repository to report issues
	channels []channel `yaml:"slackChannels,omitempty"`
}

// channel contains Slack channel's info
type channel struct {
	name     string `yaml:"name"`
	identity string `yaml:"identity"`
}

// init parses config from configFile
func init() {
	contents, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Printf("Failed to load the config file: %v", err)
		return
	}
	config := &config{}
	if err = yaml.Unmarshal(contents, &config); err != nil {
		log.Printf("Failed to unmarshal %v", contents)
	} else {
		repoConfigs = config.repoConfigs
	}
}
