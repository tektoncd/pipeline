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

package apis

import (
	"context"

	authenticationv1 "k8s.io/api/authentication/v1"
)

// This is attached to contexts passed to webhook interfaces when
// the receiver being validated is being created.
type inCreateKey struct{}

// WithinCreate is used to note that the webhook is calling within
// the context of a Create operation.
func WithinCreate(ctx context.Context) context.Context {
	return context.WithValue(ctx, inCreateKey{}, struct{}{})
}

// IsInCreate checks whether the context is a Create.
func IsInCreate(ctx context.Context) bool {
	return ctx.Value(inCreateKey{}) != nil
}

// This is attached to contexts passed to webhook interfaces when
// the receiver being validated is being updated.
type inUpdateKey struct{}

// WithinUpdate is used to note that the webhook is calling within
// the context of a Update operation.
func WithinUpdate(ctx context.Context, base interface{}) context.Context {
	return context.WithValue(ctx, inUpdateKey{}, base)
}

// IsInUpdate checks whether the context is an Update.
func IsInUpdate(ctx context.Context) bool {
	return ctx.Value(inUpdateKey{}) != nil
}

// GetBaseline returns the baseline of the update, or nil when we
// are not within an update context.
func GetBaseline(ctx context.Context) interface{} {
	return ctx.Value(inUpdateKey{})
}

// This is attached to contexts passed to webhook interfaces when
// the receiver being validated is being created.
type userInfoKey struct{}

// WithUserInfo is used to note that the webhook is calling within
// the context of a Create operation.
func WithUserInfo(ctx context.Context, ui *authenticationv1.UserInfo) context.Context {
	return context.WithValue(ctx, userInfoKey{}, ui)
}

// GetUserInfo accesses the UserInfo attached to the webhook context.
func GetUserInfo(ctx context.Context) *authenticationv1.UserInfo {
	if ui, ok := ctx.Value(userInfoKey{}).(*authenticationv1.UserInfo); ok {
		return ui
	}
	return nil
}
