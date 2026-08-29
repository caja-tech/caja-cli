package environment

import (
	"bytes"
	"caja-cli/internal/pipeline/ast"
	"fmt"
	"strings"
	"time"
)

type ObjectType string

const (
	ANY_OBJ          ObjectType = "Any"
	NUMBER_OBJ       ObjectType = "Number"
	STRING_OBJ       ObjectType = "String"
	BOOLEAN_OBJ      ObjectType = "Boolean"
	DATE_OBJ         ObjectType = "Date"
	FUNCTION_OBJ     ObjectType = "Function"
	BUILTIN_OBJ      ObjectType = "Builtin"
	ARRAY_OBJ        ObjectType = "Array"
	MAP_OBJ          ObjectType = "Map"
	RETURN_VALUE_OBJ ObjectType = "RETURN_VALUE"
	TAIL_CALL_OBJ    ObjectType = "TAIL_CALL"
	MODULE_OBJ       ObjectType = "MODULE"
	NULL_OBJ         ObjectType = "NULL"
)

type Object interface {
	Type() ObjectType
	Inspect() string
}

// FormatObject takes an environment Object and returns its formatted string representation.
func FormatObject(obj Object) string {
	if obj == nil {
		return "null"
	}
	return obj.Inspect()
}

// PrintObject formats the given environment Object and prints it to standard output.
func PrintObject(obj Object) {
	fmt.Println(FormatObject(obj))
}

func IsReferenceType(t ObjectType) bool {
	switch t {
	case NUMBER_OBJ, STRING_OBJ, BOOLEAN_OBJ, DATE_OBJ:
		return false
	}
	return true
}

// Number represents a numeric value (stored as a float64) in the evaluated environment.
type Number struct{ Value float64 }

func (n *Number) Type() ObjectType { return NUMBER_OBJ }
func (n *Number) Inspect() string  { return fmt.Sprintf("%g", n.Value) }

// String represents a string value in the evaluated environment.
type String struct{ Value string }

func (s *String) Type() ObjectType { return STRING_OBJ }
func (s *String) Inspect() string  { return s.Value }

// Boolean represents a boolean value in the evaluated environment.
type Boolean struct{ Value bool }

func (b *Boolean) Type() ObjectType { return BOOLEAN_OBJ }
func (b *Boolean) Inspect() string  { return fmt.Sprintf("%t", b.Value) }

var (
	envTrue  = &Boolean{Value: true}
	envFalse = &Boolean{Value: false}
)

// True returns the singleton environment.Boolean object representing the boolean value true.
func True() *Boolean {
	return envTrue
}

// False returns the singleton environment.Boolean object representing the boolean value false.
func False() *Boolean {
	return envFalse
}

// NativeBoolToBooleanObject converts a native Go boolean into its corresponding
// environment.Boolean object wrapper, returning either the TRUE or FALSE singleton.
func NativeBoolToBooleanObject(input bool) *Boolean {
	if input {
		return envTrue
	}
	return envFalse
}

// Null represents a nil value in the evaluated environment.
type Null struct{}

func (n *Null) Type() ObjectType { return NULL_OBJ }
func (n *Null) Inspect() string  { return "nil" }

var NullObj = &Null{}

// Date represents a temporal date value in the evaluated environment.
type Date struct{ Value time.Time }

func (d *Date) Type() ObjectType { return DATE_OBJ }
func (d *Date) Inspect() string  { return d.Value.Format("2006-01-02") }

// Function represents an evaluated function, containing its parameter list,
// body statements, and the environment in which it was declared (closure).
type Function struct {
	Parameters []*ast.Parameter
	Body       *ast.BlockStatement
	Env        *Environment
	ReturnType string
}

func (f *Function) Type() ObjectType { return FUNCTION_OBJ }
func (f *Function) Inspect() string {
	out := "fn("
	for i, p := range f.Parameters {
		out += p.String()
		if i != len(f.Parameters)-1 {
			out += ", "
		}
	}
	out += ") {\n" + f.Body.String() + "\n}"
	return out
}

// BuiltinFunction represents a native Go function wrapped for use in the Caja language.
// It accepts a slice of evaluated objects and returns an evaluated object or an error.
type BuiltinFunction func(env *Environment, args ...Object) (Object, error)

// Builtin represents a built-in function provided by the Caja runtime.
// It implements the Object interface, allowing it to be assigned to variables
// and passed as arguments just like user-defined functions.
type Builtin struct {
	Name string
	Fn   BuiltinFunction
}

func (b *Builtin) Type() ObjectType { return BUILTIN_OBJ }
func (b *Builtin) Inspect() string  { return "builtin function " + b.Name }

// Array represents an ordered collection of evaluated Objects.
// Elements holds the slice of objects contained within the array.
type Array struct {
	Elements []Object
}

func (ao *Array) Type() ObjectType { return ARRAY_OBJ }
func (ao *Array) Inspect() string {
	out := "["
	for i, el := range ao.Elements {
		out += el.Inspect()
		if i != len(ao.Elements)-1 {
			out += ", "
		}
	}
	out += "]"
	return out
}

// ReturnValue represents a wrapper object that causes the evaluator to halt
// block execution and return the enclosed value.
type ReturnValue struct {
	Value Object
}

func (rv *ReturnValue) Type() ObjectType { return RETURN_VALUE_OBJ }
func (rv *ReturnValue) Inspect() string  { return rv.Value.Inspect() }

// TailCall represents an intercepted function call in tail position.
// It holds the target function and its evaluated arguments, allowing the
// evaluator to reuse the current stack frame and execute in constant memory space.
type TailCall struct {
	Function  *Function
	Arguments []Object
}

func (tc *TailCall) Type() ObjectType { return TAIL_CALL_OBJ }
func (tc *TailCall) Inspect() string {
	out := "tail call with args: ("
	for i, arg := range tc.Arguments {
		out += arg.Inspect()
		if i != len(tc.Arguments)-1 {
			out += ", "
		}
	}
	out += ")"
	return out
}

// Module represents an imported script/module.
// Env holds the evaluated environment containing the module's exported variables and functions.
type Module struct {
	Name string
	Env  *Environment
}

func (m *Module) Type() ObjectType { return MODULE_OBJ }
func (m *Module) Inspect() string  { return "module " + m.Name }

// StructObject represents an instantiated struct object containing properties.
type StructObject struct {
	StructName string
	Fields     map[string]Object
}

// Type returns the ObjectType of the struct instance.
func (s *StructObject) Type() ObjectType {
	return ObjectType(s.StructName)
}

// Inspect returns a string representation of the struct object.
func (s *StructObject) Inspect() string {
	var out bytes.Buffer

	out.WriteString(s.StructName)
	out.WriteString(" { ")

	var fields []string
	for name, val := range s.Fields {
		fields = append(fields, fmt.Sprintf("%s: %s", name, val.Inspect()))
	}

	out.WriteString(strings.Join(fields, ", "))
	out.WriteString(" }")

	return out.String()
}

type MapPair struct {
	Key   Object
	Value Object
}

type Map struct {
	Pairs map[string]MapPair
}

func (m *Map) Type() ObjectType { return MAP_OBJ }

func (m *Map) Inspect() string {
	var out bytes.Buffer

	pairs := []string{}
	for _, pair := range m.Pairs {
		pairs = append(pairs, fmt.Sprintf("%s: %s", pair.Key.Inspect(), pair.Value.Inspect()))
	}

	out.WriteString("{")
	out.WriteString(strings.Join(pairs, ", "))
	out.WriteString("}")

	return out.String()
}
