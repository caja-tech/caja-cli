package analyzer

import (
	"caja-cli/internal/pipeline/analyzer/symbol"
	"caja-cli/internal/pipeline/ast"
	"caja-cli/internal/pipeline/environment"
	"fmt"
	"sort"
)

// ufcsCandidate is one function eligible to resolve a UFCS call.
type ufcsCandidate struct {
	modulePath  string // "" for a same-file top-level function
	moduleAlias string // the alias it was reached through, "" when it is directly callable by name
	fn          *symbol.FunctionSymbol
}

// resolveUFCS attempts to resolve n (a property expression in call position,
// e.g. the "push" in "list.push(4)") as an extension-function / UFCS call:
// receiver.fn(args...) desugars to fn(receiver, args...), where fn can be:
//   - a function exported by any module imported in this file (builtin
//     stdlib or a real, user-written .caja module), reached either through
//     its module alias (`array.push`) or a named import (`import { push }
//     from "array"`);
//   - a plain top-level function declared directly in this file.
//
// Only *symbol.FunctionSymbol-backed functions are candidates, since only
// those carry a real per-parameter Symbol to match the receiver's type
// against (BuiltinSymbol-backed modules like date/log/time/page/browser only
// have a human-readable doc string, not a structured type; every
// user-declared function is always FunctionSymbol-backed).
//
// Returns (nil, false) when nothing matched — the caller should fall through
// to its own "property access not supported" error. Returns (sym, true) when
// resolution is final: either a unique match (sym is the matched
// FunctionSymbol) or an already-reported ambiguity (sym is AnySymbol()) —
// both cases tell the caller not to also report its generic error.
func (a *Analyzer) resolveUFCS(n *ast.PropertyExpression, receiverType symbol.Symbol) (symbol.Symbol, bool) {
	methodName := n.Property.Value
	candidates := a.ufcsCandidates(methodName, receiverType)

	switch len(candidates) {
	case 0:
		return nil, false
	case 1:
		c := candidates[0]
		a.nodeSymbols[n.Property] = c.fn
		a.ufcsMatches[n] = c.modulePath
		return c.fn, true
	default:
		a.reportUFCSAmbiguity(n, methodName, candidates)
		return symbol.AnySymbol(), true
	}
}

// ufcsCandidates collects every global-scope function whose first parameter
// type matches receiverType and that is reachable as methodName, in a
// deterministic (scope-name sorted) order, deduplicated by FunctionSymbol so
// the same function reached both through its module alias and through a named
// import counts once.
func (a *Analyzer) ufcsCandidates(methodName string, receiverType symbol.Symbol) []ufcsCandidate {
	names := make([]string, 0, len(a.scopes[0]))
	for name := range a.scopes[0] {
		names = append(names, name)
	}
	sort.Strings(names)

	var candidates []ufcsCandidate
	seen := make(map[*symbol.FunctionSymbol]bool)

	addCandidate := func(modulePath, moduleAlias string, fnSym *symbol.FunctionSymbol) {
		if len(fnSym.ParamTypes()) == 0 || seen[fnSym] {
			return
		}
		if !receiverType.Equals(fnSym.ParamTypes()[0]) {
			return
		}
		seen[fnSym] = true
		candidates = append(candidates, ufcsCandidate{modulePath: modulePath, moduleAlias: moduleAlias, fn: fnSym})
	}

	for _, name := range names {
		entry := a.scopes[0][name]

		if modSym, ok := entry.Sym.(*symbol.ModuleSymbol); ok {
			if modSym.IsPrivate(methodName) {
				continue
			}
			if propSym, ok := modSym.GetSymbol(methodName); ok {
				if fnSym, ok := propSym.(*symbol.FunctionSymbol); ok {
					addCandidate(modSym.FilePath, name, fnSym)
				}
			}
			continue
		}

		if name != methodName {
			continue
		}
		if fnSym, ok := entry.Sym.(*symbol.FunctionSymbol); ok {
			// declare() stamps FilePath with the current file's own name for
			// EVERY entry, imported or not — only declareImport's FilePath
			// actually names a distinct origin module. So a genuine local
			// declaration (IsImport false) must use "" here, not
			// entry.FilePath, or codegen would wrongly treat the function as
			// living in an external "module" (this file, sanitized) instead
			// of using its own unprefixed Go name.
			modulePath := ""
			if entry.IsImport {
				modulePath = entry.FilePath
			}
			addCandidate(modulePath, "", fnSym)
		}
	}

	return candidates
}

