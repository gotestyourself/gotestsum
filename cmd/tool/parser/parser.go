// Package parser package is responsible for parsing the output of the `go test`
// command and returning additional info about failred test cases, such as file
// and line number of failed test.
package parser

import (
	"bufio"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"gotest.tools/gotestsum/internal/log"
)

// ParseFailure parses the output of the `go test` for a test failure  and
// returns the file and line number of the failed test case.
func ParseFailure(output string) (file string, line int, err error) {
	re, err := regexp.Compile(`^\s*([_\w]+\.go):(\d+):`)
	if err != nil {
		return "", 0, fmt.Errorf("failed to compile regexp: %v", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		outputLine := scanner.Text()
		// Usually the failure would contain a line like this:
		// some_test.go:42 (surrounded by white-space)
		// the full path to the file is not available
		matches := re.FindStringSubmatch(outputLine)

		if len(matches) == 3 {
			parts := matches[1:]
			file = parts[0]
			line, err = strconv.Atoi(parts[1])
			if err != nil {
				log.Debugf("failed to convert line number to int: %v", err)
				return "", 0, nil
			}
			break
		}
	}
	return file, line, scanner.Err()
}
