//go:build stubpkg && deep
// +build stubpkg,deep

package sub1

import (
	"testing"
)

func TestDeep(t *testing.T) {
	tests := []struct{ name string }{
		{name: "a"},
		{name: "b"},
		{name: "c"},
		{name: "d"},
		{name: "e"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Log(tt)
		})
	}
}
