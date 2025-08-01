/*
Copyright 2025 The Knative Authors

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

package metrics

import (
	"fmt"
	"time"

	configmap "knative.dev/pkg/configmap/parser"
)

const (
	ProtocolGRPC         = "grpc"
	ProtocolHTTPProtobuf = "http/protobuf"
	ProtocolPrometheus   = "prometheus"
	ProtocolNone         = "none"
)

// Config provides a unified observability configuration which can be used to
// manage Knative observability behavior.  Typically, this is extracted from a
// Kubernetes ConfigMap during application startup, and accessed via the
// GetConfig() method.
type Config struct {
	Protocol       string        `json:"protocol,omitempty"`
	Endpoint       string        `json:"endpoint,omitempty"`
	ExportInterval time.Duration `json:"exportInterval,omitempty"`
}

func (c *Config) Validate() error {
	switch c.Protocol {
	case ProtocolGRPC, ProtocolHTTPProtobuf:
		if c.Endpoint == "" {
			return fmt.Errorf("endpoint should be set when protocol is %q", c.Protocol)
		}
	case ProtocolNone:
		if c.Endpoint != "" {
			return fmt.Errorf("endpoint should not be set when protocol is %q", c.Protocol)
		}
	case ProtocolPrometheus:
		// Endpoint is not required, but can be used to indicate listen port
	default:
		return fmt.Errorf("unsupported protocol %q", c.Protocol)
	}

	if c.ExportInterval < 0 {
		return fmt.Errorf("export interval %q should be greater than zero", c.ExportInterval)
	}
	return nil
}

// DefaultConfig returns a configuration with default values set.
func DefaultConfig() Config {
	return Config{
		Protocol: ProtocolNone,
	}
}

// NewFromMap unpacks flat configuration values from a ConfigMap into
// the configuration used by different observability modules.
func NewFromMap(m map[string]string) (Config, error) {
	return NewFromMapWithPrefix("", m)
}

func NewFromMapWithPrefix(prefix string, m map[string]string) (Config, error) {
	c := DefaultConfig()

	err := configmap.Parse(m,
		configmap.As(prefix+"metrics-protocol", &c.Protocol),
		configmap.As(prefix+"metrics-endpoint", &c.Endpoint),
		configmap.As(prefix+"metrics-export-interval", &c.ExportInterval),
	)
	if err != nil {
		return c, err
	}

	return c, c.Validate()
}
