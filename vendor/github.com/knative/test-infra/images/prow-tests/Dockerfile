# Copyright 2018 The Knative Authors
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

FROM gcr.io/k8s-testimages/kubekins-e2e:v20180604-68e777553-experimental
LABEL maintainer "Adriano Cunha <adrcunha@google.com>"

# Install extras on top of base image

ENV DEBIAN_FRONTEND noninteractive
RUN apt-get update

# Docker
RUN gcloud components install docker-credential-gcr
RUN docker-credential-gcr configure-docker

# Extra tools through apt-get
RUN apt-get install -y uuid-runtime  # for uuidgen
RUN apt-get install -y npm  # for markdown-link-check
RUN apt-get install -y rubygems  # for mdl
RUN apt-get install -y build-essential libssl-dev # for wrk

# Extra tools through go get
RUN go get -u github.com/google/go-containerregistry/cmd/ko
RUN go get -u github.com/golang/dep/cmd/dep
RUN go get -u github.com/mattmoor/dep-collector
RUN go get -u github.com/google/licenseclassifier

# Extra tools through npm
RUN npm install -g markdown-link-check

# Extra tools through gem
RUN gem install mixlib-config -v 2.2.4  # required because ruby is 2.1
RUN gem install mdl

# Install wrk
RUN git clone https://github.com/wg/wrk.git wrk
RUN make -C wrk/
RUN cp wrk/wrk /usr/local/bin

# Extra custom tools
ADD githubhelper .
