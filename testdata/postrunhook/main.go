package main

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func run() error {
	environ := os.Environ()
	sort.Strings(environ)
	for _, v := range environ {
		for _, prefix := range []string{"TESTS_", "GOTESTSUM_"} {
			if strings.HasPrefix(v, prefix) {
				fmt.Println(v)
			}
		}
	}

	err := os.Getenv("TEST_STUB_ERROR")
	if err != "" {
		return errors.New(err)
	}
	return nil
}
