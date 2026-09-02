// +build stubpkg

/*Package nestedfails is used to generate testdata for the testjson package.
 */
package nestedfails

import (
	"testing"
)

// TestParentFailsWithSubtestFailure fails because one of its subtests failed.
func TestParentFailsWithSubtestFailure(t *testing.T) {
	t.Run("passes", func(t *testing.T) {})
	t.Run("fails", func(t *testing.T) {
		t.Fatal("this failed")
	})
}

// TestParentFailsWithoutSubtestFailure fails on its own, after all of its
// subtests passed.
func TestParentFailsWithoutSubtestFailure(t *testing.T) {
	t.Run("passes", func(t *testing.T) {})
	t.Error("the parent failed")
}

// TestDeeplyNested fails at the third level of nesting.
func TestDeeplyNested(t *testing.T) {
	t.Run("b", func(t *testing.T) {
		t.Run("c", func(t *testing.T) {
			t.Fatal("this failed")
		})
	})
}
