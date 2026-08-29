package ast

import (
	"caja-cli/internal/pipeline/lexer"
	"fmt"
)

type DiagnosticError struct {
	Token   lexer.Token
	Message string
}

func (e DiagnosticError) String() string {
	return fmt.Sprintf("[Line %d, Column %d] %s", e.Token.Line, e.Token.Column, e.Message)
}
