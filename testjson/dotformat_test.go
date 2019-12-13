package testjson

import (
	"bytes"
	"runtime"
	"testing"

	"gotest.tools/assert"
	"gotest.tools/golden"
	"gotest.tools/skip"
)

func TestScanTestOutput_WithDotsFormatter(t *testing.T) {
	skip.If(t, runtime.GOOS == "windows", "need a separate expected value for windows")
	defer patchPkgPathPrefix("github.com/gotestyourself/gotestyourself")()

	out := new(bytes.Buffer)
	dotfmt := newDotFormatter(out)
	d, ok := dotfmt.(*dotFormatter)
	if !ok {
		t.Skip("not the right formatter, missing terminal width?")
	}
	d.termWidth = 80
	shim := newFakeHandler(dotfmt, "go-test-json")
	exec, err := ScanTestOutput(shim.Config(t))

	assert.NilError(t, err)
	golden.Assert(t, out.String(), "dots-format.out")
	golden.Assert(t, shim.err.String(), "dots-format.err")
	assert.DeepEqual(t, exec, expectedExecution, cmpExecutionShallow)
}
