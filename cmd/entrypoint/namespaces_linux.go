/*
Copyright 2023 The Tekton Authors

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

package main

import (
	"math"
	"os/exec"
	"syscall"
)

// We need the max value of an unsigned 32 bit integer (4294967295), but we also need this number
// to fit into an "int". One some systems this is 32 bits, so the max uint32 can't fit into here.
// maxIntForArch is the higher of those two values.
func maxIntForArch() int {
	// We have to do over two lines as a variable. The go compiler optimizes
	// away types for constants, so int(uint32(math.MaxUint32)) is the same as int(math.MaxUint32),
	// which overflows.
	maxUint := uint32(math.MaxUint32)
	if int(maxUint) > math.MaxInt32 {
		return int(maxUint)
	}
	return math.MaxInt32
}

// dropNetworking modifies the supplied exec.Cmd to execute in a net set of namespaces that do not
// have network access
func dropNetworking(cmd *exec.Cmd) {
	// These flags control the behavior of the new process.
	// Documentation for these is available here: https://man7.org/linux/man-pages/man2/clone.2.html
	// We mostly want to just create a new network namespace, unattached to any networking devices.
	// The other flags are necessary for that to work.

	if cmd.SysProcAttr == nil {
		// We build this up piecemeal in case it was already set, to avoid overwriting anything.
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Cloneflags = syscall.CLONE_NEWNS |
		syscall.CLONE_NEWPID | // NEWPID creates a new process namespace
		syscall.CLONE_NEWNET | // NEWNET creates a new network namespace (this is the one we really care about)
		syscall.CLONE_NEWUSER // NEWUSER creates a new user namespace

	// We need to map the existing user IDs into the new namespace.
	// Just map everything.
	cmd.SysProcAttr.UidMappings = []syscall.SysProcIDMap{
		{
			ContainerID: 0,
			HostID:      0,
			// Map all users
			Size: maxIntForArch(),
		},
	}

	// This is needed to allow programs to call setgroups when in a new Gid namespace.
	// Things like apt-get install require this to work.
	cmd.SysProcAttr.GidMappingsEnableSetgroups = true
	// We need to map the existing group IDs into the new namespace.
	// Just map everything.
	cmd.SysProcAttr.GidMappings = []syscall.SysProcIDMap{
		{
			ContainerID: 0,
			HostID:      0,

			//  Map all groups
			Size: maxIntForArch(),
		},
	}
}
