// +build stubpkg

package withfails

import (
	"os"
	"testing"
	"time"
)

func TestTimeout(t *testing.T) {
	if os.Getenv("TEST_ALL") != "true" {
		t.Skip("skipping slow test")
	}
	time.Sleep(time.Minute)
}