// reportUFCSAmbiguity reports that methodName matches more than one candidate,
// listing where each match came from and the explicit call form to use instead.
func (a *Analyzer) reportUFCSAmbiguity(n *ast.PropertyExpression, methodName string, candidates []ufcsCandidate) {
	descs := make([]string, len(candidates))
	suggestions := make([]string, len(candidates))
	for i, c := range candidates {
		descs[i], suggestions[i] = describeUFCSCandidate(c, methodName)
	}
	a.reportError(n.Token, fmt.Sprintf(
		"type error: ambiguous method call '%s': matches %s. Suggestion: call it explicitly (%s)",
		methodName, joinWithLastSep(descs, "and"), joinWithLastSep(suggestions, "or")))
}

// GetUFCSModulePath reports whether a PropertyExpression was resolved via
// UFCS (receiver.fn(...) sugar) and, if so, which module owns the matched
// function: "" for a plain top-level function declared in the same file,
// otherwise the import path of the module (builtin or real .caja) that
// exports it. Used by the compiler to route codegen to the right callee (a
// builtin module dispatcher, a real module's generated Go function, or a
// same-file top-level function) with the receiver prepended to the argument
// list.
func (a *Analyzer) GetUFCSModulePath(n *ast.PropertyExpression) (string, bool) {
	modulePath, ok := a.ufcsMatches[n]
	return modulePath, ok
}

// pendingOverload records that a struct receiver's property name is claimed
// both by one of the struct's own function-typed fields and by at least one
// UFCS candidate. analyzePropertyExpression can't settle the contest — it
// cannot see the call's arguments — so it records this and returns the field
// provisionally; analyzeCallExpression decides (resolveStructOverload).
type pendingOverload struct {
	structName string
	receiver   symbol.Symbol
	field      *symbol.FunctionSymbol
	candidates []ufcsCandidate
}

// resolveStructReceiverUFCS handles a struct receiver's property in call
// position, where a free function whose first parameter is this struct (UFCS)
// can answer the name too. fieldFn is the struct's own function-typed field of
// that name, nil when it has no such field.
//
// With no field there is no contest, so it resolves the way any other receiver
// does. With one, the winner depends on the call's arguments, which this stage
// cannot see: it records the contest for analyzeCallExpression to settle via
// resolveStructOverload and returns (nil, false), so the caller falls through
// to returning the field provisionally.
func (a *Analyzer) resolveStructReceiverUFCS(n *ast.PropertyExpression, structDef *symbol.StructDefSymbol, receiverType symbol.Symbol, fieldFn *symbol.FunctionSymbol) (symbol.Symbol, bool) {
	if fieldFn == nil {
		return a.resolveUFCS(n, receiverType)
	}
	if candidates := a.structUFCSCandidates(n.Property.Value, receiverType); len(candidates) > 0 {
		a.pendingOverloads[n] = &pendingOverload{
			structName: structDef.Name,
			receiver:   receiverType,
			field:      fieldFn,
			candidates: candidates,
		}
	}
	return nil, false
}

// structUFCSCandidates returns the UFCS candidates eligible to compete with a
// struct's own same-named function field.
//
// It drops "blanket" candidates — ones whose first parameter is an
// unconstrained generic, so its Type() is ANY_OBJ and Equals matches every
// receiver. cast.to and http.toJSON are exactly this shape, so without the
// demotion a struct with a field named `to` in any file that imports cast
// would suddenly be ambiguous. A struct's own field beats a function that
// matches everything (the "inherent member wins over a blanket impl" rule);
// a candidate naming the struct concretely still competes normally.
func (a *Analyzer) structUFCSCandidates(methodName string, receiverType symbol.Symbol) []ufcsCandidate {
	all := a.ufcsCandidates(methodName, receiverType)
	specific := make([]ufcsCandidate, 0, len(all))
	for _, c := range all {
		if c.fn.ParamTypes()[0].Type() == environment.ANY_OBJ {
			continue
		}
		specific = append(specific, c)
	}
	return specific
}

