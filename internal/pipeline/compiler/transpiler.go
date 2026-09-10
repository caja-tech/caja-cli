package compiler

import (
	"bytes"
	"caja-cli/internal/pipeline/analyzer"
	"caja-cli/internal/pipeline/analyzer/symbol"
	"caja-cli/internal/pipeline/ast"
	"caja-cli/internal/pipeline/environment"

	"fmt"
	"strings"
)

// TranspileOptions controls transpiler behavior that must differ between
// `caja build` (produces a standalone binary) and `caja run` (wants
// interpreter-like ergonomics from the same generated code).
type TranspileOptions struct {
	// PrintResult, when true, prints the value of the program's last
	// top-level statement if it is an expression statement — mirroring the
	// tree-walking interpreter's `caja run` behavior. Left false for `caja
	// build` so standalone binaries keep their existing output.
	PrintResult bool
}

type transpileContext struct {
	analyzer              *analyzer.Analyzer
	currentFuncName       string
	currentFunctionParams []string
	// currentFunctionReturnType is the enclosing function's declared return
	// type, as the same Go type string used in its signature (see retType at
	// both fnCtx construction sites). Passed as expectedType when
	// transpiling a ReturnStatement's value, so an ambiguous literal whose
	// own inferred type is vague (an empty array/map literal, e.g. `return
	// []` from a function declared to return [Transition]) resolves against
	// the function's actual declared type instead of its own "any" default.
	currentFunctionReturnType string
	hasTailCall               bool
	usedModules           map[string]bool
	CurrentModulePath     string
	// CurrentSourceFile is the .caja file the statement currently being
	// transpiled came from — the top-level script, or a resolved imported
	// module path. Used to emit `//line` directives (see lineDirective) so a
	// Go panic or compile error reports a .caja file/line instead of a
	// position in the generated (and later deleted) temp Go file.
	CurrentSourceFile string
	packageLevelCode  *bytes.Buffer
	inFunction        bool
	usesAsync         bool

	// currentFunctionBody is the *ast.BlockStatement of the OUTERMOST
	// enclosing caja function currently being transpiled — never a nested
	// closure's own body. nil at the top level (script statements outside
	// any function), where the dead-name optimization in maybeShareValue
	// never applies. Nested closures INHERIT this pointer unchanged from
	// their parent ctx (see the two fnCtx construction sites), so the
	// liveness search in maybeShareValue always scans a superset scope — a
	// strict superset can only produce extra conservative false positives (a
	// missed optimization), never a false negative (a correctness bug),
	// which is what makes it safe to catch a closure capturing an outer
	// identifier regardless of where in the function that closure is defined.
	currentFunctionBody *ast.BlockStatement

	// inLoop is true iff the function whose body is CURRENTLY being
	// transpiled (the immediate enclosing function — NOT inherited across a
	// closure boundary, unlike currentFunctionBody) will be wrapped in tail
	// call optimization's `for { ... }` (see functionBodyHasSelfTailCall).
	// When true, maybeShareValue never elides the cajaShare wrap for an
	// Identifier source, since the same rebind statement re-executes against
	// the previous iteration's value on every simulated "call".
	inLoop bool

	// topLevelFileName is the outermost script's own file identity, captured
	// once from Transpile's stable `a` parameter and never reassigned.
	// ctx.analyzer (and any local `a := ctx.analyzer` alias of it) DOES get
	// swapped to each imported module's own analyzer while that module's
	// code is being transpiled, so ctx.analyzer.GlobalEnv().FileName is only
	// correct when transpiling the top-level script itself — comparing a
	// definition's FilePath against THAT (rather than this fixed field)
	// while inside a module's own code always sees "same file" (a module
	// comparing its own FilePath against its own GlobalEnv().FileName), so
	// a struct/identifier a module refers to that is ALSO defined in that
	// same module would wrongly skip the module-prefix its own type
	// definition already got. Every "is this defined outside the top-level
	// script" check must compare against this field instead.
	topLevelFileName string
}

// enableValueFormatting marks the shared caja_format_value runtime helper (and
// the stdlib imports it needs) as required. Used by both the `caja run`
// final-value auto-print and the `log.export` CSV writer, so a value never
// needs two independent formatting implementations.
func enableValueFormatting(ctx *transpileContext) {
	ctx.usedModules["format_value"] = true
	ctx.usedModules["reflect"] = true
	ctx.usedModules["sort"] = true
	ctx.usedModules["time"] = true
	ctx.usedModules["fmt"] = true
	ctx.usedModules["strings"] = true
}

// statementLine returns the .caja source line a top-level ast.Statement
// starts at, or 0 if unknown. There's no Pos()/line accessor on the
// ast.Statement interface itself, so this mirrors transpileStatement's own
// type switch — every concrete statement type embeds a Token field.
func statementLine(stmt ast.Statement) int {
	switch s := stmt.(type) {
	case *ast.LetStatement:
		return s.Token.Line
	case *ast.ConstStatement:
		return s.Token.Line
	case *ast.ImportStatement:
		return s.Token.Line
	case *ast.ReturnStatement:
		return s.Token.Line
	case *ast.AssignStatement:
		return s.Token.Line
	case *ast.BlockStatement:
		return s.Token.Line
	case *ast.TypeConstraintStatement:
		return s.Token.Line
	case *ast.TypeAliasStatement:
		return s.Token.Line
	case *ast.ExpressionStatement:
		return s.Token.Line
	case *ast.IndexAssignmentStatement:
		return s.Token.Line
	case *ast.PropertyAssignmentStatement:
		return s.Token.Line
	case *ast.AwaitStatement:
		return s.Token.Line
	default:
		return 0
	}
}

// lineDirective returns a Go `//line file:line` compiler directive that
// remaps the position Go reports (in panics, compile errors, and stack
// traces) for whatever generated code immediately follows it, back to the
// original .caja source. Must be written at column 0 with nothing else on
// its line — the Go compiler silently ignores an indented `//line` comment,
// so callers must never prepend a tab before this string.
func lineDirective(file string, line int) string {
	if file == "" || line <= 0 {
		return ""
	}
	return fmt.Sprintf("//line %s:%d\n", file, line)
}

// pinRangeToLine re-asserts the same //line directive before every internal
// line break in code, so a multi-line generated block (stream-pipe/async/
// unwrap IIFEs, which bypass transpileStatement's per-statement directives
// entirely) reports as one single .caja source line on panic, regardless of
// which physical Go line inside it actually panics. Never inserts a
// directive after the final line — callers glue real trailing code directly
// onto it with no separating newline in some contexts (e.g. array/call-
// argument position), and a dangling directive there would either get
// silently swallowed into a comment or corrupt that trailing syntax.
func pinRangeToLine(code, file string, line int) string {
	if file == "" || line <= 0 {
		return code
	}
	lines := strings.Split(code, "\n")
	if len(lines) < 2 {
		return code
	}
	directive := lineDirective(file, line)
	var buf strings.Builder
	for i, l := range lines {
		if i > 0 {
			buf.WriteString("\n")
			if !(i == len(lines)-1 && l == "") {
				buf.WriteString(directive)
			}
		}
		buf.WriteString(l)
	}
	return buf.String()
}

// splitGenericTypeArgs splits the comma-separated type arguments inside the
// outermost [...] of a generic type instantiation string (e.g.
// "cajaMap[string, float64]" -> ["string", "float64"]), respecting nested
// brackets so a nested generic type argument (e.g. a matrix's
// "cajaArray[*cajaArray[float64]]") isn't split on its own internal commas.
// Used only as a fallback when an array/map literal's element/key/value type
// can't be resolved directly from its own analyzer symbol.
func splitGenericTypeArgs(s string) []string {
	start := strings.Index(s, "[")
	end := strings.LastIndex(s, "]")
	if start == -1 || end == -1 || end <= start {
		return nil
	}
	inner := s[start+1 : end]
	var args []string
	depth := 0
	last := 0
	for i, r := range inner {
		switch r {
		case '[':
			depth++
		case ']':
			depth--
		case ',':
			if depth == 0 {
				args = append(args, strings.TrimSpace(inner[last:i]))
				last = i + 1
			}
		}
	}
	args = append(args, strings.TrimSpace(inner[last:]))
	return args
}

// ensureUnshared walks an lvalue path (an Identifier, or a chain of
// PropertyExpression/IndexExpression built on top of one — arbitrarily
// nested, e.g. `a.b[0].c` or a matrix's `matrix[i][j]`) and emits, into buf,
// whatever "if X.cajaShared { X = X.cajaClone() }" statements are needed so
// every copy-on-write-tracked container along the path is uniquely owned by
// the time execution reaches the statement that follows. Returns the Go
// lvalue expression string that reaches this exact location — reassignable,
// and safe for the caller to select/index one further step into (the caller
// is responsible for that final step; this function resolves everything up
// to, and including making unshared, the container the final step targets).
//
// This is the single mechanism behind both chained property/index
// assignment and array/map copy-on-write: Go already supports assigning
// through an arbitrarily deep chain natively (`a.B.C.D = val`), so the only
// thing missing at any level beyond the first was this check — and the
// check has the same shape everywhere, since structs, arrays, and maps all
// share the same cajaShared/cajaClone convention (see isSharableSymbol).
func ensureUnshared(expr ast.Expression, ctx *transpileContext, buf *bytes.Buffer) (string, error) {
	a := ctx.analyzer
	switch e := expr.(type) {
	case *ast.PropertyExpression:
		parent, err := ensureUnshared(e.Object, ctx, buf)
		if err != nil {
			return "", err
		}
		if objSym, _ := a.GetSymbol(e.Object); objSym != nil {
			if modSym, ok := objSym.(*symbol.ModuleSymbol); ok && modSym.FilePath != "" {
				// A module-level binding, not a COW-tracked container —
				// resolve exactly like the normal read path does, no chain
				// to cascade through.
				return fmt.Sprintf("%s_%s", sanitizeIdentifier(modSym.FilePath), e.Property.Value), nil
			}
		}
		exportedName := strings.ToUpper(e.Property.Value[:1]) + e.Property.Value[1:]
		fieldExpr := fmt.Sprintf("%s.%s", parent, exportedName)
		if sym, ok := a.GetSymbol(e); ok && isSharableSymbol(sym) {
			ctx.usedModules["cow_shared"] = true
			buf.WriteString(fmt.Sprintf("if %s.cajaShared {\n%s = %s.cajaClone()\n}\n", fieldExpr, fieldExpr, fieldExpr))
		}
		return fieldExpr, nil
	case *ast.IndexExpression:
		parent, err := ensureUnshared(e.Left, ctx, buf)
		if err != nil {
			return "", err
		}
		index, err := transpileExpression(e.Index, ctx, "")
		if err != nil {
			return "", err
		}
		leftSym, _ := a.GetSymbol(e.Left)
		var elemExpr string
		if _, isArr := leftSym.(*symbol.ArraySymbol); isArr {
			elemExpr = fmt.Sprintf("%s.Data[int(%s)]", parent, index)
		} else {
			if indexSym, _ := a.GetSymbol(e.Index); isStructKeySymbol(indexSym) {
				index += ".Key()"
			}
			elemExpr = fmt.Sprintf("%s.Data[%s]", parent, index)
		}
		if sym, ok := a.GetSymbol(e); ok && isSharableSymbol(sym) {
			ctx.usedModules["cow_shared"] = true
			buf.WriteString(fmt.Sprintf("if %s.cajaShared {\n%s = %s.cajaClone()\n}\n", elemExpr, elemExpr, elemExpr))
		}
		return elemExpr, nil
	default:
		// A plain Identifier (the base case — mirrors the single-level
		// struct-COW check this function generalizes), or any other
		// expression shape we don't specifically cascade through: transpile
		// normally, and if it's an Identifier resolving to a COW-tracked
		// container, apply the same check with no chain to walk.
		name, err := transpileExpression(expr, ctx, "")
		if err != nil {
			return "", err
		}
		if _, ok := expr.(*ast.Identifier); ok {
			if sym, ok := a.GetSymbol(expr); ok && isSharableSymbol(sym) {
				ctx.usedModules["cow_shared"] = true
				buf.WriteString(fmt.Sprintf("if %s.cajaShared {\n%s = %s.cajaClone()\n}\n", name, name, name))
			}
		}
		return name, nil
	}
}

// isSharableSymbol unwraps a NullableSymbol (mirroring the exact unwrapping
// ctx.mapSymbolToGoType already does, since a nullable struct/array/map type
// is still just a pointer under the hood) and reports whether the resolved
// type beneath is one of the copy-on-write-tracked containers: a struct
// (StructInstanceSymbol for a variable holding a struct value, StructDefSymbol
// for e.g. how a function parameter's struct type resolves — both map to the
// identical "*StructName" Go type), an array, or a map. Arrays and maps
// compile to *cajaArray[T]/*cajaMap[K,V] (see mapSymbolToGoType), which carry
// the same cajaShared/cajaSetShared/cajaClone convention as generated struct
// types, so one predicate covers all three uniformly.
func isSharableSymbol(sym symbol.Symbol) bool {
	if sym == nil {
		return false
	}
	if nullableSym, ok := sym.(*symbol.NullableSymbol); ok {
		return isSharableSymbol(nullableSym.Underlying)
	}
	switch sym.(type) {
	case *symbol.StructInstanceSymbol, *symbol.StructDefSymbol, *symbol.ArraySymbol, *symbol.MapSymbol:
		return true
	}
	return false
}

