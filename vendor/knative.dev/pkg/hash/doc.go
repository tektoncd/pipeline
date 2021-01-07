/*
Copyright 2020 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package hash contains various Knative specific hashing utilities.
//
// - ChooseSubset is a consistent hashing/mapping function providing
//   a consistent selection of N keys from M (N<=M) keys for a given
//   target.
// - BucketSet is a bucketer library which uses ChooseSubset under the
//   the hood in order to implement consistent mapping between keys and
//   set of buckets, identified by unique names. Compared to basic bucket
//   implementation which just does hash%num_buckets, when the number of
//   buckets change only a small subset of keys are supposed to migrate.
package hash
