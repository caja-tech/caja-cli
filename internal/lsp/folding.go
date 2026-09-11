package lsp

import (
	"context"
	"strings"

	"github.com/owenrumney/go-lsp/lsp"
)

// FoldingRange implements textDocument/foldingRange, giving editors
// collapsible #region/#endregion blocks (C#-style, optionally named:
// "#region Setup" ... "#endregion"). #region/#endregion are ordinary "#"
// comments to the rest of the pipeline — ast/analyzer never see them,
// because skipWhitespace swallows every comment before the lexer emits a
// token (see internal/pipeline/lexer/CLAUDE.md) — so there is no AST node
// to hang a folding range on. This scans the raw document text instead,
// tracking string/date-literal spans just enough that a "#" inside one
// isn't mistaken for a comment.
func (h *CajaHandler) FoldingRange(_ context.Context, params *lsp.FoldingRangeParams) ([]lsp.FoldingRange, error) {
	h.mu.RLock()
	text, ok := h.docs.Text(params.TextDocument.URI)
	h.mu.RUnlock()
	if !ok {
		return nil, nil
	}

	kind := lsp.FoldingRangeKindRegion
	var ranges []lsp.FoldingRange
	var openStarts []int

	for _, m := range scanRegionMarkers(text) {
		if !m.end {
			openStarts = append(openStarts, m.line)
			continue
		}
		if len(openStarts) == 0 {
			continue // unmatched #endregion — ignore rather than guess
		}
		start := openStarts[len(openStarts)-1]
		openStarts = openStarts[:len(openStarts)-1]
		if m.line > start {
			ranges = append(ranges, lsp.FoldingRange{StartLine: start, EndLine: m.line, Kind: &kind})
		}
	}
	// Any #region left on openStarts has no matching #endregion — left
	// unfolded rather than guessing an end at EOF.

	return ranges, nil
}

type regionMarker struct {
	line int // 0-based
	end  bool
}

// scanRegionMarkers finds every #region/#endregion that starts a line (only
// leading whitespace before the "#"), the same way a C# preprocessor
// directive must. A "#region"/"#endregion" appearing after other content —
// a trailing comment, or inside a string or date literal — is not a region
// marker.
func scanRegionMarkers(text string) []regionMarker {
	var markers []regionMarker
	line := 0
	atLineStart := true

	i, n := 0, len(text)
	for i < n {
		ch := text[i]
		switch ch {
		case '\n':
			line++
			atLineStart = true
			i++
		case ' ', '\t', '\r':
			i++
		case '"':
			i++
			for i < n && text[i] != '"' {
				if text[i] == '\\' && i+1 < n {
					i += 2
					continue
				}
				if text[i] == '\n' {
					line++
				}
				i++
			}
			i++ // closing quote, or EOF
			atLineStart = false
		case '\'':
			i++
			for i < n && text[i] != '\'' {
				if text[i] == '\n' {
					line++
				}
				i++
			}
			i++ // closing quote, or EOF
			atLineStart = false
		case '#':
			restStart := i + 1
			eol := strings.IndexByte(text[restStart:], '\n')
			var comment string
			if eol < 0 {
				comment = text[restStart:]
			} else {
				comment = text[restStart : restStart+eol]
			}
			if atLineStart {
				body := strings.TrimSpace(comment)
				switch {
				case body == "region" || strings.HasPrefix(body, "region "):
					markers = append(markers, regionMarker{line: line})
				case body == "endregion" || strings.HasPrefix(body, "endregion "):
					markers = append(markers, regionMarker{line: line, end: true})
				}
			}
			if eol < 0 {
				i = n
			} else {
				i = restStart + eol // leave the '\n' for the next iteration
			}
			atLineStart = false
		default:
			atLineStart = false
			i++
		}
	}
	return markers
}
