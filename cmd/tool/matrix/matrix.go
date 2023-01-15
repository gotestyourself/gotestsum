package matrix

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dnephin/pflag"
	"gotest.tools/gotestsum/internal/log"
	"gotest.tools/gotestsum/testjson"
)

func Run(name string, args []string) error {
	flags, opts := setupFlags(name)
	switch err := flags.Parse(args); {
	case err == pflag.ErrHelp:
		return nil
	case err != nil:
		usage(os.Stderr, name, flags)
		return err
	}
	opts.stdin = os.Stdin
	opts.stdout = os.Stdout
	return run(*opts)
}

type options struct {
	numPartitions           uint
	timingFilesPattern      string
	partitionTestsInPackage string
	debug                   bool

	// shims for testing
	stdin  io.Reader
	stdout io.Writer
}

func setupFlags(name string) (*pflag.FlagSet, *options) {
	opts := &options{}
	flags := pflag.NewFlagSet(name, pflag.ContinueOnError)
	flags.SetInterspersed(false)
	flags.Usage = func() {
		usage(os.Stdout, name, flags)
	}
	flags.UintVar(&opts.numPartitions, "partitions", 0,
		"number of parallel partitions to create in the test matrix")
	flags.StringVar(&opts.timingFilesPattern, "timing-files", "",
		"glob pattern to match files that contain test2json events, ex: ./logs/*.log")
	flags.StringVar(&opts.partitionTestsInPackage, "partition-tests-in-package", "",
		"partition the tests in a single package instead of partitioning by package")
	flags.BoolVar(&opts.debug, "debug", false,
		"enable debug logging")
	return flags, opts
}

func usage(out io.Writer, name string, flags *pflag.FlagSet) {
	fmt.Fprintf(out, `Usage:
    %[1]s [flags]

Read a list of packages from stdin and output a GitHub Actions matrix strategy
that splits the packages by previous run times to minimize overall CI runtime.

    echo -n "matrix=" >> $GITHUB_OUTPUT
    go list ./... | %[1]s --timing-files ./*.log --partitions 4 >> $GITHUB_OUTPUT

The output of the command is a JSON object that can be used as the matrix
strategy for a test job.


When the --partition-tests-in-package flag is set to the name of a package, this
command will output a matrix that partitions the tests in that one package. In
this mode the command reads a list of test names from stdin.

Example

    echo -n "::set-output name=matrix::"
    go test --list github.com/example/pkg | \
        %[1]s --partitions 5 \
        --partition-tests-in-package github.com/example/pkg \
        --timing-files ./*.log --max-age-days 10

Flags:
`, name)
	flags.SetOutput(out)
	flags.PrintDefaults()
}

func run(opts options) error {
	log.SetLevel(log.InfoLevel)
	if opts.debug {
		log.SetLevel(log.DebugLevel)
	}
	if opts.numPartitions < 2 {
		return fmt.Errorf("--partitions must be atleast 2")
	}
	if opts.timingFilesPattern == "" {
		return fmt.Errorf("--timing-files is required")
	}

	inputs, err := readPackagesOrFiles(opts.stdin)
	if err != nil {
		return fmt.Errorf("failed to read packages from stdin: %v", err)
	}

	files, err := readTimingReports(opts)
	if err != nil {
		return fmt.Errorf("failed to read or delete timing files: %v", err)
	}
	defer closeFiles(files)

	timing, err := aggregateByName(files, opts.partitionTestsInPackage)
	if err != nil {
		return err
	}

	buckets := createBuckets(percentile(timing), inputs, opts.numPartitions)
	return writeMatrix(opts, buckets)
}

func readPackagesOrFiles(stdin io.Reader) ([]string, error) {
	var packages []string
	scan := bufio.NewScanner(stdin)
	for scan.Scan() {
		packages = append(packages, scan.Text())
	}
	return packages, scan.Err()
}

func readTimingReports(opts options) ([]*os.File, error) {
	fileNames, err := filepath.Glob(opts.timingFilesPattern)
	if err != nil {
		return nil, err
	}

	files := make([]*os.File, 0, len(fileNames))
	for _, fileName := range fileNames {
		fh, err := os.Open(fileName)
		if err != nil {
			return nil, err
		}
		files = append(files, fh)
	}

	log.Infof("Found %v timing files in %v", len(files), opts.timingFilesPattern)
	return files, nil
}

