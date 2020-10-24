package filewatcher

import (
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"gotest.tools/v3/assert"
)

func TestHandler_HandleEvent(t *testing.T) {
	type testCase struct {
		name        string
		last        time.Time
		expectedRun bool
		event       fsnotify.Event
	}

	fn := func(t *testing.T, tc testCase) {
		var ran bool
		run := func(pkg string) error {
			ran = true
			return nil
		}

		h := handler{last: tc.last, fn: run}
		err := h.handleEvent(tc.event)
		assert.NilError(t, err)
		assert.Equal(t, ran, tc.expectedRun)
		if tc.expectedRun {
			assert.Assert(t, !h.last.IsZero())
		}
	}

	var testCases = []testCase{
		{
			name:  "Op is rename",
			event: fsnotify.Event{Op: fsnotify.Rename, Name: "file_test.go"},
		},
		{
			name:  "Op is remove",
			event: fsnotify.Event{Op: fsnotify.Remove, Name: "file_test.go"},
		},
		{
			name:  "Op is chmod",
			event: fsnotify.Event{Op: fsnotify.Chmod, Name: "file_test.go"},
		},
		{
			name:        "Op is write+chmod",
			event:       fsnotify.Event{Op: fsnotify.Write | fsnotify.Chmod, Name: "file_test.go"},
			expectedRun: true,
		},
		{
			name:        "Op is write",
			event:       fsnotify.Event{Op: fsnotify.Write, Name: "file_test.go"},
			expectedRun: true,
		},
		{
			name:        "Op is create",
			event:       fsnotify.Event{Op: fsnotify.Create, Name: "file_test.go"},
			expectedRun: true,
		},
		{
			name:  "file is not a go file",
			event: fsnotify.Event{Op: fsnotify.Write, Name: "readme.md"},
		},
		{
			name:  "under flood threshold",
			event: fsnotify.Event{Op: fsnotify.Create, Name: "file_test.go"},
			last:  time.Now(),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fn(t, tc)
		})
	}
}
