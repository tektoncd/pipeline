/*
Copyright 2020 The Knative Authors

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
Package logging assists setting up test logging and using leveled logging in tests.

The TLogger is designed to assist the test writer in creating more useful tests and
collecting log data in multiple streams, optimizing for human readability in one and
machine readability in another. It's designed to mimic the testing.T object rather closely and
use Zap logging semantics, both things already in use in Knative, to minimize the time developers
need to spend learning the tool.

Inspired by and uses go-logr.

Advantages

The TLogger enhances test design through subtle nudges and affordances:

* It encourages only logging with .V(), giving the writer a nudge to think about how important it is,
but without requiring them to fit it in a narrowly-defined category.

* Reduces boilerplate of carrying around context for errors in several different variables,
using .WithValues(), which results in more consistent and reusable code across the tests.

Porting

To port code from using testing.T to logging.TLogger, the interfaces knative.dev/pkg/test.T and
knative.dev/pkg/test.TLegacy have been created. All library functions should be refactored to use
one interface and all .Log() calls rewritten to use structured format, which works with testing and
TLogger. If a library function needs test functions not available even in test.TLegacy,
it's probably badly written.

Then any test can be incrementally rewritten to use TLogger, as it coexists with testing.T without issue.

*/
package logging
