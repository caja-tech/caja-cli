package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	cajafile "caja-cli/internal/file"
	"caja-cli/internal/pipeline/ast"
	"caja-cli/internal/pipeline/lexer"
	"caja-cli/internal/pipeline/modules"
	"caja-cli/internal/pipeline/parser"

	"github.com/owenrumney/go-lsp/lsp"
)

// indexedSymbol is one top-level declaration, recorded for workspace-wide symbol search.
type indexedSymbol struct {
	Name  string
	Kind  lsp.SymbolKind
	Token lexer.Token
	File  string
}

// workspaceIndex knows which files exist, what each declares, and which files import
// which.
//
// The server previously had no notion of a workspace at all: it saw only what the editor
// had opened. That made two things impossible. Editing a module never re-checked the
// files importing it, so a breaking change stayed invisible until you opened the other
// file yourself; and there was nothing to search for a workspace-wide symbol lookup.
//
// The index is built by parsing each file — not analyzing it. Parsing is cheap and yields
// both the declarations and the import list, which is all this needs; full semantic
// analysis of every file in a workspace on startup would not be.
type workspaceIndex struct {
	mu    sync.RWMutex
	roots []string

	symbols map[string][]indexedSymbol // file -> what it declares
	imports map[string][]string        // file -> files it imports
	// dependents is the reverse of imports, and is the whole point: when a file changes,
	// this is what says which other files need re-checking.
	dependents map[string]map[string]bool
}

func newWorkspaceIndex() *workspaceIndex {
	return &workspaceIndex{
		symbols:    make(map[string][]indexedSymbol),
		imports:    make(map[string][]string),
		dependents: make(map[string]map[string]bool),
	}
}

// setRoots records the workspace folders the client reported.
func (w *workspaceIndex) setRoots(params *lsp.InitializeParams) {
	w.mu.Lock()
	defer w.mu.Unlock()

	seen := make(map[string]bool)
	add := func(path string) {
		if path != "" && !seen[path] {
			seen[path] = true
			w.roots = append(w.roots, path)
		}
	}

	for _, folder := range params.WorkspaceFolders {
		add(uriToPath(string(folder.URI)))
	}
	// rootUri and rootPath are deprecated in favour of workspaceFolders, but older
	// clients still send only those, and a server that ignores them sees no workspace.
	if params.RootURI != nil {
		add(uriToPath(string(*params.RootURI)))
	}
	if params.RootPath != nil {
		add(*params.RootPath)
	}
}

// scan walks the workspace roots and indexes every Caja file found.
func (w *workspaceIndex) scan() {
	w.mu.RLock()
	roots := append([]string(nil), w.roots...)
	w.mu.RUnlock()

	for _, root := range roots {
		_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return nil // an unreadable directory is not worth failing the whole scan
			}
			if entry.IsDir() {
				if skipDirectory(entry.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(path, cajafile.EXTENSION) {
				w.indexFile(path)
			}
			return nil
		})
	}
}

// skipDirectory keeps the scan out of places that hold no first-party source. Dependency
// trees in particular can dwarf the project itself.
func skipDirectory(name string) bool {
	switch name {
	case "node_modules", ".git", "dist", "build", "out":
		return true
	default:
		return strings.HasPrefix(name, ".")
	}
}

// indexFile parses one file and records what it declares and what it imports.
func (w *workspaceIndex) indexFile(path string) {
	content, err := os.ReadFile(path)
	if err != nil {
		w.forget(path)
		return
	}

	p := parser.New(lexer.New(string(content)))
	prog := p.Parse()
	if prog == nil {
		w.forget(path)
		return
	}

	w.update(path, declaredSymbols(path, prog), importedFiles(path, prog))
}

// declaredSymbols extracts the top-level declarations of a parsed file.
func declaredSymbols(path string, prog *ast.Program) []indexedSymbol {
	var symbols []indexedSymbol

	record := func(name *ast.Identifier, kind lsp.SymbolKind) {
		if name != nil && name.Value != "" {
			symbols = append(symbols, indexedSymbol{
				Name: name.Value, Kind: kind, Token: name.Token, File: path,
			})
		}
	}

	for _, stmt := range prog.Statements {
		switch s := stmt.(type) {
		case *ast.LetStatement:
			kind := lsp.SymbolKindVariable
			if _, isFn := s.Value.(*ast.FunctionLiteral); isFn {
				kind = lsp.SymbolKindFunction
			}
			record(s.Name, kind)
		case *ast.ConstStatement:
			record(s.Name, lsp.SymbolKindConstant)
		case *ast.TypeAliasStatement:
			record(s.Name, typeAliasKind(s))
		case *ast.UnionStatement:
			record(s.Name, lsp.SymbolKindEnum)
		case *ast.TypeConstraintStatement:
			record(s.Name, lsp.SymbolKindInterface)
		}
	}
	return symbols
}

// importedFiles resolves a file's import statements to the paths they name. Resolution
// reuses the same rules the analyzer follows, so the graph matches what actually loads.
func importedFiles(path string, prog *ast.Program) []string {
	baseDir := filepath.Dir(path)

	var resolved []string
	for _, stmt := range prog.Statements {
		imp, isImport := stmt.(*ast.ImportStatement)
		if !isImport {
			continue
		}
		// A standard-library import names no file on disk.
		target, err := modules.Resolve(baseDir, imp.Path)
		if err != nil {
			continue
		}
		resolved = append(resolved, target)
	}
	return resolved
}

// update replaces everything known about one file, keeping the reverse edges consistent.
func (w *workspaceIndex) update(path string, symbols []indexedSymbol, imports []string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.removeEdges(path)
	w.symbols[path] = symbols
	w.imports[path] = imports

	for _, target := range imports {
		if w.dependents[target] == nil {
			w.dependents[target] = make(map[string]bool)
		}
		w.dependents[target][path] = true
	}
}

// forget drops a file that no longer exists or can no longer be read.
func (w *workspaceIndex) forget(path string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.removeEdges(path)
	delete(w.symbols, path)
	delete(w.imports, path)
}

// removeEdges clears a file's outgoing import edges. Callers hold the lock.
func (w *workspaceIndex) removeEdges(path string) {
	for _, target := range w.imports[path] {
		if set := w.dependents[target]; set != nil {
			delete(set, path)
			if len(set) == 0 {
				delete(w.dependents, target)
			}
		}
	}
}

// dependentsOf returns every file that imports the given one, directly or transitively.
// Transitive closure matters because a type flowing through a re-exporting facade can
// break a file two hops away from the one that changed.
func (w *workspaceIndex) dependentsOf(path string) []string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	seen := make(map[string]bool)
	var walk func(string)
	walk = func(current string) {
		for dependent := range w.dependents[current] {
			if seen[dependent] {
				continue
			}
			seen[dependent] = true
			walk(dependent)
		}
	}
	walk(path)

	out := make([]string, 0, len(seen))
	for dependent := range seen {
		out = append(out, dependent)
	}
	return out
}

// search returns the indexed symbols whose names match a query, using the
// case-insensitive substring rule editors expect from a workspace symbol box.
func (w *workspaceIndex) search(query string) []indexedSymbol {
	w.mu.RLock()
	defer w.mu.RUnlock()

	needle := strings.ToLower(query)

	var matches []indexedSymbol
	for _, symbols := range w.symbols {
		for _, sym := range symbols {
			// An empty query asks for everything, which is how some clients populate
			// their initial picker.
			if needle == "" || strings.Contains(strings.ToLower(sym.Name), needle) {
				matches = append(matches, sym)
			}
		}
	}
	return matches
}
