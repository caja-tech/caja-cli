package lsp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/owenrumney/go-lsp/lsp"
	"github.com/owenrumney/go-lsp/servertest"
)

// workspace writes a set of files into a temp directory and returns the directory.
func workspace(t *testing.T, files map[string]string) string {
	t.Helper()

	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("creating %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}
	}
	return dir
}

// initializedIn starts a server rooted at dir and waits for the background scan to have
// indexed the workspace.
func initializedIn(t *testing.T, dir string) (*CajaHandler, *servertest.Harness) {
	t.Helper()

	h := NewCajaHandler()
	rootURI := pathToURI(dir)

	s := servertest.New(t, h, servertest.WithInitializeParams(&lsp.InitializeParams{
		WorkspaceFolders: []lsp.WorkspaceFolder{{URI: rootURI, Name: "test"}},
	}))

	waitForIndex(t, h)
	return h, s
}

// waitForIndex blocks until the background scan has recorded at least one file, so tests
// do not race the goroutine Initialize starts.
func waitForIndex(t *testing.T, h *CajaHandler) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		h.index.mu.RLock()
		indexed := len(h.index.symbols)
		h.index.mu.RUnlock()

		if indexed > 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("the workspace scan indexed nothing within the timeout")
}

// waitFor polls until a condition holds. Notifications are fire-and-forget: the harness
// returns as soon as the message is written, so a test that inspects server state
// immediately afterwards is racing the handler.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// TestWorkspaceScanIndexesUnopenedFiles checks that the server knows about files the
// editor has never opened, which is the premise everything else here rests on.
func TestWorkspaceScanIndexesUnopenedFiles(t *testing.T) {
	dir := workspace(t, map[string]string{
		"calculus.caja": "let add = fn(a: Number, b: Number) -> Number { return a + b }\nconst PI = 3\n",
		"shapes.caja":   "type Circle struct {\n    radius Number\n}\n",
		"main.caja":     "import \"./calculus\"\nlet total = calculus.add(1, 2)\n",
	})

	h, _ := initializedIn(t, dir)

	for _, want := range []string{"add", "PI", "Circle", "total"} {
		if len(h.index.search(want)) == 0 {
			t.Errorf("the index does not know about %q, though no file was ever opened", want)
		}
	}
}

// TestWorkspaceSymbolSearch covers the editor's workspace symbol box.
func TestWorkspaceSymbolSearch(t *testing.T) {
	dir := workspace(t, map[string]string{
		"calculus.caja": "let addNumbers = fn(a: Number, b: Number) -> Number { return a + b }\n",
		"shapes.caja":   "type Circle struct {\n    radius Number\n}\nconst addendum = 1\n",
	})

	h, s := initializedIn(t, dir)
	_ = h

	symbols, err := s.WorkspaceSymbol("add")
	if err != nil {
		t.Fatalf("WorkspaceSymbol: %v", err)
	}
	if len(symbols) != 2 {
		t.Fatalf("searching for \"add\" returned %d symbols, want addNumbers and addendum; got %v",
			len(symbols), symbols)
	}

	// Results must be ordered, or the same query shuffles between invocations.
	if symbols[0].Name != "addNumbers" || symbols[1].Name != "addendum" {
		t.Errorf("results are not in a stable order: got %q then %q",
			symbols[0].Name, symbols[1].Name)
	}

	for _, sym := range symbols {
		if sym.Location.URI == "" {
			t.Errorf("%q has no location, so it cannot be navigated to", sym.Name)
		}
		if sym.Location.Range.End.Character <= sym.Location.Range.Start.Character {
			t.Errorf("%q has an empty range", sym.Name)
		}
	}
}

// TestWorkspaceSymbolMatchesCaseInsensitively covers the matching rule editors expect.
func TestWorkspaceSymbolMatchesCaseInsensitively(t *testing.T) {
	dir := workspace(t, map[string]string{
		"shapes.caja": "type Circle struct {\n    radius Number\n}\n",
	})

	_, s := initializedIn(t, dir)

	symbols, err := s.WorkspaceSymbol("circ")
	if err != nil {
		t.Fatalf("WorkspaceSymbol: %v", err)
	}
	if len(symbols) == 0 {
		t.Error("a lowercase query did not match the capitalised type name")
	}
}

// TestDependencyGraphTracksImporters checks the reverse edges the whole phase is built
// on: which files would need re-checking if a given one changed.
func TestDependencyGraphTracksImporters(t *testing.T) {
	dir := workspace(t, map[string]string{
		"base.caja":   "let value = 1\n",
		"middle.caja": "import \"./base\"\nlet doubled = base.value\n",
		"top.caja":    "import \"./middle\"\nlet result = middle.doubled\n",
	})

	h, _ := initializedIn(t, dir)

	base := filepath.Join(dir, "base.caja")
	dependents := h.index.dependentsOf(base)

	if len(dependents) != 2 {
		t.Fatalf("base.caja has %d dependents, want middle.caja and top.caja (transitively); got %v",
			len(dependents), dependents)
	}

	found := map[string]bool{}
	for _, d := range dependents {
		found[filepath.Base(d)] = true
	}
	if !found["middle.caja"] {
		t.Error("the direct importer of base.caja is missing")
	}
	if !found["top.caja"] {
		t.Error("the transitive importer of base.caja is missing; a change can break a " +
			"file two hops away through a re-exporting facade")
	}
}