// isStructKeySymbol reports whether sym resolves to a struct (either
// StructInstanceSymbol, or StructDefSymbol — the same instance-vs-declared-
// type split isSharableSymbol unwraps, e.g. a variable declared with an
// explicit struct type annotation resolves to StructDefSymbol on later
// references, not StructInstanceSymbol), used as a map key. A struct map key
// must expose a `key() -> String` field (validated at analysis time), so its
// Go map index expression needs a trailing ".Key()" to call that closure and
// get the actual string key, instead of using the struct pointer itself.
func isStructKeySymbol(sym symbol.Symbol) bool {
	switch sym.(type) {
	case *symbol.StructInstanceSymbol, *symbol.StructDefSymbol:
		return true
	}
	return false
}

// functionBodyHasSelfTailCall reports whether body contains, anywhere within
// the recursion topology transpileStatement itself uses when sharing a
// single ctx across nested transpilation (a plain BlockStatement, or an
// IfExpression's Consequence/Alternative — every other construct containing
// a block is a FunctionLiteral, which always forks a fresh transpileContext
// rather than reusing this one), a `return funcName(...)` that
// transpileStatement's ReturnStatement case rewrites into `continue` inside
// a `for{}` wrapper around the whole function body. Must run as a pre-pass
// before transpileStatement(body, fnCtx) — ctx.hasTailCall is only known
// partway through that same forward pass, too late for an earlier
// maybeShareValue call in the body to see it.
func functionBodyHasSelfTailCall(body *ast.BlockStatement, funcName string) bool {
	if body == nil || funcName == "" {
		return false
	}
	for _, stmt := range body.Statements {
		switch s := stmt.(type) {
		case *ast.ReturnStatement:
			if call, ok := s.ReturnValue.(*ast.CallExpression); ok {
				if ident, ok := call.Function.(*ast.Identifier); ok && ident.Value == funcName {
					return true
				}
			}
		case *ast.ExpressionStatement:
			if ifExpr, ok := s.Expression.(*ast.IfExpression); ok {
				if functionBodyHasSelfTailCall(ifExpr.Consequence, funcName) {
					return true
				}
				if ifExpr.Alternative != nil && functionBodyHasSelfTailCall(ifExpr.Alternative, funcName) {
					return true
				}
			}
		}
	}
	return false
}

// afterPos reports whether (line, col) is strictly after (afterLine, afterCol)
// in source-position order.
func afterPos(line, col, afterLine, afterCol int) bool {
	if line != afterLine {
		return line > afterLine
	}
	return col > afterCol
}

// identifierReadAfter reports whether name may still be read anywhere within
// node's subtree after source position (afterLine, afterCol) — a "read"
// meaning any place *ast.Identifier{Value: name} appears in an expression
// position that gets evaluated (write targets like an AssignStatement's own
// Name, or a PropertyExpression's field name, are excluded). Any occurrence
// found inside a nested *ast.FunctionLiteral body counts as "read after"
// unconditionally, regardless of its own position relative to
// (afterLine, afterCol) — a closure can be invoked at an arbitrary future
// time, even one defined textually before the point being checked, so
// position comparison inside one is meaningless. An unrecognized node type
// falls through to the default case, which conservatively returns true
// (assume it could contain a read) rather than silently under-approximating.
func identifierReadAfter(node ast.Node, name string, afterLine, afterCol int) bool {
	return identifierReadAfterWalk(node, name, afterLine, afterCol, false)
}

func identifierReadAfterWalk(node ast.Node, name string, afterLine, afterCol int, insideClosure bool) bool {
	rec := func(n ast.Node) bool {
		return identifierReadAfterWalk(n, name, afterLine, afterCol, insideClosure)
	}
	switch n := node.(type) {
	case nil:
		return false
	case *ast.Identifier:
		if n.Value != name {
			return false
		}
		if insideClosure {
			return true
		}
		return afterPos(n.Token.Line, n.Token.Column, afterLine, afterCol)
	case *ast.BlockStatement:
		for _, s := range n.Statements {
			if rec(s) {
				return true
			}
		}
		return false
	case *ast.LetStatement:
		return rec(n.Value)
	case *ast.ConstStatement:
		return rec(n.Value)
	case *ast.ImportStatement:
		return false
	case *ast.ReturnStatement:
		return rec(n.ReturnValue)
	case *ast.AssignStatement:
		return rec(n.Value)
	case *ast.TypeConstraintStatement:
		return rec(n.Predicate)
	case *ast.TypeAliasStatement:
		return false
	case *ast.ExpressionStatement:
		return rec(n.Expression)
	case *ast.IndexAssignmentStatement:
		return rec(n.Left) || rec(n.Index) || rec(n.Value)
	case *ast.PropertyAssignmentStatement:
		return rec(n.Object) || rec(n.Value)
	case *ast.AwaitStatement:
		for _, p := range n.Pipelines {
			if rec(p) {
				return true
			}
		}
		return false
	case *ast.NilLiteral, *ast.NumberLiteral, *ast.StringLiteral, *ast.BooleanLiteral, *ast.DateLiteral:
		return false
	case *ast.ArrayLiteral:
		for _, el := range n.Elements {
			if rec(el) {
				return true
			}
		}
		return false
	case *ast.FunctionLiteral:
		return identifierReadAfterWalk(n.Body, name, afterLine, afterCol, true)
	case *ast.StructLiteral:
		for _, v := range n.Fields {
			if rec(v) {
				return true
			}
		}
		return false
	case *ast.GenericIdentifier:
		return rec(n.Identifier)
	case *ast.PrefixExpression:
		return rec(n.Right)
	case *ast.InfixExpression:
		return rec(n.Left) || rec(n.Right)
	case *ast.IfExpression:
		if rec(n.Condition) || rec(n.Consequence) {
			return true
		}
		return n.Alternative != nil && rec(n.Alternative)
	case *ast.CallExpression:
		if rec(n.Function) {
			return true
		}
		for _, a := range n.Arguments {
			if rec(a) {
				return true
			}
		}
		return false
	case *ast.IndexExpression:
		return rec(n.Left) || rec(n.Index)
	case *ast.PropertyExpression:
		return rec(n.Object)
	case *ast.MapLiteral:
		for k, v := range n.Pairs {
			if rec(k) || rec(v) {
				return true
			}
		}
		return false
	case *ast.SafePipeExpression:
		return rec(n.Left) || (n.Call != nil && rec(n.Call))
	case *ast.StreamPipeExpression:
		if rec(n.Left) {
			return true
		}
		if n.Join != nil && rec(n.Join) {
			return true
		}
		return n.Call != nil && rec(n.Call)
	case *ast.JoinGroupExpression:
		for _, c := range n.Calls {
			if rec(c) {
				return true
			}
		}
		return false
	case *ast.AsyncExpression:
		return rec(n.Right)
	case *ast.UnwrapExpression:
		return rec(n.Right)
	default:
		return true // fail safe: unrecognized node type, assume it could read name
	}
}

// maybeShareValue is the single copy-on-write "aliasing" check, called
// wherever a value currently gets aliased into a new binding (let/const,
// assignment, return, a call argument, a struct-literal field). If
// sourceExpr resolves to a struct value and isn't a freshly-constructed,
// definitely-unaliased temporary (isOwned — a call result, a struct
// literal, or an explicit `move`), the generated code marks it shared via
// cajaShare before handing it off. Checking the source expression's own
// resolved type (rather than a callee's declared parameter type) keeps this
// uniform across all call sites without needing to resolve callee
// signatures.
//
// When sourceExpr is a plain identifier whose name is never read again
// within the enclosing function (see identifierReadAfter), this rebind is
// just a move of the last live reference to the object, not a new alias —
// nothing needs marking, and whatever cajaShared state the object already
// carries (set correctly by an earlier, real aliasing event, if any) is left
// untouched. This is skipped entirely inside a self-tail-recursive function
// body (ctx.inLoop), since the same rebind statement re-executes against the
// previous "iteration"'s value there and a skipped share would let one
// iteration's in-place mutation corrupt what the next iteration reads.
func maybeShareValue(sourceExpr ast.Expression, code string, ctx *transpileContext) string {
	sym, _ := ctx.analyzer.GetSymbol(sourceExpr)
	if !isSharableSymbol(sym) || isOwned(sourceExpr) {
		return code
	}

	if ident, ok := sourceExpr.(*ast.Identifier); ok &&
		!ctx.inLoop &&
		ctx.currentFunctionBody != nil &&
		ident.Token.Line > 0 { // exclude synthetic zero-position identifiers spliced
		// in during stream-pipe/join codegen — a zero line/col isn't a real
		// source position and must not feed the liveness comparison.
		if !identifierReadAfter(ctx.currentFunctionBody, ident.Value, ident.Token.Line, ident.Token.Column) {
			return code
		}
	}

	ctx.usedModules["cow_shared"] = true
	return fmt.Sprintf("cajaShare(%s)", code)
}

// isNothingReturnType reports whether sym is the built-in "Nothing" type
// analyzer.go injects as a global (analyzeProgram: `analyzer.types[0]["Nothing"]
// = symbol.NewStructDefSymbol("Nothing", ...)`) so a function can declare
// "-> Nothing" to mean "returns no value" — the analyzer already treats it
// exactly like a void return for guaranteed-return checking (see its own
// isNothing checks in analyzeFunctionLiteral). Nothing has no corresponding
// Go type emitted anywhere in the generated program (it's a synthetic
// analyzer-only placeholder, not a real `type ... struct{}` from user source),
// so codegen must treat it as void too: NULL_OBJ (the sentinel builtin
// functions like log.info use for the same purpose) and Nothing both mean
// "no Go return type, no returned value" here.
func isNothingReturnType(sym symbol.Symbol) bool {
	if sym == nil || sym.Type() == environment.NULL_OBJ {
		return true
	}
	structDef, ok := sym.(*symbol.StructDefSymbol)
	return ok && structDef.Name == "Nothing"
}

