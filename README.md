# Go Test Summary

`gotestsum` runs `go test ./...` and summarizes the results.

Requires Go version 1.10+

[![GoDoc](https://godoc.org/github.com/gotestyourself/gotestsum?status.svg)](https://godoc.org/github.com/gotestyourself/gotestsum)
[![CircleCI](https://circleci.com/gh/gotestyourself/gotestsum/tree/master.svg?style=shield)](https://circleci.com/gh/gotestyourself/gotestsum/tree/master)
[![Go Reportcard](https://goreportcard.com/badge/github.com/gotestyourself/gotestsum)](https://goreportcard.com/report/github.com/gotestyourself/gotestsum)

## Install

    go get gotest.tools/gotestsum/cmd

## Key Features

* Test summary
  * counts for run, failed, skipped tests and build errors
  * prints all skip message, test failures and output, and build errors at
    the end so they are easily visible
* Customize test output with different formats
  * `dots` - prints one chracter per test
  * `short` - a line for each test package
  * `short-verbose` - a line for each test and package
  * `standard-quiet` - the standard `go test` format
  * `standard-verbose` - the standard `go test -v` format
  * want some other format? Open an issue!
