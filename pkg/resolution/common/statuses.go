/*
Copyright 2022 The Tekton Authors

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

package common

// processing reasons
const (
	// ReasonResolutionInProgress is used to indicate that there are
	// no issues with the parameters of a request and that a
	// resolver is working on the ResolutionRequest.
	ReasonResolutionInProgress = "ResolutionInProgress"
)

// happy reasons
const (
	// ReasonResolutionSuccessful is used to indicate that
	// resolution of a resource has completed successfully.
	ReasonResolutionSuccessful = "ResolutionSuccessful"
)

// unhappy reasons
const (
	// ReasonResolutionFailed indicates that some part of the resolution
	// process failed.
	ReasonResolutionFailed = "ResolutionFailed"

	// ReasonResolutionTimedOut indicates that a resolver did not
	// manage to respond to a ResolutionRequest within a timeout.
	ReasonResolutionTimedOut = "ResolutionTimedOut"
)
