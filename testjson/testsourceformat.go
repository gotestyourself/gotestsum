package testjson

import (
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"io"
	"os"
	"strings"

	"golang.org/x/tools/go/packages"
	"gotest.tools/gotestsum/internal/color"
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

// TODO: test with external test package
// TODO: document how build tags need to be specified
func (t *testSourceFormatter) loadSource(name string) (pkgSource, error) {
	src, ok := t.astCache[name]
	if ok {
		return src, nil
	}
	cfg := &packages.Config{
		Mode:       modeAll(),
		Tests:      true,
		Fset:       token.NewFileSet(),
		BuildFlags: buildFlags(),
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

// TODO: test with source that is not gofmt formatted
func (t *testSourceFormatter) writeSource(src pkgSource, event TestEvent) error {
	root, _ := SplitTestName(event.Test)
	// TODO: make it work with subtests

	decl, ok := src.tests[root]
	if !ok {
		return fmt.Errorf("failed to locate source for %v", event.Test)
	}
	if err := t.write("\n"); err != nil {
		return err
	}
	cfg := &printer.Config{Tabwidth: 4}
	writer := &syntaxHighlighter{
		out:   t.out,
		index: newColorIndex(decl),
	}
	if err := cfg.Fprint(writer, src.fileset, decl); err != nil {
		return err
	}
	return t.write("\n")
}

var _ EventFormatter = (*testSourceFormatter)(nil)

type colorIndex map[token.Pos]func(w io.Writer) (int, error)

func newColorIndex(node ast.Node) colorIndex {
	index := make(colorIndex)
	offset := node.Pos()
	add := func(node ast.Node, fn func(w io.Writer) (int, error)) {
		if node == nil || fn == nil {
			return
		}
		index[node.Pos()-offset] = fn
		end := node.End() - offset
		if _, exists := index[end]; !exists {
			index[end] = color.Unset
		}
	}

	ast.Inspect(node, func(node ast.Node) bool {
		if node == nil {
			return true
		}

		switch n := node.(type) {
		case *ast.FuncDecl:
			add(n.Type, color.Color(color.Hex(orange)))
			add(n.Name, color.Color(color.Hex(yellow)))

		case *ast.SelectorExpr:
			//add(n.Sel, color.Color(color.Hex(blue)))
			//add(n.X, color.Color(color.Hex(lightGreen)))

		case *ast.BasicLit:
			switch n.Kind {
			case token.STRING, token.CHAR:
				add(n, color.Color(color.Hex(green)))
			default:
				add(n, color.Color(color.Hex(blue)))
			}

		case *ast.UnaryExpr:
			n.End()
			switch n.Op {
			case token.RANGE, token.FOR:
				//add(n, color.Color(color.Hex(orange)))
			}
		}
		return true
	})
	return index
}

const (
	orange     = 0xC7773E
	yellow     = 0xE6B163
	purple     = 0x9876AA
	blue       = 0x6897BB
	green      = 0x6A8759
	lightGreen = 0xAFBF7E
)

type syntaxHighlighter struct {
	out   io.Writer
	index colorIndex
	pos   token.Pos
}

func (s *syntaxHighlighter) Write(raw []byte) (int, error) {
	for i, b := range raw {
		if fn := s.index[s.pos]; fn != nil {
			if _, err := fn(s.out); err != nil {
				return i, err
			}
		}
		// replace tabs with 4 spaces here instead of the ast printer so that
		// s.pos advances the correct number of bytes to match the positions
		// in index.
		next := []byte{b}
		if b == '\t' {
			next = []byte("    ")
		}
		if _, err := s.out.Write(next); err != nil {
			return i, err
		}
		s.pos++
	}
	return len(raw), nil
}

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

func buildFlags() []string {
	flags := os.Getenv("GOFLAGS")
	if len(flags) == 0 {
		return nil
	}
	return strings.Split(os.Getenv("GOFLAGS"), " ")
}
