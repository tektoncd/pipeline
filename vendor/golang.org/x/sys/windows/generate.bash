#!/bin/bash

# Copyright 2019 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

set -e

# For successful zsyscall_windows.go generation on non-WSL env, this should
# run before mkerrors.bash.
echo "# Generating zsyscall_windows.go ..."
go run $(go env GOROOT)/src/syscall/mksyscall_windows.go -output zsyscall_windows.go eventlog.go service.go syscall_windows.go security_windows.go

echo "# Generating zerrors_windows.go ..."
./mkerrors.bash zerrors_windows.go
