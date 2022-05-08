//go:build !windows && !aix
// +build !windows,!aix

package filewatcher

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"
)

func TestWatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	dir := fs.NewDir(t, t.Name())

	r, w := io.Pipe()
	patchStdin(t, r)
	patchFloodThreshold(t, 0)

	chEvents := make(chan Event, 1)
	capture := func(event Event) error {
		chEvents <- event
		return nil
	}

	go func() {
		err := Watch(ctx, []string{dir.Path()}, capture)
		assert.Check(t, err)
	}()

	t.Run("run all tests", func(t *testing.T) {
		_, err := w.Write([]byte("a"))
		assert.NilError(t, err)

		event := <-chEvents
		expected := Event{PkgPath: "./..."}
		assert.DeepEqual(t, event, expected, cmpEvent)
	})

	t.Run("run tests on file change", func(t *testing.T) {
		fs.Apply(t, dir, fs.WithFile("file.go", ""))

		event := <-chEvents
		expected := Event{PkgPath: "./" + dir.Path()}
		assert.DeepEqual(t, event, expected, cmpEvent)

		t.Run("and rerun", func(t *testing.T) {
			_, err := w.Write([]byte("r"))
			assert.NilError(t, err)

			event := <-chEvents
			expected := Event{PkgPath: "./" + dir.Path(), useLastPath: true}
			assert.DeepEqual(t, event, expected, cmpEvent)
		})

		t.Run("and debug", func(t *testing.T) {
			_, err := w.Write([]byte("d"))
			assert.NilError(t, err)

			event := <-chEvents
			expected := Event{
				PkgPath:     "./" + dir.Path(),
				useLastPath: true,
				Debug:       true,
			}
			assert.DeepEqual(t, event, expected, cmpEvent)
		})

		t.Run("and update", func(t *testing.T) {
			_, err := w.Write([]byte("u"))
			assert.NilError(t, err)

			event := <-chEvents
			expected := Event{
				PkgPath:     "./" + dir.Path(),
				Args:        []string{"-update"},
				useLastPath: true,
			}
			assert.DeepEqual(t, event, expected, cmpEvent)
		})
	})
}

var cmpEvent = cmp.Options{
	cmp.AllowUnexported(Event{}),
	cmpopts.IgnoreTypes(make(chan struct{})),
}

func patchStdin(t *testing.T, in io.Reader) {
	orig := stdin
	stdin = in
	t.Cleanup(func() {
		stdin = orig
	})
}

func patchFloodThreshold(t *testing.T, d time.Duration) {
	orig := floodThreshold
	floodThreshold = d
	t.Cleanup(func() {
		floodThreshold = orig
	})
}
