package testjson

import (
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

func TestRelativePackagePath(t *testing.T) {
	prefix := "gotest.tools/gotestsum/testjson"
	patchPkgPathPrefix(t, prefix)
	relPath := RelativePackagePath(prefix + "/extra/relpath")
	assert.Equal(t, relPath, "extra/relpath")

	relPath = RelativePackagePath(prefix)
	assert.Equal(t, relPath, ".")
}

func TestGetPkgPathPrefix(t *testing.T) {
	t.Run("with go path", func(t *testing.T) {
		skip.If(t, isGoModuleEnabled())
		assert.Equal(t, getPkgPathPrefix(), "gotest.tools/gotestsum/testjson")
	})
	t.Run("with go modules", func(t *testing.T) {
		skip.If(t, !isGoModuleEnabled())
		assert.Equal(t, getPkgPathPrefix(), "gotest.tools/gotestsum")
	})
}
