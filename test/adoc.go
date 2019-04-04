/*
Copyright 2018 Knative Authors LLC
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
Package test holds the project's test helpers and end-to-end tests (e2e).

Create Pipeline resources

To create Tekton objects (e.g. Task, Pipeline, …), you
can use the builder (./builder) package to reduce noise:

	func MyTest(t *testing.T){
		// Pipeline
		pipeline := tb.Pipeline("tomatoes", "namespace",
			tb.PipelineSpec(tb.PipelineTask("foo", "banana")),
		)
	 	// … and PipelineRun
		pipelineRun := tb.PipelineRun("pear", "namespace",
			tb.PipelineRunSpec("tomatoes", tb.PipelineRunServiceAccount("inexistent")),
		)
		// And do something with them
		// […]
		if _, err := c.PipelineClient.Create(pipeline); err != nil {
			t.Fatalf("Failed to create Pipeline `%s`: %s", "tomatoes", err)
		}
		if _, err := c.PipelineRunClient.Create(pipelineRun); err != nil {
			t.Fatalf("Failed to create PipelineRun `%s`: %s", "pear", err)
		}
	}
*/
package test
