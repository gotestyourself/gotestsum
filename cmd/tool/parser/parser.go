// Package parser package is responsible for parsing the output of the `go test`
// command and returning additional info about failred test cases, such as file
// and line number of failed test.
package parser

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gotest.tools/gotestsum/internal/log"
)

// ParseFailure parses the output of the `go test` for a test failure  and
// returns the file and line number of the failed test case.
func ParseFailure(output string) (file string, line int, err error) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		outputLine := scanner.Text()
		// Usually the failure would contain a line like this:
		// Error Trace:	/Users/user/proje/path/to/package/some_test.go:42
		// where the full path to the file is in the same line as "Error Trace:"
		if strings.Contains(outputLine, "Error Trace") {
			absolutePath := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(outputLine), "Error Trace:"))
			currentPath, err := os.Getwd()
			if err != nil {
				return "", 0, fmt.Errorf("failed getting current path: %v", err)
			}

			relPathRaw, err := filepath.Rel(currentPath, absolutePath)
			if err != nil {
				log.Debugf("failed to get relative path from trace: %v", err)
				return "", 0, err
			}
			// in case we're deeply nested, remove any repeating dots
			relPath := filepath.Clean(strings.TrimLeft(relPathRaw, "./"))
			parts := strings.Split(relPath, ":")
			if len(parts) != 2 {
				log.Debugf("failed to split the trace path: %s", relPath)
				return "", 0, nil
			}
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
