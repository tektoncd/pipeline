#!/bin/bash

# Usage: create-release-branch.sh v0.4.1 release-0.4

release=$1
target=$2

# Fetch the latest tags and checkout a new branch from the wanted tag.
git fetch upstream --tags
git checkout -b "$target" "$release"

# Update openshift's master and take all needed files from there.
git fetch openshift master
git checkout openshift/master -- openshift OWNERS_ALIASES OWNERS Makefile
make generate-dockerfiles
git add openshift OWNERS_ALIASES OWNERS Makefile
git commit -m "Add openshift specific files."