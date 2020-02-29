/*
Copyright 2020 The Knative Authors

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

package leaderelection

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/uuid"
)

const ConfigMapNameEnv = "CONFIG_LEADERELECTION_NAME"

var (
	errEmptyLeaderElectionConfig = errors.New("empty leader election configuration")
	validResourceLocks           = sets.NewString("leases", "configmaps", "endpoints")
)

// NewConfigFromMap returns a Config for the given map, or an error.
func NewConfigFromMap(data map[string]string) (*Config, error) {
	config := &Config{
		EnabledComponents: sets.NewString(),
	}

	if resourceLock := data["resourceLock"]; !validResourceLocks.Has(resourceLock) {
		return nil, fmt.Errorf(`resourceLock: invalid value %q: valid values are "leases","configmaps","endpoints"`, resourceLock)
	} else {
		config.ResourceLock = resourceLock
	}

	if leaseDuration, err := time.ParseDuration(data["leaseDuration"]); err != nil {
		return nil, fmt.Errorf("leaseDuration: invalid duration: %q", data["leaseDuration"])
	} else {
		config.LeaseDuration = leaseDuration
	}

	if renewDeadline, err := time.ParseDuration(data["renewDeadline"]); err != nil {
		return nil, fmt.Errorf("renewDeadline: invalid duration: %q", data["renewDeadline"])
	} else {
		config.RenewDeadline = renewDeadline
	}

	if retryPeriod, err := time.ParseDuration(data["retryPeriod"]); err != nil {
		return nil, fmt.Errorf("retryPeriod: invalid duration: %q", data["retryPeriod"])
	} else {
		config.RetryPeriod = retryPeriod
	}

	// enabledComponents are not validated here, because they are dependent on
	// the component. Components should provide additional validation for this
	// field.
	if enabledComponents, ok := data["enabledComponents"]; ok {
		tokens := strings.Split(enabledComponents, ",")
		config.EnabledComponents = sets.NewString(tokens...)
	}

	return config, nil
}

// NewConfigFromConfigMap returns a new Config from the given ConfigMap.
func NewConfigFromConfigMap(configMap *corev1.ConfigMap) (*Config, error) {
	if configMap == nil {
		config := defaultConfig()
		return config, nil
	}

	return NewConfigFromMap(configMap.Data)
}

// Config represents the leader election config for a set of components
// contained within a single namespace. Typically these will correspond to a
// single source repository, viz: serving or eventing.
type Config struct {
	ResourceLock      string
	LeaseDuration     time.Duration
	RenewDeadline     time.Duration
	RetryPeriod       time.Duration
	EnabledComponents sets.String
}

func (c *Config) GetComponentConfig(name string) ComponentConfig {
	if c.EnabledComponents.Has(name) {
		return ComponentConfig{
			LeaderElect:   true,
			ResourceLock:  c.ResourceLock,
			LeaseDuration: c.LeaseDuration,
			RenewDeadline: c.RenewDeadline,
			RetryPeriod:   c.RetryPeriod,
		}
	}

	return defaultComponentConfig()
}

func defaultConfig() *Config {
	return &Config{
		ResourceLock:      "leases",
		LeaseDuration:     15 * time.Second,
		RenewDeadline:     10 * time.Second,
		RetryPeriod:       2 * time.Second,
		EnabledComponents: sets.NewString(),
	}
}

// ComponentConfig represents the leader election config for a single component.
type ComponentConfig struct {
	LeaderElect   bool
	ResourceLock  string
	LeaseDuration time.Duration
	RenewDeadline time.Duration
	RetryPeriod   time.Duration
}

func defaultComponentConfig() ComponentConfig {
	return ComponentConfig{
		LeaderElect: false,
	}
}

// ConfigMapName returns the name of the configmap to read for leader election
// settings.
func ConfigMapName() string {
	cm := os.Getenv(ConfigMapNameEnv)
	if cm == "" {
		return "config-leader-election"
	}
	return cm
}

// UniqueID returns a unique ID for use with a leader elector that prevents from
// pods running on the same host from colliding with one another.
func UniqueID() (string, error) {
	id, err := os.Hostname()
	if err != nil {
		return "", err
	}

	return (id + "_" + string(uuid.NewUUID())), nil
}
