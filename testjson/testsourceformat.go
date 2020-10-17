package testjson

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"io"
	"os"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
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
	root, sub := SplitTestName(event.Test)
	if sub != "" {
		// TODO: make it work with subtests, for now print all subs as part of the root
		return nil
	}

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
		line: func(pos token.Pos) int {
			return src.fileset.Position(pos + decl.Pos()).Line
		},
	}
	if err := cfg.Fprint(writer, src.fileset, decl); err != nil {
		return err
	}
	return t.write("\n")
}

var _ EventFormatter = (*testSourceFormatter)(nil)

type colorIndex map[token.Pos]func(w io.Writer) (int, error)

func newColorIndex(node ast.Node) colorIndex {
	if color.NoColor {
		return nil
	}
	index := make(colorIndex)
	offset := node.Pos()
	add := func(node ast.Node, c color.Attribute) {
		if node == nil {
			return
		}
		index[node.Pos()-offset] = color.Color(c)
		end := node.End() - offset
		if _, exists := index[end]; !exists {
			index[end] = color.Unset
		}
	}

	h := &highlighter{add: add}
	astutil.Apply(node, h.Pre, h.Post)
	return index
}

type highlighter struct {
	add             func(node ast.Node, c color.Attribute)
	inFuncFieldList bool
}

func (h *highlighter) Pre(c *astutil.Cursor) bool {
	node := c.Node()
	if node == nil {
		return false
	}

	switch n := node.(type) {
	case *ast.FuncDecl:
		h.add(n.Name, color.Hex(yellow))

	case *ast.FuncType:
		h.add(newTokenPos(n.Pos(), token.FUNC), color.Hex(orange))
		h.inFuncFieldList = true

	case *ast.BasicLit:
		switch n.Kind {
		case token.STRING, token.CHAR:
			h.add(n, color.Hex(green))
		default:
			h.add(n, color.Hex(blue))
		}

	case *ast.RangeStmt:
		h.add(newTokenPos(n.For, token.FOR), color.Hex(orange))
		start := n.TokPos + token.Pos(len(n.Tok.String()))
		h.add(position{start: start, end: n.X.Pos()}, color.Hex(orange))

	case *ast.UnaryExpr:
		fmt.Println("UNARY GOT YA")
		switch n.Op {
		case token.RANGE:
			h.add(newTokenPos(n.Pos(), token.RANGE), color.Hex(orange))
		}

	case *ast.Ident:
		switch n.Name {
		case "string":
			h.add(n, color.Hex(orange))
		}

	case *ast.SelectorExpr:
		switch {
		case h.inFuncFieldList:
			h.add(n.Sel, color.Hex(blue))
			h.add(n.X, color.Hex(lightGreen))
		case isCallExpr(c.Parent()):
			if isPackageIdent(n.X) {
				h.add(n.X, color.Hex(lightGreen))
			}
			h.add(n.Sel, color.Hex(lightYellow))
		default:
			if isPackageIdent(n.X) {
				h.add(n.X, color.Hex(lightGreen))
			}
			// TODO: only for constants
			//h.add(n.Sel, color.Hex(purple))
		}
	}
	return true
}

func (h *highlighter) Post(c *astutil.Cursor) bool {
	node := c.Node()
	switch node.(type) {
	case *ast.FuncType:
		h.inFuncFieldList = false
	}
	return true
}

func isCallExpr(n ast.Node) bool {
	_, ok := n.(*ast.CallExpr)
	return ok
}

func isPackageIdent(n ast.Expr) bool {
	ident, ok := n.(*ast.Ident)
	return ok && ident.Obj == nil
}

const (
	orange      = 0xC7773E
	yellow      = 0xE6B163
	purple      = 0x9876AA
	blue        = 0x6897BB
	green       = 0x6A8759
	lightGreen  = 0xAFBF7E
	lightYellow = 0xB09D79
	red         = 0xFF0000
)

type position struct {
	start, end token.Pos
}

func newTokenPos(start token.Pos, tok token.Token) position {
	end := int(start) + len(tok.String())
	return position{start: start, end: token.Pos(end)}
}

func (p position) Pos() token.Pos {
	return p.start
}

func (p position) End() token.Pos {
	return p.end
}

type syntaxHighlighter struct {
	out   io.Writer
	index colorIndex
	pos   token.Pos
	line  func(pos token.Pos) int
}

func (s *syntaxHighlighter) Write(raw []byte) (int, error) {
	if len(raw) == 0 {
		return 0, nil
	}
	buf := bufio.NewWriter(s.out)
	// TODO: better way to handle this? in one place
	if s.pos == 0 {
		s.writeLineNumber(buf)
	}
	for _, b := range raw {
		if fn := s.index[s.pos]; fn != nil {
			fn(buf)
		}
		// replace tabs with 4 spaces here instead of the ast printer so that
		// s.pos advances the correct number of bytes to match the positions
		// in index.
		next := []byte{b}
		if b == '\t' {
			next = []byte("    ")
		}
		buf.Write(next)
		if b == '\n' {
			s.writeLineNumber(buf)
		}
		s.pos++
	}
	return len(raw), buf.Flush()
}

func (s *syntaxHighlighter) writeLineNumber(out io.Writer) {
	color.Color(color.Hex(0x777777))(out)
	fmt.Fprintf(out, "%4d ", s.line(s.pos+1))
	color.Unset(out)
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
