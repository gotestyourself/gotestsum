package slowest

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/env"
)

func TestRunSkipStatementParameters(t *testing.T) {
	for _, skipStmt := range []string{"testing.Short", "default", `if testing.Short() { custom.Skip("custom") }`} {
		t.Run(skipStmt, func(t *testing.T) {
			dir := t.TempDir()
			assert.NilError(t, os.WriteFile(
				filepath.Join(dir, "go.mod"), []byte("module example.com/slowestfixture\n\ngo 1.24.0\n"), 0600))
			const source = `package fixture
import "testing"
var custom = new(testing.T)
var t = "global"
func TestFirst(first *testing.T) { first.Log("first") }
func TestSecond(second *testing.T) { second.Log("second") }
func TestDefault(t *testing.T) { t.Log("control") }
func TestUnnamed(*testing.T) { if t != "global" { panic("shadowed") } }
func TestBlank(_ *testing.T) { if t != "global" { panic("shadowed") } }
`
			path := filepath.Join(dir, "fixture_test.go")
			assert.NilError(t, os.WriteFile(path, []byte(source), 0600))
			const results = `{"Action":"pass","Package":"example.com/slowestfixture","Test":"TestFirst","Elapsed":1}
{"Action":"pass","Package":"example.com/slowestfixture","Test":"TestSecond","Elapsed":1}
{"Action":"pass","Package":"example.com/slowestfixture","Test":"TestDefault","Elapsed":1}
{"Action":"pass","Package":"example.com/slowestfixture","Test":"TestUnnamed","Elapsed":1}
{"Action":"pass","Package":"example.com/slowestfixture","Test":"TestBlank","Elapsed":1}
`
			jsonfile := filepath.Join(dir, "results.json")
			assert.NilError(t, os.WriteFile(jsonfile, []byte(results), 0600))
			env.ChangeWorkingDir(t, dir)
			assert.NilError(t, Run("slowest", []string{"--jsonfile", jsonfile, "--skip-stmt", skipStmt}))

			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, 0)
			assert.NilError(t, err)
			conf := types.Config{Importer: importer.Default()}
			_, err = conf.Check("example.com/slowestfixture", fset, []*ast.File{file}, nil)
			assert.NilError(t, err)

			if skipStmt != testingShortSkipPreset && skipStmt != defaultSkipPreset {
				var calls int
				ast.Inspect(file, func(node ast.Node) bool {
					selector, ok := node.(*ast.SelectorExpr)
					if !ok || selector.Sel.Name != "Skip" {
						return true
					}
					if receiver, ok := selector.X.(*ast.Ident); ok && receiver.Name == "custom" {
						calls++
					}
					return true
				})
				assert.Equal(t, calls, 5)
			}
		})
	}
}

func TestRunSkipStatementExample(t *testing.T) {
	for _, skipStmt := range []string{"testing.Short", "default", `if testing.Short() { return }`} {
		t.Run(skipStmt, func(t *testing.T) {
			dir := t.TempDir()
			assert.NilError(t, os.WriteFile(
				filepath.Join(dir, "go.mod"), []byte("module example.com/examplefixture\n\ngo 1.24.0\n"), 0600))
			const testSource = `package fixture
import "testing"
func TestSlow(slow *testing.T) { slow.Log("test") }
`
			const exampleSource = `package fixture
import (
    "fmt"
    "testing"
)
var _ = testing.Short
func Example() {
    fmt.Println("example")
    // Output: example
}
`
			testPath := filepath.Join(dir, "a_test.go")
			examplePath := filepath.Join(dir, "z_test.go")
			assert.NilError(t, os.WriteFile(testPath, []byte(testSource), 0600))
			assert.NilError(t, os.WriteFile(examplePath, []byte(exampleSource), 0600))
			const results = `{"Action":"pass","Package":"example.com/examplefixture","Test":"TestSlow","Elapsed":1}
{"Action":"pass","Package":"example.com/examplefixture","Test":"Example","Elapsed":1}
`
			jsonfile := filepath.Join(dir, "results.json")
			assert.NilError(t, os.WriteFile(jsonfile, []byte(results), 0600))
			env.ChangeWorkingDir(t, dir)
			err := Run("slowest", []string{"--jsonfile", jsonfile, "--skip-stmt", skipStmt})
			if skipStmt == testingShortSkipPreset || skipStmt == defaultSkipPreset {
				assert.ErrorContains(t, err, "Example has no test parameter")
				for path, want := range map[string]string{testPath: testSource, examplePath: exampleSource} {
					got, err := os.ReadFile(path)
					assert.NilError(t, err)
					assert.Equal(t, string(got), want)
				}
				return
			}
			assert.NilError(t, err)
		})
	}
}
