// +build stubpkg

package fails

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

func TestWithStderr(t *testing.T) {
	fmt.Fprintln(os.Stderr, "this is stderr")
}

func TestParallelTheFirst(t *testing.T) {
	t.Parallel()
	time.Sleep(10 * time.Millisecond)
	t.Fatal("failed the first")
}

func TestParallelTheSecond(t *testing.T) {
	t.Parallel()
	time.Sleep(6 * time.Millisecond)
	t.Fatal("failed the second")
}

func TestParallelTheThird(t *testing.T) {
	t.Parallel()
	time.Sleep(2 * time.Millisecond)
	t.Fatal("failed the third")

}

func TestNestedParallelFailures(t *testing.T) {
	for _, name := range []string{"a", "b", "c", "d"} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			t.Fatal("failed sub " + name)
		})
	}
}
