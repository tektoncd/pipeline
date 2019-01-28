# dep-collector

`dep-collector` is a tool for gathering up a collection of licenses for Go
dependencies that have been pulled into the idiomatic `vendor/` directory.
The resulting file from running `dep-collector` is intended for inclusion
in container images to respect the licenses of the included software.

## Basic Usage

You can run `dep-collector` on one or more Go import paths as entrypoints,
and it will:

1. Walk the transitive dependencies to identify vendored software packages,
1. Search for licenses for each vendored dependency,
1. Dump a file containing the licenses for each vendored import.

For example (single import path):

```shell
$ dep-collector .
===========================================================
Import: github.com/mattmoor/dep-collector/vendor/github.com/google/licenseclassifier

                                 Apache License
                           Version 2.0, January 2004
                        http://www.apache.org/licenses/
...

```

For example (multiple import paths):

```shell
$ dep-collector ./cmd/controller ./cmd/sleeper

===========================================================
Import: github.com/mattmoor/warm-image/vendor/cloud.google.com/go

                                 Apache License
                           Version 2.0, January 2004
                        http://www.apache.org/licenses/
```

## CSV Usage

You can also run `dep-collector` in a mode that produces CSV output,
including basic classification of the license.

> In order to run dep-collector in this mode, you must first run:
> go get github.com/google/licenseclassifier

For example:

```shell
$ dep-collector -csv .
github.com/google/licenseclassifier,Static,,https://github.com/mattmoor/dep-collector/blob/master/vendor/github.com/google/licenseclassifier/LICENSE,Apache-2.0
github.com/google/licenseclassifier/stringclassifier,Static,,https://github.com/mattmoor/dep-collector/blob/master/vendor/github.com/google/licenseclassifier/stringclassifier/LICENSE,Apache-2.0
github.com/sergi/go-diff,Static,,https://github.com/mattmoor/dep-collector/blob/master/vendor/github.com/sergi/go-diff/LICENSE,MIT

```

The columns here are:

* Import Path,
* How the dependency is linked in (always reports "static"),
* A column for whether any modifications have been made (always empty),
* The URL by which to access the license file (assumes `master`),
* A classification of what license this is ([using this](https://github.com/google/licenseclassifier)).

## Check mode

`dep-collector` also includes a mode that will check for "forbidden" licenses.

> In order to run dep-collector in this mode, you must first run:
> go get github.com/google/licenseclassifier

For example (failing):

```shell
$ dep-collector -check ./foo/bar/baz
2018/07/20 22:01:29 Error checking license collection: Errors validating licenses:
Found matching forbidden license in "foo.io/bar/vendor/github.com/BurntSushi/toml":WTFPL
```

For example (passing):

```shell
$ dep-collector -check .
2018/07/20 22:29:09 No errors found.
```