// ctx.mapSymbolToGoType converts a semantic symbol to a static Go type string.
func (ctx *transpileContext) mapSymbolToGoType(sym symbol.Symbol) string {
	if sym == nil {
		return "any"
	}

	if constraint, ok := sym.(*symbol.ConstraintSymbol); ok {
		return "*" + constraint.Name
	}

	// Interface checks must happen BEFORE sym.Type() because some symbols
	// (like NullableSymbol) forward their Type() to their underlying symbol!
	if nullableSym, ok := sym.(*symbol.NullableSymbol); ok {
		underlyingType := ctx.mapSymbolToGoType(nullableSym.Underlying)
		if strings.HasPrefix(underlyingType, "*") || strings.HasPrefix(underlyingType, "map[") ||
			strings.HasPrefix(underlyingType, "[]") || strings.HasPrefix(underlyingType, "func(") ||
			underlyingType == "any" {
			return underlyingType
		}
		return "*" + underlyingType
	}
	if _, ok := sym.(*symbol.AsyncSymbol); ok {
		// AsyncSymbol.Type() forwards to its underlying symbol (like
		// NullableSymbol above), so this check must also happen before the
		// generic sym.Type() switch below. asyncTask stores its value as
		// `any` internally (see transpileAsyncExpression), so every async
		// value shares this one non-generic Go type regardless of its
		// underlying element type.
		return "*asyncTask"
	}
	if activeSym, ok := sym.(*symbol.ActiveSymbol); ok {
		// ActiveSymbol.Type() also forwards to its underlying symbol (see
		// its own doc comment for why that's safe here), so this check must
		// also happen before the generic sym.Type() switch below. Unlike
		// asyncTask, cajaActive is itself generic over the underlying Go
		// type, so every active value's Go type still varies by element.
		ctx.usedModules["active_cell"] = true
		return "*cajaActive[" + ctx.mapSymbolToGoType(activeSym.Underlying) + "]"
	}
	if arrSym, ok := sym.(*symbol.ArraySymbol); ok {
		elType := ctx.mapSymbolToGoType(arrSym.ElementSymbol())
		if elType == "" {
			elType = "any"
		}
		ctx.usedModules["cow_array"] = true
		return "*cajaArray[" + elType + "]"
	}
	if mapSym, ok := sym.(*symbol.MapSymbol); ok {
		kType := ctx.mapSymbolToGoType(mapSym.Key)
		vType := ctx.mapSymbolToGoType(mapSym.Value)
		if kType == "" {
			kType = "any"
		} else if strings.HasPrefix(kType, "*") {
			kType = "string"
		}
		if vType == "" {
			vType = "any"
		}
		ctx.usedModules["cow_map"] = true
		return "*cajaMap[" + kType + ", " + vType + "]"
	}
	if fnSym, ok := sym.(*symbol.FunctionSymbol); ok {
		var paramTypes []string
		for _, p := range fnSym.ParamTypes() {
			pt := ctx.mapSymbolToGoType(p)
			if pt == "" {
				pt = "any"
			}
			paramTypes = append(paramTypes, pt)
		}

		retType := ""
		if !isNothingReturnType(fnSym.ReturnType()) {
			retType = ctx.mapSymbolToGoType(fnSym.ReturnType())
		}

		goFunc := fmt.Sprintf("func(%s)", strings.Join(paramTypes, ", "))
		if retType != "" {
			goFunc += " " + retType
		}
		return goFunc
	}
	if structDef, ok := sym.(*symbol.StructDefSymbol); ok {
		baseName := structDef.Name
		if structDef.FilePath != "" && ctx != nil && structDef.FilePath != ctx.topLevelFileName {
			baseName = sanitizeIdentifier(structDef.FilePath) + "_" + baseName
		}
		if len(structDef.InstantiatedTypes) > 0 {
			var typeArgs []string
			for _, t := range structDef.InstantiatedTypes {
				typeArgs = append(typeArgs, ctx.mapSymbolToGoType(t))
			}
			return fmt.Sprintf("*%s[%s]", baseName, strings.Join(typeArgs, ", "))
		}
		return "*" + baseName
	}
	if unionSym, ok := sym.(*symbol.UnionSymbol); ok {
		baseName := unionSym.Name
		if unionSym.FilePath != "" && ctx != nil && unionSym.FilePath != ctx.topLevelFileName {
			baseName = sanitizeIdentifier(unionSym.FilePath) + "_" + baseName
		}
		return baseName
	}
	if structInst, ok := sym.(*symbol.StructInstanceSymbol); ok {
		baseName := structInst.Def.Name
		if structInst.Def.FilePath != "" && ctx != nil && structInst.Def.FilePath != ctx.topLevelFileName {
			baseName = sanitizeIdentifier(structInst.Def.FilePath) + "_" + baseName
		}
		if len(structInst.Def.InstantiatedTypes) > 0 {
			var typeArgs []string
			for _, t := range structInst.Def.InstantiatedTypes {
				typeArgs = append(typeArgs, ctx.mapSymbolToGoType(t))
			}
			return fmt.Sprintf("*%s[%s]", baseName, strings.Join(typeArgs, ", "))
		}
		return "*" + baseName
	}

	switch sym.Type() {
	case environment.ANY_OBJ:
		if genSym, ok := sym.(*symbol.GenericSymbol); ok {
			return genSym.Name
		}
		return "any"
	case environment.NUMBER_OBJ:
		return "float64"
	case environment.STRING_OBJ:
		return "string"
	case environment.BOOLEAN_OBJ:
		return "bool"
	case environment.DATE_OBJ:
		return "time.Time"
	case environment.ELEMENT_OBJ:
		ctx.usedModules["syscall/js"] = true
		return "js.Value"
	default:
		return ""
	}
}

// Transpile walks the AST and returns the equivalent Go source code.
func Transpile(program *ast.Program, a *analyzer.Analyzer, opts TranspileOptions) (string, error) {
	streamPipeCounter = 0

	var bodyBuf bytes.Buffer
	var pkgLevelBuf bytes.Buffer

	topSourceFile := ""
	if a.GlobalEnv() != nil {
		topSourceFile = a.GlobalEnv().FileName
	}

	ctx := &transpileContext{
		analyzer:          a,
		usedModules:       make(map[string]bool),
		packageLevelCode:  &pkgLevelBuf,
		CurrentSourceFile: topSourceFile,
		topLevelFileName:  topSourceFile,
	}

	// Prepend imported custom modules in topological order
	if a.GlobalEnv() != nil {
		orderedModules := getOrderedModules(program, a.GlobalEnv().ModuleASTs)
		for _, modPath := range orderedModules {
			modAST := a.GlobalEnv().ModuleASTs[modPath]
			modAnalyzer := a.GlobalEnv().ModuleAnalyzers[modPath].(*analyzer.Analyzer)
			ctx.analyzer = modAnalyzer
			ctx.CurrentModulePath = modPath
			ctx.CurrentSourceFile = a.GlobalEnv().ModuleFilePaths[modPath]
			bodyBuf.WriteString(fmt.Sprintf("\t// --- Module: %s ---\n", modPath))
			for _, stmt := range modAST.Statements {
				code, err := transpileStatement(stmt, ctx)
				if err != nil {
					return "", err
				}
				if code == "" {
					continue
				}

				_, isTypeAlias := stmt.(*ast.TypeAliasStatement)
				_, isUnion := stmt.(*ast.UnionStatement)
				if isTypeAlias || isUnion {
					ctx.packageLevelCode.WriteString(code + "\n")
					continue
				}

				bodyBuf.WriteString(lineDirective(ctx.CurrentSourceFile, statementLine(stmt)))
				bodyBuf.WriteString("\t")
				bodyBuf.WriteString(code)
				bodyBuf.WriteString("\n")

				if letStmt, ok := stmt.(*ast.LetStatement); ok {
					bodyBuf.WriteString(fmt.Sprintf("\t_ = %s\n", sanitizeIdentifier(ctx.CurrentModulePath)+"_"+letStmt.Name.Value))
				} else if constStmt, ok := stmt.(*ast.ConstStatement); ok {
					bodyBuf.WriteString(fmt.Sprintf("\t_ = %s\n", sanitizeIdentifier(ctx.CurrentModulePath)+"_"+constStmt.Name.Value))
				}
			}
			bodyBuf.WriteString("\n")
		}
	}
	ctx.CurrentModulePath = ""
	ctx.CurrentSourceFile = topSourceFile
	ctx.analyzer = a

	for i, stmt := range program.Statements {
		code, err := transpileStatement(stmt, ctx)
		if err != nil {
			return "", err
		}
		if code == "" {
			continue
		}

		_, isTypeAlias := stmt.(*ast.TypeAliasStatement)
		_, isUnion := stmt.(*ast.UnionStatement)
		if isTypeAlias || isUnion {
			ctx.packageLevelCode.WriteString(code + "\n")
			continue
		}

		if opts.PrintResult && i == len(program.Statements)-1 {
			if exprStmt, ok := stmt.(*ast.ExpressionStatement); ok {
				if sym, ok := a.GetSymbol(exprStmt.Expression); ok && !isNothingReturnType(sym) {
					ctx.usedModules["print_result"] = true
					enableValueFormatting(ctx)
					bodyBuf.WriteString(lineDirective(ctx.CurrentSourceFile, statementLine(stmt)))
					bodyBuf.WriteString(fmt.Sprintf("\t_cajaResult := %s\n\tcaja_print_result(_cajaResult)\n", code))
					continue
				}
			}
			// A top-level `return <expr>` as the script's final statement is
			// the interpreter-era convention for "this is the script's
			// result" (transpileStatement's *ast.ReturnStatement case
			// otherwise discards the value via `_ = val` at top level, since
			// a bare Go `return` can't carry one out of func main). Print it
			// the same way a trailing bare expression is printed, instead of
			// re-emitting the original discarding form.
			if retStmt, ok := stmt.(*ast.ReturnStatement); ok && retStmt.ReturnValue != nil {
				if sym, ok := a.GetSymbol(retStmt.ReturnValue); ok && !isNothingReturnType(sym) {
					val, err := transpileExpression(retStmt.ReturnValue, ctx, "")
					if err != nil {
						return "", err
					}
					val = maybeShareValue(retStmt.ReturnValue, val, ctx)
					ctx.usedModules["print_result"] = true
					enableValueFormatting(ctx)
					bodyBuf.WriteString(lineDirective(ctx.CurrentSourceFile, statementLine(stmt)))
					bodyBuf.WriteString(fmt.Sprintf("\t_cajaResult := %s\n\tcaja_print_result(_cajaResult)\n", val))
					continue
				}
			}
		}

		bodyBuf.WriteString(lineDirective(ctx.CurrentSourceFile, statementLine(stmt)))
		bodyBuf.WriteString("\t")
		bodyBuf.WriteString(code)
		bodyBuf.WriteString("\n")

		// Add dummy usage for variables to prevent "declared but not used" errors
		if letStmt, ok := stmt.(*ast.LetStatement); ok {
			bodyBuf.WriteString(fmt.Sprintf("\t_ = %s\n", letStmt.Name.Value))
		} else if constStmt, ok := stmt.(*ast.ConstStatement); ok {
			bodyBuf.WriteString(fmt.Sprintf("\t_ = %s\n", constStmt.Name.Value))
		}
	}

	bodyCode := bodyBuf.String()

	var finalBuf bytes.Buffer
	finalBuf.WriteString("package main\n\n")

	// fmt, os, runtime, and strings are always needed: every generated
	// main() opens with a recover handler (see below) that uses all four —
	// runtime.Callers/CallersFrames plus strings.HasSuffix to locate the
	// .caja source line a panic originated at — regardless of whether the
	// caja source itself needs them.
	finalBuf.WriteString("import \"fmt\"\n")
	finalBuf.WriteString("import \"os\"\n")
	finalBuf.WriteString("import \"runtime\"\n")
	finalBuf.WriteString("import \"strings\"\n")
	if ctx.usedModules["reflect"] {
		finalBuf.WriteString("import \"reflect\"\n")
	}
	if ctx.usedModules["sort"] {
		finalBuf.WriteString("import \"sort\"\n")
	}
	if ctx.usedModules["csv"] {
		finalBuf.WriteString("import \"encoding/csv\"\n")
	}
	if ctx.usedModules["math"] || ctx.usedModules["math_rand"] {
		finalBuf.WriteString("import \"math\"\n")
	}
	if ctx.usedModules["math_rand"] {
		finalBuf.WriteString("import \"math/rand\"\n")
	}
	if ctx.usedModules["utf8"] {
		finalBuf.WriteString("import \"unicode/utf8\"\n")
	}
	if ctx.usedModules["json"] {
		finalBuf.WriteString("import \"encoding/json\"\n")
	}
	if ctx.usedModules["strconv"] {
		finalBuf.WriteString("import \"strconv\"\n")
	}
	if ctx.usedModules["http"] {
		finalBuf.WriteString("import \"net/http\"\n")
		finalBuf.WriteString("import \"net\"\n")
		finalBuf.WriteString("import \"io\"\n")
		finalBuf.WriteString("import \"context\"\n")
		finalBuf.WriteString("import \"os/signal\"\n")
		finalBuf.WriteString("import \"syscall\"\n")
	}
	if ctx.usedModules["http_distributed_rate_limiter"] {
		finalBuf.WriteString("import \"bufio\"\n")
	}
	if strings.Contains(bodyCode, "sync.") || ctx.usedModules["sync"] {
		finalBuf.WriteString("import \"sync\"\n")
	}
	if ctx.usedModules["fnv"] {
		finalBuf.WriteString("import \"hash/fnv\"\n")
	}
	if ctx.usedModules["syscall/js"] {
		finalBuf.WriteString("import \"syscall/js\"\n")
	}

	needsTime := false
	for _, stmt := range program.Statements {
		if hasDateSymbol(stmt, a) {
			needsTime = true
			break
		}
	}
	if needsTime || strings.Contains(bodyCode, "time.") || ctx.usedModules["time"] || ctx.usedModules["http"] || ctx.usedModules["syscall/js"] {
		finalBuf.WriteString("import \"time\"\n")
	}

	finalBuf.WriteString("\nfunc main() {\n")
	// Turn an unhandled runtime panic into a clean, .caja-source-located
	// message instead of a raw Go stack trace pointing at line numbers
	// inside a deleted temp dir. caja_panic_location (builtins.go) walks the
	// preserved panic stack for the first frame whose file the `//line`
	// directives (emitted per-statement above) mapped to a .caja file.
	finalBuf.WriteString("\tdefer func() {\n\t\tif r := recover(); r != nil {\n\t\t\tif loc := caja_panic_location(); loc != \"\" {\n\t\t\t\tfmt.Fprintf(os.Stderr, \"error: %v\\n    at %s\\n\", r, loc)\n\t\t\t} else {\n\t\t\t\tfmt.Fprintln(os.Stderr, \"error:\", r)\n\t\t\t}\n\t\t\tos.Exit(1)\n\t\t}\n\t}()\n")
	if ctx.usedModules["log_export"] {
		finalBuf.WriteString("\tdefer caja_flush_export()\n")
	}
	finalBuf.WriteString(bodyCode)
	if ctx.usedModules["async_panic_guard"] {
		// Best-effort safety net for a fire-and-forget async/pipe task that
		// panicked but was never unwrap/await-ed: not watertight (a task
		// that's truly never synchronized with could still be abandoned
		// mid-execution when main returns, same as any non-panicking
		// goroutine would be), but catches the common case where it finishes
		// before the rest of the script does.
		finalBuf.WriteString("\tcaja_check_async_panic()\n")
	}
	if ctx.usedModules["syscall/js"] {
		// Every browser-module program blocks forever here, not just ones
		// that happen to call browser.on today — see below for why —
		// but NOT via a bare `select {}`. Confirmed by hitting this for
		// real: `select {}` alone is fine (main is the only ever-blocked
		// goroutine, and Go's deadlock detector doesn't flag a lone
		// goroutine parked in an empty select), but the moment ANY other
		// goroutine also blocks on an ordinary channel — e.g.
		// browser.fetch's caja_browser_fetch, which bridges a JS Promise
		// onto a channel receive, called from inside a browser.on
		// handler — Go's checkdead() sees two blocked goroutines with no
		// Go-visible way to wake either one (a pending JS Promise callback
		// isn't tracked by the scheduler at all) and kills the whole wasm
		// instance with "fatal error: all goroutines are asleep - deadlock!",
		// tearing down every registered listener with it. A time.Sleep loop
		// avoids this because it registers a real, scheduler-tracked timer
		// (backed by JS's own setTimeout under GOOS=js) — checkdead()
		// explicitly treats a pending timer as proof the program isn't
		// stuck, regardless of how many other goroutines are separately
		// blocked waiting on a JS callback. The sleep interval only needs to
		// be short enough to be a negligible wakeup cost; it doesn't gate
		// anything.
		//
		// Every browser-module program stays alive for the page's whole
		// lifetime rather than exiting after its first pass, mirroring how a
		// real page's own script never "returns" either — its JS environment
		// just sits there waiting for whatever happens next. Gating this on
		// "used browser at all" instead of "used on/fetch specifically"
		// also matters on its own: tying it to one builtin is fragile, since
		// every future event/async-registering builtin would have to
		// remember to opt back in, and a forgotten one fails silently (it
		// registers fine, then never fires once main exits). Blocking here
		// does not freeze the browser tab: Go's wasm scheduler cooperatively
		// yields back to the browser's own event loop between ticks rather
		// than spinning natively.
		finalBuf.WriteString("\tfor {\n\t\ttime.Sleep(time.Second)\n\t}\n")
	}
	finalBuf.WriteString("}\n")

	// A `//line` directive stays in effect until the next one or EOF — reset
	// it here so it doesn't leak into the injected runtime helpers below
	// (which aren't .caja source) and get misreported as one by
	// caja_panic_location's suffix check.
	finalBuf.WriteString(lineDirective("caja-runtime", 1))

	injectBuiltinDependencies(ctx, &finalBuf)

	// asyncTask is the runtime representation of an async/await value. Value is
	// stored as any (rather than a distinct monomorphized struct per element
	// type) and type-asserted back at each await site, since the analyzer
	// already guarantees the assertion is safe. Completion is signaled by
	// closing done, not by sending on it, so a task can be awaited more than
	// once (e.g. once inside a WaitGroup-style join barrier, then again
	// individually) — receiving from a closed channel never blocks and can be
	// done any number of times.
	if ctx.usesAsync {
		finalBuf.WriteString(`
type asyncTask struct {
	done chan struct{}
	val  any
}
`)
	}

	if needsTime {
		finalBuf.WriteString(`
func parseDate(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}
`)
	}

	finalBuf.WriteString("\n")
	finalBuf.WriteString(pkgLevelBuf.String())

	return finalBuf.String(), nil
}

