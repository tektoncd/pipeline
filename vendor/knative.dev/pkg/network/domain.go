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

package network

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

const (
	resolverFileName    = "/etc/resolv.conf"
	clusterDomainEnvKey = "CLUSTER_DOMAIN"
	defaultDomainName   = "cluster.local"
)

var (
	domainName = defaultDomainName
	once       sync.Once
)

// GetServiceHostname returns the fully qualified service hostname
func GetServiceHostname(name, namespace string) string {
	return fmt.Sprintf("%s.%s.svc.%s", name, namespace, GetClusterDomainName())
}

// GetClusterDomainName returns cluster's domain name or an error
// Closes issue: https://github.com/knative/eventing/issues/714
func GetClusterDomainName() string {
	once.Do(func() {
		f, err := os.Open(resolverFileName)
		if err != nil {
			return
		}
		defer f.Close()
		domainName = getClusterDomainName(f)
	})
	return domainName
}

func getClusterDomainName(r io.Reader) string {
	// First look in the conf file.
	for scanner := bufio.NewScanner(r); scanner.Scan(); {
		elements := strings.Split(scanner.Text(), " ")
		if elements[0] != "search" {
			continue
		}
		for _, e := range elements[1:] {
			if strings.HasPrefix(e, "svc.") {
				return strings.TrimSuffix(e[4:], ".")
			}
		}
	}

	// Then look in the ENV.
	if domain := os.Getenv(clusterDomainEnvKey); len(domain) > 0 {
		return domain
	}

	// For all abnormal cases return default domain name.
	return defaultDomainName
}
