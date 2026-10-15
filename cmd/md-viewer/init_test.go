package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// moduleRoot is the absolute path of the module, from this package's directory.
func moduleRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("Abs of the module root: %v", err)
	}
	return root
}

// goFiles calls visit for every non-test .go file under root, with its slash-separated path
// relative to root and its parsed syntax. Directories whose names start with "." or "_", and
// testdata, are skipped as the go tool skips them. untracked is skipped too: it holds scratch
// work, not project code. Every file is parsed whatever its build constraints, so every GOOS
// is covered.
func goFiles(root string, visit func(rel string, f *ast.File) error) error {
	fset := token.NewFileSet()
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDir(root, path, d) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		return visit(filepath.ToSlash(rel), f)
	})
}

// skipDir reports whether a walk under root skips the directory d at path: names starting with
// "." or "_", testdata, and untracked, which holds scratch work, not project code.
func skipDir(root, path string, d fs.DirEntry) bool {
	name := d.Name()
	return path != root && (strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") ||
		name == "testdata" || name == "untracked")
}

// plannerLabel is a planner's label: one capital letter from a fixed set and one or two digits,
// as a whole word.
var plannerLabel = regexp.MustCompile(`\b[PFDQ][0-9]{1,2}\b`)

// labelScope reports whether the file at the slash-separated path rel is text the label gate reads:
// every .go file, README.md and docs/*.md.
func labelScope(rel string) bool {
	return strings.HasSuffix(rel, ".go") || rel == "README.md" ||
		(strings.HasPrefix(rel, "docs/") && strings.HasSuffix(rel, ".md"))
}

// plannerLabels lists "file:line" for each line under root that holds a planner label, in the
// files labelScope accepts, test files included.
func plannerLabels(root string) ([]string, error) {
	var found []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDir(root, path, d) {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !labelScope(rel) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for n, line := range strings.Split(string(data), "\n") {
			if plannerLabel.MatchString(line) {
				found = append(found, fmt.Sprintf("%s:%d", rel, n+1))
			}
		}
		return nil
	})
	return found, err
}

// TestNoPlannerLabels: no Go file, README or doc names a planner's label (the labels belong to a
// planning tree, not to this repository). The fixture rows show the check finds a label and passes
// an expectation number such as DOC-14. The fixture labels are split across two literals so that
// this file holds none.
func TestNoPlannerLabels(t *testing.T) {
	t.Run("fixture", func(t *testing.T) {
		dir := writeTree(t, map[string]string{
			"x.go":           "package x\n\n// x (P" + "12)\n",
			"docs/SPEC.md":   "A rule (DOC-14) and a note (SPEED-6).\n",
			"docs/x.txt":     "not scanned: (F" + "3)\n",
			"README.md":      "See (D" + "1).\n",
			"nested/y.go":    "package y\n\n// the label is Q" + "9\n",
			"nested/y_ok.go": "package y\n\n// no label here: DOC-5\n",
		})
		got, err := plannerLabels(dir)
		if err != nil {
			t.Fatalf("plannerLabels: %v", err)
		}
		want := []string{"README.md:1", "nested/y.go:3", "x.go:3"}
		if !slices.Equal(got, want) {
			t.Errorf("plannerLabels = %q, want %q", got, want)
		}
	})

	t.Run("module", func(t *testing.T) {
		got, err := plannerLabels(moduleRoot(t))
		if err != nil {
			t.Fatalf("plannerLabels: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("planner labels at %q, want none", got)
		}
	})
}

// initFuncs lists "file: func init" for each func init declared in a non-test .go file under root.
func initFuncs(root string) ([]string, error) {
	var found []string
	err := goFiles(root, func(rel string, f *ast.File) error {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && fn.Recv == nil && fn.Name.Name == "init" {
				found = append(found, rel+": func init")
			}
		}
		return nil
	})
	return found, err
}

// goStatements lists the file of each go statement in a non-test .go file under root, once per
// statement.
func goStatements(root string) ([]string, error) {
	var found []string
	err := goFiles(root, func(rel string, f *ast.File) error {
		ast.Inspect(f, func(n ast.Node) bool {
			if _, ok := n.(*ast.GoStmt); ok {
				found = append(found, rel)
			}
			return true
		})
		return nil
	})
	return found, err
}

