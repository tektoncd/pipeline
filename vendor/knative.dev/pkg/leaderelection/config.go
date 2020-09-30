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

const (
	configMapNameEnv = "CONFIG_LEADERELECTION_NAME"
	// KnativeResourceLock is the only supported lock mechanism for Knative.
	KnativeResourceLock = resourcelock.LeasesResourceLock
)

// MaxBuckets is the maximum number of buckets to allow users to define.
// This is a variable so that it may be customized in the binary entrypoint.
var MaxBuckets uint32 = 10

// NewConfigFromMap returns a Config for the given map, or an error.
func NewConfigFromMap(data map[string]string) (*Config, error) {
	config := defaultConfig()

	if err := cm.Parse(data,
		cm.AsDuration("leaseDuration", &config.LeaseDuration),
		cm.AsDuration("renewDeadline", &config.RenewDeadline),
		cm.AsDuration("retryPeriod", &config.RetryPeriod),

		cm.AsUint32("buckets", &config.Buckets),
	); err != nil {
		return nil, err
	}

	if config.Buckets < 1 || config.Buckets > MaxBuckets {
		return nil, fmt.Errorf("buckets: value must be between %d <= %d <= %d", 1, config.Buckets, MaxBuckets)
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
	Buckets       uint32
	LeaseDuration time.Duration
	RenewDeadline time.Duration
	RetryPeriod   time.Duration

	// This field is deprecated and will be removed once downstream
	// repositories have removed their validation of it.
	// TODO(https://github.com/knative/pkg/issues/1478): Remove this field.
	EnabledComponents sets.String
}

func (c *Config) GetComponentConfig(name string) ComponentConfig {
	return ComponentConfig{
		Component:     name,
		Buckets:       c.Buckets,
		LeaseDuration: c.LeaseDuration,
		RenewDeadline: c.RenewDeadline,
		RetryPeriod:   c.RetryPeriod,
	}
}

func defaultConfig() *Config {
	return &Config{
		Buckets:       1,
		LeaseDuration: 15 * time.Second,
		RenewDeadline: 10 * time.Second,
		RetryPeriod:   2 * time.Second,
	}
}

// ComponentConfig represents the leader election config for a single component.
type ComponentConfig struct {
	Component     string
	Buckets       uint32
	LeaseDuration time.Duration
	RenewDeadline time.Duration
	RetryPeriod   time.Duration
	// LeaseName is a function to customize the lease name given the index i.
	// If not present, a name in format {Component}.{queue-name}.{i}-of-{Buckets}
	// will be use.
	// Autoscaler need to know the Lease names to filter out Leases which are not
	// used for Autoscaler. Instead of exposing the names from leadelection package,
	// we let Autoscaler to pass them in.
	LeaseName func(i uint32) string `json:"-"`
	// Identity is the unique string identifying a resource lock holder across
	// all participants in an election. If not present, a new unique string will
	// be generated to be used as identity for each BuildElector call.
	// Autoscaler uses the pod IP as identity.
	Identity string
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
