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
	return run(*opts)
}

type options struct {
	pruneFilesMaxAgeDays uint
	buckets              uint
	timingFilesPattern   string
	debug                bool
}

func setupFlags(name string) (*pflag.FlagSet, *options) {
	opts := &options{}
	flags := pflag.NewFlagSet(name, pflag.ContinueOnError)
	flags.SetInterspersed(false)
	flags.Usage = func() {
		usage(os.Stdout, name, flags)
	}
	flags.UintVar(&opts.pruneFilesMaxAgeDays, "max-age-days", 0,
		"timing files older than this value will be deleted")
	flags.UintVar(&opts.buckets, "buckets", 4,
		"number of parallel buckets to create in the test matrix")
	flags.StringVar(&opts.timingFilesPattern, "timing-files", "",
		"glob pattern to match files that contain test2json events, ex: ./logs/*.log")
	flags.BoolVar(&opts.debug, "debug", false,
		"enable debug logging")
	return flags, opts
}

func usage(out io.Writer, name string, flags *pflag.FlagSet) {
	fmt.Fprintf(out, `Usage:
    %[1]s [flags]

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
	if opts.buckets < 2 {
		return fmt.Errorf("--buckets must be atleast 2")
	}
	if opts.timingFilesPattern == "" {
		return fmt.Errorf("--timing-files is required")
	}

	pkgs, err := readPackages(os.Stdin)
	if err != nil {
		return fmt.Errorf("failed to read packages from stdin: %v", err)
	}

	files, err := readAndPruneTimingReports(opts)
	if err != nil {
		return fmt.Errorf("failed to read or delete timing files: %v", err)
	}
	defer closeFiles(files)

	pkgTiming, err := packageTiming(files)
	if err != nil {
		return err
	}

	buckets := bucketPackages(packagePercentile(pkgTiming), pkgs, opts.buckets)
	return writeBuckets(buckets)
}

func readPackages(stdin io.Reader) ([]string, error) {
	var packages []string
	scan := bufio.NewScanner(stdin)
	for scan.Scan() {
		packages = append(packages, scan.Text())
	}
	return packages, scan.Err()
}

func readAndPruneTimingReports(opts options) ([]*os.File, error) {
	fileNames, err := filepath.Glob(opts.timingFilesPattern)
	if err != nil {
		return nil, err
	}

	var files []*os.File
	for _, fileName := range fileNames {
		fh, err := os.Open(fileName)
		if err != nil {
			return nil, err
		}

		event, err := parseEvent(fh)
		if err != nil {
			return nil, fmt.Errorf("failed to read first event from %v: %v", fh.Name(), err)
		}

		age := time.Since(event.Time)
		maxAge := time.Duration(opts.pruneFilesMaxAgeDays) * 24 * time.Hour
		if opts.pruneFilesMaxAgeDays == 0 || age < maxAge {
			if _, err := fh.Seek(0, io.SeekStart); err != nil {
				return nil, fmt.Errorf("failed to reset file: %v", err)
			}
			files = append(files, fh)
			continue
		}

		log.Infof("Removing %v because it is from %v", fh.Name(), event.Time.Format(time.RFC1123))
		_ = fh.Close()
		if err := os.Remove(fh.Name()); err != nil {
			return nil, err
		}
	}

	log.Infof("Found %v timing files in %v", len(files), opts.timingFilesPattern)
	return files, nil
}

func parseEvent(reader io.Reader) (testjson.TestEvent, error) {
	event := testjson.TestEvent{}
	err := json.NewDecoder(reader).Decode(&event)
	return event, err
}

func packageTiming(files []*os.File) (map[string][]time.Duration, error) {
	timing := make(map[string][]time.Duration)
	for _, fh := range files {
		exec, err := testjson.ScanTestOutput(testjson.ScanConfig{Stdout: fh})
		if err != nil {
			return nil, fmt.Errorf("failed to read events from %v: %v", fh.Name(), err)
		}

		for _, pkg := range exec.Packages() {
			log.Debugf("package elapsed time %v %v", pkg, exec.Package(pkg).Elapsed())
			timing[pkg] = append(timing[pkg], exec.Package(pkg).Elapsed())
		}
	}
	return timing, nil
}

func packagePercentile(timing map[string][]time.Duration) map[string]time.Duration {
	result := make(map[string]time.Duration)
	for pkg, times := range timing {
		lenTimes := len(times)
		if lenTimes == 0 {
			result[pkg] = 0
			continue
		}

		sort.Slice(times, func(i, j int) bool {
			return times[i] < times[j]
		})

		r := int(math.Ceil(0.85 * float64(lenTimes)))
		if r == 0 {
			result[pkg] = times[0]
			continue
		}
		result[pkg] = times[r-1]
	}
	return result
}

func closeFiles(files []*os.File) {
	for _, fh := range files {
		_ = fh.Close()
	}
}

func bucketPackages(timing map[string]time.Duration, packages []string, n uint) []bucket {
	sort.SliceStable(packages, func(i, j int) bool {
		return timing[packages[i]] >= timing[packages[j]]
	})

	buckets := make([]bucket, n)
	for _, pkg := range packages {
		i := minBucket(buckets)
		buckets[i].Total += timing[pkg]
		buckets[i].Packages = append(buckets[i].Packages, pkg)
		log.Debugf("adding %v (%v) to bucket %v with total %v",
			pkg, timing[pkg], i, buckets[i].Total)
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
		case b.Total == min && len(buckets[i].Packages) < len(buckets[n].Packages):
			n = i
		}
	}
	return n
}

type bucket struct {
	Total    time.Duration
	Packages []string
}

func writeBuckets(buckets []bucket) error {
	out := make(map[int]string)
	for i, bucket := range buckets {
		out[i] = strings.Join(bucket.Packages, " ")
	}

	raw, err := json.Marshal(out)
	if err != nil {
		return fmt.Errorf("failed to json encode output: %v", err)
	}
	log.Debugf(string(raw))
	fmt.Println(string(raw))
	return nil
}
