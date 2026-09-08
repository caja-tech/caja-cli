package ast

import "testing"

// TestMightReturnIfWithoutElse guards against a regression where IfExpression's
// Alternative field is a concrete *BlockStatement, so when there's no else
// branch it's a nil pointer - passing it through MightReturn's Node interface
// parameter without a nil check boxes it as a non-nil interface value,
// bypassing the top-level `node == nil` guard and panicking on n.Statements.
func TestMightReturnIfWithoutElse(t *testing.T) {
	ifNoElse := &IfExpression{
		Consequence: &BlockStatement{Statements: []Statement{}},
		Alternative: nil,
	}

	if MightReturn(ifNoElse) {
		t.Errorf("expected false for an if with no return in its consequence and no else branch")
	}

	ifConsequenceReturns := &IfExpression{
		Consequence: &BlockStatement{Statements: []Statement{&ReturnStatement{}}},
		Alternative: nil,
	}

	if !MightReturn(ifConsequenceReturns) {
		t.Errorf("expected true when the consequence contains a return, regardless of a missing else branch")
	}
}
