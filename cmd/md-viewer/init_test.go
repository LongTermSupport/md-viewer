package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
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
			name := d.Name()
			if path != root && (strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") ||
				name == "testdata" || name == "untracked") {
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
