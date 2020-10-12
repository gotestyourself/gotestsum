package testjson

import (
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"io"
	"strings"

	"golang.org/x/tools/go/packages"
)

func newTestSourceFormatter(out io.Writer) *testSourceFormatter {
	return &testSourceFormatter{out: out, astCache: make(map[string]pkgSource)}
}

type testSourceFormatter struct {
	out      io.Writer
	astCache map[string]pkgSource
}

func (t *testSourceFormatter) Format(event TestEvent, exec *Execution) error {
	output, err := testNameFormat(event, exec)
	if err != nil {
		return err
	}

	if !event.PackageEvent() && event.Action == ActionFail {
		src, err := t.loadSource(event.Package)
		if err != nil {
			return err
		}
		if err := t.writeSource(src, event); err != nil {
			return err
		}
	}

	return t.write(output)
}

func (t *testSourceFormatter) write(v string) error {
	_, err := t.out.Write([]byte(v))
	return err
}

func (t *testSourceFormatter) writeSource(src pkgSource, event TestEvent) error {
	decl, ok := src.tests[event.Test]
	if !ok {
		return fmt.Errorf("failed to locate source for %v", event.Test)
	}
	if err := t.write("\n"); err != nil {
		return err
	}
	cfg := &printer.Config{
		Mode:     printer.UseSpaces,
		Tabwidth: 4,
	}
	if err := cfg.Fprint(t.out, src.fileset, decl); err != nil {
		return err
	}
	return t.write("\n")
}

// TODO: test with external test package.
func (t *testSourceFormatter) loadSource(name string) (pkgSource, error) {
	src, ok := t.astCache[name]
	if ok {
		return src, nil
	}
	cfg := &packages.Config{
		Mode:  modeAll(),
		Tests: true,
		Fset:  token.NewFileSet(),
	}
	pkgs, err := packages.Load(cfg, name)
	if err != nil {
		return src, err
	}

	src = pkgSource{
		fileset: cfg.Fset,
		tests:   make(map[string]*ast.FuncDecl),
	}
	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			return src, errPkgLoad(pkg)
		}

		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}

				if fn.Name == nil || !strings.HasPrefix(fn.Name.Name, "Test") {
					continue
				}

				src.tests[fn.Name.Name] = fn
			}
		}
		t.astCache[name] = src
	}
	return src, nil
}

var _ EventFormatter = (*testSourceFormatter)(nil)

type pkgSource struct {
	fileset *token.FileSet
	tests   map[string]*ast.FuncDecl
}

func errPkgLoad(pkg *packages.Package) error {
	buf := new(strings.Builder)
	for _, err := range pkg.Errors {
		buf.WriteString("\n" + err.Error())
	}
	return fmt.Errorf("failed to load package %v %v", pkg.PkgPath, buf.String())
}

// TODO: can any be removed?
func modeAll() packages.LoadMode {
	mode := packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles
	mode = mode | packages.NeedImports | packages.NeedDeps
	mode = mode | packages.NeedTypes | packages.NeedTypesSizes
	mode = mode | packages.NeedSyntax | packages.NeedTypesInfo
	return mode
}