func hasDateSymbol(node ast.Node, a *analyzer.Analyzer) bool {
	if sym, ok := a.GetSymbol(node); ok && sym != nil && sym.Type() == environment.DATE_OBJ {
		return true
	}
	// Note: Deep traversal would check children, but for our simple transpiler, we assume it's fine.
	// For actual implementation, an ast.Visitor or a full pass would be cleaner.
	return false // Simplified for brevity
}

// comparableGoTypes are the Go types produced by mapSymbolToGoType that are
// already valid map/sync.Map keys on their own. Anything else (slices, maps,
// struct pointers) is hashed into a uint64 via caja_memo_hash instead.
var comparableGoTypes = map[string]bool{
	"float64":   true,
	"string":    true,
	"bool":      true,
	"time.Time": true,
}

// transpileMemoBinding emits a memoized function bound via `let`/`const` as
// two predeclared local closures: `name` is the caching wrapper (checks and
// populates a package-level sync.Map keyed on the arguments), and
// `name_impl` holds the original function body. Both are predeclared before
// either is assigned so they can safely reference each other and anything
// else in scope — needed for recursive memoized functions (e.g. Fibonacci)
// and for closures over outer variables, since neither can be a real
// package-level Go func (see the memoization plan for why).
func transpileMemoBinding(name string, fnLit *ast.FunctionLiteral, ctx *transpileContext) (string, error) {
	a := ctx.analyzer
	sym, _ := a.GetSymbol(fnLit)
	fnSym, _ := sym.(*symbol.FunctionSymbol)

	var params []string
	var paramNames []string
	var paramGoTypes []string
	for i, param := range fnLit.Parameters {
		pt := "any"
		if fnSym != nil && i < len(fnSym.ParamTypes()) {
			if t := ctx.mapSymbolToGoType(fnSym.ParamTypes()[i]); t != "" {
				pt = t
			}
		}
		params = append(params, fmt.Sprintf("%s %s", param.Name, pt))
		paramNames = append(paramNames, param.Name)
		paramGoTypes = append(paramGoTypes, pt)
	}

	retType := ""
	if fnSym != nil && !isNothingReturnType(fnSym.ReturnType()) {
		retType = ctx.mapSymbolToGoType(fnSym.ReturnType())
	}

	fnCtx := &transpileContext{
		analyzer:                  a,
		currentFunctionParams:     paramNames,
		currentFunctionReturnType: retType,
		usedModules:               ctx.usedModules,
		CurrentModulePath:         ctx.CurrentModulePath,
		CurrentSourceFile:         ctx.CurrentSourceFile,
		packageLevelCode:          ctx.packageLevelCode,
		topLevelFileName:          ctx.topLevelFileName,
		inFunction:                true,
	}
	if fnSym != nil {
		fnCtx.currentFuncName = fnSym.Name
	}
	if ctx.currentFunctionBody != nil {
		fnCtx.currentFunctionBody = ctx.currentFunctionBody
	} else {
		fnCtx.currentFunctionBody = fnLit.Body
	}
	if fnCtx.currentFuncName != "" {
		fnCtx.inLoop = functionBodyHasSelfTailCall(fnLit.Body, fnCtx.currentFuncName)
	}

	body, err := transpileStatement(fnLit.Body, fnCtx)
	if err != nil {
		return "", err
	}
	if fnCtx.hasTailCall {
		body = fmt.Sprintf("{\nfor %s\n}", body)
	}

	wrapperName := prefixIdentifier(ctx, name)
	implName := wrapperName + "_impl"
	cacheVar := "_memo_" + wrapperName

	sig := fmt.Sprintf("func(%s)", strings.Join(params, ", "))
	if retType != "" {
		sig += " " + retType
	}

	keyParts := make([]string, len(paramNames))
	keyFieldTypes := make([]string, len(paramNames))
	for i, pn := range paramNames {
		if comparableGoTypes[paramGoTypes[i]] {
			keyParts[i] = pn
			keyFieldTypes[i] = paramGoTypes[i]
		} else {
			keyParts[i] = fmt.Sprintf("caja_memo_hash(%s)", pn)
			keyFieldTypes[i] = "uint64"
			ctx.usedModules["fnv"] = true
			ctx.usedModules["json"] = true
		}
	}

	var keyExpr string
	switch len(keyParts) {
	case 0:
		keyExpr = "struct{}{}"
	case 1:
		keyExpr = keyParts[0]
	default:
		var fields []string
		for i := range keyParts {
			fields = append(fields, fmt.Sprintf("F%d %s", i, keyFieldTypes[i]))
		}
		keyExpr = fmt.Sprintf("struct{ %s }{ %s }", strings.Join(fields, "; "), strings.Join(keyParts, ", "))
	}

	if ctx.packageLevelCode != nil {
		ctx.packageLevelCode.WriteString(fmt.Sprintf("var %s sync.Map\n", cacheVar))
	}
	ctx.usedModules["sync"] = true

	var out bytes.Buffer
	out.WriteString(fmt.Sprintf("var %s %s\n", wrapperName, sig))
	out.WriteString(fmt.Sprintf("var %s %s\n", implName, sig))
	out.WriteString(fmt.Sprintf("%s = %s {\n", wrapperName, sig))
	out.WriteString(fmt.Sprintf("key := %s\n", keyExpr))
	out.WriteString(fmt.Sprintf("if v, ok := %s.Load(key); ok {\n", cacheVar))
	out.WriteString(fmt.Sprintf("return v.(%s)\n", retType))
	out.WriteString("}\n")
	out.WriteString(fmt.Sprintf("result := %s(%s)\n", implName, strings.Join(paramNames, ", ")))
	if fnSym != nil && isSharableSymbol(fnSym.ReturnType()) {
		// The cache is a long-lived second owner of whatever gets stored
		// here, regardless of whether the impl's own return statement saw
		// it as a fresh, uniquely-owned value (e.g. a struct literal
		// returned directly, trusted as "owned" and left unmarked there).
		// Without this, the very first call's result is stored unshared,
		// and mutating the caller's copy corrupts the cached entry for
		// every future call with the same key. Marking it once here is
		// enough — cajaShared lives on the struct instance itself, so it
		// stays set across every future cache hit that hands out this
		// same pointer (see the `return v.(...)` cache-hit path above).
		ctx.usedModules["cow_shared"] = true
		out.WriteString("result = cajaShare(result)\n")
	}
	out.WriteString(fmt.Sprintf("%s.Store(key, result)\n", cacheVar))
	out.WriteString("return result\n")
	out.WriteString("}\n")
	out.WriteString(fmt.Sprintf("%s = %s %s", implName, sig, body))

	return out.String(), nil
}

