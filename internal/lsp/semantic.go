package lsp

import (
	"context"
	"sort"

	"caja-cli/internal/lsp/posmap"
	"caja-cli/internal/pipeline/analyzer/symbol"
	"caja-cli/internal/pipeline/ast"

	"github.com/owenrumney/go-lsp/lsp"
)

// Semantic token types, in the order the legend advertises them. The index into this
// slice is what goes on the wire, so entries may be appended but never reordered.
var semanticTokenTypes = []string{
	"namespace",
	"type",
	"struct",
	"enum",
	"enumMember",
	"interface",
	"typeParameter",
	"parameter",
	"variable",
	"property",
	"function",
	"number",
	"string",
}

// Semantic token modifiers. These are a bitmask on the wire, so the same rule applies:
// append only.
var semanticTokenModifiers = []string{
	"declaration",
	"readonly",
	"defaultLibrary",
	"async",
}

const (
	tokNamespace = iota
	tokType
	tokStruct
	tokEnum
	tokEnumMember
	tokInterface
	tokTypeParameter
	tokParameter
	tokVariable
	tokProperty
	tokFunction
	tokNumber
	tokString
)

const (
	modDeclaration = 1 << iota
	modReadonly
	modDefaultLibrary
	modAsync
)

// semanticLegend is what the client is told to expect, and must match the tables above.
func semanticLegend() lsp.SemanticTokensLegend {
	return lsp.SemanticTokensLegend{
		TokenTypes:     semanticTokenTypes,
		TokenModifiers: semanticTokenModifiers,
	}
}

// semanticToken is one classified span, before delta encoding.
type semanticToken struct {
	line      int
	startChar int
	length    int
	tokenType int
	modifiers int
}

// SemanticTokensFull classifies the whole document.
//
// This is the half of syntax highlighting a TextMate grammar cannot do. A grammar sees
// only text, so it cannot tell a type from a capitalised variable, cannot know that a
// `let` bound to a function literal is a function, cannot mark a stdlib module apart from
// a local one, and cannot colour the expressions inside a string interpolation. All of
// that is a question about the symbol table, which is exactly what the server has.
//
// Keywords, operators and comments are deliberately left to the grammar: they are purely
// lexical, the grammar already gets them right, and a client layers semantic tokens over
// grammar scopes rather than replacing them.
func (h *CajaHandler) SemanticTokensFull(_ context.Context, params *lsp.SemanticTokensParams) (res *lsp.SemanticTokens, err error) {
	defer recoverInto("semanticTokens", params.TextDocument.URI, &res, &err)

	state, ix, ok := h.documentFor(params.TextDocument.URI)
	if !ok {
		return &lsp.SemanticTokens{Data: []int{}}, nil
	}

	tokens := collectSemanticTokens(state, ix)
	return &lsp.SemanticTokens{Data: encodeSemanticTokens(tokens)}, nil
}

func collectSemanticTokens(state *DocumentState, ix *posmap.LineIndex) []semanticToken {
	var tokens []semanticToken

	// A node's classification is usually clearer from its parent than from itself — a
	// name is a parameter because a FunctionLiteral holds it, not because of anything the
	// Identifier knows. Inspect visits parents before children, so a parent that
	// classifies one of its children claims it here and the generic child case skips it.
	// Without this every such name is emitted twice, and two tokens covering the same
	// span encode a negative delta.
	claimed := make(map[ast.Node]bool)

	emit := func(node ast.Node, tokenType, modifiers int) {
		if node == nil || claimed[node] {
			return
		}
		claimed[node] = true

		tok := ast.StartToken(node)
		if tok.Line == 0 {
			return
		}
		r := ix.TokenRange(tok)
		// A span that wraps lines cannot be expressed as a single semantic token; the
		// protocol requires each to sit on one line.
		if r.End.Line != r.Start.Line {
			return
		}
		tokens = append(tokens, semanticToken{
			line:      r.Start.Line,
			startChar: r.Start.Character,
			length:    r.End.Character - r.Start.Character,
			tokenType: tokenType,
			modifiers: modifiers,
		})
	}

	ast.Inspect(state.Prog, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.TypeRef:
			emit(node, tokType, 0)

		case *ast.Parameter:
			emit(node, tokParameter, modDeclaration)

		case *ast.LetStatement:
			kind := tokVariable
			mods := modDeclaration
			if _, isFn := node.Value.(*ast.FunctionLiteral); isFn {
				kind = tokFunction
			}
			if _, isAsync := node.Value.(*ast.AsyncExpression); isAsync {
				mods |= modAsync
			}
			emit(node.Name, kind, mods)

		case *ast.ConstStatement:
			emit(node.Name, tokVariable, modDeclaration|modReadonly)

		case *ast.ImportStatement:
			if node.Name != nil {
				emit(node.Name, tokNamespace, modDeclaration|builtinModifier(node.Name.Value))
			}
			for _, named := range node.NamedImports {
				emit(named, tokFunction, modDeclaration)
			}

		case *ast.TypeAliasStatement:
			kind := tokType
			if node.StructDefinition != nil {
				kind = tokStruct
			}
			emit(node.Name, kind, modDeclaration)
			emitStructFields(node, emit)

		case *ast.UnionStatement:
			emit(node.Name, tokEnum, modDeclaration)
			for _, variant := range node.Variants {
				emit(variant, tokEnumMember, 0)
			}

		case *ast.TypeConstraintStatement:
			emit(node.Name, tokInterface, modDeclaration)
			emit(node.BaseType, tokType, 0)

		case *ast.PropertyExpression:
			emit(node.Property, tokProperty, 0)

		case *ast.PropertyAssignmentStatement:
			emit(node.Property, tokProperty, 0)

		case *ast.NamedArgument:
			emit(node.Name, tokParameter, 0)

		case *ast.NumberLiteral:
			emit(node, tokNumber, 0)

		case *ast.StringLiteral:
			emit(node, tokString, 0)

		case *ast.Identifier:
			if claimed[node] {
				break // already classified, with more context, by its parent
			}
			// A bare identifier is classified from the symbol it resolves to, which is
			// the distinction a grammar cannot make.
			emit(node, identifierTokenType(state, node), identifierModifiers(state, node))
		}
		return true
	})

	return tokens
}

