package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"gotest.tools/gotestsum/internal/junitxml"
	"gotest.tools/gotestsum/internal/log"
	"gotest.tools/gotestsum/testjson"
)

type eventHandler struct {
	formatter            testjson.EventFormatter
	err                  io.Writer
	jsonFile             writeSyncer
	jsonFileTimingEvents writeSyncer
	maxFails             int
}

type writeSyncer interface {
	io.WriteCloser
	Sync() error
}

func (h *eventHandler) Err(text string) error {
	_, _ = h.err.Write([]byte(text + "\n"))
	// always return nil, no need to stop scanning if the stderr write fails
	return nil
}

func (h *eventHandler) Event(event testjson.TestEvent, execution *testjson.Execution) error {
	// ignore artificial events with no raw Bytes()
	if len(event.Bytes()) > 0 {
		if h.jsonFile != nil {
			_, err := h.jsonFile.Write(append(event.Bytes(), '\n'))
			if err != nil {
				return fmt.Errorf("failed to write JSON file: %w", err)
			}
		}
		if h.jsonFileTimingEvents != nil && event.Action.IsTerminal() {
			_, err := h.jsonFileTimingEvents.Write(append(event.Bytes(), '\n'))
			if err != nil {
				return fmt.Errorf("failed to write JSON file: %w", err)
			}
		}
	}

	err := h.formatter.Format(event, execution)
	if err != nil {
		return fmt.Errorf("failed to format event: %w", err)
	}

	if h.maxFails > 0 && len(execution.Failed()) >= h.maxFails {
		return fmt.Errorf("ending test run because max failures was reached")
	}
	return nil
}

func (h *eventHandler) Flush() {
	if h.jsonFile != nil {
		if err := h.jsonFile.Sync(); err != nil {
			log.Errorf("Failed to sync JSON file: %v", err)
		}
	}
	if h.jsonFileTimingEvents != nil {
		if err := h.jsonFileTimingEvents.Sync(); err != nil {
			log.Errorf("Failed to sync JSON file: %v", err)
		}
	}
}

func (h *eventHandler) Close() error {
	if h.jsonFile != nil {
		if err := h.jsonFile.Close(); err != nil {
			log.Errorf("Failed to close JSON file: %v", err)
		}
	}
	if h.jsonFileTimingEvents != nil {
		if err := h.jsonFileTimingEvents.Close(); err != nil {
			log.Errorf("Failed to close JSON file: %v", err)
		}
	}
	return nil
}

var _ testjson.EventHandler = &eventHandler{}

func newEventHandler(opts *options) (*eventHandler, error) {
	formatter := testjson.NewEventFormatter(opts.stdout, opts.format, opts.formatOptions)
	if formatter == nil {
		return nil, fmt.Errorf("unknown format %s", opts.format)
	}
	handler := &eventHandler{
		formatter: formatter,
		err:       opts.stderr,
		maxFails:  opts.maxFails,
	}
	var err error
	if opts.jsonFile != "" {
		_ = os.MkdirAll(filepath.Dir(opts.jsonFile), 0o755)
		handler.jsonFile, err = os.Create(opts.jsonFile)
		if err != nil {
			return handler, fmt.Errorf("failed to create file: %w", err)
		}
	}
	if opts.jsonFileTimingEvents != "" {
		_ = os.MkdirAll(filepath.Dir(opts.jsonFileTimingEvents), 0o755)
		handler.jsonFileTimingEvents, err = os.Create(opts.jsonFileTimingEvents)
		if err != nil {
			return handler, fmt.Errorf("failed to create file: %w", err)
		}
	}
	return handler, nil
}

func writeJUnitFile(opts *options, execution *testjson.Execution) error {
	if opts.junitFile == "" {
		return nil
	}
	_ = os.MkdirAll(filepath.Dir(opts.junitFile), 0o755)
	junitFile, err := os.Create(opts.junitFile)
	if err != nil {
		return fmt.Errorf("failed to open JUnit file: %v", err)
	}
	defer func() {
		if err := junitFile.Close(); err != nil {
			log.Errorf("Failed to close JUnit file: %v", err)
		}
	}()

	return junitxml.Write(junitFile, execution, junitxml.Config{
		ProjectName:             opts.junitProjectName,
		FormatTestSuiteName:     opts.junitTestSuiteNameFormat.Value(),
		FormatTestCaseClassname: opts.junitTestCaseClassnameFormat.Value(),
		HideEmptyPackages:       opts.junitHideEmptyPackages,
	})
}

func postRunHook(opts *options, execution *testjson.Execution) error {
	command := opts.postRunHookCmd.Value()
	if len(command) == 0 {
		return nil
	}

	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdout = opts.stdout
	cmd.Stderr = opts.stderr
	cmd.Env = append(
		os.Environ(),
		"GOTESTSUM_JSONFILE="+opts.jsonFile,
		"GOTESTSUM_JSONFILE_TIMING_EVENTS="+opts.jsonFileTimingEvents,
		"GOTESTSUM_JUNITFILE="+opts.junitFile,
		fmt.Sprintf("TESTS_TOTAL=%d", execution.Total()),
		fmt.Sprintf("TESTS_FAILED=%d", len(execution.Failed())),
		fmt.Sprintf("TESTS_SKIPPED=%d", len(execution.Skipped())),
		fmt.Sprintf("TESTS_ERRORS=%d", len(execution.Errors())),
	)
	return cmd.Run()
}