func parseEvent(reader io.Reader) (testjson.TestEvent, error) {
	event := testjson.TestEvent{}
	err := json.NewDecoder(reader).Decode(&event)
	return event, err
}

func aggregateByName(files []*os.File, pkgName string) (map[string][]time.Duration, error) {
	timing := make(map[string][]time.Duration)
	for _, fh := range files {
		exec, err := testjson.ScanTestOutput(testjson.ScanConfig{Stdout: fh})
		if err != nil {
			return nil, fmt.Errorf("failed to read events from %v: %v", fh.Name(), err)
		}

		if pkgName != "" {
			pkg := exec.Package(pkgName)
			if pkg == nil {
				return nil, nil
			}

			for _, tc := range pkg.TestCases() {
				timing[tc.Test.Name()] = append(timing[tc.Test.Name()], tc.Elapsed)
			}
			continue
		}

		for _, pkg := range exec.Packages() {
			timing[pkg] = append(timing[pkg], exec.Package(pkg).Elapsed())
		}
	}
	return timing, nil
}

func percentile(timing map[string][]time.Duration) map[string]time.Duration {
	result := make(map[string]time.Duration)
	for group, times := range timing {
		lenTimes := len(times)
		if lenTimes == 0 {
			result[group] = 0
			continue
		}

		sort.Slice(times, func(i, j int) bool {
			return times[i] < times[j]
		})

		r := int(math.Ceil(0.85 * float64(lenTimes)))
		if r == 0 {
			result[group] = times[0]
			continue
		}
		result[group] = times[r-1]
	}
	return result
}

func closeFiles(files []*os.File) {
	for _, fh := range files {
		_ = fh.Close()
	}
}

func createBuckets(timing map[string]time.Duration, item []string, n uint) []bucket {
	sort.SliceStable(item, func(i, j int) bool {
		return timing[item[i]] >= timing[item[j]]
	})

	buckets := make([]bucket, n)
	for _, name := range item {
		i := minBucket(buckets)
		buckets[i].Total += timing[name]
		buckets[i].Items = append(buckets[i].Items, name)
		log.Debugf("adding %v (%v) to bucket %v with total %v",
			name, timing[name], i, buckets[i].Total)
	}
	return buckets
}

func minBucket(buckets []bucket) int {
	var n int
	var min time.Duration = -1
	for i, b := range buckets {
		switch {
		case min < 0 || b.Total < min:
			min = b.Total
			n = i
		case b.Total == min && len(buckets[i].Items) < len(buckets[n].Items):
			n = i
		}
	}
	return n
}

type bucket struct {
	Total time.Duration
	// Items is the name of packages in the default mode, or the name of tests
	// in partition-by-test mode.
	Items []string
}

type matrix struct {
	Include []Partition `json:"include"`
}

type Partition struct {
	ID               int    `json:"id"`
	EstimatedRuntime string `json:"estimatedRuntime"`
	Packages         string `json:"packages"`
	Tests            string `json:"tests,omitempty"`
	Description      string `json:"description"`
}

func writeMatrix(opts options, buckets []bucket) error {
	m := matrix{Include: make([]Partition, 0, len(buckets))}
	for i, bucket := range buckets {
		if len(bucket.Items) == 0 {
			continue
		}

		p := Partition{
			ID:               i,
			EstimatedRuntime: bucket.Total.String(),
		}

		if opts.partitionTestsInPackage != "" {
			p.Packages = opts.partitionTestsInPackage
			p.Description = fmt.Sprintf("partition %d with %d tests", p.ID, len(bucket.Items))
			p.Tests = fmt.Sprintf("-run='^%v$'", strings.Join(bucket.Items, "$,^"))

			m.Include = append(m.Include, p)
			continue
		}

		p.Packages = strings.Join(bucket.Items, " ")

		var extra string
		if len(bucket.Items) > 1 {
			extra = fmt.Sprintf(" and %d others", len(bucket.Items)-1)
		}
		p.Description = fmt.Sprintf("partition %d - package %v%v",
			p.ID, testjson.RelativePackagePath(bucket.Items[0]), extra)

		m.Include = append(m.Include, p)
	}

	log.Debugf("%v\n", debugMatrix(m))

	err := json.NewEncoder(opts.stdout).Encode(m)
	if err != nil {
		return fmt.Errorf("failed to json encode output: %v", err)
	}
	return nil
}

type debugMatrix matrix

func (d debugMatrix) String() string {
	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return fmt.Sprintf("failed to marshal: %v", err.Error())
	}
	return string(raw)
}