func emitStructFields(node *ast.TypeAliasStatement, emit func(ast.Node, int, int)) {
	if node.StructDefinition == nil {
		return
	}
	for i := range node.StructDefinition.Fields {
		field := &node.StructDefinition.Fields[i]
		if field.Name == nil {
			continue
		}
		mods := modDeclaration
		if field.IsConstant {
			mods |= modReadonly
		}
		emit(field.Name, tokProperty, mods)
	}
}

// identifierTokenType classifies a use of a name by what it resolves to.
func identifierTokenType(state *DocumentState, node *ast.Identifier) int {
	if state.Analyzer == nil {
		return tokVariable
	}
	if state.Analyzer.IsDeclaredType(node.Value) {
		return tokType
	}

	sym, ok := state.Analyzer.GetSymbol(node)
	if !ok {
		return tokVariable
	}

	switch sym.(type) {
	case *symbol.ModuleSymbol:
		return tokNamespace
	case *symbol.FunctionSymbol, *symbol.BuiltinSymbol:
		return tokFunction
	case *symbol.StructDefSymbol:
		return tokStruct
	case *symbol.UnionSymbol:
		return tokEnum
	case *symbol.ConstraintSymbol:
		return tokInterface
	case *symbol.GenericSymbol:
		return tokTypeParameter
	case *symbol.AsyncSymbol:
		return tokVariable
	default:
		return tokVariable
	}
}

func identifierModifiers(state *DocumentState, node *ast.Identifier) int {
	if state.Analyzer == nil {
		return 0
	}

	mods := 0
	if sym, ok := state.Analyzer.GetSymbol(node); ok {
		switch sym.(type) {
		case *symbol.AsyncSymbol:
			mods |= modAsync
		case *symbol.ModuleSymbol:
			mods |= builtinModifier(node.Value)
		}
	}
	return mods
}

// builtinModifier marks the standard library apart from a user's own modules, which is
// information only the symbol table has.
func builtinModifier(moduleName string) int {
	if _, _, isStandard := symbol.GetStandardModule(moduleName); isStandard {
		return modDefaultLibrary
	}
	return 0
}

// encodeSemanticTokens converts absolute positions into the protocol's delta encoding:
// five integers per token, each position relative to the previous token.
func encodeSemanticTokens(tokens []semanticToken) []int {
	// The protocol requires tokens in source order, and the AST walk yields declarations
	// before the expressions inside them rather than strictly left to right.
	sort.SliceStable(tokens, func(i, j int) bool {
		if tokens[i].line != tokens[j].line {
			return tokens[i].line < tokens[j].line
		}
		return tokens[i].startChar < tokens[j].startChar
	})

	data := make([]int, 0, len(tokens)*5)
	prevLine, prevChar := 0, 0

	for _, tok := range tokens {
		if tok.length <= 0 {
			continue
		}

		deltaLine := tok.line - prevLine
		deltaChar := tok.startChar
		if deltaLine == 0 {
			deltaChar = tok.startChar - prevChar
			// Two tokens claiming the same span would encode a negative delta, which is
			// invalid; the first classification wins.
			if deltaChar < 0 {
				continue
			}
		}

		data = append(data, deltaLine, deltaChar, tok.length, tok.tokenType, tok.modifiers)
		prevLine, prevChar = tok.line, tok.startChar
	}
	return data
}