func transpileStatement(stmt ast.Statement, ctx *transpileContext) (string, error) {
	a := ctx.analyzer
	switch s := stmt.(type) {
	case *ast.LetStatement:
		if fnLit, ok := s.Value.(*ast.FunctionLiteral); ok && fnLit.IsMemo {
			return transpileMemoBinding(s.Name.Value, fnLit, ctx)
		}

		sym, ok := a.GetSymbol(s)
		varType := ""
		if ok {
			varType = ctx.mapSymbolToGoType(sym)
		}

		// sym is nil (ok == false) when a.GetSymbol found nothing, but a nil
		// interface always fails a concrete-type assertion cleanly, so
		// isActive is already false in that case with no need to also
		// check ok here.
		if activeSym, isActive := sym.(*symbol.ActiveSymbol); isActive {
			if callExpr, deps, isReactive := reactiveCallInfo(s.Value); isReactive {
				val, err := transpileReactiveCallExpression(callExpr, deps, activeSym, ctx)
				if err != nil {
					return "", err
				}
				return fmt.Sprintf("var %s %s = %s", prefixIdentifier(ctx, s.Name.Value), varType, val), nil
			}

			// Plain "source" active variable: the initializer is transpiled
			// against the *underlying* type (not varType, which is the
			// wrapped *cajaActive[T]) and wrapped in newCajaActive.
			underlyingType := ctx.mapSymbolToGoType(activeSym.Underlying)
			val, err := transpileExpression(s.Value, ctx, underlyingType)
			if err != nil {
				return "", err
			}
			if val == "" {
				return "", nil
			}
			val = maybeShareValue(s.Value, val, ctx)
			return fmt.Sprintf("var %s %s = newCajaActive(%s)", prefixIdentifier(ctx, s.Name.Value), varType, val), nil
		}

		val, err := transpileExpression(s.Value, ctx, varType, sym)
		if err != nil {
			return "", err
		}

		if val == "" {
			return "", nil
		}
		val = maybeShareValue(s.Value, val, ctx)

		if varType != "" && varType != "any" {
			return fmt.Sprintf("var %s %s = %s", prefixIdentifier(ctx, s.Name.Value), varType, val), nil
		}
		return fmt.Sprintf("%s := %s", prefixIdentifier(ctx, s.Name.Value), val), nil

	case *ast.ConstStatement:
		if fnLit, ok := s.Value.(*ast.FunctionLiteral); ok && fnLit.IsMemo {
			return transpileMemoBinding(s.Name.Value, fnLit, ctx)
		}

		sym, ok := a.GetSymbol(s)
		varType := ""
		if ok {
			varType = ctx.mapSymbolToGoType(sym)
		}

		val, err := transpileExpression(s.Value, ctx, varType, sym)
		if err != nil {
			return "", err
		}

		if val == "" {
			return "", nil
		}
		val = maybeShareValue(s.Value, val, ctx)

		return fmt.Sprintf("%s := %s // const", prefixIdentifier(ctx, s.Name.Value), val), nil

	case *ast.ReturnStatement:
		if callNode, ok := s.ReturnValue.(*ast.CallExpression); ok {
			if ident, ok := callNode.Function.(*ast.Identifier); ok && ctx.currentFuncName != "" && ident.Value == ctx.currentFuncName {
				ctx.hasTailCall = true
				var buf bytes.Buffer
				for i, arg := range callNode.Arguments {
					argStr, err := transpileExpression(arg, ctx, "")
					if err != nil {
						return "", err
					}
					argStr = maybeShareValue(arg, argStr, ctx)
					buf.WriteString(fmt.Sprintf("_tco%d := %s\n", i, argStr))
				}
				for i, param := range ctx.currentFunctionParams {
					buf.WriteString(fmt.Sprintf("%s = _tco%d\n", param, i))
				}
				buf.WriteString("continue")
				return buf.String(), nil
			}
		}

		// A bare `Nothing {}` literal has no backing Go type anywhere in the
		// generated program (it's the analyzer's synthetic void-marker
		// struct, never a real `type Nothing struct{}` from user source — see
		// isNothingReturnType), and being a fields-less literal it can't have
		// side effects worth preserving, so skip transpiling it rather than
		// emit a reference to an undefined "Nothing" Go type.
		if structLit, ok := s.ReturnValue.(*ast.StructLiteral); ok && structLit.StructName == "Nothing" {
			s = &ast.ReturnStatement{Token: s.Token}
		}

		val, err := transpileExpression(s.ReturnValue, ctx, ctx.currentFunctionReturnType)
		if err != nil {
			return "", err
		}
		if val != "" {
			val = maybeShareValue(s.ReturnValue, val, ctx)
		}
		// A void Go function (top level, or a "-> Nothing" Caja function —
		// see isNothingReturnType) can't take a return value even when the
		// Caja source explicitly wrote one (e.g. `return Nothing {}`), so the
		// transpiled value is evaluated for side effects only and discarded.
		if !ctx.inFunction || ctx.currentFunctionReturnType == "" {
			if val != "" {
				return fmt.Sprintf("_ = %s\n\treturn", val), nil
			}
			return "return", nil
		}
		return fmt.Sprintf("return %s", val), nil
	case *ast.AssignStatement:
		val, err := transpileExpression(s.Value, ctx, "")
		if err != nil {
			return "", err
		}
		val = maybeShareValue(s.Value, val, ctx)
		targetName := resolveIdentifierGoName(s.Name, ctx)
		if targetSym, ok := a.GetSymbol(s.Name); ok {
			if _, isActive := targetSym.(*symbol.ActiveSymbol); isActive {
				return fmt.Sprintf("%s.Set(%s)", targetName, val), nil
			}
		}
		return fmt.Sprintf("%s = %s", targetName, val), nil
	case *ast.ExpressionStatement:
		val, err := transpileExpression(s.Expression, ctx, "")
		if err != nil {
			return "", err
		}
		return val, nil
	case *ast.AwaitStatement:
		return transpileAwaitStatement(s, ctx)
	case *ast.IndexAssignmentStatement:
		val, err := transpileExpression(s.Value, ctx, "")
		if err != nil {
			return "", err
		}
		val = maybeShareValue(s.Value, val, ctx)

		var buf bytes.Buffer
		left, err := ensureUnshared(s.Left, ctx, &buf)
		if err != nil {
			return "", err
		}
		index, err := transpileExpression(s.Index, ctx, "")
		if err != nil {
			return "", err
		}
		leftSym, _ := a.GetSymbol(s.Left)
		if _, isArr := leftSym.(*symbol.ArraySymbol); isArr {
			buf.WriteString(fmt.Sprintf("%s.Data[int(%s)] = %s", left, index, val))
			return buf.String(), nil
		}

		if indexSym, _ := a.GetSymbol(s.Index); isStructKeySymbol(indexSym) {
			index += ".Key()"
		}

		buf.WriteString(fmt.Sprintf("%s.Data[%s] = %s", left, index, val))
		return buf.String(), nil
	case *ast.PropertyAssignmentStatement:
		val, err := transpileExpression(s.Value, ctx, "")
		if err != nil {
			return "", err
		}
		val = maybeShareValue(s.Value, val, ctx)

		objSym, _ := a.GetSymbol(s.Object)
		if modSym, ok := objSym.(*symbol.ModuleSymbol); ok && modSym.FilePath != "" {
			return fmt.Sprintf("%s_%s = %s", sanitizeIdentifier(modSym.FilePath), s.Property.Value, val), nil
		}

		var buf bytes.Buffer
		left, err := ensureUnshared(s.Object, ctx, &buf)
		if err != nil {
			return "", err
		}
		exportedName := strings.ToUpper(s.Property.Value[:1]) + s.Property.Value[1:]
		buf.WriteString(fmt.Sprintf("%s.%s = %s", left, exportedName, val))
		return buf.String(), nil
	case *ast.BlockStatement:
		var buf bytes.Buffer
		buf.WriteString("{\n")
		for _, bstmt := range s.Statements {
			code, err := transpileStatement(bstmt, ctx)
			if err != nil {
				return "", err
			}
			buf.WriteString(lineDirective(ctx.CurrentSourceFile, statementLine(bstmt)))
			buf.WriteString("\t")
			buf.WriteString(code)
			buf.WriteString("\n")
		}
		buf.WriteString("}")
		return buf.String(), nil
	case *ast.ImportStatement:
		return fmt.Sprintf("// import %s", s.Path), nil
	case *ast.TypeConstraintStatement:
		baseTypeGo := ctx.mapSymbolToGoType(func() symbol.Symbol { sym, _ := a.GetSymbol(s.BaseType); return sym }())
		baseTypeNoPtr := strings.TrimPrefix(baseTypeGo, "*")
		predGo, err := transpileExpression(s.Predicate, ctx, "")
		if err != nil {
			return "", err
		}
		var buf bytes.Buffer
		buf.WriteString(fmt.Sprintf("type %s %s\n", s.Name.Value, baseTypeNoPtr))
		buf.WriteString(fmt.Sprintf("var validate_%s func(%s) *%s = func(val %s) *%s {\n	pred := %s\n	if pred(val) {\n		res := (*%s)(val)\n		return res\n	}\n	return nil\n}", s.Name.Value, baseTypeGo, s.Name.Value, baseTypeGo, s.Name.Value, predGo, s.Name.Value))
		return buf.String(), nil
	case *ast.TypeAliasStatement:
		if s.StructDefinition != nil {
			var buf bytes.Buffer
			baseName := prefixIdentifier(ctx, s.Name.Value)
			structName := baseName
			receiverType := baseName
			if len(s.TypeParameters) > 0 {
				var tps []string
				for _, tp := range s.TypeParameters {
					tps = append(tps, fmt.Sprintf("%s any", tp))
				}
				structName = fmt.Sprintf("%s[%s]", baseName, strings.Join(tps, ", "))
				receiverType = fmt.Sprintf("%s[%s]", baseName, strings.Join(s.TypeParameters, ", "))
			}
			buf.WriteString(fmt.Sprintf("type %s struct {\n", structName))
			var sharableFieldNames []string
			sym, ok := a.GetSymbol(s)
			if ok {
				if structDef, isStruct := sym.(*symbol.StructDefSymbol); isStruct {
					for _, field := range s.StructDefinition.Fields {
						fieldSym := structDef.Fields[field.Name.Value]
						goType := ctx.mapSymbolToGoType(fieldSym.Type)
						if goType == "" {
							goType = "any"
						}
						// Capitalize field name to make it exported in Go
						exportedName := strings.ToUpper(field.Name.Value[:1]) + field.Name.Value[1:]
						buf.WriteString(fmt.Sprintf("\t%s %s\n", exportedName, goType))
						if isSharableSymbol(fieldSym.Type) {
							sharableFieldNames = append(sharableFieldNames, exportedName)
						}
					}
				}
			}
			// cajaShared backs the copy-on-write scheme: once a struct value
			// is aliased into a second binding (see maybeShareValue), it's
			// marked shared permanently until a mutation clones it
			// (cajaClone) — a conservative "shared bit", not a refcount,
			// since Go gives no hook for detecting when an alias goes out of
			// scope. Unexported, so it can never collide with a user field
			// (every user field name is capitalized above) — but that also
			// means caja_format_value must skip unexported fields, or this
			// would leak into printed/exported struct output.
			buf.WriteString("\tcajaShared bool\n")
			buf.WriteString("}")

			ctx.usedModules["cow_shared"] = true
			var cascade strings.Builder
			for _, fieldName := range sharableFieldNames {
				// Aliasing this struct implicitly aliases everything
				// reachable through it — s.Books is the same pointer either
				// way the struct itself is reached, so it must be marked
				// shared too, or a later `lib.books[i] = x` (which only
				// checks lib.Books' own bit, not lib's) would mutate an
				// array another binding still sees. cajaSetShared's own
				// nil-guard makes this safe to call unconditionally even
				// for a nullable/unset field.
				cascade.WriteString(fmt.Sprintf("\ts.%s.cajaSetShared()\n", fieldName))
			}
			buf.WriteString(fmt.Sprintf(`
func (s *%s) cajaSetShared() {
	if s == nil {
		return
	}
	s.cajaShared = true
%s}
func (s *%s) cajaClone() *%s {
	if s == nil {
		return nil
	}
	clone := *s
	clone.cajaShared = false
	return &clone
}
`, receiverType, cascade.String(), receiverType, receiverType))

			return buf.String(), nil
		}
		if s.TargetType != "" || s.Signature != nil {
			sym, ok := a.GetSymbol(s)
			if ok {
				goType := ctx.mapSymbolToGoType(sym)
				if goType != "" && goType != "any" {
					return fmt.Sprintf("type %s %s", prefixIdentifier(ctx, s.Name.Value), goType), nil
				}
			}
		}
		return fmt.Sprintf("// type %s ...", prefixIdentifier(ctx, s.Name.Value)), nil

	case *ast.UnionStatement:
		var buf bytes.Buffer
		unionName := prefixIdentifier(ctx, s.Name.Value)
		buf.WriteString(fmt.Sprintf("type %s interface{ is%s() }\n", unionName, unionName))
		sym, ok := a.GetSymbol(s)
		if ok {
			if unionSym, isUnion := sym.(*symbol.UnionSymbol); isUnion {
				for _, variantIdent := range s.Variants {
					variantDef, exists := unionSym.Variants[variantIdent.Value]
					if !exists {
						continue
					}
					// Use the same prefixing prefixIdentifier applies to the
					// variant's own `type X struct {...}` declaration (not
					// mapSymbolToGoType's cross-file-relative prefixing,
					// which is wrong here: within a module's own transpile
					// pass, a struct declared in that same module is never
					// "foreign" relative to itself, so it would come back
					// unprefixed and the method would land on a Go type
					// name that's never actually declared).
					variantGoType := "*" + prefixIdentifier(ctx, variantDef.Name)
					buf.WriteString(fmt.Sprintf("func (%s) is%s() {}\n", variantGoType, unionName))
				}
			}
		}
		return buf.String(), nil

	default:
		// Fallback for unsupported statements
		return fmt.Sprintf("// unsupported statement: %T", stmt), nil
	}
}

func transpileExpression(expr ast.Expression, ctx *transpileContext, expectedType string, expectedSym ...symbol.Symbol) (string, error) {
	val, err := transpileExpressionInternal(expr, ctx, expectedType)
	if err != nil {
		return "", err
	}
	if len(expectedSym) > 0 && expectedSym[0] != nil {
		exprSym, _ := ctx.analyzer.GetSymbol(expr)
		if exprSym != nil {
			if nullSym, ok := expectedSym[0].(*symbol.NullableSymbol); ok {
				if constraint, ok := nullSym.Underlying.(*symbol.ConstraintSymbol); ok {
					if constraint.BaseType.Equals(exprSym) {
						return fmt.Sprintf("validate_%s(%s)", constraint.Name, val), nil
					}
				}
			}
		}
	}
	return val, nil
}

// reactiveCallInfo reports whether expr is a call expression with at least
// one `react`-marked argument, and if so returns the call and the
// identifiers of each react-marked dependency (in argument order) — used
// by transpileStatement's *ast.LetStatement case to route to
// transpileReactiveCallExpression instead of ordinary call codegen.
func reactiveCallInfo(expr ast.Expression) (*ast.CallExpression, []*ast.Identifier, bool) {
	callExpr, ok := expr.(*ast.CallExpression)
	if !ok {
		return nil, nil, false
	}
	var deps []*ast.Identifier
	for _, arg := range callExpr.Arguments {
		if prefix, ok := arg.(*ast.PrefixExpression); ok && prefix.Operator == "react" {
			if ident, ok := prefix.Right.(*ast.Identifier); ok {
				deps = append(deps, ident)
			}
		}
	}
	if len(deps) == 0 {
		return nil, nil, false
	}
	return callExpr, deps, true
}

