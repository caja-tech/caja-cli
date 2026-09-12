package ast

import (
	goast "go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"testing"

	"caja-cli/internal/pipeline/lexer"
)

// allNodes lists one zero value of every struct in this package that implements Node.
// TestAllNodesListIsComplete keeps it honest against the source, so a node type added to
// ast.go without being added here fails the build's tests rather than silently becoming
// invisible to every traversal in the codebase.
var allNodes = []Node{
	&Program{},
	&NilLiteral{},
	&NumberLiteral{},
	&StringLiteral{},
	&InterpolatedStringLiteral{},
	&BooleanLiteral{},
	&DateLiteral{},
	&ArrayLiteral{},
	&Parameter{},
	&FunctionLiteral{},
	&StructLiteral{},
	&LetStatement{},
	&ConstStatement{},
	&ImportStatement{},
	&ReturnStatement{},
	&AssignStatement{},
	&BlockStatement{},
	&TypeConstraintStatement{},
	&UnionStatement{},
	&TypeAliasStatement{},
	&ExpressionStatement{},
	&Identifier{},
	&GenericIdentifier{},
	&PrefixExpression{},
	&InfixExpression{},
	&IsExpression{},
	&IfExpression{},
	&NamedArgument{},
	&CallExpression{},
	&IndexExpression{},
	&IndexAssignmentStatement{},
	&PropertyExpression{},
	&PropertyAssignmentStatement{},
	&MapLiteral{},
	&SafePipeExpression{},
	&StreamPipeExpression{},
	&JoinGroupExpression{},
	&AsyncExpression{},
	&UnwrapExpression{},
	&AwaitStatement{},
}

// nonNodeHelpers are the structs in this package that deliberately do not implement Node.
// Each carries no token of its own, so it has no position and cannot be the answer to
// "what is under the cursor". Children reaches through them to whatever inside them does
// have a position.
var nonNodeHelpers = map[string]string{
	"InterpolatedStringSegment": "a text run inside a string has no token; Children yields the segment's embedded expression instead",
	"StructField":               "field declarations have no token of their own; Children yields the field-name identifier",
	"StructDefinition":          "a struct body is addressed through its owning TypeAliasStatement",
	"FunctionSignature":         "a function type carries no token; its parameter types are still plain strings",
}

// TestAllNodesListIsComplete cross-checks allNodes against the structs actually declared
// in ast.go. This is what stops the registry below from rotting: a new node type must be
// classified, either as a Node to be traversed or as a documented non-Node helper.
func TestAllNodesListIsComplete(t *testing.T) {
	listed := make(map[string]bool, len(allNodes))
	for _, n := range allNodes {
		listed[reflect.TypeOf(n).Elem().Name()] = true
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "ast.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing ast.go: %v", err)
	}

	declared := structNames(file)
	if len(declared) == 0 {
		t.Fatal("found no struct declarations in ast.go; the scan is broken")
	}

	for _, name := range declared {
		switch {
		case listed[name]:
		case nonNodeHelpers[name] != "":
		default:
			t.Errorf("ast.go declares %s but it is neither in allNodes nor in nonNodeHelpers.\n"+
				"Add it to allNodes and teach Children about it, or record why it carries no position.", name)
		}
	}

	for name := range listed {
		if !contains(declared, name) {
			t.Errorf("allNodes lists %s, which no longer exists in ast.go", name)
		}
	}
}

// TestChildrenReachesEveryNodeField is the drift guard. For every node type it fills each
// field that can hold another node with a distinct sentinel, then asserts Children hands
// every one of them back.
//
// The three private traversal switches this replaced had decayed to covering 25, 11 and
// 18 of the 44 node types, because nothing connected "a new syntax node exists" to "the
// walkers know about it". This test is that connection.
func TestChildrenReachesEveryNodeField(t *testing.T) {
	for _, proto := range allNodes {
		typ := reflect.TypeOf(proto).Elem()

		t.Run(typ.Name(), func(t *testing.T) {
			value := reflect.New(typ)
			planted := plantSentinels(t, value.Elem())

			if len(planted) == 0 {
				return // a leaf: nothing to reach
			}

			returned := make(map[Node]bool)
			for _, child := range Children(value.Interface().(Node)) {
				returned[child] = true
			}

			for field, sentinel := range planted {
				if !returned[sentinel] {
					t.Errorf("Children(*%s) did not return the node held in field %q.\n"+
						"Every field that can hold a node must be reachable, or that syntax "+
						"becomes invisible to hover, go-to-definition and signature help.",
						typ.Name(), field)
				}
			}
		})
	}
}

