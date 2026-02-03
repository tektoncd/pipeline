// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"
	"strings"

	version "github.com/hashicorp/go-version"
)

// ServerVersion returns the version of the server
func (c *Client) ServerVersion() (string, *Response, error) {
	v := struct {
		Version string `json:"version"`
	}{}
	resp, err := c.getParsedResponse("GET", "/version", nil, nil, &v)
	return v.Version, resp, err
}

// CheckServerVersionConstraint validates that the login's server satisfies a
// given version constraint such as ">= 1.11.0+dev"
func (c *Client) CheckServerVersionConstraint(constraint string) error {
	if err := c.loadServerVersion(); err != nil {
		return err
	}

	check, err := version.NewConstraint(constraint)
	if err != nil {
		return err
	}
	if !check.Check(c.serverVersion) {
		c.mutex.RLock()
		url := c.url
		c.mutex.RUnlock()
		return fmt.Errorf("gitea server at %s does not satisfy version constraint %s", url, constraint)
	}
	return nil
}

// SetGiteaVersion configures the Client to assume the given version of the
// Gitea server, instead of querying the server for it when initializing.
// Use "" to skip all canonical ways in the SDK to check for versions
func SetGiteaVersion(v string) ClientOption {
	if v == "" {
		return func(c *Client) error {
			c.ignoreVersion = true
			return nil
		}
	}
	return func(c *Client) (err error) {
		c.getVersionOnce.Do(func() {
			c.serverVersion, err = version.NewVersion(v)
		})
		return
	}
}

// predefined versions only have to be parsed by library once
var (
	version1_11_0 = version.Must(version.NewVersion("1.11.0"))
	version1_11_5 = version.Must(version.NewVersion("1.11.5"))
	version1_12_0 = version.Must(version.NewVersion("1.12.0"))
	version1_12_3 = version.Must(version.NewVersion("1.12.3"))
	version1_13_0 = version.Must(version.NewVersion("1.13.0"))
	version1_14_0 = version.Must(version.NewVersion("1.14.0"))
	version1_15_0 = version.Must(version.NewVersion("1.15.0"))
	version1_16_0 = version.Must(version.NewVersion("1.16.0"))
	version1_17_0 = version.Must(version.NewVersion("1.17.0"))
	version1_22_0 = version.Must(version.NewVersion("1.22.0"))
	version1_23_0 = version.Must(version.NewVersion("1.23.0"))
	version1_25_0 = version.Must(version.NewVersion("1.25.0"))
)

// ErrUnknownVersion is an unknown version from the API
type ErrUnknownVersion struct {
	raw string
}

// Error fulfills error
func (e *ErrUnknownVersion) Error() string {
	return fmt.Sprintf("unknown version: %s", e.raw)
}

func (*ErrUnknownVersion) Is(target error) bool {
	_, ok := target.(*ErrUnknownVersion)
	return ok
}

// checkServerVersionGreaterThanOrEqual is the canonical way in the SDK to check for versions for API compatibility reasons
func (c *Client) checkServerVersionGreaterThanOrEqual(v *version.Version) error {
	if c.ignoreVersion {
		return nil
	}
	if err := c.loadServerVersion(); err != nil {
		return err
	}

	if !c.serverVersion.GreaterThanOrEqual(v) {
		c.mutex.RLock()
		url := c.url
		c.mutex.RUnlock()
		return fmt.Errorf("gitea server at %s is older than %s", url, v.Original())
	}
	return nil
}

// loadServerVersion init the serverVersion variable
func (c *Client) loadServerVersion() (err error) {
	c.getVersionOnce.Do(func() {
		raw, _, err2 := c.ServerVersion()
		if err2 != nil {
			err = err2
			return
		}
		if c.serverVersion, err = version.NewVersion(raw); err != nil {
			if strings.TrimSpace(raw) != "" {
				// Version was something, just not recognized
				c.serverVersion = version1_11_0
				err = &ErrUnknownVersion{raw: raw}
			}
			return
		}
	})
	return
}
