// +build stubpkg,timeout

package stub

import (
	"testing"
	"time"
)

func TestTimeout(t *testing.T) {
	time.Sleep(time.Minute)
}