// windowCreations lists the file of each place a non-test .go file under root creates an app.Window:
// new(app.Window) or a composite literal app.Window{...}, once per creation. app is the local name
// of gioui.org/app in that file.
func windowCreations(root string) ([]string, error) {
	var found []string
	err := goFiles(root, func(rel string, f *ast.File) error {
		local := ""
		for _, imp := range f.Imports {
			if imp.Path.Value != `"gioui.org/app"` {
				continue
			}
			local = "app"
			if imp.Name != nil {
				local = imp.Name.Name
			}
		}
		if local == "" {
			return nil
		}
		isWindow := func(e ast.Expr) bool {
			sel, ok := e.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Window" {
				return false
			}
			x, ok := sel.X.(*ast.Ident)
			return ok && x.Name == local
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CallExpr:
				if id, ok := x.Fun.(*ast.Ident); ok && id.Name == "new" && len(x.Args) == 1 && isWindow(x.Args[0]) {
					found = append(found, rel)
				}
			case *ast.CompositeLit:
				if isWindow(x.Type) {
					found = append(found, rel)
				}
			}
			return true
		})
		return nil
	})
	return found, err
}

// writeTree writes files, a map of relative path to content, under a new temporary directory,
// and returns that directory.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatalf("mkdir for %s: %v", name, err)
		}
		writeFile(t, path, content)
	}
	return dir
}

// writeFile writes content to path, failing the test on error.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// TestNoInitFunctions: no package of md-viewer declares func init (SPEED-6). The fixture row
// shows the check finds one when there is one.
func TestNoInitFunctions(t *testing.T) {
	t.Run("fixture with an init", func(t *testing.T) {
		dir := writeTree(t, map[string]string{
			"x.go":      "package x\n\nfunc init() {}\n",
			"x_test.go": "package x\n\nfunc init() {}\n",
		})
		got, err := initFuncs(dir)
		if err != nil {
			t.Fatalf("initFuncs: %v", err)
		}
		if want := []string{"x.go: func init"}; !slices.Equal(got, want) {
			t.Errorf("initFuncs = %q, want %q", got, want)
		}
	})

	t.Run("module", func(t *testing.T) {
		root := moduleRoot(t)
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
			t.Fatalf("the module root %s has no go.mod: %v", root, err)
		}
		got, err := initFuncs(root)
		if err != nil {
			t.Fatalf("initFuncs: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("initFuncs = %q, want none", got)
		}
	})
}

// TestOneWindow (WIN-3): the module creates exactly one app.Window, in gioapp.Run. The fixture
// row shows the check counts each creation, both the call and the literal.
func TestOneWindow(t *testing.T) {
	t.Run("fixture with two windows", func(t *testing.T) {
		dir := writeTree(t, map[string]string{
			"x.go": "package x\n\nimport \"gioui.org/app\"\n\n" +
				"func f() {\n\t_ = new(app.Window)\n\t_ = &app.Window{}\n}\n",
		})
		got, err := windowCreations(dir)
		if err != nil {
			t.Fatalf("windowCreations: %v", err)
		}
		if want := []string{"x.go", "x.go"}; !slices.Equal(got, want) {
			t.Errorf("windowCreations = %q, want %q", got, want)
		}
	})

	t.Run("module", func(t *testing.T) {
		got, err := windowCreations(moduleRoot(t))
		if err != nil {
			t.Fatalf("windowCreations: %v", err)
		}
		if want := []string{"internal/window/gioapp/gioapp.go"}; !slices.Equal(got, want) {
			t.Errorf("windowCreations = %q, want %q", got, want)
		}
	})
}

// TestGoStatements: the go statements of md-viewer are exactly the spawn of image decoding, the
// spawn of the opener's wait, and the window's goroutine in main (SPEED-6). The fixture row shows
// the check counts each statement.
func TestGoStatements(t *testing.T) {
	t.Run("fixture with two statements", func(t *testing.T) {
		dir := writeTree(t, map[string]string{
			"x.go": "package x\n\nfunc f(g func()) {\n\tgo g()\n\tgo g()\n}\n",
		})
		got, err := goStatements(dir)
		if err != nil {
			t.Fatalf("goStatements: %v", err)
		}
		if want := []string{"x.go", "x.go"}; !slices.Equal(got, want) {
			t.Errorf("goStatements = %q, want %q", got, want)
		}
	})

	t.Run("module", func(t *testing.T) {
		got, err := goStatements(moduleRoot(t))
		if err != nil {
			t.Fatalf("goStatements: %v", err)
		}
		want := []string{
			"cmd/md-viewer/main.go",
			"internal/app/viewer.go",
			"internal/render/image.go",
		}
		slices.Sort(got)
		if !slices.Equal(got, want) {
			t.Errorf("goStatements = %q, want %q", got, want)
		}
	})
}
