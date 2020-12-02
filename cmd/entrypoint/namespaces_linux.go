package main

import (
	"os/exec"
	"syscall"
)

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
			Size: 4294967295,
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
			Size: 4294967295,
		},
	}
}
