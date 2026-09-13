package lsp

import (
	"context"
	"sort"

	"caja-cli/internal/pipeline/ast"

	"github.com/owenrumney/go-lsp/lsp"
)

// DidSave re-indexes the saved file and re-checks whatever depends on it.
//
// Saving is the moment a change becomes visible to other files: the analyzer loads
// imports from disk, so until a module is written out, a file importing it still sees the
// old definitions. This is what closes the loop that was previously missing entirely —
// renaming an export used to leave every importing file showing stale, passing
// diagnostics until you reopened it by hand.
func (h *CajaHandler) DidSave(_ context.Context, params *lsp.DidSaveTextDocumentParams) error {
	h.onFileChanged(uriToPath(string(params.TextDocument.URI)))
	return nil
}

// DidChangeWatchedFiles handles Caja files created, changed or deleted outside the
// editor — a git checkout, a generator, another tool.
func (h *CajaHandler) DidChangeWatchedFiles(_ context.Context, params *lsp.DidChangeWatchedFilesParams) error {
	for _, change := range params.Changes {
		path := uriToPath(string(change.URI))
		if path == "" {
			continue
		}
		if change.Type == lsp.FileDeleted {
			h.index.forget(path)
		} else {
			h.index.indexFile(path)
		}
		h.revalidateDependents(path)
	}
	return nil
}

// onFileChanged re-indexes one file and re-checks its dependents.
func (h *CajaHandler) onFileChanged(path string) {
	if path == "" {
		return
	}
	h.index.indexFile(path)
	h.revalidateDependents(path)
}

// revalidateDependents re-runs analysis for the open documents that import a changed
// file, so their diagnostics reflect it.
//
// Only open documents are re-checked. Publishing diagnostics for a file the editor has
// not opened would leave them with nothing to attach to and no way to clear them.
func (h *CajaHandler) revalidateDependents(path string) {
	dependents := h.index.dependentsOf(path)
	if len(dependents) == 0 {
		return
	}

	affected := make(map[string]bool, len(dependents))
	for _, dependent := range dependents {
		affected[dependent] = true
	}

	h.mu.RLock()
	var queue []lsp.DocumentURI
	for uri := range h.workers {
		if affected[uriToPath(string(uri))] {
			queue = append(queue, uri)
		}
	}
	h.mu.RUnlock()

	for _, uri := range queue {
		h.requestValidation(uri)
	}
}

// WorkspaceSymbol answers the editor's "go to symbol in workspace" box.
func (h *CajaHandler) WorkspaceSymbol(_ context.Context, params *lsp.WorkspaceSymbolParams) (res []lsp.SymbolInformation, err error) {
	defer recoverInto("workspaceSymbol", "", &res, &err)

	matches := h.index.search(params.Query)

	// A stable order matters: the index is a map, so without sorting the same query
	// returns the same symbols in a different order each time.
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Name != matches[j].Name {
			return matches[i].Name < matches[j].Name
		}
		return matches[i].File < matches[j].File
	})

	symbols := make([]lsp.SymbolInformation, 0, len(matches))
	for _, match := range matches {
		ix := h.indexFor(pathToURI(match.File))
		symbols = append(symbols, lsp.SymbolInformation{
			Name: match.Name,
			Kind: match.Kind,
			Location: lsp.Location{
				URI:   pathToURI(match.File),
				Range: ix.TokenRange(match.Token),
			},
		})
	}
	return symbols, nil
}

// indexOpenDocument records what an analyzed document declares and imports, so an open
// file participates in the dependency graph even when it was never on disk at scan time.
//
// Imports are resolved the same way the workspace scan resolves them, rather than read
// back out of the analyzer. The analyzer's ModuleASTs cache is keyed by the import string
// the source wrote ("./calculus"), not by the file it resolved to, so using it here wrote
// edges pointing at names that match no file and silently erased the real ones.
func (h *CajaHandler) indexOpenDocument(path string, prog *ast.Program) {
	if path == "" || prog == nil {
		return
	}
	h.index.update(path, declaredSymbols(path, prog), importedFiles(path, prog))
}
