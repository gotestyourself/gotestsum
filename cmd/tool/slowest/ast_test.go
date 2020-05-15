package slowest

import (
	"bytes"
	"go/format"
	"go/token"
	"testing"

	"gotest.tools/v3/assert"
)

func TestParseSkipStatement_Preset_testingShort(t *testing.T) {
	stmt, err := parseSkipStatement("testing.Short")
	assert.NilError(t, err)
	expected := `if testing.Short() {
	t.Skip("too slow for testing.Short")
}`
	buf := new(bytes.Buffer)
	err = format.Node(buf, token.NewFileSet(), stmt)
	assert.NilError(t, err)
	assert.DeepEqual(t, buf.String(), expected)
}
