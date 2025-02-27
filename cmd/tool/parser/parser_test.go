package parser

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestParseFailure_Ok(t *testing.T) {
	// given
	failure := `Error Trace:	/project/path/to/package/some_test.go:42`

	// when
	file, line, err := ParseFailure(failure)

	// then
	assert.NilError(t, err)
	assert.Equal(t, file, "project/path/to/package/some_test.go")
	assert.Equal(t, line, 42)
}
