//go:build testdata
// +build testdata

package flaky

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"testing"
)

var seed int
var seedfile = seedFile()
var once = new(sync.Once)

func setup(t *testing.T) {
	once.Do(func() {
		raw, err := os.ReadFile(seedfile)
		if err != nil {
			t.Fatalf("failed to read seed: %v", err)
		}
		n, err := strconv.ParseInt(string(raw), 10, 64)
		if err != nil {
			t.Fatalf("failed to parse seed: %v", err)
		}
		seed = int(n)

		err = os.WriteFile(seedfile, []byte(strconv.Itoa(seed+1)), 0644)
		if err != nil {
			t.Fatalf("failed to write seed: %v", err)
		}
	})
	fmt.Fprintln(os.Stderr, "SEED: ", seed)
}

func seedFile() string {
	if name, ok := os.LookupEnv("TEST_SEEDFILE"); ok {
		return name
	}
	return "/tmp/gotestsum-flaky-seedfile"
}

func TestAlwaysPasses(t *testing.T) {
}

func TestFailsRarely(t *testing.T) {
	setup(t)
	if seed%20 != 1 {
		t.Fatal("not this time")
	}
}

func TestFailsSometimes(t *testing.T) {
	setup(t)
	if seed%4 != 2 {
		t.Fatal("not this time")
	}
}

func TestFailsOften(t *testing.T) {
	setup(t)

	t.Run("subtest always passes", func(t *testing.T) {})
	t.Run("subtest may fail", func(t *testing.T) {
		if seed%20 != 6 {
			t.Fatal("not this time")
		}
	})
}

func TestFailsOftenDoesNotPrefixMatch(t *testing.T) {}

func TestFailsSometimesDoesNotPrefixMatch(t *testing.T) {}
