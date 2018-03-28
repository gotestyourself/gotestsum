// +build stubpkg

package good

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestPassed(t *testing.T) {}

func TestPassedWithLog(t *testing.T) {
	t.Log("this is a log")
}

func TestPassedWithStdout(t *testing.T) {
	fmt.Println("this is a Print")
}

func TestSkipped(t *testing.T) {
	t.Skip()
}

func TestSkippedWitLog(t *testing.T) {
	t.Skip("the skip message")
}

func TestWithStderr(t *testing.T) {
	fmt.Fprintln(os.Stderr, "this is stderr")
}

func TestParallelTheFirst(t *testing.T) {
	t.Parallel()
	time.Sleep(10 * time.Millisecond)
}

func TestParallelTheSecond(t *testing.T) {
	t.Parallel()
	time.Sleep(6 * time.Millisecond)
}

func TestParallelTheThird(t *testing.T) {
	t.Parallel()
	time.Sleep(2 * time.Millisecond)
}

func TestNestedSuccess(t *testing.T) {
	for _, name := range []string{"a", "b", "c", "d"} {
		t.Run(name, func(t *testing.T) {
			t.Run("sub", func(t *testing.T) {})
		})
	}
}
