package factory

import (
	"fmt"
	"os"
	"strings"
)

// HostDriverIdentifier is a mapping of hostname to scm driver.
type HostDriverIdentifier map[string]string

// Identify looks up the provided hostname, and returns the driver mapping.
//
// If no mapping exists, then it returns an error.
func (u HostDriverIdentifier) Identify(host string) (string, error) {
	d, ok := u[host]
	if ok {
		return d, nil
	}
	return "", unknownDriverError{hostname: host}
}

// MappingFunc is a type for adding names to the list of mappings from hosts to
// drivers.
type MappingFunc func(HostDriverIdentifier)

// Mapping adds a host,driver combination to the DriverIdentifier.
func Mapping(host, driver string) MappingFunc {
	return func(m HostDriverIdentifier) {
		m[host] = driver
	}
}

// NewDriverIdentifier creates and returns a new HostDriverIdentifier.
func NewDriverIdentifier(extras ...MappingFunc) HostDriverIdentifier {
	u := HostDriverIdentifier{
		"github.com": "github",
		"gitlab.com": "gitlab",
	}
	for _, e := range extras {
		e(u)
	}
	e := os.Getenv("GIT_DRIVERS")
	for _, v := range strings.Split(e, ",") {
		parts := strings.Split(v, "=")
		if len(parts) == 2 {
			u[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return u
}

type unknownDriverError struct {
	hostname string
}

func (e unknownDriverError) Error() string {
	return fmt.Sprintf("unable to identify driver from hostname: %s", e.hostname)
}