// transpileReactiveCallExpression compiles a call with one or more
// react-marked arguments into an IIFE that computes the initial value,
// registers the result cell as a dependent of each dependency (via
// addDependent), and returns the cell. A dependency's own eventual Set call
// triggers cajaPropagate, which recomputes this cell (via cajaRecompute,
// calling the recompute closure defined here) in topological order relative
// to every other transitively-affected cell — not immediately/inline the
// way a plain per-edge subscriber callback would, which is what let a
// shared downstream consumer of two changed dependencies (a "diamond")
// observe a torn intermediate state and recompute twice. See cajaPropagate
// and cajaActive.Set's doc comments (builtins.go) for the full design and
// the verified reason goroutine-based propagation was rejected instead.
//
// A react-marked argument that is itself a derived active value (chained
// reactivity, e.g. `d = h(react b)` where `b = f(react a)`) is fully
// supported, not rejected: callExpr is transpiled through the ordinary
// call-codegen path, which already auto-unwraps any active identifier
// (including a derived one) via .Get() — chaining requires no special
// handling here at all, only cajaPropagate's topological ordering to be
// correct once fan-in is involved.
func transpileReactiveCallExpression(callExpr *ast.CallExpression, deps []*ast.Identifier, resultSym *symbol.ActiveSymbol, ctx *transpileContext) (string, error) {
	ctx.usedModules["active_cell"] = true
	resultGoType := ctx.mapSymbolToGoType(resultSym.Underlying)

	callStr, err := transpileExpression(callExpr, ctx, "")
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("func() *cajaActive[%s] {\n", resultGoType))
	buf.WriteString(fmt.Sprintf("cell := newCajaActive(%s)\n", callStr))
	buf.WriteString(fmt.Sprintf("cell.recompute = func() { cell.cajaSetFromRecompute(%s) }\n", callStr))
	for _, dep := range deps {
		buf.WriteString(fmt.Sprintf("%s.addDependent(cell)\n", resolveIdentifierGoName(dep, ctx)))
	}
	buf.WriteString("return cell\n}()")
	return buf.String(), nil
}

// resolveIdentifierGoName resolves an Identifier to its bare (possibly
// cross-module-prefixed) Go variable name, with no active-unwrap applied —
// the raw name a declaration/assignment target or a `react` operand needs.
// Ordinary reads go through transpileExpressionInternal's *ast.Identifier
// case instead, which calls this and then conditionally appends .Get().
func resolveIdentifierGoName(e *ast.Identifier, ctx *transpileContext) string {
	a := ctx.analyzer
	_, filePath, ok := a.GetDefinition(e)
	// Deliberately compared against a.GlobalEnv().FileName here, NOT the
	// stable ctx.topLevelFileName used for struct/union symbol names
	// elsewhere: entry.FilePath (see analyzer/scope.go's declare/
	// declareImport) is set to "whichever file's analyzer processed this
	// declaration" for EVERY binding, local params/lets included — there's
	// no such thing as a "local struct type" the way there is a local
	// variable, so the struct-symbol case can safely treat "not the
	// top-level script" as "always prefixed". Here, comparing against
	// ctx.topLevelFileName would wrongly module-prefix a perfectly
	// ordinary function parameter or local `let` just because the function
	// enclosing it happens to live inside an imported module's file —
	// a.GlobalEnv().FileName instead reflects "the file whose code is
	// CURRENTLY being transpiled" (ctx.analyzer is swapped per-module
	// during that loop), which is what a local binding needs to be
	// compared against to correctly stay unprefixed while its own
	// enclosing module's code is what's being generated, and only get
	// prefixed when referenced from elsewhere.
	if ok && filePath != "" && filePath != a.GlobalEnv().FileName {
		// Identifier was defined in another module
		return sanitizeIdentifier(filePath) + "_" + e.Value
	}
	// If it's a global variable defined in the current module being transpiled (not main)
	if ctx.CurrentModulePath != "" {
		_, isGlobal := a.GlobalScope()[e.Value]
		_, filePath, ok := a.GetDefinition(e)
		if isGlobal && ok && filePath == ctx.CurrentModulePath {
			return sanitizeIdentifier(ctx.CurrentModulePath) + "_" + e.Value
		}
	}
	return e.Value
}