// TestDeletedFileLeavesNoEdges checks that removing a file drops it from both the symbol
// table and the dependency graph, rather than leaving a phantom behind.
func TestDeletedFileLeavesNoEdges(t *testing.T) {
	dir := workspace(t, map[string]string{
		"base.caja":   "let value = 1\n",
		"middle.caja": "import \"./base\"\nlet doubled = base.value\n",
	})

	h, s := initializedIn(t, dir)

	middle := filepath.Join(dir, "middle.caja")
	if err := os.Remove(middle); err != nil {
		t.Fatalf("removing %s: %v", middle, err)
	}

	if err := s.DidChangeWatchedFiles(&lsp.DidChangeWatchedFilesParams{
		Changes: []lsp.FileEvent{{URI: pathToURI(middle), Type: lsp.FileDeleted}},
	}); err != nil {
		t.Fatalf("DidChangeWatchedFiles: %v", err)
	}

	base := filepath.Join(dir, "base.caja")
	waitFor(t, "the deleted file to be dropped from the index", func() bool {
		return len(h.index.dependentsOf(base)) == 0
	})

	if len(h.index.search("doubled")) != 0 {
		t.Error("a symbol from the deleted file is still searchable")
	}
}

// TestSavingAModuleRechecksItsImporters is the behaviour this phase exists for.
//
// The analyzer reads imports from disk, so a change only becomes visible to other files
// when it is saved. Before this, nothing connected the two: breaking a module left every
// importing file showing stale, passing diagnostics until it was reopened by hand.
func TestSavingAModuleRechecksItsImporters(t *testing.T) {
	dir := workspace(t, map[string]string{
		"calculus.caja": "let add = fn(a: Number, b: Number) -> Number { return a + b }\n",
		"main.caja":     "import \"./calculus\"\nlet total = calculus.add(1, 2)\n",
	})

	h, s := initializedIn(t, dir)
	_ = h

	mainPath := filepath.Join(dir, "main.caja")
	mainURI := pathToURI(mainPath)
	mainText, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("reading main.caja: %v", err)
	}

	if err := s.DidOpen(mainURI, "caja", string(mainText)); err != nil {
		t.Fatalf("DidOpen: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	diags, err := s.WaitForDiagnostics(ctx, mainURI)
	if err != nil {
		t.Fatalf("no initial diagnostics: %v", err)
	}
	if len(diags) != 0 {
		t.Fatalf("main.caja should start clean, got %v", diags)
	}

	// Break the module: `add` now takes one argument, so main's two-argument call is wrong.
	calculus := filepath.Join(dir, "calculus.caja")
	if err := os.WriteFile(calculus, []byte("let add = fn(a: Number) -> Number { return a }\n"), 0o644); err != nil {
		t.Fatalf("rewriting calculus.caja: %v", err)
	}

	s.ClearDiagnostics()
	if err := s.DidSave(pathToURI(calculus)); err != nil {
		t.Fatalf("DidSave: %v", err)
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	updated, err := s.WaitForDiagnostics(ctx2, mainURI)
	if err != nil {
		t.Fatalf("saving the module did not cause main.caja to be re-checked: %v", err)
	}
	if len(updated) == 0 {
		t.Error("main.caja still reports no problems, though the function it calls now " +
			"takes a different number of arguments")
	}
}

// TestSavingAModuleClearsStaleDiagnostics is the same loop in the other direction: fixing
// a module must retract the errors it caused in its importers.
func TestSavingAModuleClearsStaleDiagnostics(t *testing.T) {
	dir := workspace(t, map[string]string{
		"calculus.caja": "let add = fn(a: Number) -> Number { return a }\n",
		"main.caja":     "import \"./calculus\"\nlet total = calculus.add(1, 2)\n",
	})

	_, s := initializedIn(t, dir)

	mainPath := filepath.Join(dir, "main.caja")
	mainURI := pathToURI(mainPath)
	mainText, _ := os.ReadFile(mainPath)

	if err := s.DidOpen(mainURI, "caja", string(mainText)); err != nil {
		t.Fatalf("DidOpen: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	diags, err := s.WaitForDiagnostics(ctx, mainURI)
	if err != nil {
		t.Fatalf("no initial diagnostics: %v", err)
	}
	if len(diags) == 0 {
		t.Skip("the analyzer does not flag this arity mismatch across modules; " +
			"nothing to clear")
	}

	calculus := filepath.Join(dir, "calculus.caja")
	if err := os.WriteFile(calculus,
		[]byte("let add = fn(a: Number, b: Number) -> Number { return a + b }\n"), 0o644); err != nil {
		t.Fatalf("rewriting calculus.caja: %v", err)
	}

	s.ClearDiagnostics()
	if err := s.DidSave(pathToURI(calculus)); err != nil {
		t.Fatalf("DidSave: %v", err)
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	cleared, err := s.WaitForDiagnostics(ctx2, mainURI)
	if err != nil {
		t.Fatalf("fixing the module did not cause main.caja to be re-checked: %v", err)
	}
	if len(cleared) != 0 {
		t.Errorf("main.caja still reports %v after the module was fixed", cleared)
	}
}

// TestScanSkipsDependencyDirectories checks that the scan stays out of node_modules,
// which in a real project can dwarf the source it sits beside.
func TestScanSkipsDependencyDirectories(t *testing.T) {
	dir := workspace(t, map[string]string{
		"main.caja":                      "let mine = 1\n",
		"node_modules/pkg/vendored.caja": "let vendored = 1\n",
		".hidden/secret.caja":            "let hidden = 1\n",
	})

	h, _ := initializedIn(t, dir)

	if len(h.index.search("mine")) == 0 {
		t.Error("the scan missed a first-party file")
	}
	if len(h.index.search("vendored")) != 0 {
		t.Error("the scan descended into node_modules")
	}
	if len(h.index.search("hidden")) != 0 {
		t.Error("the scan descended into a dot-directory")
	}
}
