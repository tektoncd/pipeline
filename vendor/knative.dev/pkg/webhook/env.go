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

package webhook

import (
	"crypto/tls"
	"fmt"
	"os"
	"strconv"
)

const (
	portEnvKey = "WEBHOOK_PORT"

	// Webhook is the name of the override key used inside of the logging config for Webhook Controller.
	webhookNameEnvKey = "WEBHOOK_NAME"

	secretNameEnvKey = "WEBHOOK_SECRET_NAME" //nolint:gosec // This is not a hardcoded credential

	tlsMinVersionEnvKey = "WEBHOOK_TLS_MIN_VERSION"
)

// PortFromEnv returns the webhook port set by portEnvKey, or default port if env var is not set.
func PortFromEnv(defaultPort int) int {
	if os.Getenv(portEnvKey) == "" {
		return defaultPort
	}
	port, err := strconv.Atoi(os.Getenv(portEnvKey))
	if err != nil {
		panic(fmt.Sprintf("failed to convert the environment variable %q : %v", portEnvKey, err))
	} else if port == 0 {
		panic(fmt.Sprintf("the environment variable %q can't be zero", portEnvKey))
	}
	return port
}

func NameFromEnv() string {
	if webhook := os.Getenv(webhookNameEnvKey); webhook != "" {
		return webhook
	}

	panic(fmt.Sprintf(`The environment variable %[1]q is not set.
This should be unique for the webhooks in a namespace
If this is a process running on Kubernetes, then initialize this variable via:
  env:
  - name: %[1]s
    value: webhook
`, webhookNameEnvKey))
}

func SecretNameFromEnv(defaultSecretName string) string {
	secret := os.Getenv(secretNameEnvKey)
	if secret == "" {
		return defaultSecretName
	}
	return secret
}

func TLSMinVersionFromEnv(defaultTLSMinVersion uint16) uint16 {
	switch tlsMinVersion := os.Getenv(tlsMinVersionEnvKey); tlsMinVersion {
	case "1.2":
		return tls.VersionTLS12
	case "1.3":
		return tls.VersionTLS13
	case "":
		return defaultTLSMinVersion
	default:
		panic(fmt.Sprintf("the environment variable %q has to be either '1.2' or '1.3'", tlsMinVersionEnvKey))
	}
}