func transpileExpressionInternal(expr ast.Expression, ctx *transpileContext, expectedType string) (string, error) {
	if expr == nil {
		return "", nil
	}
	a := ctx.analyzer
	switch e := expr.(type) {
	case *ast.Identifier:
		importedMod := a.GetImportedModule(e)
		if importedMod != "" && builtinModules[importedMod] {
			return transpileBuiltinProperty(importedMod, e.Value, ctx)
		}
		name := resolveIdentifierGoName(e, ctx)
		// An ordinary read of an `active` identifier transparently unwraps
		// to its underlying value here — the one, centralized place every
		// identifier read flows through — so arithmetic, function
		// arguments, string interpolation, etc. all just work with no
		// per-call-site handling (see ActiveSymbol's doc comment). A
		// `react`-marked argument reads through this exact path too (its
		// PrefixExpression codegen just forwards here, like move does) —
		// the *dependency list* used to wire addDependent calls is extracted
		// separately, directly off the AST (see reactiveCallInfo), never
		// from this transpiled text.
		if sym, ok := a.GetSymbol(e); ok {
			if _, isActive := sym.(*symbol.ActiveSymbol); isActive {
				return name + ".Get()", nil
			}
		}
		return name, nil
	case *ast.NumberLiteral:
		return formatNumberLiteral(e.Value), nil
	case *ast.StringLiteral:
		return fmt.Sprintf("%q", e.Value), nil
	case *ast.BooleanLiteral:
		return fmt.Sprintf("%v", e.Value), nil
	case *ast.NilLiteral:
		return "nil", nil
	case *ast.DateLiteral:
		return fmt.Sprintf(`parseDate(%q)`, e.Value), nil
	case *ast.StructLiteral:
		var buf bytes.Buffer
		sym, _ := a.GetSymbol(e)
		goType := e.StructName
		if sym != nil {
			goType = strings.TrimPrefix(ctx.mapSymbolToGoType(sym), "*")
		}

		// Wrapped in an outer (...): `&Type{...}` immediately followed by a
		// selector/index (e.g. this literal used as a bare `Point{x:1}.x`
		// expression, or piped into something that appends `.Field`/`[i]`)
		// would otherwise parse as `&(Type{...}.Field)` — Go's `&` binds
		// looser than `.`/`[]` — which fails to compile since a struct
		// literal's field/element isn't addressable. The parens make `&`
		// bind to the whole literal first, matching what's actually meant.
		buf.WriteString(fmt.Sprintf("(&%s{\n", goType))
		for name, val := range e.Fields {
			valStr, err := transpileExpression(val, ctx, "")
			if err != nil {
				return "", err
			}
			valStr = maybeShareValue(val, valStr, ctx)
			exportedName := strings.ToUpper(name[:1]) + name[1:]
			buf.WriteString(fmt.Sprintf("%s: %s,\n", exportedName, valStr))
		}
		buf.WriteString("})")
		return buf.String(), nil
	case *ast.ArrayLiteral:
		sym, _ := a.GetSymbol(e)
		goType := ctx.mapSymbolToGoType(sym)
		usedFallback := false
		// An empty array literal's own symbol is ArraySymbol(Any) — a
		// non-empty but vague "*cajaArray[any]" that would otherwise never
		// let expectedType (e.g. a function's declared [Transition] return
		// type) correct it, exactly the bug already fixed for MapLiteral's
		// equivalent "*cajaMap[any, any]" case below.
		if (goType == "" || goType == "*cajaArray[any]") && expectedType != "" {
			goType = expectedType
			usedFallback = true
		}
		if goType == "" {
			goType = "*cajaArray[any]"
		}
		ctx.usedModules["cow_array"] = true
		wrapperType := strings.TrimPrefix(goType, "*")
		elemType := "any"
		if arrSym, ok := sym.(*symbol.ArraySymbol); ok && !usedFallback {
			if et := ctx.mapSymbolToGoType(arrSym.ElementSymbol()); et != "" {
				elemType = et
			}
		} else if args := splitGenericTypeArgs(wrapperType); len(args) == 1 {
			elemType = args[0]
		}
		var elements []string
		for _, el := range e.Elements {
			elStr, err := transpileExpression(el, ctx, "")
			if err != nil {
				return "", err
			}
			elStr = maybeShareValue(el, elStr, ctx)
			elements = append(elements, elStr)
		}
		// Parenthesized for the same reason StructLiteral's is — see its comment.
		return fmt.Sprintf("(&%s{Data: []%s{%s}})", wrapperType, elemType, strings.Join(elements, ", ")), nil
	case *ast.MapLiteral:
		sym, _ := a.GetSymbol(e)
		goType := ctx.mapSymbolToGoType(sym)
		usedFallback := false
		if goType == "" || goType == "*cajaMap[any, any]" {
			if expectedType != "" {
				goType = expectedType
				usedFallback = true
			} else {
				goType = "*cajaMap[any, any]"
			}
		}
		ctx.usedModules["cow_map"] = true
		wrapperType := strings.TrimPrefix(goType, "*")
		kType, vType := "any", "any"
		// When goType came from expectedType (sym's own key/value types were
		// empty or vaguely "any, any" — that's exactly why the fallback
		// fired), extract kType/vType from the corrected wrapperType instead
		// of re-deriving from that same unhelpful sym.
		if mapSym, ok := sym.(*symbol.MapSymbol); ok && !usedFallback {
			if kt := ctx.mapSymbolToGoType(mapSym.Key); kt != "" {
				kType = kt
				if strings.HasPrefix(kType, "*") {
					kType = "string"
				}
			}
			if vt := ctx.mapSymbolToGoType(mapSym.Value); vt != "" {
				vType = vt
			}
		} else if args := splitGenericTypeArgs(wrapperType); len(args) == 2 {
			kType, vType = args[0], args[1]
		}
		var pairs []string
		for k, v := range e.Pairs {
			kStr, err := transpileExpression(k, ctx, "")
			if err != nil {
				return "", err
			}
			if kSym, _ := a.GetSymbol(k); isStructKeySymbol(kSym) {
				kStr += ".Key()"
			}

			vStr, err := transpileExpression(v, ctx, "")
			if err != nil {
				return "", err
			}
			vStr = maybeShareValue(v, vStr, ctx)
			pairs = append(pairs, fmt.Sprintf("%s: %s", kStr, vStr))
		}
		// Parenthesized for the same reason StructLiteral's is — see its comment.
		return fmt.Sprintf("(&%s{Data: map[%s]%s{%s}})", wrapperType, kType, vType, strings.Join(pairs, ", ")), nil
	case *ast.IndexExpression:
		left, err := transpileExpression(e.Left, ctx, "")
		if err != nil {
			return "", err
		}
		index, err := transpileExpression(e.Index, ctx, "")
		if err != nil {
			return "", err
		}
		leftSym, _ := a.GetSymbol(e.Left)
		if _, isArr := leftSym.(*symbol.ArraySymbol); isArr {
			return fmt.Sprintf("%s.Data[int(%s)]", left, index), nil
		}

		if _, isMap := leftSym.(*symbol.MapSymbol); !isMap {
			// analyzeIndexExpression only lets an ArraySymbol, a MapSymbol, or
			// ANY_OBJ reach here -- so a leftSym that's neither of the first two
			// is ANY_OBJ, meaning its concrete shape (map vs array, or neither)
			// is only known at runtime. This happens once a value has passed
			// through one level of a Map<_, Any>/parsed-JSON-style container:
			// the container itself is a concrete MapSymbol, but each value
			// pulled out of it widens to Any, losing the "it's actually a map"
			// static information a second level of indexing would need. Emit a
			// runtime type-switch instead of assuming .Data exists on whatever
			// Go type this expression happens to have -- it won't, for a bare
			// `any`, and this is exactly how a nested JSON object/array read
			// via http.parseJSON is reached (parsed["a"]["b"]).
			ctx.usedModules["dynamic_index"] = true
			ctx.usedModules["cow_map"] = true
			ctx.usedModules["cow_array"] = true
			return fmt.Sprintf("caja_dynamic_index(%s, %s)", left, index), nil
		}

		if indexSym, _ := a.GetSymbol(e.Index); isStructKeySymbol(indexSym) {
			index += ".Key()"
		}

		return fmt.Sprintf("%s.Data[%s]", left, index), nil
	case *ast.PrefixExpression:
		right, err := transpileExpression(e.Right, ctx, "")
		if err != nil {
			return "", err
		}
		if e.Operator == "move" || e.Operator == "react" {
			// Go doesn't have move; just return the underlying identifier.
			// react's operand already goes through the ordinary Identifier
			// codegen above, which auto-appends .Get() for an active
			// identifier — exactly what's needed here, since this text
			// becomes an ordinary call argument (e.g. reactFn(counter.Get())).
			// The *dependency list* used to wire addDependent calls is
			// extracted separately, directly off the AST (see
			// reactiveCallInfo), not from this codegen path.
			return right, nil
		}
		if e.Operator == "!" {
			return fmt.Sprintf("(!%s)", right), nil
		}
		return fmt.Sprintf("(%s%s)", e.Operator, right), nil
	case *ast.IfExpression:
		cond, err := transpileExpression(e.Condition, ctx, "")
		if err != nil {
			return "", err
		}
		consequence, err := transpileStatement(e.Consequence, ctx)
		if err != nil {
			return "", err
		}
		res := fmt.Sprintf("if %s %s", cond, consequence)
		if e.Alternative != nil {
			alt, err := transpileStatement(e.Alternative, ctx)
			if err != nil {
				return "", err
			}
			res += fmt.Sprintf(" else %s", alt)
		}
		return res, nil
	case *ast.InfixExpression:
		left, err := transpileExpression(e.Left, ctx, "")
		if err != nil {
			return "", err
		}
		right, err := transpileExpression(e.Right, ctx, "")
		if err != nil {
			return "", err
		}
		if e.Operator == "%" {
			ctx.usedModules["math"] = true
			return fmt.Sprintf("math.Mod(%s, %s)", left, right), nil
		}
		if e.Operator == "^" {
			ctx.usedModules["math"] = true
			return fmt.Sprintf("math.Pow(%s, %s)", left, right), nil
		}
		return fmt.Sprintf("(%s %s %s)", left, e.Operator, right), nil
	case *ast.IsExpression:
		left, err := transpileExpression(e.Left, ctx, "")
		if err != nil {
			return "", err
		}

		variantGoType := "any"
		if sym, ok := a.GetSymbol(e); ok {
			if nullSym, isNullable := sym.(*symbol.NullableSymbol); isNullable {
				variantGoType = ctx.mapSymbolToGoType(nullSym.Underlying)
			}
		}

		return fmt.Sprintf("func() %s { if v, ok := (%s).(%s); ok { return v }; return nil }()", variantGoType, left, variantGoType), nil
	case *ast.SafePipeExpression:
		leftExpr, err := transpileExpressionInternal(e.Left, ctx, "")
		if err != nil {
			return "", err
		}

		leftSym, _ := ctx.analyzer.GetSymbol(e.Left)
		leftGoType := ctx.mapSymbolToGoType(leftSym)
		if leftGoType == "" {
			leftGoType = "any"
		}

		resultSym, _ := ctx.analyzer.GetSymbol(e)
		resultGoType := ctx.mapSymbolToGoType(resultSym)
		if resultGoType == "" {
			resultGoType = "any"
		}

		needsDeref := false
		if nullableSym, ok := leftSym.(*symbol.NullableSymbol); ok {
			underlyingType := ctx.mapSymbolToGoType(nullableSym.Underlying)
			if !(strings.HasPrefix(underlyingType, "*") || strings.HasPrefix(underlyingType, "map[") ||
				strings.HasPrefix(underlyingType, "[]") || strings.HasPrefix(underlyingType, "func(") ||
				underlyingType == "any") {
				needsDeref = true
			}
		}

		valName := "_val"
		if needsDeref {
			valName = "*_val"
		}

		originalArg := e.Call.Arguments[0]
		e.Call.Arguments[0] = &ast.Identifier{Value: valName}

		callExpr, err := transpileExpressionInternal(e.Call, ctx, "")

		e.Call.Arguments[0] = originalArg

		if err != nil {
			return "", err
		}

		return fmt.Sprintf("func(_val %s) %s { if _val != nil { return %s }; return nil }(%s)", leftGoType, resultGoType, callExpr, leftExpr), nil
	case *ast.StreamPipeExpression:
		return transpileStreamPipeExpression(e, ctx)
	case *ast.AsyncExpression:
		return transpileAsyncExpression(e, ctx)
	case *ast.UnwrapExpression:
		return transpileUnwrapExpression(e, ctx)
	case *ast.CallExpression:
		if prop, ok := e.Function.(*ast.PropertyExpression); ok {
			objSym, _ := a.GetSymbol(prop.Object)
			if modSym, isMod := objSym.(*symbol.ModuleSymbol); isMod && builtinModules[modSym.FilePath] {
				return transpileBuiltinCall(modSym.FilePath, prop.Property.Value, e.Arguments, ctx)
			}
		} else if ident, ok := e.Function.(*ast.Identifier); ok {
			importedMod := a.GetImportedModule(ident)
			if importedMod != "" && builtinModules[importedMod] {
				return transpileBuiltinCall(importedMod, ident.Value, e.Arguments, ctx)
			}
		}

		fn, err := transpileExpression(e.Function, ctx, "")
		if err != nil {
			return "", err
		}
		if fn == "print" {
			fn = "fmt.Println"
		}

		if len(e.TypeArguments) > 0 {
			var typeArgsGo []string
			for _, t := range e.TypeArguments {
				if sym, ok := ctx.analyzer.GetGlobalType(t); ok {
					typeArgsGo = append(typeArgsGo, ctx.mapSymbolToGoType(sym))
				} else {
					typeArgsGo = append(typeArgsGo, "any")
				}
			}
			fn = fmt.Sprintf("%s[%s]", fn, strings.Join(typeArgsGo, ", "))
		}

		args := []string{}
		for _, arg := range e.Arguments {
			argStr, err := transpileExpression(arg, ctx, "")
			if err != nil {
				return "", err
			}
			argStr = maybeShareValue(arg, argStr, ctx)
			args = append(args, argStr)
		}
		return fmt.Sprintf("%s(%s)", fn, strings.Join(args, ", ")), nil
	case *ast.PropertyExpression:
		objSym, _ := a.GetSymbol(e.Object)
		if modSym, ok := objSym.(*symbol.ModuleSymbol); ok {
			if builtinModules[modSym.Name()] {
				if ident, ok := e.Object.(*ast.Identifier); ok {
					return transpileBuiltinProperty(ident.Value, e.Property.Value, ctx)
				}
			} else if modSym.FilePath != "" {
				return sanitizeIdentifier(modSym.FilePath) + "_" + e.Property.Value, nil
			}
		}

		obj, err := transpileExpression(e.Object, ctx, "")
		if err != nil {
			return "", err
		}
		prop := strings.ToUpper(e.Property.Value[:1]) + e.Property.Value[1:]

		if e.Safe {
			objSym, _ := a.GetSymbol(e.Object)
			objType := ctx.mapSymbolToGoType(objSym)
			if objType == "" {
				objType = "any"
			}

			propSym, _ := a.GetSymbol(e)
			propType := ctx.mapSymbolToGoType(propSym)
			if propType == "" {
				propType = "any"
			}

			// We need to know if the actual struct field is a primitive value type so we can take its address.
			fieldIsValueType := false
			if nullableObj, isNullable := objSym.(*symbol.NullableSymbol); isNullable {
				objSym = nullableObj.Underlying
			}
			var structDef *symbol.StructDefSymbol
			if inst, isInst := objSym.(*symbol.StructInstanceSymbol); isInst {
				structDef = inst.Def
			} else if def, isDef := objSym.(*symbol.StructDefSymbol); isDef {
				structDef = def
			}

			if structDef != nil {
				if fieldSym, exists := structDef.Fields[e.Property.Value]; exists {
					fieldGoType := ctx.mapSymbolToGoType(fieldSym.Type)
					if fieldGoType == "float64" || fieldGoType == "bool" || fieldGoType == "string" {
						fieldIsValueType = true
					}
				}
			}

			if !fieldIsValueType {
				return fmt.Sprintf(`func(obj %s) %s { if obj != nil { return obj.%s }; return nil }(%s)`, objType, propType, prop, obj), nil
			} else {
				return fmt.Sprintf(`func(obj %s) %s { if obj != nil { v := obj.%s; return &v }; return nil }(%s)`, objType, propType, prop, obj), nil
			}
		}

		return fmt.Sprintf("%s.%s", obj, prop), nil
	case *ast.FunctionLiteral:
		sym, _ := a.GetSymbol(e)
		fnSym, _ := sym.(*symbol.FunctionSymbol)
		var params []string
		var paramNames []string
		for i, param := range e.Parameters {
			pt := "any"
			if fnSym != nil && i < len(fnSym.ParamTypes()) {
				t := ctx.mapSymbolToGoType(fnSym.ParamTypes()[i])
				if t != "" {
					pt = t
				}
			}
			params = append(params, fmt.Sprintf("%s %s", param.Name, pt))
			paramNames = append(paramNames, param.Name)
		}
		retType := ""
		if fnSym != nil && !isNothingReturnType(fnSym.ReturnType()) {
			retType = ctx.mapSymbolToGoType(fnSym.ReturnType())
		}

		fnCtx := &transpileContext{
			analyzer:                  a,
			currentFuncName:           "",
			currentFunctionParams:     paramNames,
			currentFunctionReturnType: retType,
			hasTailCall:               false,
			usedModules:               ctx.usedModules,
			CurrentModulePath:         ctx.CurrentModulePath,
			CurrentSourceFile:         ctx.CurrentSourceFile,
			topLevelFileName:          ctx.topLevelFileName,
			inFunction:            true,
		}
		if fnSym != nil {
			fnCtx.currentFuncName = fnSym.Name
		}
		if ctx.currentFunctionBody != nil {
			fnCtx.currentFunctionBody = ctx.currentFunctionBody
		} else {
			fnCtx.currentFunctionBody = e.Body
		}
		if fnCtx.currentFuncName != "" {
			fnCtx.inLoop = functionBodyHasSelfTailCall(e.Body, fnCtx.currentFuncName)
		}

		body, err := transpileStatement(e.Body, fnCtx)
		if err != nil {
			return "", err
		}

		if fnCtx.hasTailCall {
			body = fmt.Sprintf("{\nfor %s\n}", body)
		}

		goFunc := fmt.Sprintf("func(%s)", strings.Join(params, ", "))

		isGeneric := fnSym != nil && len(fnSym.TypeParameters) > 0
		if isGeneric {
			var typeParamsGo []string
			for _, tp := range fnSym.TypeParameters {
				typeParamsGo = append(typeParamsGo, fmt.Sprintf("%s comparable", tp))
			}
			goFunc = fmt.Sprintf("func %s[%s](%s)", prefixIdentifier(ctx, fnSym.Name), strings.Join(typeParamsGo, ", "), strings.Join(params, ", "))
		}

		if retType != "" {
			goFunc += " " + retType
		}

		if isGeneric {
			if ctx.packageLevelCode != nil {
				ctx.packageLevelCode.WriteString(fmt.Sprintf("%s %s\n", goFunc, body))
			}
			return "", nil
		}

		return fmt.Sprintf("%s %s", goFunc, body), nil
	default:
		// Fallback for unsupported expressions
		return fmt.Sprintf("UnsupportedExpr(%T)", expr), nil
	}
}

// streamPipeCounter generates collision-free variable names for the
// goroutine/channel plumbing emitted per stream pipeline expression. It is
// package-level (rather than carried on transpileContext) because nested
// transpileContexts built for function literals (see the *ast.FunctionLiteral
// case above) do not copy every context field into child contexts, so a
// context-carried counter could silently diverge across sibling closures.
var streamPipeCounter int

