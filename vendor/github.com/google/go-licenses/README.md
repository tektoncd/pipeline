# Licenses tool

> This is not an officially supported Google product.

`go-licenses` analyzes the dependency tree of a Go package/binary. It can output a
report on the libraries used and under what license they can be used. It can
also collect all of the license documents, copyright notices and source code
into a directory in order to comply with license terms on redistribution.

## Before you start

To use this tool, make sure:

* [You have Go v1.16 or later installed](https://golang.org/dl/).
* Change directory to your go project, **for example**:

  ```shell
  git clone git@github.com:google/go-licenses.git
  cd go-licenses
  ```

* Download required modules:

  ```shell
  go mod download
  ```

## Installation

Use the following command to download and install this tool:

```shell
go install github.com/google/go-licenses@latest
```

If you were using `go get` to install this tool, note that
[starting in Go 1.17, go get is deprecated for installing binaries](https://go.dev/doc/go-get-install-deprecation).

## Reports

```shell
$ go-licenses report github.com/google/go-licenses
W0410 06:02:57.077781   31529 library.go:86] "golang.org/x/sys/unix" contains non-Go code that can't be inspected for further dependencies:
/home/username/go/pkg/mod/golang.org/x/sys@v0.0.0-20220111092808-5a964db01320/unix/asm_linux_amd64.s
W0410 06:02:59.476443   31529 library.go:86] "golang.org/x/crypto/curve25519/internal/field" contains non-Go code that can't be inspected for further dependencies:
/home/username/go/pkg/mod/golang.org/x/crypto@v0.0.0-20220112180741-5e0467b6c7ce/curve25519/internal/field/fe_amd64.s
W0410 06:02:59.486045   31529 library.go:86] "golang.org/x/crypto/internal/poly1305" contains non-Go code that can't be inspected for further dependencies:
/home/username/go/pkg/mod/golang.org/x/crypto@v0.0.0-20220112180741-5e0467b6c7ce/internal/poly1305/sum_amd64.s
W0410 06:02:59.872215   31529 library.go:253] module github.com/google/go-licenses has empty version, defaults to HEAD. The license URL may be incorrect. Please verify!
W0410 06:02:59.880621   31529 library.go:253] module github.com/google/go-licenses has empty version, defaults to HEAD. The license URL may be incorrect. Please verify!
github.com/emirpasic/gods,https://github.com/emirpasic/gods/blob/v1.12.0/LICENSE,BSD-2-Clause
github.com/golang/glog,https://github.com/golang/glog/blob/23def4e6c14b/LICENSE,Apache-2.0
github.com/golang/groupcache/lru,https://github.com/golang/groupcache/blob/41bb18bfe9da/LICENSE,Apache-2.0
github.com/google/go-licenses,https://github.com/google/go-licenses/blob/HEAD/LICENSE,Apache-2.0
github.com/google/go-licenses/internal/third_party/pkgsite,https://github.com/google/go-licenses/blob/HEAD/internal/third_party/pkgsite/LICENSE,BSD-3-Clause
github.com/google/licenseclassifier,https://github.com/google/licenseclassifier/blob/3043a050f148/LICENSE,Apache-2.0
github.com/google/licenseclassifier/stringclassifier,https://github.com/google/licenseclassifier/blob/3043a050f148/stringclassifier/LICENSE,Apache-2.0
github.com/jbenet/go-context/io,https://github.com/jbenet/go-context/blob/d14ea06fba99/LICENSE,MIT
github.com/kevinburke/ssh_config,https://github.com/kevinburke/ssh_config/blob/01f96b0aa0cd/LICENSE,MIT
github.com/mitchellh/go-homedir,https://github.com/mitchellh/go-homedir/blob/v1.1.0/LICENSE,MIT
github.com/otiai10/copy,https://github.com/otiai10/copy/blob/v1.6.0/LICENSE,MIT
github.com/sergi/go-diff/diffmatchpatch,https://github.com/sergi/go-diff/blob/v1.2.0/LICENSE,MIT
github.com/spf13/cobra,https://github.com/spf13/cobra/blob/v1.4.0/LICENSE.txt,Apache-2.0
github.com/spf13/pflag,https://github.com/spf13/pflag/blob/v1.0.5/LICENSE,BSD-3-Clause
github.com/src-d/gcfg,https://github.com/src-d/gcfg/blob/v1.4.0/LICENSE,BSD-3-Clause
github.com/xanzy/ssh-agent,https://github.com/xanzy/ssh-agent/blob/v0.2.1/LICENSE,Apache-2.0
go.opencensus.io,https://github.com/census-instrumentation/opencensus-go/blob/v0.23.0/LICENSE,Apache-2.0
golang.org/x/crypto,https://cs.opensource.google/go/x/crypto/+/5e0467b6:LICENSE,BSD-3-Clause
golang.org/x/mod/semver,https://cs.opensource.google/go/x/mod/+/9b9b3d81:LICENSE,BSD-3-Clause
golang.org/x/net,https://cs.opensource.google/go/x/net/+/69e39bad:LICENSE,BSD-3-Clause
golang.org/x/sys,https://cs.opensource.google/go/x/sys/+/5a964db0:LICENSE,BSD-3-Clause
golang.org/x/tools,https://cs.opensource.google/go/x/tools/+/v0.1.10:LICENSE,BSD-3-Clause
golang.org/x/xerrors,https://cs.opensource.google/go/x/xerrors/+/5ec99f83:LICENSE,BSD-3-Clause
gopkg.in/src-d/go-billy.v4,https://github.com/src-d/go-billy/blob/v4.3.2/LICENSE,Apache-2.0
gopkg.in/src-d/go-git.v4,https://github.com/src-d/go-git/blob/v4.13.1/LICENSE,Apache-2.0
gopkg.in/warnings.v0,https://github.com/go-warnings/warnings/blob/v0.1.2/LICENSE,BSD-2-Clause
```

This command prints out a comma-separated report (CSV) listing the libraries
used by a binary/package, the URL where their licenses can be viewed and the
type of license. A library is considered to be one or more Go packages that
share a license file.

URLs are versioned based on go modules metadata.

**Tip**: go-licenses writes the report to stdout and info/warnings/errors logs
to stderr. To save the CSV to a file `licenses.csv` in bash, run:

```bash
go-licenses report github.com/google/go-licenses > licenses.csv
```

Or, to also save error logs to an `errors` file, run:

```bash
go-licenses report github.com/google/go-licenses > licenses.csv 2> errors
```

**Note**: some warnings and errors may be expected, refer to [Warnings and Errors](#warnings-and-errors) for more information.

## Reports with Custom Templates

```shell
go-licenses report github.com/google/go-licenses --template testdata/modules/hello01/licenses.tpl
W0822 16:56:50.696198   10200 library.go:94] "golang.org/x/sys/unix" contains non-Go code that can't be inspected for further dependencies:
/Users/willnorris/go/pkg/mod/golang.org/x/sys@v0.0.0-20220722155257-8c9f86f7a55f/unix/asm_bsd_arm64.s
/Users/willnorris/go/pkg/mod/golang.org/x/sys@v0.0.0-20220722155257-8c9f86f7a55f/unix/zsyscall_darwin_arm64.1_13.s
/Users/willnorris/go/pkg/mod/golang.org/x/sys@v0.0.0-20220722155257-8c9f86f7a55f/unix/zsyscall_darwin_arm64.s
W0822 16:56:51.466449   10200 library.go:94] "golang.org/x/crypto/chacha20" contains non-Go code that can't be inspected for further dependencies:
/Users/willnorris/go/pkg/mod/golang.org/x/crypto@v0.0.0-20220112180741-5e0467b6c7ce/chacha20/chacha_arm64.s
W0822 16:56:51.475139   10200 library.go:94] "golang.org/x/crypto/curve25519/internal/field" contains non-Go code that can't be inspected for further dependencies:
/Users/willnorris/go/pkg/mod/golang.org/x/crypto@v0.0.0-20220112180741-5e0467b6c7ce/curve25519/internal/field/fe_arm64.s
W0822 16:56:51.602250   10200 library.go:269] module github.com/google/go-licenses has empty version, defaults to HEAD. The license URL may be incorrect. Please verify!
W0822 16:56:51.605074   10200 library.go:269] module github.com/google/go-licenses has empty version, defaults to HEAD. The license URL may be incorrect. Please verify!

 - github.com/emirpasic/gods ([BSD-2-Clause](https://github.com/emirpasic/gods/blob/v1.12.0/LICENSE))
 - github.com/golang/glog ([Apache-2.0](https://github.com/golang/glog/blob/23def4e6c14b/LICENSE))
 - github.com/golang/groupcache/lru ([Apache-2.0](https://github.com/golang/groupcache/blob/41bb18bfe9da/LICENSE))
 - github.com/google/go-licenses ([Apache-2.0](https://github.com/google/go-licenses/blob/HEAD/LICENSE))
 - github.com/google/go-licenses/internal/third_party/pkgsite ([BSD-3-Clause](https://github.com/google/go-licenses/blob/HEAD/internal/third_party/pkgsite/LICENSE))
 - github.com/google/licenseclassifier ([Apache-2.0](https://github.com/google/licenseclassifier/blob/3043a050f148/LICENSE))
 - github.com/google/licenseclassifier/licenses ([Unlicense](https://github.com/google/licenseclassifier/blob/3043a050f148/licenses/Unlicense.txt))
 - github.com/google/licenseclassifier/stringclassifier ([Apache-2.0](https://github.com/google/licenseclassifier/blob/3043a050f148/stringclassifier/LICENSE))
 - github.com/jbenet/go-context/io ([MIT](https://github.com/jbenet/go-context/blob/d14ea06fba99/LICENSE))
 - github.com/kevinburke/ssh_config ([MIT](https://github.com/kevinburke/ssh_config/blob/01f96b0aa0cd/LICENSE))
 - github.com/mitchellh/go-homedir ([MIT](https://github.com/mitchellh/go-homedir/blob/v1.1.0/LICENSE))
 - github.com/otiai10/copy ([MIT](https://github.com/otiai10/copy/blob/v1.6.0/LICENSE))
 - github.com/sergi/go-diff/diffmatchpatch ([MIT](https://github.com/sergi/go-diff/blob/v1.2.0/LICENSE))
 - github.com/spf13/cobra ([Apache-2.0](https://github.com/spf13/cobra/blob/v1.5.0/LICENSE.txt))
 - github.com/spf13/pflag ([BSD-3-Clause](https://github.com/spf13/pflag/blob/v1.0.5/LICENSE))
 - github.com/src-d/gcfg ([BSD-3-Clause](https://github.com/src-d/gcfg/blob/v1.4.0/LICENSE))
 - github.com/xanzy/ssh-agent ([Apache-2.0](https://github.com/xanzy/ssh-agent/blob/v0.2.1/LICENSE))
 - go.opencensus.io ([Apache-2.0](https://github.com/census-instrumentation/opencensus-go/blob/v0.23.0/LICENSE))
 - golang.org/x/crypto ([BSD-3-Clause](https://cs.opensource.google/go/x/crypto/+/5e0467b6:LICENSE))
 - golang.org/x/mod/semver ([BSD-3-Clause](https://cs.opensource.google/go/x/mod/+/86c51ed2:LICENSE))
 - golang.org/x/net ([BSD-3-Clause](https://cs.opensource.google/go/x/net/+/a158d28d:LICENSE))
 - golang.org/x/sys ([BSD-3-Clause](https://cs.opensource.google/go/x/sys/+/8c9f86f7:LICENSE))
 - golang.org/x/tools ([BSD-3-Clause](https://cs.opensource.google/go/x/tools/+/v0.1.12:LICENSE))
 - gopkg.in/src-d/go-billy.v4 ([Apache-2.0](https://github.com/src-d/go-billy/blob/v4.3.2/LICENSE))
 - gopkg.in/src-d/go-git.v4 ([Apache-2.0](https://github.com/src-d/go-git/blob/v4.13.1/LICENSE))
 - gopkg.in/warnings.v0 ([BSD-2-Clause](https://github.com/go-warnings/warnings/blob/v0.1.2/LICENSE))
```

This command executes a specified Go template file to generate a report of
licenses.  The template file is passed a slice of structs containing license
data:

```go
[]struct {
  Name        string
  Version     string
  LicenseURL  string
  LicenseName string
  LicensePath string
}
```

Each struct also has a `LicenseText` method which will return the text of the license stored at `LicensePath` if present,
or an empty string if not.

Example template rendering licenses as markdown:

````
{{ range . }}
## {{ .Name }}

* Name: {{ .Name }}
* Version: {{ .Version }}
* License: [{{ .LicenseName }}]({{ .LicenseURL }})

```
{{ .LicenseText }}
```
{{ end }}
````

## Save licenses, copyright notices and source code (depending on license type)

```shell
go-licenses save "github.com/google/go-licenses" --save_path="/tmp/go-licenses-cli"
```

This command analyzes a binary/package's dependencies and determines what needs
to be redistributed alongside that binary/package in order to comply with the
license terms. This typically includes the license itself and a copyright
notice, but may also include the dependency's source code. All of the required
artifacts will be saved in the directory indicated by `--save_path`.

## Checking for forbidden licenses

```shell
$ go-licenses check github.com/logrusorgru/aurora
Forbidden license type WTFPL for library github.com/logrusorgru/auroraexit status 1
```

This command analyzes a package's dependencies and determines if any are
considered forbidden by the license classifer. See
[github.com/google/licenseclassifier](https://github.com/google/licenseclassifier/blob/842c0d70d7027215932deb13801890992c9ba364/license_type.go#L323)
for licenses considered forbidden.

## Usages

### Global
Typically, specify the Go package that builds your Go binary.
go-licenses expects the same package argument format as `go build`.  For examples:

* A rooted import path like `github.com/google/go-licenses` or `github.com/google/go-licenses/licenses`.
* A relative path that denotes the package in that directory, like `.` or `./cmd/some-command`.

To learn more about package argument, run `go help packages`.

To learn more about go-licenses usages, run `go-licenses help`.

### Report

Report usage (default csv output):

```shell
go-licenses report <package> [package...]
```

Report usage (using custom template file):

```shell
go-licenses report <package> [package...] --template=<template_file>
```

### Save

Save licenses, copyright notices and source code (depending on license type):

```shell
go-licenses save <package> [package...] --save_path=<save_path>
```

### Check

Checking for forbidden and unknown licenses usage:

```shell
go-licenses check <package> [package...]
```

**Tip**: Usually you'll want to

* append `/...` to the end of an import path prefix (e.g., your repo path) to include all packages matching that pattern
* add `--include_tests` to also check packages only imported by testing code (e.g., testing libraries/frameworks)

```shell
go-licenses check --include_tests github.com/google/go-licenses/...
```

Checking for disallowed license types:

```shell
go-licenses check <package> [package...] --disallowed_types=<comma separated license types> 
```

Supported license types:

* See `forbidden` list: [github.com/google/licenseclassifier](https://github.com/google/licenseclassifier/blob/e6a9bb99b5a6f71d5a34336b8245e305f5430f99/license_type.go#L341)
* See `notice` list:  [github.com/google/licenseclassifier](https://github.com/google/licenseclassifier/blob/e6a9bb99b5a6f71d5a34336b8245e305f5430f99/license_type.go#L249)
* See `permissive` list:  [github.com/google/licenseclassifier](https://github.com/google/licenseclassifier/blob/e6a9bb99b5a6f71d5a34336b8245e305f5430f99/license_type.go#L321)
* See `reciprocal` list:  [github.com/google/licenseclassifier](https://github.com/google/licenseclassifier/blob/e6a9bb99b5a6f71d5a34336b8245e305f5430f99/license_type.go#L225)
* See `restricted` list:  [github.com/google/licenseclassifier](https://github.com/google/licenseclassifier/blob/e6a9bb99b5a6f71d5a34336b8245e305f5430f99/license_type.go#L185)
* See `unencumbered` list:  [github.com/google/licenseclassifier](https://github.com/google/licenseclassifier/blob/e6a9bb99b5a6f71d5a34336b8245e305f5430f99/license_type.go#L324)
* `unknown`

Allow only specific license names:

```shell
go-licenses check <package> [package...] --allowed_licenses=<comma separated license names> 
```

* See supported license names: [github.com/google/licenseclassifier](https://github.com/google/licenseclassifier/blob/e6a9bb99b5a6f71d5a34336b8245e305f5430f99/license_type.go#L28)

### Build tags

To read dependencies from packages with
[build tags](https://golang.org/pkg/go/build/#hdr-Build_Constraints). Use the
`$GOFLAGS` environment variable.

```shell
$ GOFLAGS="-tags=tools" go-licenses report google.golang.org/grpc/test/tools
github.com/BurntSushi/toml,https://github.com/BurntSushi/toml/blob/master/COPYING,MIT
google.golang.org/grpc/test/tools,Unknown,Apache-2.0
honnef.co/go/tools/lint,Unknown,BSD-3-Clause
golang.org/x/lint,Unknown,BSD-3-Clause
golang.org/x/tools,Unknown,BSD-3-Clause
honnef.co/go/tools,Unknown,MIT
honnef.co/go/tools/ssa,Unknown,BSD-3-Clause
github.com/client9/misspell,https://github.com/client9/misspell/blob/master/LICENSE,MIT
github.com/golang/protobuf/proto,https://github.com/golang/protobuf/blob/master/proto/LICENSE,BSD-3-Clause
```

### Ignoring packages

Use the `--ignore` global flag to specify package path prefixes to be ignored.
For example, to ignore your organization's internal packages under `github.com/example-corporation`:

```shell
$ go-licenses check \
    github.com/example-corporation/example-product \
    --ignore github.com/example-corporation
```

Note that dependencies from the ignored packages are still resolved and checked.
This flag makes effect to `check`, `report` and `save` commands.

### Include testing packages

Use the `--include_tests` global flag to include packages only imported by testing code (e.g., testing libraries/frameworks).
Example command:

```shell
go-licenses check --include_tests "github.com/google/go-licenses/..."
```

This flag makes effect to `check`, `report` and `save` commands.

## Warnings and errors

The tool will log warnings and errors in some scenarios. This section provides
guidance on addressing them.

### Dependency contains non-Go code

A warning will be logged when a dependency contains non-Go code. This is because
it is not possible to check the non-Go code for further dependencies, which may
conceal additional license requirements. You should investigate this code to
determine whether it has dependencies and take action to comply with their
license terms.

### Error discovering URL

In order to determine the URL where a license file can be viewed, this tool
generally performs the following steps:

1. Locates the license file on disk.
2. Parses go module metadata and finds the remote repo and version.
3. Adds the license file path to this URL.

There are cases this tool finds an invalid/incorrect URL or fails to find the URL.
Welcome [creating an issue](https://github.com/google/go-licenses/issues).
