package gotestsum

import (
	"github.com/astralkn/gotestmng/pkg/junitxml"
	"github.com/astralkn/gotestmng/pkg/options"
	"io"
	"os"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"gotest.tools/gotestsum/testjson"
)

type eventHandler struct {
	formatter testjson.EventFormatter
	err       io.Writer
	jsonFile  io.WriteCloser
}

func (h *eventHandler) Err(text string) error {
	_, err := h.err.Write([]byte(text + "\n"))
	return err
}

func (h *eventHandler) Event(event testjson.TestEvent, execution *testjson.Execution) error {
	if h.jsonFile != nil {
		_, err := h.jsonFile.Write(append(event.Bytes(), '\n'))
		if err != nil {
			return errors.Wrap(err, "failed to write JSON file")
		}
	}

	err := h.formatter.Format(event, execution)
	if err != nil {
		return errors.Wrap(err, "failed to format event")
	}
	return nil
}

func (h *eventHandler) Close() error {
	if h.jsonFile != nil {
		if err := h.jsonFile.Close(); err != nil {
			log.WithError(err).Error("failed to close JSON file")
		}
	}
	return nil
}

var _ testjson.EventHandler = &eventHandler{}

func newEventHandler(opts *options.Options, stdout io.Writer, stderr io.Writer) (*eventHandler, error) {
	formatter := testjson.NewEventFormatter(stdout, opts.Format)
	if formatter == nil {
		return nil, errors.Errorf("unknown format %s", opts.Format)
	}
	handler := &eventHandler{
		formatter: formatter,
		err:       stderr,
	}
	var err error
	if opts.JsonFile != "" {
		handler.jsonFile, err = os.Create(opts.JsonFile)
		if err != nil {
			return handler, errors.Wrap(err, "failed to open JSON file")
		}
	}
	return handler, nil
}

func writeJUnitFile(opts *options.Options, execution *testjson.Execution) error {
	if opts.JunitFile == "" {
		return nil
	}
	junitFile, err := os.Create(opts.JunitFile)
	if err != nil {
		return err
	}
	defer func() {
		if err := junitFile.Close(); err != nil {
			log.WithError(err).Error("failed to close JUnit file")
		}
	}()

	opts.JUnitTestSuite, err = junitxml.Write(junitFile, execution, junitxml.Config{
		FormatTestSuiteName:     opts.JunitTestSuiteNameFormat.Value(),
		FormatTestCaseClassname: opts.JunitTestCaseClassnameFormat.Value(),
	})
	return err
}
