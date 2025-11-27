package testjson

import (
	"bufio"
	"bytes"
	"testing"

	"gotest.tools/v3/assert"
)

func flushGitHubActionsBuffer(t *testing.T, buf *bufio.Writer, out *bytes.Buffer) string {
	t.Helper()
	assert.NilError(t, buf.Flush())
	return out.String()
}

func TestWriteGitHubActionsError_FailureAnnotations(t *testing.T) {
	out := new(bytes.Buffer)
	writer := bufio.NewWriter(out)

	event := TestEvent{Test: "pkg.TestFailure"}
	lines := []string{"\tfailure_test.go:42: something went wrong"}

	writeGitHubActionsError(writer, event, lines, newGitHubActionsErrorPatterns())

	assert.Equal(t,
		flushGitHubActionsBuffer(t, writer, out),
		"::error file=failure_test.go,line=42,title=pkg.TestFailure::something went wrong\n",
	)
}

func TestWriteGitHubActionsError_PanicPrefersTestFile(t *testing.T) {
	out := new(bytes.Buffer)
	writer := bufio.NewWriter(out)

	event := TestEvent{Test: "pkg.TestPanics"}
	lines := []string{
		"panic: runtime error: index out of range",
		"\t/usr/local/go/src/runtime/panic.go:88 +0x123",
		"\t/home/user/project/example_test.go:45 +0x456",
		"\t/home/user/project/example.go:12 +0x222",
	}

	writeGitHubActionsError(writer, event, lines, newGitHubActionsErrorPatterns())

	assert.Equal(t,
		flushGitHubActionsBuffer(t, writer, out),
		"::error file=example_test.go,line=45,title=pkg.TestPanics::panic: runtime error: index out of range\n",
	)
}

func TestWriteGitHubActionsError_PanicRequiresStrictMatch(t *testing.T) {
	out := new(bytes.Buffer)
	writer := bufio.NewWriter(out)

	event := TestEvent{Test: "pkg.TestLogsPanicWord"}
	lines := []string{"\tfailure_test.go:12: panic: not a real panic"}

	writeGitHubActionsError(writer, event, lines, newGitHubActionsErrorPatterns())

	assert.Equal(t,
		flushGitHubActionsBuffer(t, writer, out),
		"::error file=failure_test.go,line=12,title=pkg.TestLogsPanicWord::panic: not a real panic\n",
	)
}
