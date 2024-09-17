package filewatcher

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/env"
	"gotest.tools/v3/fs"
)

func TestFSEventHandler_HandleEvent(t *testing.T) {
	type testCase struct {
		name        string
		last        time.Time
		expectedRun bool
		event       fsnotify.Event
	}

	fn := func(t *testing.T, tc testCase) {
		var ran bool
		run := func(Event) error {
			ran = true
			return nil
		}

		h := fsEventHandler{last: tc.last, fn: run}
		err := h.handleEvent(tc.event)
		assert.NilError(t, err)
		assert.Equal(t, ran, tc.expectedRun)
		if tc.expectedRun {
			assert.Assert(t, !h.last.IsZero())
		}
	}

	var testCases = []testCase{
		{
			name:        "Op is rename",
			event:       fsnotify.Event{Op: fsnotify.Rename, Name: "file_test.go"},
			expectedRun: true,
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

func TestHasGoFiles(t *testing.T) {
	t.Run("none", func(t *testing.T) {
		tmpDir := fs.NewDir(t, t.Name(), fs.WithFile("readme.md", ""))
		defer tmpDir.Remove()
		assert.Assert(t, !hasGoFiles(tmpDir.Path()))
	})
	t.Run("empty", func(t *testing.T) {
		tmpDir := fs.NewDir(t, t.Name())
		defer tmpDir.Remove()
		assert.Assert(t, !hasGoFiles(tmpDir.Path()))
	})
	t.Run("some go files", func(t *testing.T) {
		tmpDir := fs.NewDir(t, t.Name(), fs.WithFile("main.go", ""))
		defer tmpDir.Remove()
		assert.Assert(t, hasGoFiles(tmpDir.Path()))
	})
	t.Run("many go files", func(t *testing.T) {
		tmpDir := fs.NewDir(t, t.Name())
		for i := 0; i < 47; i++ {
			fs.Apply(t, tmpDir, fs.WithFile(fmt.Sprintf("file%d.go", i), ""))
		}
		defer tmpDir.Remove()
		assert.Assert(t, hasGoFiles(tmpDir.Path()))
	})
}

func TestFindAllDirs(t *testing.T) {
	goFile := fs.WithFile("file.go", "")
	dirOne := fs.NewDir(t, t.Name(),
		goFile,
		fs.WithFile("not-a-dir", ""),
		fs.WithDir("no-go-files"),
		fs.WithDir(".starts-with-dot", goFile))
	defer dirOne.Remove()
	var path string
	for i := 1; i <= 10; i++ {
		path = filepath.Join(path, fmt.Sprintf("%d", i))
		var ops []fs.PathOp
		if i != 4 && i != 5 {
			ops = []fs.PathOp{goFile}
		}
		fs.Apply(t, dirOne, fs.WithDir(path, ops...))
	}

	dirTwo := fs.NewDir(t, t.Name(),
		goFile,
		// subdir should be ignored, dirTwo is used without /... suffix
		fs.WithDir("subdir", goFile))
	defer dirTwo.Remove()

	dirs := findAllDirs([]string{dirOne.Path() + "/...", dirTwo.Path()}, maxDepth)
	expected := []string{
		dirOne.Path(),
		dirOne.Join("1"),
		dirOne.Join("1/2"),
		dirOne.Join("1/2/3"),
		dirOne.Join("1/2/3/4/5/6"),
		dirOne.Join("1/2/3/4/5/6/7"),
		dirTwo.Path(),
	}
	assert.DeepEqual(t, dirs, expected)
}

func TestFindAllDirs_DefaultPath(t *testing.T) {
	goFile := fs.WithFile("file.go", "")
	dirOne := fs.NewDir(t, t.Name(),
		goFile,
		fs.WithDir("a", goFile),
		fs.WithDir("b", goFile))
	defer dirOne.Remove()

	defer env.ChangeWorkingDir(t, dirOne.Path())()
	dirs := findAllDirs([]string{}, maxDepth)
	expected := []string{".", "a", "b"}
	assert.DeepEqual(t, dirs, expected)
}
