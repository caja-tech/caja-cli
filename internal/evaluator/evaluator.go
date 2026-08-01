package evaluator

import (
	"caja-cli/internal/environment"
	"caja-cli/internal/parser"
	"fmt"
)

type Evaluator struct {
	env *environment.Environment
}

func New() *Evaluator {
	return &Evaluator{env: environment.New()}
}

func (e *Evaluator) Eval(n parser.Node) (float64, error) {
	switch node := n.(type) {
	case *parser.Program:
		return e.evalProgram(node)
	case *parser.ExpressionStatement:
		return e.Eval(node.Expression)
	case *parser.AssignStatement:
		val, err := e.Eval(node.Value)
		if err != nil {
			return 0, err
		}
		e.env.Set(node.Name.Value, val)
		return val, nil
	case *parser.NumberLiteral:
		return node.Value, nil
	case *parser.Identifier:
		return e.evalIdentifier(node)
	case *parser.InfixExpression:
		left, err := e.Eval(node.Left)
		if err != nil {
			return 0, err
		}

		right, err := e.Eval(node.Right)
		if err != nil {
			return 0, err
		}

		return e.evalInfixExpression(node.Operator, left, right)
	}

	return 0, fmt.Errorf("unknown node type: %T", n)
}

func (e *Evaluator) evalProgram(program *parser.Program) (float64, error) {
	var result float64
	var err error

	for _, statement := range program.Statements {
		result, err = e.Eval(statement)
		if err != nil {
			return 0, err
		}
	}

	return result, nil
}

func (e *Evaluator) evalIdentifier(node *parser.Identifier) (float64, error) {
	val, ok := e.env.Get(node.Value)
	if !ok {
		return 0, fmt.Errorf("identifier not found: %s", node.Value)
	}
	return val, nil
}

func (e *Evaluator) evalInfixExpression(operator string, left, right float64) (float64, error) {
	switch operator {
	case "+":
		return left + right, nil
	case "-":
		return left - right, nil
	case "*":
		return left * right, nil
	case "/":
		if right == 0 {
			return 0, fmt.Errorf("division by zero")
		}
		return left / right, nil
	default:
		return 0, fmt.Errorf("unknown operator: %s", operator)
	}
}
