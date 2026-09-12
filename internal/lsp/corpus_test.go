package lsp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"caja-cli/internal/pipeline/lexer"

	"github.com/owenrumney/go-lsp/lsp"
	"github.com/owenrumney/go-lsp/servertest"
)

// The repo carries two large .caja corpora that the language server has never been run
// against. They are the cheapest broad coverage available: every construct the compiler
// supports is already written down, in files that are kept working by other suites.
const (
	compilerSamplesDir = "../pipeline/compiler/samples"
	scriptFixturesDir  = "../script/tests"
)

type corpusFile struct {
	name string // short label for subtests
	path string // absolute, because module resolution reads from disk
	text string
}

func (c corpusFile) uri() lsp.DocumentURI { return lsp.DocumentURI("file://" + c.path) }

// compilerSampleEntries returns the entry script of each compiler sample directory.
// Layout is <samples>/<name>/<name>.caja, with any sibling .caja files being modules
// that the entry imports — opening the entry exercises those through the import path.
func compilerSampleEntries(t *testing.T) []corpusFile {
	t.Helper()

	dirs, err := os.ReadDir(compilerSamplesDir)
	if err != nil {
		t.Fatalf("reading %s: %v", compilerSamplesDir, err)
	}

	var files []corpusFile
	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}
		entry := filepath.Join(compilerSamplesDir, dir.Name(), dir.Name()+".caja")
		if _, err := os.Stat(entry); err != nil {
			continue // a sample whose entry is named differently; not our concern here
		}
		files = append(files, readCorpusFile(t, dir.Name(), entry))
	}

	if len(files) == 0 {
		t.Fatalf("no sample entries found under %s", compilerSamplesDir)
	}
	return files
}

// scriptFixtures returns the flat module-system fixture corpus. Unlike the compiler
// samples these are a mix of positive and negative cases, so callers must not assume
// they analyze cleanly.
func scriptFixtures(t *testing.T) []corpusFile {
	t.Helper()

	entries, err := os.ReadDir(scriptFixturesDir)
	if err != nil {
		t.Fatalf("reading %s: %v", scriptFixturesDir, err)
	}

	var files []corpusFile
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".caja") {
			continue
		}
		path := filepath.Join(scriptFixturesDir, entry.Name())
		files = append(files, readCorpusFile(t, entry.Name(), path))
	}

	if len(files) == 0 {
		t.Fatalf("no fixtures found under %s", scriptFixturesDir)
	}
	return files
}

func readCorpusFile(t *testing.T, name, path string) corpusFile {
	t.Helper()

	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("resolving %s: %v", path, err)
	}
	text, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("reading %s: %v", abs, err)
	}
	return corpusFile{name: name, path: abs, text: string(text)}
}

