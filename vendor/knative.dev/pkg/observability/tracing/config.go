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

package tracing

import (
	"errors"
	"fmt"

	configmap "knative.dev/pkg/configmap/parser"
)

const (
	ProtocolGRPC         = "grpc"
	ProtocolHTTPProtobuf = "http/protobuf"
	ProtocolNone         = "none"
	ProtocolStdout       = "stdout"
)

type Config struct {
	Protocol     string  `json:"protocol,omitempty"`
	Endpoint     string  `json:"endpoint,omitempty"`
	SamplingRate float64 `json:"samplingRate,omitempty"`
}

func (c *Config) Validate() error {
	switch c.Protocol {
	case ProtocolGRPC, ProtocolHTTPProtobuf:
		if c.Endpoint == "" {
			return fmt.Errorf("endpoint should be set for protocol %q", c.Protocol)
		}
	case ProtocolNone, ProtocolStdout:
		if c.Endpoint != "" {
			return errors.New("endpoint should not be set when protocol is 'none'")
		}
	default:
		return fmt.Errorf("unsupported protocol %q", c.Protocol)
	}

	if c.SamplingRate < 0 {
		return fmt.Errorf("sampling rate %f should be greater or equal to zero", c.SamplingRate)
	} else if c.SamplingRate > 1.0 {
		return fmt.Errorf("sampling rate %f should be less than or equal to one", c.SamplingRate)
	}
	return nil
}

func DefaultConfig() Config {
	return Config{
		Protocol: ProtocolNone,
	}
}

func NewFromMap(m map[string]string) (Config, error) {
	return NewFromMapWithPrefix("", m)
}

func NewFromMapWithPrefix(prefix string, m map[string]string) (Config, error) {
	c := DefaultConfig()

	err := configmap.Parse(m,
		configmap.As(prefix+"tracing-protocol", &c.Protocol),
		configmap.As(prefix+"tracing-endpoint", &c.Endpoint),
		configmap.As(prefix+"tracing-sampling-rate", &c.SamplingRate),
	)
	if err != nil {
		return c, err
	}

	return c, c.Validate()
}
