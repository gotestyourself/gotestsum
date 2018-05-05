# gotestsum

[![GoDoc](https://godoc.org/gotest.tools/gotestsum?status.svg)](https://godoc.org/gotest.tools/gotestsum)
[![CircleCI](https://circleci.com/gh/gotestyourself/gotestsum/tree/master.svg?style=shield)](https://circleci.com/gh/gotestyourself/gotestsum/tree/master)
[![Go Reportcard](https://goreportcard.com/badge/gotest.tools/gotestsum)](https://goreportcard.com/report/gotest.tools/gotestsum)

`gotestsum` runs `go test --json ./...`, ingests the test output, and prints
customizable output:
 * print test counts: tests run, skipped, failed, package build
   errors, and elapsed time.
 * print a summary of all failure and skip message after the tests have run
 * write a JUnit  XML, or
   [Go TestEvent JSON](https://golang.org/cmd/test2json/#hdr-Output_Format)
   file for ingestion by CI systems.
 * print customized test output with different formats. Formats from most condensed to most
   verbose:
   * `dots` - prints one character per test.
   * `short` - prints a line for each test package (a more condensed version of the
       `go test` default output).
   * `standard-quiet` - prints the default `go test` format.
   * `short-verbose` - prints a line for each test and package.
   * `standard-verbose` - prints the standard `go test -v` format.
   * want some other format? Open an issue!

Requires Go version 1.10+

## Install

    go get gotest.tools/gotestsum

## Example Output

### short (default)

Prints a condensed format using relative package paths and symbols for test
results.  Skip, failure, and error messages are printed after all the tests
have completed.

```
✓  cmd (10ms)
✖  pkg/do
✓  pkg/log (11ms)
↷  pkg/untested

DONE 47 tests, 3 skipped, 5 failed in 0.120s
```

TODO: add failure and skip messages to example output

### dots

Prints the package name, followed by a `.` for passed tests, `✖` for failed
tests, and `↷` for skipped tests. Skip, failure, and error messages are printed
after all the tests have completed.

```
[cmd]···↷···········[pkg/do]···↷↷✖·✖✖····✖··✖····[pkg/log]········
DONE 47 tests, 3 skipped, 5 failed in 0.120s
```

TODO: add failure and skip messages to example output


## Thanks

This package is heavily influenced by the [pytest](https://docs.pytest.org) test runner for `python`.
