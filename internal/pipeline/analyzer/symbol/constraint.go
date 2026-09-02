package symbol

import (
	"caja-cli/internal/pipeline/ast"
	"caja-cli/internal/pipeline/environment"
	"fmt"
)

type ConstraintSymbol struct {
	Name      string
	BaseType  Symbol
	Predicate ast.Expression
}

func NewConstraintSymbol(name string, baseType Symbol, predicate ast.Expression) *ConstraintSymbol {
	return &ConstraintSymbol{
		Name:      name,
		BaseType:  baseType,
		Predicate: predicate,
	}
}

func (s *ConstraintSymbol) Equals(other Symbol) bool {
	if other == AnySymbol() {
		return true
	}
	if otherConstraint, ok := other.(*ConstraintSymbol); ok {
		return s.Name == otherConstraint.Name
	}
	return false
}

func (s *ConstraintSymbol) Type() environment.ObjectType {
	return environment.ObjectType(s.Name)
}

func (s *ConstraintSymbol) String() string {
	return fmt.Sprintf("%s", s.Name)
}
