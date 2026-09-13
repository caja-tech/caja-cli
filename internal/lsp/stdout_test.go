package lsp

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain turns on strict panic mode for the whole package so the recover helpers
// re-panic instead of degrading. Production wants a silent degrade; tests want the
// original stack. Set CAJA_LSP_STRICT=0 to opt out — useful when sweeping the corpus for
// discovery, where re-panicking would abort the run at the first defect instead of
// reporting every one.
func TestMain(m *testing.M) {
	strictPanics = os.Getenv("CAJA_LSP_STRICT") != "0"
	os.Exit(m.Run())
}

// TestNoStdoutWrites guards the single most damaging class of bug in this package.
// The server runs over stdio (server.RunStdio), so stdout IS the JSON-RPC transport:
// any stray write corrupts the protocol stream rather than merely printing noise.
// internal/lsp/CLAUDE.md asks humans to grep for fmt.Print before shipping; this is the
// version that actually holds.
func TestNoStdoutWrites(t *testing.T) {
	banned := map[string]string{
		"fmt.Print":   "writes to stdout",
		"fmt.Printf":  "writes to stdout",
		"fmt.Println": "writes to stdout",
		"fmt.Fprint":  "may target os.Stdout",
		"fmt.Fprintf": "may target os.Stdout",
		"os.Stdout":   "is the JSON-RPC transport",
		"println":     "writes to stderr, but is debug scaffolding",
		"print":       "writes to stderr, but is debug scaffolding",
	}

	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("listing sources: %v", err)
	}

	checked := 0
	fset := token.NewFileSet()
	for _, path := range sources {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		checked++

		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", path, err)
		}

		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			name := renderCallee(call.Fun)
			if reason, bad := banned[name]; bad {
				pos := fset.Position(call.Pos())
				t.Errorf("%s:%d: %s(...) is forbidden in this package because it %s",
					path, pos.Line, name, reason)
			}
			return true
		})
	}

	// A glob that silently matched nothing would make this test vacuously pass forever.
	if checked == 0 {
		t.Fatal("no non-test sources found to scan")
	}
}

// renderCallee flattens a call target to "fmt.Printf" or "println", ignoring anything
// more complex (method values, calls through variables) that the banned list can't name.
func renderCallee(fun ast.Expr) string {
	switch f := fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		if pkg, ok := f.X.(*ast.Ident); ok {
			return pkg.Name + "." + f.Sel.Name
		}
	}
	return ""
}
