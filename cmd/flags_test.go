package cmd

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestNoSummaryValue_SetAndString(t *testing.T) {
	t.Run("none", func(t *testing.T) {
		assert.Equal(t, newHideSummaryValue().String(), "none")
	})
	t.Run("one", func(t *testing.T) {
		value := newHideSummaryValue()
		assert.NilError(t, value.Set("output"))
		assert.Equal(t, value.String(), "output")
	})
	t.Run("some", func(t *testing.T) {
		value := newHideSummaryValue()
		assert.NilError(t, value.Set("errors,failed"))
		assert.Equal(t, value.String(), "failed,errors")
	})
	t.Run("bad value", func(t *testing.T) {
		value := newHideSummaryValue()
		assert.ErrorContains(t, value.Set("bogus"), "must be one or more of")
	})
}

func TestStringSlice(t *testing.T) {
	value := "one \ntwo  three\n\tfour\t five   \n"
	var v []string
	ss := (*stringSlice)(&v)
	assert.NilError(t, ss.Set(value))
	assert.DeepEqual(t, v, []string{"one", "two", "three", "four", "five"})
}