// plantSentinels fills every node-holding field of a struct with a uniquely identifiable
// node, returning them keyed by field name.
func plantSentinels(t *testing.T, target reflect.Value) map[string]Node {
	t.Helper()

	planted := make(map[string]Node)
	typ := target.Type()

	for i := range typ.NumField() {
		field := typ.Field(i)
		if !target.Field(i).CanSet() {
			continue
		}

		switch field.Type.Kind() {
		case reflect.Interface, reflect.Pointer:
			node, ok := sentinelFor(field.Type)
			if !ok {
				continue
			}
			target.Field(i).Set(reflect.ValueOf(node))
			planted[field.Name] = node

		case reflect.Slice:
			node, ok := sentinelFor(field.Type.Elem())
			if !ok {
				continue
			}
			slice := reflect.MakeSlice(field.Type, 1, 1)
			slice.Index(0).Set(reflect.ValueOf(node))
			target.Field(i).Set(slice)
			planted[field.Name] = node

		case reflect.Map:
			key, keyOK := sentinelFor(field.Type.Key())
			val, valOK := sentinelFor(field.Type.Elem())
			if !valOK {
				continue
			}
			m := reflect.MakeMap(field.Type)
			if keyOK {
				m.SetMapIndex(reflect.ValueOf(key), reflect.ValueOf(val))
				planted[field.Name+" (key)"] = key
			} else {
				// A string-keyed map, such as StructLiteral.Fields.
				m.SetMapIndex(reflect.ValueOf("field"), reflect.ValueOf(val))
			}
			target.Field(i).Set(m)
			planted[field.Name] = val
		}
	}

	return planted
}

// sentinelFor builds a distinct node assignable to t, or reports that t cannot hold one.
// Every sentinel carries a token so that the source-order sorting inside Children has
// something to work with.
func sentinelFor(t reflect.Type) (Node, bool) {
	nodeType := reflect.TypeOf((*Node)(nil)).Elem()

	// Concrete pointer field, e.g. *Identifier or *BlockStatement.
	if t.Kind() == reflect.Pointer && t.Implements(nodeType) {
		value := reflect.New(t.Elem())
		stampToken(value.Elem())
		return value.Interface().(Node), true
	}

	// Interface field: Node, Statement or Expression.
	if t.Kind() == reflect.Interface {
		for _, candidate := range []Node{&Identifier{}, &ExpressionStatement{}} {
			if reflect.TypeOf(candidate).AssignableTo(t) {
				fresh := reflect.New(reflect.TypeOf(candidate).Elem())
				stampToken(fresh.Elem())
				return fresh.Interface().(Node), true
			}
		}
	}

	return nil, false
}

// stampToken gives a sentinel a unique, increasing source position so that Children's
// source-order sorting is well defined for it.
var sentinelSeq int

func stampToken(target reflect.Value) {
	field := target.FieldByName("Token")
	if !field.IsValid() || !field.CanSet() || field.Type() != reflect.TypeOf(lexer.Token{}) {
		return
	}
	sentinelSeq++
	field.Set(reflect.ValueOf(lexer.Token{
		Type:    lexer.IDENT,
		Literal: "sentinel",
		Line:    1,
		Column:  sentinelSeq,
	}))
}

func structNames(file *goast.File) []string {
	var names []string
	for _, decl := range file.Decls {
		gen, ok := decl.(*goast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			ts, ok := spec.(*goast.TypeSpec)
			if !ok {
				continue
			}
			if _, isStruct := ts.Type.(*goast.StructType); isStruct {
				names = append(names, ts.Name.Name)
			}
		}
	}
	return names
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
