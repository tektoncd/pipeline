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
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/tools/leaderelection/resourcelock"

	cm "knative.dev/pkg/configmap"
)

const configMapNameEnv = "CONFIG_LEADERELECTION_NAME"

// MaxBuckets is the maximum number of buckets to allow users to define.
// This is a variable so that it may be customized in the binary entrypoint.
var MaxBuckets uint32 = 10

var validResourceLocks = sets.NewString(resourcelock.LeasesResourceLock)

// NewConfigFromMap returns a Config for the given map, or an error.
func NewConfigFromMap(data map[string]string) (*Config, error) {
	config := defaultConfig()

	if err := cm.Parse(data,
		cm.AsString("resourceLock", &config.ResourceLock),

		cm.AsDuration("leaseDuration", &config.LeaseDuration),
		cm.AsDuration("renewDeadline", &config.RenewDeadline),
		cm.AsDuration("retryPeriod", &config.RetryPeriod),

		cm.AsUint32("buckets", &config.Buckets),

		// enabledComponents are not validated here, because they are dependent on
		// the component. Components should provide additional validation for this
		// field.
		cm.AsStringSet("enabledComponents", &config.EnabledComponents),
	); err != nil {
		return nil, err
	}

	if config.Buckets < 1 || config.Buckets > MaxBuckets {
		return nil, fmt.Errorf("buckets: value must be between %d <= %d <= %d", 1, config.Buckets, MaxBuckets)
	}
	if !validResourceLocks.Has(config.ResourceLock) {
		return nil, fmt.Errorf(`resourceLock: invalid value %q: valid values are "leases"`, config.ResourceLock)
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
	Buckets           uint32
	LeaseDuration     time.Duration
	RenewDeadline     time.Duration
	RetryPeriod       time.Duration
	EnabledComponents sets.String
}

func (c *Config) GetComponentConfig(name string) ComponentConfig {
	if c.EnabledComponents.Has(name) {
		return ComponentConfig{
			Component:     name,
			LeaderElect:   true,
			Buckets:       c.Buckets,
			ResourceLock:  c.ResourceLock,
			LeaseDuration: c.LeaseDuration,
			RenewDeadline: c.RenewDeadline,
			RetryPeriod:   c.RetryPeriod,
		}
	}

	return defaultComponentConfig(name)
}

func defaultConfig() *Config {
	return &Config{
		ResourceLock:      "leases",
		Buckets:           1,
		LeaseDuration:     15 * time.Second,
		RenewDeadline:     10 * time.Second,
		RetryPeriod:       2 * time.Second,
		EnabledComponents: sets.NewString(),
	}
}

// ComponentConfig represents the leader election config for a single component.
type ComponentConfig struct {
	Component     string
	LeaderElect   bool
	Buckets       uint32
	ResourceLock  string
	LeaseDuration time.Duration
	RenewDeadline time.Duration
	RetryPeriod   time.Duration
}

// statefulSetID is a envconfig Decodable controller ordinal and name.
type statefulSetID struct {
	ssName  string
	ordinal int
}

func (ssID *statefulSetID) Decode(v string) error {
	if i := strings.LastIndex(v, "-"); i != -1 {
		ui, err := strconv.ParseUint(v[i+1:], 10, 64)
		ssID.ordinal = int(ui)
		ssID.ssName = v[:i]
		return err
	}
	return fmt.Errorf("%q is not a valid stateful set controller ordinal", v)
}

var _ envconfig.Decoder = (*statefulSetID)(nil)

// statefulSetConfig represents the required information for a StatefulSet service.
type statefulSetConfig struct {
	StatefulSetID statefulSetID `envconfig:"STATEFUL_CONTROLLER_ORDINAL" required:"true"`
	ServiceName   string        `envconfig:"STATEFUL_SERVICE_NAME" required:"true"`
	Port          string        `envconfig:"STATEFUL_SERVICE_PORT" default:"80"`
	Protocol      string        `envconfig:"STATEFUL_SERVICE_PROTOCOL" default:"http"`
}

// newStatefulSetConfig builds a stateful set LE config.
func newStatefulSetConfig() (*statefulSetConfig, error) {
	ssc := &statefulSetConfig{}
	if err := envconfig.Process("", ssc); err != nil {
		return nil, err
	}
	return ssc, nil
}

func defaultComponentConfig(name string) ComponentConfig {
	return ComponentConfig{
		Component:   name,
		LeaderElect: false,
	}
}

// ConfigMapName returns the name of the configmap to read for leader election
// settings.
func ConfigMapName() string {
	cm := os.Getenv(configMapNameEnv)
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

	return id + "_" + string(uuid.NewUUID()), nil
}
