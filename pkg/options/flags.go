package options

import (
	"encoding/csv"
	"github.com/astralkn/gotestmng/pkg/junitxml"
	"github.com/astralkn/gotestmng/pkg/testjson"
	"path"
	"strings"

	"github.com/pkg/errors"
)

type NoSummaryValue struct {
	Value testjson.Summary
}

func NewNoSummaryValue() *NoSummaryValue {
	return &NoSummaryValue{Value: testjson.SummarizeAll}
}

func readAsCSV(val string) ([]string, error) {
	if val == "" {
		return nil, nil
	}
	return csv.NewReader(strings.NewReader(val)).Read()
}

func (s *NoSummaryValue) Set(val string) error {
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
		s.Value -= summary
	}
	return nil
}

func (s *NoSummaryValue) Type() string {
	return "summary"
}

func (s *NoSummaryValue) String() string {
	// flip all the bits, since the flag value is the negative of what is stored
	return (testjson.SummarizeAll ^ s.Value).String()
}

var JunitFieldFormatValues = "full, relative, short"

type JunitFieldFormatValue struct {
	value junitxml.FormatFunc
}

func (f *JunitFieldFormatValue) Set(val string) error {
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
	return errors.Errorf("invalid value: %v, must be one of: "+JunitFieldFormatValues, val)
}

func (f *JunitFieldFormatValue) Type() string {
	return "field-format"
}

func (f *JunitFieldFormatValue) String() string {
	return "full"
}

func (f *JunitFieldFormatValue) Value() junitxml.FormatFunc {
	if f == nil {
		return nil
	}
	return f.value
}
