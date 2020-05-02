package slowest

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"strings"

	"golang.org/x/tools/go/packages"
	"gotest.tools/gotestsum/log"
	"gotest.tools/gotestsum/testjson"
)

func writeTestSkip(tcs []testjson.TestCase, skipStmt ast.Stmt) error {
	fset := token.NewFileSet()
	cfg := packages.Config{
		Mode:  modeAll(),
		Tests: true,
		Fset:  fset,
		// FIXME: BuildFlags: strings.Split(os.Getenv("GOFLAGS"), " "),
	}
	pkgNames, index := testNamesByPkgName(tcs)
	pkgs, err := packages.Load(&cfg, pkgNames...)
	if err != nil {
		return fmt.Errorf("failed to load packages: %w", err)
	}

	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			return errPkgLoad(pkg)
		}
		tcs, ok := index[pkg.PkgPath]
		if !ok {
			log.Debugf("skipping %v, no slow tests", pkg.PkgPath)
			continue
		}

		log.Debugf("rewriting %v for %d test cases", pkg.PkgPath, len(tcs))
		for _, file := range pkg.Syntax {
			path := fset.File(file.Pos()).Name()
			log.Debugf("looking for test cases in: %v", path)
			if !rewriteAST(file, tcs, skipStmt) {
				continue
			}
			if err := writeFile(path, file, fset); err != nil {
				return fmt.Errorf("failed to write ast to file %v: %w", path, err)
			}
		}
	}
	return errTestCasesNotFound(index)
}

// TODO: sometimes this writes the new AST with strange indentation. It appears
// to be non-deterministic. Given the same input, it only happens sometimes.
func writeFile(path string, file *ast.File, fset *token.FileSet) error {
	fh, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fh.Close()
	return format.Node(fh, fset, file)
}

// TODO: support preset values. Maybe that will help with the problem
// of strange indentation described on writeFile.
func parseSkipStatement(text string) (ast.Stmt, error) {
	// Add some required boilerplate around the statement to make it a valid file
	text = "package stub\nfunc Stub() {\n" + text + "\n}\n"
	file, err := parser.ParseFile(token.NewFileSet(), "fragment", text, 0)
	if err != nil {
		return nil, err
	}
	stmt := file.Decls[0].(*ast.FuncDecl).Body.List[0]
	return stmt, nil
}

func rewriteAST(file *ast.File, testNames set, skipStmt ast.Stmt) bool {
	var modified bool
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		name := fd.Name.Name // TODO: can this be nil?
		if _, ok := testNames[name]; !ok {
			continue
		}

		fd.Body.List = append([]ast.Stmt{skipStmt}, fd.Body.List...)
		modified = true
		delete(testNames, name)
	}
	return modified
}

type set map[string]struct{}

// FIXME: this should drop subtests from the index, so that errTestCasesNotFound
// does not report an error when we can't find the test function.
func testNamesByPkgName(tcs []testjson.TestCase) ([]string, map[string]set) {
	pkgs := make([]string, 0, len(tcs))
	index := make(map[string]set)
	for _, tc := range tcs {
		if len(index[tc.Package]) == 0 {
			pkgs = append(pkgs, tc.Package)
			index[tc.Package] = make(map[string]struct{})
		}
		index[tc.Package][tc.Test] = struct{}{}
	}
	return pkgs, index
}

func errPkgLoad(pkg *packages.Package) error {
	buf := new(strings.Builder)
	for _, err := range pkg.Errors {
		buf.WriteString("\n" + err.Error())
	}
	return fmt.Errorf("failed to load package %v %v", pkg.PkgPath, buf.String())
}

func errTestCasesNotFound(index map[string]set) error {
	var missed []string
	for pkg, tcs := range index {
		for tc := range tcs {
			missed = append(missed, fmt.Sprintf("%v.%v", pkg, tc))
		}
	}
	if len(missed) == 0 {
		return nil
	}
	return fmt.Errorf("failed to find source for test cases: %v", strings.Join(missed, ","))
}

func modeAll() packages.LoadMode {
	mode := packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles
	mode = mode | packages.NeedImports | packages.NeedDeps
	mode = mode | packages.NeedTypes | packages.NeedTypesSizes
	mode = mode | packages.NeedSyntax | packages.NeedTypesInfo
	return mode
}
