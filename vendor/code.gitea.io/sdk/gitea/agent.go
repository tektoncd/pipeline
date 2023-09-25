// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

//go:build !windows

package gitea

import (
	"fmt"
	"net"
	"os"

	"golang.org/x/crypto/ssh/agent"
)

// hasAgent returns true if the ssh agent is available
func hasAgent() bool {
	if _, err := os.Stat(os.Getenv("SSH_AUTH_SOCK")); err != nil {
		return false
	}

	return true
}

// GetAgent returns a ssh agent
func GetAgent() (agent.Agent, error) {
	if !hasAgent() {
		return nil, fmt.Errorf("no ssh agent available")
	}

	sshAgent, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
	if err != nil {
		return nil, err
	}

	return agent.NewClient(sshAgent), nil
}
