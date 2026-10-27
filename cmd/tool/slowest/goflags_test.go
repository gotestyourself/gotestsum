package slowest

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/env"
)

func TestRunWithGoFlags(t *testing.T) {
	for _, tc := range []struct {
		name   string
		flags  string
		tagged bool
	}{
		{name: "empty"},
		{name: "build tag", flags: "-tags=slowestfixture", tagged: true},
		{name: "single quoted", flags: "'-ldflags=-s -w'"},
		{name: "double quoted", flags: `"-ldflags=-s -w"`},
		{name: "repeated spaces", flags: "-tags=slowestfixture   -trimpath", tagged: true},
		{name: "tabs", flags: "\t-trimpath\t-tags=slowestfixture\t", tagged: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			assert.NilError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/goflagsfixture\n\ngo 1.24.0\n"), 0600))
			source := "package fixture\nimport \"testing\"\nfunc TestSlow(t *testing.T) { t.Log(\"test\") }\n"
			if tc.tagged {
				source = "//go:build slowestfixture\n\n" + source
			}
			path := filepath.Join(dir, "fixture_test.go")
			assert.NilError(t, os.WriteFile(path, []byte(source), 0600))
			const results = `{"Action":"pass","Package":"example.com/goflagsfixture","Test":"TestSlow","Elapsed":1}`
			jsonfile := filepath.Join(dir, "results.json")
			assert.NilError(t, os.WriteFile(jsonfile, []byte(results), 0600))
			env.ChangeWorkingDir(t, dir)
			t.Setenv("GOFLAGS", tc.flags)
			assert.NilError(t, Run("slowest", []string{"--jsonfile", jsonfile, "--skip-stmt", "testing.Short"}))
			out, err := exec.Command("go", "test", "-json", "-short", "-count=1", ".").CombinedOutput()
			assert.NilError(t, err, "%s", out)
			assert.Assert(t, bytes.Contains(out, []byte(`"Action":"skip"`)), "%s", out)
		})
	}
}
