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
	RETURN_VALUE_OBJ ObjectType = "RETURN_VALUE"
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
func (f *Function) Inspect() string  { return "fn(...)" }

// ReturnValue represents a wrapper object that causes the evaluator to halt
// block execution and return the enclosed value.
type ReturnValue struct {
	Value Object
}

func (rv *ReturnValue) Type() ObjectType { return RETURN_VALUE_OBJ }
func (rv *ReturnValue) Inspect() string  { return rv.Value.Inspect() }
