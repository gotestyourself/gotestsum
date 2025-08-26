//go:build stubpkg

package withattributes

import "testing"

func TestSomeAttributes(t *testing.T) {
	t.Attr("hello", "world")
	t.Attr("other", "side")
}
