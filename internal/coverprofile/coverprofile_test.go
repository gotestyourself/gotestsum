package coverprofile

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/tools/cover"
	"gotest.tools/v3/assert"
)

func TestParseCoverProfile(t *testing.T) {
	testcases := []struct {
		args           []string
		expectedResult bool
		expectedFile   string
	}{
		{
			args:           []string{"-coverprofile=cover.out"},
			expectedResult: true,
			expectedFile:   "cover.out",
		},
		{
			args:           []string{"-coverprofile=cover.out", "-covermode=count"},
			expectedResult: true,
			expectedFile:   "cover.out",
		},
		{
			args:           []string{"-covermode=count", "-coverprofile=cover.out"},
			expectedResult: true,
			expectedFile:   "cover.out",
		},
		{
			args:           []string{"-covermode=count", "-coverprofile=cover.out", "-covermode=set"},
			expectedResult: true,
			expectedFile:   "cover.out",
		},
		{
			args:           []string{"-covermode=count", "-covermode=set"},
			expectedResult: false,
			expectedFile:   "",
		},
		{
			args:           []string{"-covermode=count"},
			expectedResult: false,
			expectedFile:   "",
		},
	}

	for _, tc := range testcases {
		result, filename := ParseCoverProfile(tc.args)
		assert.Equal(t, tc.expectedResult, result)
		assert.Equal(t, tc.expectedFile, filename)
	}
}

func TestWriteCoverProfile(t *testing.T) {
	input := "testdata/basic/cover.out.0"
	profiles, err := cover.ParseProfiles(input)
	assert.NilError(t, err)
	tempDir := os.TempDir()
	tempfilePath := filepath.Join(tempDir, "cover.out")
	assert.NilError(t, WriteCoverProfile(profiles, tempfilePath))
}

func TestCombine(t *testing.T) {
	temdir := os.TempDir()
	main := "testdata/basic/cover.out.0"
	others := []string{"testdata/basic/cover.out.1", "testdata/basic/cover.out.2"}
	otherProfiles := []*cover.Profile{}
	for _, other := range others {
		profiles, err := cover.ParseProfiles(other)
		assert.NilError(t, err)
		otherProfiles = append(otherProfiles, profiles...)
	}
	assert.NilError(t, combine(main, otherProfiles, filepath.Join(temdir, "cover.out")))
}

func TestCombineProfile(t *testing.T) {
	main := "testdata/basic/cover.out.0"
	mainProfile, err := cover.ParseProfiles(main)
	assert.NilError(t, err)
	others := []string{"testdata/basic/cover.out.1", "testdata/basic/cover.out.2"}
	otherProfiles := []*cover.Profile{}
	for _, other := range others {
		profiles, err := cover.ParseProfiles(other)
		assert.NilError(t, err)
		otherProfiles = append(otherProfiles, profiles...)
	}
	expectProfile, err := cover.ParseProfiles("testdata/basic/cover.out.expect")
	assert.NilError(t, err)
	merged := CombineProfiles(mainProfile, otherProfiles...)
	assert.Assert(t, len(merged) > 0)
	assert.DeepEqual(t, merged, expectProfile)
}