// callMatches reports whether fn would accept argSymbols with no type errors.
//
// It must stay equivalent to the per-argument checking loop in
// analyzeCallExpression, or resolveStructOverload could pick a candidate the
// checker then rejects. That loop only runs generic inference when the callee
// actually declares type parameters, which is why this does too: a struct
// field's function type never has TypeParameters, so inferring against one
// would bind a stray generic freely and claim a match that isn't there.
// inferTypes/substituteTypes are pure and report nothing, so this is
// side-effect free.
func callMatches(fn *symbol.FunctionSymbol, argSymbols []symbol.Symbol) bool {
	params := fn.ParamTypes()
	if len(argSymbols) != len(params) {
		return false
	}

	if len(fn.TypeParameters) == 0 {
		for i, param := range params {
			if argSymbols[i] == nil || !param.Equals(argSymbols[i]) {
				return false
			}
		}
		return true
	}

	inferred := make(map[string]symbol.Symbol)
	for i, param := range params {
		if argSymbols[i] == nil {
			return false
		}
		if err := inferTypes(param, argSymbols[i], inferred); err != nil {
			return false
		}
	}
	for i, param := range params {
		if !substituteTypes(param, inferred).Equals(argSymbols[i]) {
			return false
		}
	}
	return true
}

// needsExpectedType reports whether analyzing expr depends on the expected-type
// context its caller would push — which resolveStructOverload cannot supply,
// since the whole point is that the callee isn't chosen yet.
//
// Three constructs read that context: an empty array literal (to infer its
// element type), and a function literal missing parameter types or a return
// type (to infer them from the expected signature). Nested analysis does NOT
// mask the stack, so a bare [] buried inside an argument still reads the outer
// expectation — hence the recursive walk.
//
// Unenumerated node kinds deliberately return true. Over-reporting costs a
// recoverable "annotate the arguments" diagnostic; under-reporting silently
// picks the wrong overload or leaves a wrong type in nodeSymbols, and the
// analyzer has no way to roll either back.
func needsExpectedType(expr ast.Expression) bool {
	switch e := expr.(type) {
	case nil:
		return false
	case *ast.NilLiteral, *ast.NumberLiteral, *ast.StringLiteral, *ast.BooleanLiteral,
		*ast.DateLiteral, *ast.Identifier, *ast.GenericIdentifier:
		return false
	case *ast.ArrayLiteral:
		if len(e.Elements) == 0 {
			return true
		}
		return anyNeedsExpectedType(e.Elements)
	case *ast.FunctionLiteral:
		// Mirrors analyzeFunctionLiteral, which consults the expectation stack
		// for exactly these two gaps. Neither is reachable through today's
		// parser — an unannotated parameter is a syntax error, and an omitted
		// return type is filled in as "Nothing" rather than left empty — so
		// this arm currently always answers false. It is kept in lockstep with
		// that reader anyway: if either form becomes parseable, answering false
		// here would silently pick an overload off a guessed signature, which
		// is the one failure mode this whole function exists to avoid.
		if e.ReturnType == "" {
			return true
		}
		for _, p := range e.Parameters {
			if p == nil || p.Type == "" {
				return true
			}
		}
		return false
	case *ast.MapLiteral:
		for k, v := range e.Pairs {
			if needsExpectedType(k) || needsExpectedType(v) {
				return true
			}
		}
		return false
	case *ast.StructLiteral:
		for _, v := range e.Fields {
			if needsExpectedType(v) {
				return true
			}
		}
		return false
	case *ast.InfixExpression:
		return needsExpectedType(e.Left) || needsExpectedType(e.Right)
	case *ast.PrefixExpression:
		return needsExpectedType(e.Right)
	case *ast.IndexExpression:
		return needsExpectedType(e.Left) || needsExpectedType(e.Index)
	case *ast.PropertyExpression:
		return needsExpectedType(e.Object)
	case *ast.IsExpression:
		return needsExpectedType(e.Left)
	case *ast.CallExpression:
		return needsExpectedType(e.Function) || anyNeedsExpectedType(e.Arguments)
	default:
		return true
	}
}

