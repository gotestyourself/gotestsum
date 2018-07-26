# gotestsum

`gotestsum` runs tests, prints friendly test output and a summary of the test run.  Requires Go 1.10+.

## Install

Download a binary from [releases](https://github.com/gotestyourself/gotestsum/releases), or get the
source with `go get gotest.tools/gotestsum` (you may need to run `dep ensure`).

## Demo

![Demo](https://raw.githubusercontent.com/gotestyourself/gotestsum/master/docs/demo.gif)

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
- [Custom command](#custom-go-test-command)
### Format

Set a format with the `--format` flag or the `GOTESTSUM_FORMAT` environment
variable.
```
gotestsum --format short-verbose
```

The supported formats are:
 * `dots` - output one character per test.
 * `short` (default) - output a line for each test package.
 * `standard-quiet` - the default `go test` format.
 * `short-verbose` - output a line for each test and package.
 * `standard-verbose` - the standard `go test -v` format.

Have a suggestion for some other format? Please open an issue!

### Summary

After the tests are done a summary of the test run is printed.
The summary includes:
 * A count of the tests run, skipped, failed, build errors, and elapsed time.
 * Test output of all failed and skipped tests, and any build errors.

To disable parts of the summary use `--no-summary section`.

Example: hide skipped tests in the summary
```
gotestsum --no-summary=skipped
```

Example: hide failed and skipped
```
gotestsum --no-summary=skipped,failed
```

### JUnit XML

In addition to the normal test output you can write a JUnit XML file for
integration with CI systems. Write a file using the `--junitfile` flag or
the `GOTESTSUM_JUNITFILE` environment variable.

```
gotestsum --junitfile unit-tests.xml
```

### JSON file output

In addition to the normal test output you can write a line-delimited JSON
file with all the [test2json](https://golang.org/cmd/test2json/#hdr-Output_Format)
output that was written by `go test --json`. This file can be used to calculate
statistics about the test run.

```
gotestsum --jsonfile test-output.log
```

### Custom `go test` command

By default `gotestsum` runs `go test --json ./...`. You can change this by
specifying additional positional arguments after a `--`. You can change just the
test directory value (which defaults to `./...`) by setting the `TEST_DIRECTORY`
environment variable.

You can use `--debug` to echo the command before it is run.

Example: set build tags
```
gotestsum -- -tags=integration ./...
```

Example: run only a single package
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
the script only contains the `test2json` output. Any stderr produced will
be considered an error (to match the behaviour of `go test --json`).

Example: using `TEST_DIRECTORY`
```
TEST_DIRECTORY=./io/http gotestsum
```

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
