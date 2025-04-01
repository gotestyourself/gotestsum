package coverprofile

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/tools/cover"
)

// ParseCoverProfile parse the coverprofile file from the flag
func ParseCoverProfile(args []string) (bool, string) {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-coverprofile=") {
			return true, strings.TrimPrefix(arg, "-coverprofile=")
		}
	}

	return false, ""
}

// WriteCoverProfile writes the cover profile to the file
func WriteCoverProfile(profiles []*cover.Profile, filename string) error {
	// Create a tmp file to write the merged profiles to. Then use os.Rename to
	// atomically move the file to the main profile to mimic the effect of
	// atomic replacement of the file. Note, we can't put the file on tempfs
	// using the all the nice utilities around tempfiles. In places like docker
	// containers, calling os.Rename on a file that is on tempfs to a file on
	// normal filesystem partition will fail with errno 18 invalid cross device
	// link.
	tempFile := fmt.Sprintf("%s.tmp", filename)
	f, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("failed to create coverprofile file: %v", err)
	}

	dumpProfiles(profiles, f)
	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close temp file %s: %v", tempFile, err)
	}

	if err := os.Rename(tempFile, filename); err != nil {
		return fmt.Errorf("failed to rename temp file %s to %s: %v", tempFile, filename, err)
	}

	return nil
}

// CombineProfiles combines the cover profiles together
func CombineProfiles(this []*cover.Profile, others ...*cover.Profile) []*cover.Profile {
	merged := this
	for _, p := range others {
		merged = addProfile(merged, p)
	}
	return merged
}

// A helper function to expose the destination of the merged profile so we
// testing is easier.
func combine(main string, others []*cover.Profile, out string) error {
	mainProfiles, err := cover.ParseProfiles(main)
	if err != nil {
		return fmt.Errorf("failed to parse coverprofile %s: %v", main, err)
	}

	merged := mainProfiles

	for _, other := range others {
		merged = CombineProfiles(mainProfiles, other)
	}

	return WriteCoverProfile(merged, out)
}

// Combine merges the `others` cover profile with the main cover profile file
func Combine(main string, others []*cover.Profile) error {
	return combine(main, others, main)
}