func anyNeedsExpectedType(exprs []ast.Expression) bool {
	for _, e := range exprs {
		if needsExpectedType(e) {
			return true
		}
	}
	return false
}

// describeUFCSCandidate renders one candidate for a diagnostic: how to refer
// to it, and the explicit call form that sidesteps the ambiguity.
func describeUFCSCandidate(c ufcsCandidate, methodName string) (desc, suggestion string) {
	if c.moduleAlias == "" {
		return fmt.Sprintf("the directly-callable function '%s'", methodName),
			fmt.Sprintf("%s(...)", methodName)
	}
	return fmt.Sprintf("'%s'", c.moduleAlias),
		fmt.Sprintf("%s.%s(...)", c.moduleAlias, methodName)
}

// reportStructOverloadAmbiguity reports that methodName is claimed both by a
// struct's own function field and by UFCS candidates, in a way this call can't
// settle. detail explains why.
//
// Both escapes are offered because the field has no alternative call syntax —
// (p.f)(x) parses to the same node shape — so binding it first is the only way
// to force the field.
func (a *Analyzer) reportStructOverloadAmbiguity(n *ast.PropertyExpression, methodName, structName, detail string, candidates []ufcsCandidate) {
	descs := []string{fmt.Sprintf("the property '%s' on struct '%s'", methodName, structName)}
	suggestions := []string{}
	for _, c := range candidates {
		desc, suggestion := describeUFCSCandidate(c, methodName)
		descs = append(descs, desc)
		suggestions = append(suggestions, suggestion)
	}

	a.reportError(n.Token, fmt.Sprintf(
		"type error: ambiguous method call '%s': matches %s%s. Suggestion: call the function explicitly (%s), or bind the property first (let f = <receiver>.%s)",
		methodName, joinWithLastSep(descs, "and"), detail,
		joinWithLastSep(suggestions, "or"), methodName))
}

