# safecast

[![Go Report Card](https://goreportcard.com/badge/fortio.org/safecast)](https://goreportcard.com/report/fortio.org/safecast)
[![GoDoc](https://godoc.org/fortio.org/safecast?status.svg)](https://pkg.go.dev/fortio.org/safecast)
[![codecov](https://codecov.io/gh/fortio/safecast/branch/main/graph/badge.svg)](https://codecov.io/gh/fortio/safecast)
[![Maintainability](https://api.codeclimate.com/v1/badges/bf83c496d49b169cd744/maintainability)](https://codeclimate.com/github/fortio/safecast/maintainability)
[![CI Checks](https://github.com/fortio/safecast/actions/workflows/include.yml/badge.svg)](https://github.com/fortio/safecast/actions/workflows/include.yml)

Avoid accidental overflow of numbers during go type conversions (e.g instead of `shorter := bigger.(int8)` type conversions use `shorter := safecast.MustConv[int8](bigger)`.

Safecast allows you to safely convert between numeric types in Go and return errors (or panic when using the `Must*` variants) when the cast would result in a loss of precision, range or sign.

See https://pkg.go.dev/fortio.org/safecast for docs and example.
This is usable from any go with generics (1.18 or later) though our CI uses the latest go.

`safecast` is about avoiding [gosec G115](https://github.com/securego/gosec#available-rules) and [CWE-190: Integer Overflow or Wraparound](https://cwe.mitre.org/data/definitions/190.html) class of overflow and loss of precision bugs, extended to float64/float32 issues.

Credit for the idea (and a finding a bug in the first implementation) goes to [@ccoVeille](https://github.com/ccoVeille), Please see https://github.com/ccoVeille/go-safecast for an different style API and implementation to pick whichever fits your style best (though I believe this implementation and API is better both in simplicity of principle (round trip check) and performance and api surface).

Please note that conversions from integer to float are suffering from CPU architecture differences and issues at the "edge" (max int) which are handled by Convert but you should use the new int only Conv/MustConv if possible, to avoid them.
