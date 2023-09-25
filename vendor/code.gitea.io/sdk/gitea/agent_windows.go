// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

//go:build windows

package gitea

import (
	"fmt"

	"github.com/davidmz/go-pageant"
	"golang.org/x/crypto/ssh/agent"
)

// hasAgent returns true if pageant is available
func hasAgent() bool {
	return pageant.Available()
}

// GetAgent returns a ssh agent
func GetAgent() (agent.Agent, error) {
	if !hasAgent() {
		return nil, fmt.Errorf("no pageant available")
	}

	return pageant.New(), nil
}
