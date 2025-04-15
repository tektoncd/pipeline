#!/usr/bin/env bash

# Copyright 2019 The Knative Authors
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

# -count argument when running go test
readonly BENCH_RUN_COUNT=${BENCH_RUN_COUNT:-5}

# -benchtime argument when running go test
readonly BENCH_RUN_TIME=${BENCH_RUN_TIME:-5s}

function microbenchmarks_run() {
  if [ "$1" != "" ]; then
    OUTPUT_FILE="$1"
  else
    OUTPUT_FILE="${ARTIFACTS:-$(mktemp -d)}/bench-result.txt"
  fi

  # Run all microbenchmarks
  go clean
  go test -bench=. -benchtime=$BENCH_RUN_TIME -count=$BENCH_RUN_COUNT -benchmem -run="^$" -v ./...   >> "$OUTPUT_FILE" || exit
}

# To run microbenchmarks on your machine and compare your revision with upstream/main:
#
# git fetch upstream
# source microbenchmarks.sh
# microbenchmarks_run_and_compare upstream/main
#
# NOTE: Hypothetically we should run these microbenchmarks on a machine running only a kernel and a shell,
# but this might be sometimes hard to achieve unless you have a spare computer to play with.
# When running microbenchmarks on your dev machine, make sure you have the bare minimum applications running on your laptop,
# otherwise you'll get a variance too high.
# Close Slack, the browser, the IDE, stop Docker, stop Netflix and grab a coffee while waiting for the results.
# Depending on the benchmark complexity/reproducibility, if the variance is too high, repeat the benchmarks.
function microbenchmarks_run_and_compare() {
  if [ "$1" == "" ] || [ $# -gt 1 ]; then
    echo "Error: Expecting an argument" >&2
    echo "usage: $(basename $0) revision_to_compare" >&2
    exit 1
  fi

  if [ "$2" == "" ]; then
    OUTPUT_DIR=${ARTIFACTS:-$(mktemp -d)}
  else
    OUTPUT_DIR="$2"
  fi

  mkdir -p "$OUTPUT_DIR"

  # Benchstat is required to compare the bench results
  GO111MODULE=off go get golang.org/x/perf/cmd/benchstat

  # Revision to use to compare with actual
  REVISION="$1"

  echo "--- Outputs will be at $OUTPUT_DIR"

  # Run this revision benchmarks
  microbenchmarks_run "$OUTPUT_DIR/new.txt"

  echo "--- This revision results:"
  cat "$OUTPUT_DIR/new.txt"

  # Run other revision benchmarks
  git checkout "$REVISION"
  microbenchmarks_run "$OUTPUT_DIR/old.txt"

  echo "--- $REVISION results:"
  cat "$OUTPUT_DIR/old.txt"

  # Print results in console
  echo "--- Benchstat:"
  benchstat "$OUTPUT_DIR/old.txt" "$OUTPUT_DIR/new.txt"

  # Generate html results
  benchstat -html "$OUTPUT_DIR/old.txt" "$OUTPUT_DIR/new.txt" >> "$OUTPUT_DIR/results.html"
}
