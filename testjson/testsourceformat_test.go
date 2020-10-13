package testjson

import (
	"bytes"
	"testing"

	"github.com/fatih/color"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/env"
	"gotest.tools/v3/golden"
)

func TestTestSourceFormatter(t *testing.T) {
	defer patchPkgPathPrefix("github.com/gotestyourself/gotestyourself")()
	defer env.Patch(t, "GOFLAGS", "-tags=stubpkg")()
	defer patchNoColor()()

	out := new(bytes.Buffer)
	shim := newFakeHandler(newTestSourceFormatter(out), "go-test-json-v2")
	_, err := ScanTestOutput(shim.Config(t))

	assert.NilError(t, err)
	golden.Assert(t, out.String(), "testsource-format.out")
	golden.Assert(t, shim.err.String(), "testsource-format.err")
	t.Fail()
}

func patchNoColor() func() {
	var orig bool
	orig, color.NoColor = color.NoColor, false
	return func() {
		color.NoColor = orig
	}
}
