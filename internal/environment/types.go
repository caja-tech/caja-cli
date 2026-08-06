package environment

import (
	"caja-cli/internal/syntax"
	"fmt"
	"time"
)

type ObjectType string

const (
	ANY_OBJ          ObjectType = "ANY"
	NUMBER_OBJ       ObjectType = "NUMBER"
	STRING_OBJ       ObjectType = "STRING"
	BOOLEAN_OBJ      ObjectType = "BOOLEAN"
	DATE_OBJ         ObjectType = "DATE"
	FUNCTION_OBJ     ObjectType = "FUNCTION"
	ARRAY_OBJ        ObjectType = "ARRAY"
	RETURN_VALUE_OBJ ObjectType = "RETURN_VALUE"
	TAIL_CALL_OBJ    ObjectType = "TAIL_CALL"
)

type Object interface {
	Type() ObjectType
	Inspect() string
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

// Date represents a temporal date value in the evaluated environment.
type Date struct{ Value time.Time }

func (d *Date) Type() ObjectType { return DATE_OBJ }
func (d *Date) Inspect() string  { return d.Value.Format("2006-01-02") }

// Function represents an evaluated function, containing its parameter list,
// body statements, and the environment in which it was declared (closure).
type Function struct {
	Parameters []*syntax.Parameter
	Body       *syntax.BlockStatement
	Env        *Environment
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
