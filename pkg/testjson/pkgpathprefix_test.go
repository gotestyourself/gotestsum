package testjson

import (
	"testing"

	"gotest.tools/assert"
	"gotest.tools/skip"
)

func TestRelativePackagePath(t *testing.T) {
	prefix := "gotest.tools/gotestsum/testjson"
	defer patchPkgPathPrefix(prefix)()
	relPath := RelativePackagePath(prefix + "/extra/relpath")
	assert.Equal(t, relPath, "extra/relpath")

	relPath = RelativePackagePath(prefix)
	assert.Equal(t, relPath, ".")
}

func TestGetPkgPathPrefix(t *testing.T) {
	t.Run("with go path", func(t *testing.T) {
		skip.If(t, isGoModuleEnabled())
		assert.Equal(t, getPkgPathPrefix(), "github.com/astralkn/gotestmng/pkg/testjson")
	})
	t.Run("with go modules", func(t *testing.T) {
		skip.If(t, !isGoModuleEnabled())
		assert.Equal(t, getPkgPathPrefix(), "github.com/astralkn/gotestmng")
	})
}
