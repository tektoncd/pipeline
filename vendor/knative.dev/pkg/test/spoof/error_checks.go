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

// spoof contains logic to make polling HTTP requests against an endpoint with optional host spoofing.

package spoof

import (
	"errors"
	"net"
	"strings"
)

func isTCPTimeout(err error) bool {
	if err == nil {
		return false
	}
	var errNet net.Error
	if !errors.As(err, &errNet) {
		return false
	}
	return errNet.Timeout()
}

func isDNSError(err error) bool {
	if err == nil {
		return false
	}
	// Checking by casting to url.Error and casting the nested error
	// seems to be not as robust as string check.
	msg := strings.ToLower(err.Error())
	// Example error message:
	//   > Get http://this.url.does.not.exist: dial tcp: lookup this.url.does.not.exist on 127.0.0.1:53: no such host
	return strings.Contains(msg, "no such host") || strings.Contains(msg, ":53")
}

func isConnectionRefused(err error) bool {
	// The alternative for the string check is:
	// 	errNo := (((err.(*url.Error)).Err.(*net.OpError)).Err.(*os.SyscallError).Err).(syscall.Errno)
	// if errNo == syscall.Errno(0x6f) {...}
	// But with assertions, of course.
	return err != nil && strings.Contains(err.Error(), "connect: connection refused")
}

func isConnectionReset(err error) bool {
	return err != nil && strings.Contains(err.Error(), "connection reset by peer")
}
