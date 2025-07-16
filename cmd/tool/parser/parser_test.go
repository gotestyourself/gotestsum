package parser

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestParseFailure_Ok(t *testing.T) {
	// given
	failure := `			 some_1s_test.go:42:  \n`

	// when
	file, line, err := ParseFailure(failure)

	// then
	assert.NilError(t, err)
	assert.Equal(t, file, "some_1s_test.go")
	assert.Equal(t, line, 42)
}
