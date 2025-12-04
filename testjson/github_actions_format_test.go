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

func TestWriteGitHubActionsError_IgnoresNonTestLogs(t *testing.T) {
	out := new(bytes.Buffer)
	writer := bufio.NewWriter(out)

	event := TestEvent{Test: "pkg.TestWithTelemetry"}
	lines := []string{
		"\texample.go:140: [request-handler.log] 2025-12-04T17:55:24Z INFO Worker [RequestHandler] finished",
	}

	writeGitHubActionsError(writer, event, lines, newGitHubActionsErrorPatterns())

	assert.Equal(t, flushGitHubActionsBuffer(t, writer, out), "")
}

func TestWriteGitHubActionsError_UsesAdditionalLinesForMessage(t *testing.T) {
	out := new(bytes.Buffer)
	writer := bufio.NewWriter(out)

	event := TestEvent{Test: "pkg.TestHasDiff"}
	lines := []string{
		"\tmy_integration_test.go:42:",
		"\t\tExpected",
		"\t\t    <int>: 0",
		"\t\tto equal",
		"\t\t    <int>: 1",
		"",
	}

	writeGitHubActionsError(writer, event, lines, newGitHubActionsErrorPatterns())

	assert.Equal(t,
		flushGitHubActionsBuffer(t, writer, out),
		"::error file=my_integration_test.go,line=42,title=pkg.TestHasDiff::Expected <int>: 0 to equal <int>: 1\n",
	)
}

func TestWriteGitHubActionsError_IncludesRepoRelativeFile(t *testing.T) {
	patchPkgPathPrefix(t, "github.com/example/project")
	out := new(bytes.Buffer)
	writer := bufio.NewWriter(out)

	event := TestEvent{
		Test:    "pkg.TestHasFailure",
		Package: "github.com/example/project/internal/foo",
	}
	lines := []string{"\tfoo_test.go:12: boom"}

	writeGitHubActionsError(writer, event, lines, newGitHubActionsErrorPatterns())

	assert.Equal(t,
		flushGitHubActionsBuffer(t, writer, out),
		"::error file=internal/foo/foo_test.go,line=12,title=pkg.TestHasFailure::boom\n",
	)
}

func TestWriteGitHubActionsError_PanicUsesRepoRelativeFile(t *testing.T) {
	patchPkgPathPrefix(t, "github.com/example/project")
	out := new(bytes.Buffer)
	writer := bufio.NewWriter(out)

	event := TestEvent{
		Test:    "pkg.TestPanicsHard",
		Package: "github.com/example/project/pkg/bar",
	}
	lines := []string{
		"panic: oh no",
		"\t/home/runner/work/project/pkg/bar/bar_test.go:55 +0x123",
	}

	writeGitHubActionsError(writer, event, lines, newGitHubActionsErrorPatterns())

	assert.Equal(t,
		flushGitHubActionsBuffer(t, writer, out),
		"::error file=pkg/bar/bar_test.go,line=55,title=pkg.TestPanicsHard::panic: oh no\n",
	)
}