// transpileStreamPipeExpression generates a Go IIFE implementing a
// channel-based concurrency pipeline (generator -> N single-worker stages ->
// sink) for a |>>/?>> chain, following the classic "Concurrency in Go"
// pattern. Each stage is exactly one goroutine reading its in-channel in
// write order and writing to its out-channel, so ordering is preserved
// end-to-end without any fan-out/fan-in bookkeeping, while still letting item
// N move on to the next stage before item N+1 finishes the previous one.
func transpileStreamPipeExpression(e *ast.StreamPipeExpression, ctx *transpileContext) (string, error) {
	a := ctx.analyzer
	ctx.usedModules["async_panic_guard"] = true
	ctx.usedModules["sync"] = true

	// Flatten the nested chain (e.Left may itself be a *StreamPipeExpression)
	// into source-to-sink order.
	var stages []*ast.StreamPipeExpression
	for cur := e; ; {
		stages = append(stages, cur)
		next, ok := cur.Left.(*ast.StreamPipeExpression)
		if !ok {
			break
		}
		cur = next
	}
	for i, j := 0, len(stages)-1; i < j; i, j = i+1, j-1 {
		stages[i], stages[j] = stages[j], stages[i]
	}

	sourceExpr := stages[0].Left
	sourceStr, err := transpileExpressionInternal(sourceExpr, ctx, "")
	if err != nil {
		return "", err
	}

	var elemSym symbol.Symbol
	if sourceSym, ok := a.GetSymbol(sourceExpr); ok {
		if arrSym, ok := sourceSym.(*symbol.ArraySymbol); ok {
			elemSym = arrSym.ElementSymbol()
		}
	}
	elemType := ctx.mapSymbolToGoType(elemSym)
	if elemType == "" {
		elemType = "any"
	}

	resultSym, _ := a.GetSymbol(e)
	resultGoType := ctx.mapSymbolToGoType(resultSym)
	if resultGoType == "" {
		resultGoType = "any"
	}

	id := streamPipeCounter
	streamPipeCounter++
	prefix := fmt.Sprintf("_stream%d", id)

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("func() %s {\n", resultGoType))
	buf.WriteString(fmt.Sprintf("%s_done := make(chan struct{})\n", prefix))
	buf.WriteString(fmt.Sprintf("defer close(%s_done)\n\n", prefix))

	buf.WriteString(fmt.Sprintf("%s_ch0 := make(chan %s)\n", prefix, elemType))
	buf.WriteString("go func() {\n")
	buf.WriteString(fmt.Sprintf("defer close(%s_ch0)\n", prefix))
	buf.WriteString("defer func() {\nif r := recover(); r != nil {\ncaja_report_async_panic(r)\n}\n}()\n")
	buf.WriteString(fmt.Sprintf("for _, v := range %s.Data {\n", sourceStr))
	buf.WriteString("select {\n")
	buf.WriteString(fmt.Sprintf("case %s_ch0 <- v:\n", prefix))
	buf.WriteString(fmt.Sprintf("case <-%s_done:\n", prefix))
	buf.WriteString("return\n")
	buf.WriteString("}\n}\n}()\n\n")

	prevCh := prefix + "_ch0"
	finalElemType := elemType
	for i, stage := range stages {
		var outElemSym symbol.Symbol
		if sym, ok := a.GetStreamStageType(stage); ok {
			outElemSym = sym
		}
		outElemType := ctx.mapSymbolToGoType(outElemSym)
		if outElemType == "" {
			outElemType = "any"
		}
		finalElemType = outElemType
		outCh := fmt.Sprintf("%s_ch%d", prefix, i+1)

		var stageBody bytes.Buffer
		if stage.Join != nil {
			if err := writeJoinStageBody(&stageBody, stage, ctx, prefix, i); err != nil {
				return "", err
			}
		} else {
			originalArg := stage.Call.Arguments[0]
			stage.Call.Arguments[0] = &ast.Identifier{Value: "v"}
			callExpr, err := transpileExpressionInternal(stage.Call, ctx, "")
			stage.Call.Arguments[0] = originalArg
			if err != nil {
				return "", err
			}
			stageBody.WriteString(fmt.Sprintf("out := %s\n", callExpr))
		}

		buf.WriteString(fmt.Sprintf("%s := make(chan %s)\n", outCh, outElemType))
		buf.WriteString("go func() {\n")
		buf.WriteString(fmt.Sprintf("defer close(%s)\n", outCh))
		buf.WriteString("defer func() {\nif r := recover(); r != nil {\ncaja_report_async_panic(r)\n}\n}()\n")
		buf.WriteString(fmt.Sprintf("for v := range %s {\n", prevCh))
		if stage.Safe {
			buf.WriteString("if v == nil {\ncontinue\n}\n")
		}
		buf.Write(stageBody.Bytes())
		buf.WriteString("select {\n")
		buf.WriteString(fmt.Sprintf("case %s <- out:\n", outCh))
		buf.WriteString(fmt.Sprintf("case <-%s_done:\n", prefix))
		buf.WriteString("return\n")
		buf.WriteString("}\n}\n}()\n\n")

		prevCh = outCh
	}

	buf.WriteString(fmt.Sprintf("%s_out := &cajaArray[%s]{}\n", prefix, finalElemType))
	buf.WriteString(fmt.Sprintf("for v := range %s {\n", prevCh))
	buf.WriteString(fmt.Sprintf("%s_out.Data = append(%s_out.Data, v)\n", prefix, prefix))
	buf.WriteString("}\n")
	buf.WriteString("caja_check_async_panic()\n")
	buf.WriteString(fmt.Sprintf("return %s_out\n", prefix))
	buf.WriteString("}()")

	return pinRangeToLine(buf.String(), ctx.CurrentSourceFile, e.Token.Line), nil
}

// writeJoinStageBody generates the per-item body of a fixed-size parallel
// join stage: each of stage.Join.Calls is invoked concurrently (one
// goroutine per call, joined via a sync.WaitGroup — not a worker pool, since
// the count is fixed and known at compile time), then stage.Call is invoked
// with the N results substituted into its reserved leading argument slots
// (see parseStreamPipeExpressionCommon's fusion logic and
// analyzeJoinStage), plus whatever extra curried arguments follow them.
func writeJoinStageBody(buf *bytes.Buffer, stage *ast.StreamPipeExpression, ctx *transpileContext, prefix string, stageIdx int) error {
	a := ctx.analyzer
	n := len(stage.Join.Calls)
	joinVarNames := make([]string, n)

	for j, joinCall := range stage.Join.Calls {
		joinVarNames[j] = fmt.Sprintf("%s_stage%d_join%d", prefix, stageIdx, j)

		var joinElemSym symbol.Symbol
		if sym, ok := a.GetSymbol(joinCall); ok {
			joinElemSym = sym
		}
		joinElemType := ctx.mapSymbolToGoType(joinElemSym)
		if joinElemType == "" {
			joinElemType = "any"
		}
		buf.WriteString(fmt.Sprintf("var %s %s\n", joinVarNames[j], joinElemType))
	}

	wgName := fmt.Sprintf("%s_stage%d_wg", prefix, stageIdx)
	buf.WriteString(fmt.Sprintf("var %s sync.WaitGroup\n", wgName))
	buf.WriteString(fmt.Sprintf("%s.Add(%d)\n", wgName, n))

	for j, joinCall := range stage.Join.Calls {
		originalArg := joinCall.Arguments[0]
		joinCall.Arguments[0] = &ast.Identifier{Value: "v"}
		callExpr, err := transpileExpressionInternal(joinCall, ctx, "")
		joinCall.Arguments[0] = originalArg
		if err != nil {
			return err
		}
		buf.WriteString(fmt.Sprintf("go func() {\ndefer %s.Done()\ndefer func() {\nif r := recover(); r != nil {\ncaja_report_async_panic(r)\n}\n}()\n%s = %s\n}()\n", wgName, joinVarNames[j], callExpr))
	}
	buf.WriteString(fmt.Sprintf("%s.Wait()\n", wgName))

	for j, name := range joinVarNames {
		stage.Call.Arguments[j] = &ast.Identifier{Value: name}
	}
	callExpr, err := transpileExpressionInternal(stage.Call, ctx, "")
	for j := range joinVarNames {
		stage.Call.Arguments[j] = nil
	}
	if err != nil {
		return err
	}
	buf.WriteString(fmt.Sprintf("out := %s\n", callExpr))

	return nil
}

// transpileAsyncExpression compiles `async <expr>` to an IIFE that starts a
// goroutine immediately (eager start) and returns a pointer to a shared
// asyncTask. Completion is signaled by closing the `done` channel rather
// than sending the value over it, because closing (unlike a single-value
// send) can be observed by any number of receives — required so that a
// binding can be awaited more than once (e.g. once inside a
// `await p1 & p2 & p3` barrier and again individually afterward). Go's
// memory model guarantees a receive that observes the close happens-after
// the write to `val` that preceded it, so no extra locking is needed.
func transpileAsyncExpression(node *ast.AsyncExpression, ctx *transpileContext) (string, error) {
	ctx.usesAsync = true
	ctx.usedModules["async_panic_guard"] = true
	ctx.usedModules["sync"] = true

	rightExpr, err := transpileExpression(node.Right, ctx, "")
	if err != nil {
		return "", err
	}

	// close(t.done) must be deferred (not a plain trailing statement) so a
	// panic in the task body still wakes up any unwrap/await waiting on it
	// instead of leaving them blocked forever; the recover-defer is
	// registered after it so the panic is recorded before that close fires.
	code := fmt.Sprintf(
		"func() *asyncTask {\nt := &asyncTask{done: make(chan struct{})}\ngo func() {\ndefer close(t.done)\ndefer func() {\nif r := recover(); r != nil {\ncaja_report_async_panic(r)\n}\n}()\nt.val = %s\n}()\nreturn t\n}()",
		rightExpr,
	)
	return pinRangeToLine(code, ctx.CurrentSourceFile, node.Token.Line), nil
}

// transpileUnwrapExpression compiles `unwrap <expr>` to blocking on the
// task's completion signal and type-asserting the cached value back to its
// known element type. This is the only construct that extracts a value out
// of an async handle — `await` is a pure synchronization barrier and never
// does (see transpileAwaitStatement). Safe to call any number of times on
// the same task, since closing `done` is repeatable.
func transpileUnwrapExpression(node *ast.UnwrapExpression, ctx *transpileContext) (string, error) {
	ctx.usesAsync = true
	ctx.usedModules["async_panic_guard"] = true
	ctx.usedModules["sync"] = true

	rightExpr, err := transpileExpressionInternal(node.Right, ctx, "")
	if err != nil {
		return "", err
	}

	elemType := "any"
	if sym, ok := ctx.analyzer.GetSymbol(node); ok {
		if t := ctx.mapSymbolToGoType(sym); t != "" {
			elemType = t
		}
	}

	// caja_check_async_panic runs before the type assertion: if the async
	// task's body panicked, .val is still its zero value, and asserting that
	// into elemType would panic again with a confusing generic "interface
	// conversion" error that masks the real one.
	code := fmt.Sprintf(
		"func() %s {\n<-(%s).done\ncaja_check_async_panic()\nreturn (%s).val.(%s)\n}()",
		elemType, rightExpr, rightExpr, elemType,
	)
	return pinRangeToLine(code, ctx.CurrentSourceFile, node.Token.Line), nil
}

// transpileAwaitStatement compiles the WaitGroup-style join barrier
// `await p1 & p2 & ... & pn` to sequential blocking waits on each task's
// completion signal, values discarded (this form produces no result — see
// ast.AwaitStatement). No sync.WaitGroup is needed: every async binding
// already started its own goroutine at creation (eager start), so the
// sequential waits achieve the same result.
func transpileAwaitStatement(node *ast.AwaitStatement, ctx *transpileContext) (string, error) {
	ctx.usesAsync = true
	ctx.usedModules["async_panic_guard"] = true
	ctx.usedModules["sync"] = true

	var buf bytes.Buffer
	for i, p := range node.Pipelines {
		pExpr, err := transpileExpressionInternal(p, ctx, "")
		if err != nil {
			return "", err
		}
		if i > 0 {
			buf.WriteString("\n")
		}
		buf.WriteString(fmt.Sprintf("<-(%s).done", pExpr))
	}
	// Checked once, after every pipeline in the barrier has signaled
	// completion (not per-pipeline) — must wait for all of them before
	// deciding whether to abort.
	buf.WriteString("\ncaja_check_async_panic()")
	return buf.String(), nil
}

func getOrderedModules(p *ast.Program, asts map[string]*ast.Program) []string {
	var ordered []string
	visited := make(map[string]bool)
	var visit func(prog *ast.Program)
	visit = func(prog *ast.Program) {
		for _, stmt := range prog.Statements {
			if imp, ok := stmt.(*ast.ImportStatement); ok {
				modPath := imp.Path
				if asts[modPath] != nil && !visited[modPath] {
					visited[modPath] = true
					visit(asts[modPath])
					ordered = append(ordered, modPath)
				}
			}
		}
	}
	visit(p)
	return ordered
}

// formatNumberLiteral renders a Caja Number literal as Go source text that is
// unambiguously an untyped *float* constant, never an untyped int constant.
// Go decides a numeric literal's default kind purely syntactically (whether
// the text contains '.'/'e'/'E'), and %v alone drops the decimal point for
// whole numbers (5.0 -> "5"). That's harmless for a normal (non-generic)
// float64 parameter, since Go implicitly converts an untyped int constant to
// float64 there — but it breaks generic calls: Go infers a type parameter
// from an untyped int constant's *default* type (int), not from the
// call's surrounding context, so e.g. identity(5) against
// func identity[T comparable](x T) T infers T=int even though Caja's Number
// is always float64, producing generated Go that fails to compile.
func formatNumberLiteral(v float64) string {
	s := fmt.Sprintf("%v", v)
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	return s
}

func sanitizeIdentifier(path string) string {
	s := strings.ReplaceAll(path, "/", "_")
	s = strings.ReplaceAll(s, ".", "_")
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, "@", "_") // scoped node_modules-style paths, e.g. "@caja/query"
	return s
}

func prefixIdentifier(ctx *transpileContext, name string) string {
	if ctx.CurrentModulePath != "" {
		_, isGlobalVar := ctx.analyzer.GlobalScope()[name]
		_, isGlobalType := ctx.analyzer.GetGlobalType(name)
		if isGlobalVar || isGlobalType {
			return sanitizeIdentifier(ctx.CurrentModulePath) + "_" + name
		}
	}
	return name
}
