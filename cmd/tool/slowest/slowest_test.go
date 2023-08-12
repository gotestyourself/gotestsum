package slowest

import (
	"bytes"
	"testing"

	"gotest.tools/v3/env"
	"gotest.tools/v3/golden"
)

func TestUsage_WithFlagsFromSetupFlags(t *testing.T) {
	env.PatchAll(t, nil)

	name := "gotestsum tool slowest"
	flags, _ := setupFlags(name)
	buf := new(bytes.Buffer)
	usage(buf, name, flags)

	golden.Assert(t, buf.String(), "cmd-flags-help-text")
}
