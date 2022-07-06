// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scm

import (
	"strings"
)

// Split splits the full repository name into segments.
func Split(s string) (owner, name string) {
	parts := strings.SplitN(s, "/", 2)
	switch len(parts) {
	case 1:
		name = parts[0]
	case 2:
		owner = parts[0]
		name = parts[1]
	}
	return
}

// Join joins the repository owner and name segments to
// create a fully qualified repository name.
func Join(owner, name string) string {
	return owner + "/" + name
}

// URLJoin joins the given paths so that there is only ever one '/' character between the paths
func URLJoin(paths ...string) string {
	var buffer strings.Builder
	last := len(paths) - 1
	for i, path := range paths {
		p := path
		if i > 0 {
			buffer.WriteString("/")
			p = strings.TrimPrefix(p, "/")
		}
		if i < last {
			p = strings.TrimSuffix(p, "/")
		}
		buffer.WriteString(p)
	}
	return buffer.String()
}

// TrimRef returns ref without the path prefix.
func TrimRef(ref string) string {
	ref = strings.TrimPrefix(ref, "refs/heads/")
	ref = strings.TrimPrefix(ref, "refs/tags/")
	return ref
}

// ExpandRef returns name expanded to the fully qualified
// reference path (e.g refs/heads/master).
func ExpandRef(name, prefix string) string {
	prefix = strings.TrimSuffix(prefix, "/")
	if strings.HasPrefix(name, "refs/") {
		return name
	}
	return prefix + "/" + name
}

// IsTag returns true if the reference path points to
// a tag object.
func IsTag(ref string) bool {
	return strings.HasPrefix(ref, "refs/tags/")
}

// ConvertStatusInputsToStatuses converts the inputs to status objects
func ConvertStatusInputsToStatuses(inputs []*StatusInput) []*Status {
	answer := []*Status{}
	for _, input := range inputs {
		answer = append(answer, ConvertStatusInputToStatus(input))
	}
	return answer
}

// ConvertStatusInputToStatus converts the input to a status
func ConvertStatusInputToStatus(input *StatusInput) *Status {
	if input == nil {
		return nil
	}
	return &Status{
		State:  input.State,
		Label:  input.Label,
		Desc:   input.Desc,
		Target: input.Target,
	}
}

// IsScmNotFound returns true if the resource is not found
func IsScmNotFound(err error) bool {
	if err != nil {
		// I think that we should instead rely on the http status (404)
		// until jenkins-x go-scm is updated t return that in the error this works for github and gitlab
		return strings.Contains(err.Error(), ErrNotFound.Error())
	}
	return false
}
