package main

import (
	"encoding/csv"
	"path"
	"strings"

	"github.com/google/shlex"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"gotest.tools/gotestsum/internal/junitxml"
	"gotest.tools/gotestsum/testjson"
)

type noSummaryValue struct {
	value testjson.Summary
}

func newNoSummaryValue() *noSummaryValue {
	return &noSummaryValue{value: testjson.SummarizeAll}
}

func readAsCSV(val string) ([]string, error) {
	if val == "" {
		return nil, nil
	}
	return csv.NewReader(strings.NewReader(val)).Read()
}

func (s *noSummaryValue) Set(val string) error {
	v, err := readAsCSV(val)
	if err != nil {
		return err
	}
	for _, item := range v {
		summary, ok := testjson.NewSummary(item)
		if !ok {
			return errors.Errorf("value must be one or more of: %s",
				testjson.SummarizeAll.String())
		}
		s.value -= summary
	}
	return nil
}

func (s *noSummaryValue) Type() string {
	return "summary"
}

func (s *noSummaryValue) String() string {
	// flip all the bits, since the flag value is the negative of what is stored
	return (testjson.SummarizeAll ^ s.value).String()
}

var junitFieldFormatValues = "full, relative, short"

type junitFieldFormatValue struct {
	value junitxml.FormatFunc
}

func (f *junitFieldFormatValue) Set(val string) error {
	switch val {
	case "full":
		return nil
	case "relative":
		f.value = testjson.RelativePackagePath
		return nil
	case "short":
		f.value = path.Base
		return nil
	}
	return errors.Errorf("invalid value: %v, must be one of: "+junitFieldFormatValues, val)
}

func (f *junitFieldFormatValue) Type() string {
	return "field-format"
}

func (f *junitFieldFormatValue) String() string {
	return "full"
}

func (f *junitFieldFormatValue) Value() junitxml.FormatFunc {
	if f == nil {
		return nil
	}
	return f.value
}

type commandValue struct {
	original string
	command  []string
}

func (c *commandValue) String() string {
	return c.original
}

func (c *commandValue) Set(raw string) error {
	var err error
	c.command, err = shlex.Split(raw)
	c.original = raw
	return err
}

func (c *commandValue) Type() string {
	return "command"
}

func (c *commandValue) Value() []string {
	if c == nil {
		return nil
	}
	return c.command
}

var _ pflag.Value = (*stringSlice)(nil)

// stringSlice is a flag.Value which populates the string slice by splitting
// the raw flag value on spaces.
type stringSlice []string

func (s *stringSlice) String() string {
	return strings.Join(*s, " ")
}

func (s *stringSlice) Set(raw string) error {
	*s = append(*s, strings.Split(raw, " ")...)
	return nil
}

func (s *stringSlice) Type() string {
	return "list"
}
