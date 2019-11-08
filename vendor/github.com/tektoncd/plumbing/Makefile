# Copyright 2019 The Tekton Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

images: test-runner skopeo tkn

skopeo:
	docker build . -f tekton/images/skopeo/Dockerfile -t gcr.io/tekton-releases/dogfooding/skopeo

test-runner:
	docker build . -f tekton/images/test-runner/Dockerfile -t gcr.io/tekton-releases/dogfooding/test-runner

tkn:
	docker build . -f tekton/images/tkn/Dockerfile -t gcr.io/tekton-releases/dogfooding/tkn

push: images
	docker push gcr.io/tekton-releases/dogfooding/skopeo
	docker push gcr.io/tekton-releases/dogfooding/test-runner
	docker push gcr.io/tekton-releases/dogfooding/tkn