// openCorpusFile opens a document and waits for the validation worker to publish, so the
// astCache is populated before any read request is issued.
func openCorpusFile(t *testing.T, s *servertest.Harness, f corpusFile) []lsp.Diagnostic {
	t.Helper()

	if err := s.DidOpen(f.uri(), "caja", f.text); err != nil {
		t.Fatalf("DidOpen(%s): %v", f.name, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	diags, err := s.WaitForDiagnostics(ctx, f.uri())
	if err != nil {
		t.Fatalf("no diagnostics published for %s within timeout: %v", f.name, err)
	}
	return diags
}

// TestCompilerSamplesProduceNoDiagnostics turns the compiler's sample corpus into a
// consistency check between the two analysis entry points. Every one of these programs
// compiles, so the language server — which re-implements the parse-then-analyze sequence
// rather than calling internal/script — must agree that they are clean. A failure here
// means the two pipelines have drifted, which is exactly the risk internal/lsp/CLAUDE.md
// warns about.
func TestCompilerSamplesProduceNoDiagnostics(t *testing.T) {
	for _, f := range compilerSampleEntries(t) {
		t.Run(f.name, func(t *testing.T) {
			h := NewCajaHandler()
			s := servertest.New(t, h)

			for _, d := range openCorpusFile(t, s, f) {
				t.Errorf("unexpected diagnostic at %d:%d: %s",
					d.Range.Start.Line+1, d.Range.Start.Character+1, d.Message)
			}
		})
	}
}

// TestScriptFixturesDoNotCrash opens the module-system corpus, which deliberately mixes
// valid programs with negative cases (missing imports, private access, ambiguous
// wildcards). Diagnostics are expected and not asserted; what is asserted is that the
// server survives, publishes, and caches a usable document state for each one.
func TestScriptFixturesDoNotCrash(t *testing.T) {
	for _, f := range scriptFixtures(t) {
		t.Run(f.name, func(t *testing.T) {
			h := NewCajaHandler()
			s := servertest.New(t, h)

			openCorpusFile(t, s, f)

			h.mu.RLock()
			state, cached := h.astCache[f.uri()]
			h.mu.RUnlock()

			if !cached || state == nil || state.Prog == nil {
				t.Fatalf("astCache not populated for %s; read requests would return nil", f.name)
			}
		})
	}
}

// probePositions returns the positions worth probing in a source file: the start, an
// interior offset, and the end of every token, plus the start and end of every line.
// Sweeping literally every column would be ~4x slower for no extra signal, since the
// position-lookup code branches on token boundaries.
func probePositions(text string) []lsp.Position {
	seen := make(map[lsp.Position]bool)
	var out []lsp.Position

	add := func(line, char int) {
		if line < 0 || char < 0 {
			return
		}
		p := lsp.Position{Line: line, Character: char}
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}

	for i, line := range strings.Split(text, "\n") {
		add(i, 0)
		add(i, len(strings.TrimRight(line, "\r"))) // end of line, past the last token
	}

	lx := lexer.New(text)
	for {
		tok := lx.NextToken()
		if tok.Type == lexer.EOF {
			break
		}
		// Tokens are 1-indexed by the lexer; LSP positions are 0-indexed.
		line, col := tok.Line-1, tok.Column-1
		add(line, col)
		add(line, col+len(tok.Literal)/2)
		add(line, col+len(tok.Literal))
	}

	return out
}

// TestCorpusPositionSweep drives all four read requests across every token boundary of
// every sample. It asserts only that the server answers — no panic, no error — because
// the point is to flush out the gaps in the hand-maintained position-lookup switches in
// ast_util.go, which silently return nil for node types they were never taught about.
// Requests go straight to the handler rather than through the RPC harness: the corpus is
// large enough that round-trip cost would dominate, and the handler is the unit at risk.
func TestCorpusPositionSweep(t *testing.T) {
	// Both corpora are swept. The script fixtures matter most here: they deliberately
	// include malformed and semantically invalid programs, which is exactly the shape of
	// buffer the server sees mid-keystroke.
	files := append(compilerSampleEntries(t), scriptFixtures(t)...)
	if testing.Short() {
		files = files[:min(8, len(files))]
	}

	// A recovered panic surfaces to the caller as an empty result and a nil error, which
	// is indistinguishable from "no symbol here". Observing the recover hook directly is
	// the only way this sweep can see a crash, and it lets one run report every defect
	// rather than aborting at the first.
	sweep := newPanicCollector(t)

	for _, f := range files {
		t.Run(f.name, func(t *testing.T) {
			h := NewCajaHandler()
			s := servertest.New(t, h)
			openCorpusFile(t, s, f)

			ctx := context.Background()
			uri := f.uri()

			if _, err := h.DocumentSymbol(ctx, &lsp.DocumentSymbolParams{
				TextDocument: lsp.TextDocumentIdentifier{URI: uri},
			}); err != nil {
				t.Errorf("DocumentSymbol: %v", err)
			}
			if _, err := h.FoldingRange(ctx, &lsp.FoldingRangeParams{
				TextDocument: lsp.TextDocumentIdentifier{URI: uri},
			}); err != nil {
				t.Errorf("FoldingRange: %v", err)
			}
			if _, err := h.SemanticTokensFull(ctx, &lsp.SemanticTokensParams{
				TextDocument: lsp.TextDocumentIdentifier{URI: uri},
			}); err != nil {
				t.Errorf("SemanticTokensFull: %v", err)
			}

			for _, pos := range probePositions(f.text) {
				sweep.at(f.name, pos)

				if _, err := h.Hover(ctx, &lsp.HoverParams{
					TextDocumentPositionParams: textDocPos(uri, pos),
				}); err != nil {
					t.Errorf("Hover at %d:%d: %v", pos.Line, pos.Character, err)
				}
				if _, err := h.Definition(ctx, &lsp.DefinitionParams{
					TextDocumentPositionParams: textDocPos(uri, pos),
				}); err != nil {
					t.Errorf("Definition at %d:%d: %v", pos.Line, pos.Character, err)
				}
				if _, err := h.Completion(ctx, &lsp.CompletionParams{
					TextDocumentPositionParams: textDocPos(uri, pos),
				}); err != nil {
					t.Errorf("Completion at %d:%d: %v", pos.Line, pos.Character, err)
				}
				if _, err := h.SignatureHelp(ctx, &lsp.SignatureHelpParams{
					TextDocumentPositionParams: textDocPos(uri, pos),
				}); err != nil {
					t.Errorf("SignatureHelp at %d:%d: %v", pos.Line, pos.Character, err)
				}
				if _, err := h.DocumentHighlight(ctx, &lsp.DocumentHighlightParams{
					TextDocumentPositionParams: textDocPos(uri, pos),
				}); err != nil {
					t.Errorf("DocumentHighlight at %d:%d: %v", pos.Line, pos.Character, err)
				}
				if _, err := h.References(ctx, &lsp.ReferenceParams{
					TextDocumentPositionParams: textDocPos(uri, pos),
					Context:                    lsp.ReferenceContext{IncludeDeclaration: true},
				}); err != nil {
					t.Errorf("References at %d:%d: %v", pos.Line, pos.Character, err)
				}
				if _, err := h.SelectionRange(ctx, &lsp.SelectionRangeParams{
					TextDocument: lsp.TextDocumentIdentifier{URI: uri},
					Positions:    []lsp.Position{pos},
				}); err != nil {
					t.Errorf("SelectionRange at %d:%d: %v", pos.Line, pos.Character, err)
				}
				// PrepareRename legitimately refuses names it cannot rename, so an error
				// here is a valid answer; the sweep only cares that it does not crash.
				_, _ = h.PrepareRename(ctx, &lsp.PrepareRenameParams{
					TextDocumentPositionParams: textDocPos(uri, pos),
				})
			}
		})
	}

	sweep.report()
}

// panicCollector records recovered panics against the corpus position being probed when
// they fired, and collapses them by crash site so a defect hit at ten thousand positions
// reports as one finding with a count.
type panicCollector struct {
	t *testing.T

	curFile string
	curPos  lsp.Position

	counts map[string]int
	first  map[string]string
}

func newPanicCollector(t *testing.T) *panicCollector {
	t.Helper()

	c := &panicCollector{
		t:      t,
		counts: make(map[string]int),
		first:  make(map[string]string),
	}

	// Strict mode re-panics before the sweep can aggregate, which defeats the purpose.
	prevStrict := strictPanics
	strictPanics = false

	onRecover = func(rec any, op string, _ lsp.DocumentURI, stack []byte) {
		site := crashSite(stack)
		key := op + " @ " + site
		c.counts[key]++
		if _, ok := c.first[key]; !ok {
			c.first[key] = fmt.Sprintf("%s: %v (first seen in %s at %d:%d)",
				site, rec, c.curFile, c.curPos.Line, c.curPos.Character)
		}
	}

	t.Cleanup(func() {
		onRecover = nil
		strictPanics = prevStrict
	})
	return c
}

func (c *panicCollector) at(file string, pos lsp.Position) {
	c.curFile, c.curPos = file, pos
}

func (c *panicCollector) report() {
	for key, n := range c.counts {
		c.t.Errorf("%s panicked at %d probe positions\n    %s", key, n, c.first[key])
	}
}

// crashSite picks the deepest frame inside this module's own packages, skipping the
// recover plumbing, so panics are grouped by where they actually originate.
func crashSite(stack []byte) string {
	lines := strings.Split(string(stack), "\n")
	for i, line := range lines {
		if !strings.HasPrefix(line, "caja-cli/") {
			continue
		}
		fn := strings.TrimSpace(line)
		if strings.Contains(fn, "logRecovered") || strings.Contains(fn, "recoverInto") ||
			strings.Contains(fn, "recoverWorker") {
			continue
		}
		if i+1 < len(lines) {
			loc := strings.TrimSpace(lines[i+1])
			loc = strings.TrimSuffix(loc, " +0x0")
			if idx := strings.LastIndex(loc, "/"); idx != -1 {
				loc = loc[idx+1:]
			}
			if idx := strings.Index(loc, " "); idx != -1 {
				loc = loc[:idx]
			}
			return loc
		}
		return fn
	}
	return "unknown"
}

func textDocPos(uri lsp.DocumentURI, pos lsp.Position) lsp.TextDocumentPositionParams {
	return lsp.TextDocumentPositionParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
		Position:     pos,
	}
}
