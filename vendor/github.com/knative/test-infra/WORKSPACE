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

# Required rules for building kubernetes/test-infra
# These all come from http://github.com/kubernetes/test-infra/blob/master/WORKSPACE

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "1868ff68d6079e31b2f09b828b58d62e57ca8e9636edff699247c9108518570b",
    url = "https://github.com/bazelbuild/rules_go/releases/download/0.11.1/rules_go-0.11.1.tar.gz",
)

load("@io_bazel_rules_go//go:def.bzl", "go_rules_dependencies", "go_register_toolchains")

go_rules_dependencies()

go_register_toolchains(
    go_version = "1.10.2",
)

git_repository(
    name = "io_bazel_rules_k8s",
    commit = "3756369d4920033c32c12d16207e8ee14fee1b18",
    remote = "https://github.com/bazelbuild/rules_k8s.git",
)

http_archive(
    name = "io_bazel_rules_docker",
    sha256 = "cef4e7adfc1df999891e086bf42bed9092cfdf374adb902f18de2c1d6e1e0197",
    strip_prefix = "rules_docker-198367210c55fba5dded22274adde1a289801dc4",
    urls = ["https://github.com/bazelbuild/rules_docker/archive/198367210c55fba5dded22274adde1a289801dc4.tar.gz"],
)

# External repositories

git_repository(
    name = "k8s",
    remote = "http://github.com/kubernetes/test-infra.git",
    commit = "dd12621d6178838097847abf5842ad8d08fc9308",  # HEAD as of 8/1/2018
)