// resolveStructOverload settles a pendingOverload: the receiver is a struct
// whose own function-typed field shares its name with one or more UFCS
// candidates. Returns the winning function symbol, plus the argument symbols
// it had to analyze to decide (nil when it decided without looking at them,
// which is the common case).
//
// On a UFCS win it records the match so codegen routes through
// transpileUFCSCall, and restamps the property nodes so the LSP reports the
// winner rather than the field returned provisionally.
//
// Every error path returns AnySymbol() after exactly one diagnostic: the
// analyzer tests assert exact error counts and per-index ordering.
func (a *Analyzer) resolveStructOverload(n *ast.CallExpression, prop *ast.PropertyExpression, p *pendingOverload) (symbol.Symbol, []symbol.Symbol) {
	methodName := prop.Property.Value
	explicitN := len(n.Arguments)

	// Named arguments can only ever mean the field: a UFCS call rejects them
	// outright, since argument 0 is implicit and can't be named.
	if len(n.NamedArguments) > 0 {
		return p.field, nil
	}

	// Symmetrically, explicit type arguments can only mean the function: a
	// field's function type never carries TypeParameters. A pendingOverload is
	// only ever recorded with at least one candidate, so this always settles on
	// the function side — a win or an ambiguity, never a fall back to the
	// field. Should the winner turn out to be non-generic after all, the
	// ordinary checking path reports "expected 0 generic type arguments"
	// against it.
	if len(n.TypeArguments) > 0 {
		return a.decideWithoutField(prop, methodName, p.field, p.candidates), nil
	}

	viable := make([]ufcsCandidate, 0, len(p.candidates))
	for _, c := range p.candidates {
		if c.fn.Arity() == explicitN+1 {
			viable = append(viable, c)
		}
	}

	// Only the function side can fit this argument count — and if nothing
	// fits, the field still gives the precise arity error.
	if p.field.Arity() != explicitN {
		return a.decideWithoutField(prop, methodName, p.field, viable), nil
	}
	if len(viable) == 0 {
		return p.field, nil
	}

	// Both sides fit the argument count. If any candidate's declared
	// parameters (minus the receiver) are identical to the field's, no call
	// could ever tell them apart — report that directly rather than letting
	// the argument-based tiebreak below produce a vaguer message. This also
	// covers the zero-argument case, where there is nothing to discriminate on.
	for _, c := range viable {
		if sameDeclaredParams(p.field, c.fn) {
			a.reportStructOverloadAmbiguity(prop, methodName, p.structName,
				" with the same signature", []ufcsCandidate{c})
			return symbol.AnySymbol(), nil
		}
	}

	// The remaining tiebreak needs argument types, and analyzing arguments
	// here means doing it without the expected-type context (the callee is
	// precisely what's undecided). Bail out rather than guess when any
	// argument would actually depend on that context.
	if anyNeedsExpectedType(n.Arguments) {
		a.reportStructOverloadAmbiguity(prop, methodName, p.structName,
			fmt.Sprintf(", which both take %d argument(s), and this call's arguments need a known target type to choose between them", explicitN),
			viable)
		return symbol.AnySymbol(), nil
	}

	// Analyze each argument exactly once and keep the symbols locally — the
	// caller feeds them to the checking loop instead of re-analyzing, which
	// would double every diagnostic inside them. They are deliberately NOT
	// read back via GetSymbol: analyze() short-circuits pipe/join arguments
	// without stamping nodeSymbols, so that lookup can return nil.
	argSymbols := make([]symbol.Symbol, explicitN)
	for i, arg := range n.Arguments {
		argSymbols[i] = a.analyze(arg)
	}

	receiverArgs := append([]symbol.Symbol{p.receiver}, argSymbols...)
	matches := make([]ufcsCandidate, 0, len(viable))
	for _, c := range viable {
		if callMatches(c.fn, receiverArgs) {
			matches = append(matches, c)
		}
	}
	// When the field rejects these arguments the contest is the function
	// side's alone — including the "neither accepts them" case, where falling
	// back to the field gets precise per-argument type errors.
	if !callMatches(p.field, argSymbols) {
		return a.decideWithoutField(prop, methodName, p.field, matches), argSymbols
	}
	if len(matches) == 0 {
		return p.field, argSymbols
	}
	a.reportStructOverloadAmbiguity(prop, methodName, p.structName, "", matches)
	return symbol.AnySymbol(), argSymbols
}

// decideWithoutField settles a contest the struct's own field has dropped out
// of, leaving only UFCS candidates: exactly one wins, several are ambiguous,
// and none falls back to the field so the ordinary checking path reports its
// own precise error against the member the call most likely meant.
func (a *Analyzer) decideWithoutField(prop *ast.PropertyExpression, methodName string, field *symbol.FunctionSymbol, candidates []ufcsCandidate) symbol.Symbol {
	switch len(candidates) {
	case 0:
		return field
	case 1:
		return a.winUFCS(prop, candidates[0])
	default:
		a.reportUFCSAmbiguity(prop, methodName, candidates)
		return symbol.AnySymbol()
	}
}

// winUFCS records a UFCS win over a struct field and returns the winning
// function. Both property nodes are restamped because analyzePropertyExpression
// already stored the field symbol provisionally, and the LSP reads them for
// hover and signature help.
func (a *Analyzer) winUFCS(prop *ast.PropertyExpression, c ufcsCandidate) symbol.Symbol {
	a.ufcsMatches[prop] = c.modulePath
	a.nodeSymbols[prop.Property] = c.fn
	a.nodeSymbols[prop] = c.fn
	return c.fn
}

// sameDeclaredParams reports whether a field's function type and a UFCS
// candidate declare the same parameters once the candidate's receiver slot is
// dropped — i.e. whether the two are indistinguishable at every call.
func sameDeclaredParams(field, candidate *symbol.FunctionSymbol) bool {
	fieldParams := field.ParamTypes()
	candParams := candidate.ParamTypes()[1:]
	if len(fieldParams) != len(candParams) {
		return false
	}
	for i, p := range fieldParams {
		if !p.Equals(candParams[i]) {
			return false
		}
	}
	return true
}
