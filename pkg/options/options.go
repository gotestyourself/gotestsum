package options

import "github.com/astralkn/gotestmng/pkg/junitxml"

type Options struct {
	Args                         []string
	Format                       string
	Debug                        bool
	RawCommand                   bool
	JsonFile                     string
	JunitFile                    string
	NoColor                      bool
	NoSummary                    *NoSummaryValue
	JunitTestSuiteNameFormat     *JunitFieldFormatValue
	JunitTestCaseClassnameFormat *JunitFieldFormatValue
	Version                      bool
	Post                         bool
	Token                        string
	Owner                        string
	Repo                         string
	JUnitTestSuite               junitxml.JUnitTestSuites
}
