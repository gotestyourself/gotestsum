# gotestsum

`gotestsum` runs tests, prints friendly test output and a summary of the test run.  Requires Go 1.10+.

## Install

Download a binary from [releases](https://github.com/gotestyourself/gotestsum/releases), or from
source with `go get gotest.tools/gotestsum` (you may need to run `dep ensure` if your version of Go
does not support modules).

## Demo
A demonstration of three `--format` options.

![Demo](https://i.ibb.co/XZfhmXq/demo.gif)
<br />[Source](https://github.com/gotestyourself/gotestsum/tree/readme-demo/scripts)

## Docs

[![GoDoc](https://godoc.org/gotest.tools/gotestsum?status.svg)](https://godoc.org/gotest.tools/gotestsum)
[![CircleCI](https://circleci.com/gh/gotestyourself/gotestsum/tree/master.svg?style=shield)](https://circleci.com/gh/gotestyourself/gotestsum/tree/master)
[![Go Reportcard](https://goreportcard.com/badge/gotest.tools/gotestsum)](https://goreportcard.com/report/gotest.tools/gotestsum)

`gotestsum` works by running `go test --json ./...` and reading the JSON
output.

### TOC

- [Format](#format)
- [Summary](#summary)
- [JUnit XML](#junit-xml)
- [JSON file](#json-file-output)
- [Using go test flags and custom commands](#custom-go-test-command)
- [Executing a compiled test binary](#executing-a-compiled-test-binary)

### Format

Set a format with the `--format` flag or the `GOTESTSUM_FORMAT` environment
variable.
```
gotestsum --format short-verbose
```

Supported formats:
 * `dots` - print a character for each test.
 * `pkgname` (default) - print a line for each package.
 * `pkgname-and-test-fails` - print a line for each package, and failed test output.
 * `testname` - print a line for each test and package.
 * `standard-quiet` - the standard `go test` format.
 * `standard-verbose` - the standard `go test -v` format.

Have a suggestion for some other format? Please open an issue!

### Summary

A summary of the test run is printed after the test output.

```
DONE 101 tests[, 3 skipped][, 2 failures][, 1 error] in 0.103s
```

The summary includes:
 * A count of: tests run, tests skipped, tests failed, and package build errors.
 * Elapsed time including time to build.
 * Test output of all failed and skipped tests, and any package build errors.

To disable parts of the summary use `--no-summary section`.

Example: hide skipped tests in the summary
```
gotestsum --no-summary=skipped
```

Example: hide failed and skipped
```
gotestsum --no-summary=skipped,failed
```

Example: hide output in the summary, only print names of failed and skipped tests
and errors
```
gotestsum --no-summary=output
```

### JUnit XML

When the `--junitfile` flag or `GOTESTSUM_JUNITFILE` environment variable are set
to a file path `gotestsum` will write a test report, in JUnit XML format, to the file.
This file can be used to integrate with CI systems.

```
gotestsum --junitfile unit-tests.xml
```

If the package names in the `testsuite.name` or `testcase.classname` fields do not
work with your CI system these values can be customized using the
`--junitfile-testsuite-name`, or `--junitfile-testcase-classname` flags. These flags
accept the following values:

* `short` - the base name of the package (the single term specified by the 
  package statement).
* `relative` - a package path relative to the root of the repository
* `full` - the full package path (default)


Note: If Go is not installed, or the `go` binary is not in `PATH`, the `GOVERSION`
environment variable can be set to remove the "failed to lookup go version for junit xml"
warning.

### JSON file output

When the `--jsonfile` flag or `GOTESTSUM_JSONFILE` environment variable are set
to a file path `gotestsum` will write a line-delimited JSON file with all the
[test2json](https://golang.org/cmd/test2json/#hdr-Output_Format)
output that was written by `go test --json`. This file can be used to compare test
runs, or find flaky tests.

```
gotestsum --jsonfile test-output.log
```

### Custom `go test` command

By default `gotestsum` runs tests using the command `go test --json ./...`. You
can change the command with positional arguments after a `--`. You can change just the
test directory value (which defaults to `./...`) by setting the `TEST_DIRECTORY`
environment variable.

You can use `--debug` to echo the command before it is run.

Example: set build tags
```
gotestsum -- -tags=integration ./...
```

Example: run tests in a single package
```
gotestsum -- ./io/http
```

Example: enable coverage
```
gotestsum -- -coverprofile=cover.out ./...
```

Example: run a script instead of `go test`
```
gotestsum --raw-command -- ./scripts/run_tests.sh
```

Note: when using `--raw-command` you must ensure that the stdout produced by
the script only contains the `test2json` output. Any stderr produced by the script
will be considered an error (this behaviour is necessary because package build errors
are only reported by writting to stderr, not the `test2json` stdout). Any stderr
produced by tests is not considered an error (it will be in the `test2json` stdout).

Example: using `TEST_DIRECTORY`
```
TEST_DIRECTORY=./io/http gotestsum
```

### Executing a compiled test binary

`gotestsum` supports executing a compiled test binary (created with `go test -c`) by running
it as a custom command.

The `-json` flag is handled by `go test` itself, it is not available when using a
compiled test binary, so `go tool test2json` must be used to get the output
that `gotestsum` expects.

Example:

```
gotestsum --raw-command -- go tool test2json -p pkgname ./binary.test -test.v
```

`pkgname` is the name of the package being tested, it will show up in the test
output. `./binary.test` is the path to the compiled test binary. The `-test.v`
must be included so that `go tool test2json` receives all the output.

To execute a test binary without installing Go, see
[running without go](./docs/running-without-go.md).


### Run tests when a file is modified

[filewatcher](https://github.com/dnephin/filewatcher) will automatically set the
`TEST_DIRECTORY` environment variable which makes it easy to integrate
`gotestsum`.

Example: run tests for a package when any file in that package is saved
```
filewatcher gotestsum
```

## Thanks

This package is heavily influenced by the [pytest](https://docs.pytest.org) test runner for `python`.
