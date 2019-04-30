/*
Copyright 2018 The Knative Authors
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

/*
Package builder holds Builder functions that can be used to create
struct in tests with less noise.

One of the most important characteristic of a unit test (and any type
of test really) is readability. This means it should be easy to read
but most importantly it should clearly show the intent of the
test. The setup (and cleanup) of the tests should be as small as
possible to avoid the noise. Those builders exists to help with that.

There is two types of functions defined in that package :

	* Builders: create and return a struct
	* Modifiers: return a function
	  that will operate on a given struct. They can be applied to other
	  Modifiers or Builders.

Most of the Builder (and Modifier) that accepts Modifiers defines a
type (`TypeOp`) that can be satisfied by existing function in this
package, from other package *or* inline. An example would be the
following.

	// Definition
	type TaskOp func(*v1alpha1.Task)
	func Task(name string, ops ...TaskOp) *v1alpha1.Task {
		// […]
	}
	func TaskNamespace(namespace string) TaskOp {
		return func(t *v1alpha1.Task) {
			// […]
		}
	}
	// Usage
	t := Task("foo", TaskNamespace("bar"), func(t *v1alpha1.Task){
		// Do something with the Task struct
		// […]
	})

The main reason to define the `Op` type, and using it in the methods
signatures is to group Modifier function together. It makes it easier
to see what is a Modifier (or Builder) and on what it operates.

By convention, this package is import with the "tb" as alias. The
examples make that assumption.

*/
package builder
