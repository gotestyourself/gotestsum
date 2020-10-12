package testjson

import (
	"bytes"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/env"
	"gotest.tools/v3/golden"
)

func TestTestSourceFormatter(t *testing.T) {
	defer patchPkgPathPrefix("github.com/gotestyourself/gotestyourself")()
	defer env.Patch(t, "GOFLAGS", "-tags=stubpkg")()

	out := new(bytes.Buffer)
	shim := newFakeHandler(newTestSourceFormatter(out), "go-test-json-v2")
	_, err := ScanTestOutput(shim.Config(t))

	assert.NilError(t, err)
	golden.Assert(t, out.String(), "testsource-format.out")
	golden.Assert(t, shim.err.String(), "testsource-format.err")
}
