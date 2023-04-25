//go:build stubpkg && deep
// +build stubpkg,deep

package failsub

import (
	"testing"
)

func TestFailSub(t *testing.T) {
	t.Error("failsub")
}
