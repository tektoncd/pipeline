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

package config

import (
	yaml "gopkg.in/yaml.v2"
)

var defaultChannel = Channel{Name: "performance", Identity: "CBDMABCTF"}

// Channel contains Slack channel's info.
type Channel struct {
	Name     string `yaml:"name,omitempty"`
	Identity string `yaml:"identity,omitempty"`
}

// SlackConfig contains slack configuration for the benchmarks.
type SlackConfig struct {
	BenchmarkChannels map[string][]Channel `yaml:"benchmarkChannels,omitempty"`
}

// GetSlackChannels returns the slack channels to alert on for the given benchmark.
// If any error happens, or the config is not found, return the default channel.
func GetSlackChannels(benchmarkName string) []Channel {
	cfg, err := loadConfig()
	if err != nil {
		return []Channel{defaultChannel}
	}
	return getSlackChannels(cfg.SlackConfig, benchmarkName)
}

func getSlackChannels(configStr, benchmarkName string) []Channel {
	slackConfig := &SlackConfig{}
	if err := yaml.Unmarshal([]byte(configStr), slackConfig); err != nil {
		return []Channel{defaultChannel}
	}
	if channels, ok := slackConfig.BenchmarkChannels[benchmarkName]; ok {
		return channels
	}

	return []Channel{defaultChannel}
}
