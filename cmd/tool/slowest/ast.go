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
	"gotest.tools/gotestsum/internal/log"
	"gotest.tools/gotestsum/testjson"
)

type skipStatement func(*ast.FuncDecl) (ast.Stmt, error)

const (
	defaultSkipPreset      = "default"
	testingShortSkipPreset = "testing.Short"
)

func writeTestSkip(tcs []testjson.TestCase, skipStmt skipStatement) error {
	fset := token.NewFileSet()
	cfg := packages.Config{
		Mode:       modeAll(),
		Tests:      true,
		Fset:       fset,
		BuildFlags: buildFlags(),
	}
	pkgNames, index := testNamesByPkgName(tcs)
	pkgs, err := packages.Load(&cfg, pkgNames...)
	if err != nil {
		return fmt.Errorf("failed to load packages: %v", err)
	}
	type sourceFile struct {
		path string
		file *ast.File
	}
	var pending []sourceFile

	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			return errPkgLoad(pkg)
		}
		tcs, ok := index[normalizePkgName(pkg.PkgPath)]
		if !ok {
			log.Debugf("skipping %v, no slow tests", pkg.PkgPath)
			continue
		}

		log.Debugf("rewriting %v for %d test cases", pkg.PkgPath, len(tcs))
		for _, file := range pkg.Syntax {
			path := fset.File(file.Pos()).Name()
			log.Debugf("looking for test cases in: %v", path)
			modified, err := rewriteAST(file, tcs, skipStmt)
			if err != nil {
				return fmt.Errorf("failed to rewrite %v: %v", path, err)
			}
			if !modified {
				continue
			}
			pending = append(pending, sourceFile{path: path, file: file})
		}
	}
	if err := errTestCasesNotFound(index); err != nil {
		return err
	}
	for _, item := range pending {
		if err := writeFile(item.path, item.file, fset); err != nil {
			return fmt.Errorf("failed to write ast to file %v: %v", item.path, err)
		}
	}
	return nil
}

// normalizePkgName removes the _test suffix from a package name. External test
// packages (those named package_test) may contain tests, but the test2json output
// always uses the non-external package name. The _test suffix must be removed
// so that any slow tests in an external test package can be found.
func normalizePkgName(name string) string {
	return strings.TrimSuffix(name, "_test")
}

func writeFile(path string, file *ast.File, fset *token.FileSet) error {
	fh, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if err := fh.Close(); err != nil {
			log.Errorf("Failed to close file %v: %v", path, err)
		}
	}()
	return format.Node(fh, fset, file)
}

func parseSkipStatement(text string) (ast.Stmt, error) {
	switch text {
	case defaultSkipPreset, testingShortSkipPreset:
		return testingShortSkipStatement("t"), nil
	}
	// Add some required boilerplate around the statement to make it a valid file
	text = "package stub\nfunc Stub() {\n" + text + "\n}\n"
	file, err := parser.ParseFile(token.NewFileSet(), "fragment", text, 0)
	if err != nil {
		return nil, err
	}
	stmt := file.Decls[0].(*ast.FuncDecl).Body.List[0]
	return stmt, nil
}

func newSkipStatement(text string) (skipStatement, error) {
	switch text {
	case defaultSkipPreset, testingShortSkipPreset:
		return func(fd *ast.FuncDecl) (ast.Stmt, error) {
			if fd.Type.Params == nil || len(fd.Type.Params.List) == 0 {
				return nil, fmt.Errorf("%s has no test parameter for the testing.Short preset", fd.Name.Name)
			}
			param := fd.Type.Params.List[0]
			if len(param.Names) > 0 && param.Names[0].Name != "_" {
				return testingShortSkipStatement(param.Names[0].Name), nil
			}

			// Name an unused parameter without shadowing names in the test body.
			used := make(map[string]bool)
			ast.Inspect(fd, func(node ast.Node) bool {
				if ident, ok := node.(*ast.Ident); ok {
					used[ident.Name] = true
				}
				return true
			})
			name := "t"
			for used[name] {
				name += "_"
			}
			param.Names = []*ast.Ident{ast.NewIdent(name)}
			return testingShortSkipStatement(name), nil
		}, nil
	}

	stmt, err := parseSkipStatement(text)
	return func(*ast.FuncDecl) (ast.Stmt, error) { return stmt, nil }, err
}

func testingShortSkipStatement(name string) ast.Stmt {
	return &ast.IfStmt{
		Cond: &ast.CallExpr{Fun: &ast.SelectorExpr{X: ast.NewIdent("testing"), Sel: ast.NewIdent("Short")}},
		Body: &ast.BlockStmt{List: []ast.Stmt{&ast.ExprStmt{X: &ast.CallExpr{
			Fun:  &ast.SelectorExpr{X: ast.NewIdent(name), Sel: ast.NewIdent("Skip")},
			Args: []ast.Expr{&ast.BasicLit{Kind: token.STRING, Value: `"too slow for testing.Short"`}},
		}}}},
	}
}

func rewriteAST(file *ast.File, testNames set, skipStmt skipStatement) (bool, error) {
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

		stmt, err := skipStmt(fd)
		if err != nil {
			return false, err
		}
		fd.Body.List = append([]ast.Stmt{stmt}, fd.Body.List...)
		modified = true
		delete(testNames, name)
	}
	return modified, nil
}

type set map[string]struct{}

// testNamesByPkgName strips subtest names from test names, then builds
// and returns a slice of all the packages names, and a mapping of package name
// to set of failed tests in that package.
//
// subtests are removed because the AST lookup currently only works for top-level
// functions, not t.Run subtests.
func testNamesByPkgName(tcs []testjson.TestCase) ([]string, map[string]set) {
	var pkgs []string
	index := make(map[string]set)
	for _, tc := range tcs {
		testName := tc.Test.Name()
		if tc.Test.IsSubTest() {
			root, _ := tc.Test.Split()
			testName = root
		}
		if len(index[tc.Package]) == 0 {
			pkgs = append(pkgs, tc.Package)
			index[tc.Package] = make(map[string]struct{})
		}
		index[tc.Package][testName] = struct{}{}
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
	return fmt.Errorf("failed to find source for test cases:\n%v", strings.Join(missed, "\n"))
}

func modeAll() packages.LoadMode {
	mode := packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles
	mode = mode | packages.NeedImports | packages.NeedDeps
	mode = mode | packages.NeedTypes | packages.NeedTypesSizes
	mode = mode | packages.NeedSyntax | packages.NeedTypesInfo
	return mode
}

func buildFlags() []string {
	flags := os.Getenv("GOFLAGS")
	if len(flags) == 0 {
		return nil
	}
	return strings.Split(os.Getenv("GOFLAGS"), " ")
}
