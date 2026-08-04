package evaluator

import (
	"caja-cli/internal/environment"
	"caja-cli/internal/parser"
	"fmt"
	"math"
)

type Evaluator struct {
	env *environment.Environment
}

// New initializes and returns a new Evaluator with a fresh environment.
func New() *Evaluator {
	return &Evaluator{env: environment.New()}
}

// Eval recursively evaluates a single AST node and returns its numeric
// value. It dispatches on the concrete node type: programs, statements,
// literals, identifiers, return statements, and infix expressions.
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
	case *parser.ReturnStatement:
		return e.Eval(node.ReturnValue)
	case *parser.IfExpression:
		return e.evalIfExpression(node)
	case *parser.BlockStatement:
		return e.evalBlockStatement(node)
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

// evalProgram evaluates every statement in the program sequentially. When a
// ReturnStatement is encountered its value is returned immediately. If the
// program ends without a return statement an error is produced.
func (e *Evaluator) evalProgram(program *parser.Program) (float64, error) {
	var result float64
	var err error

	for _, statement := range program.Statements {
		result, err = e.Eval(statement)
		if err != nil {
			return 0, err
		}

		if _, ok := statement.(*parser.ReturnStatement); ok {
			return result, nil
		}
	}

	return 0, fmt.Errorf("execution error: script finished without a return statement")
}

// evalIdentifier resolves an identifier to its value in the current
// environment. An error is returned if the identifier has not been assigned.
func (e *Evaluator) evalIdentifier(node *parser.Identifier) (float64, error) {
	val, ok := e.env.Get(node.Value)
	if !ok {
		return 0, fmt.Errorf("identifier not found: %s", node.Value)
	}
	return val, nil
}

// evalInfixExpression applies a binary arithmetic operator (+, -, *, /) to
// the left and right operands. Division by zero produces an error.
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
	case "%":
		if right == 0 {
			return 0, fmt.Errorf("modulo by zero")
		}
		return math.Mod(left, right), nil
	case "^":
		return math.Pow(left, right), nil
	case "<":
		return boolToFloat(left < right), nil
	case ">":
		return boolToFloat(left > right), nil
	case "<=":
		return boolToFloat(left <= right), nil
	case ">=":
		return boolToFloat(left >= right), nil
	case "==":
		return boolToFloat(left == right), nil
	case "!=":
		return boolToFloat(left != right), nil
	default:
		return 0, fmt.Errorf("unknown operator: %s", operator)
	}
}

// evalBlockStatement evaluates a sequence of statements in a block,
// returning the result of the last evaluated statement.
func (e *Evaluator) evalBlockStatement(block *parser.BlockStatement) (float64, error) {
	var result float64
	var err error

	for _, statement := range block.Statements {
		result, err = e.Eval(statement)
		if err != nil {
			return 0, err
		}
	}

	return result, nil
}

// evalIfExpression evaluates an if-else conditional expression,
// executing the consequence block if the condition is truthy,
// and the alternative block otherwise.
func (e *Evaluator) evalIfExpression(ie *parser.IfExpression) (float64, error) {
	condition, err := e.Eval(ie.Condition)
	if err != nil {
		return 0, err
	}

	if isTruthy(condition) {
		return e.evalBlockStatement(ie.Consequence)
	}

	if ie.Alternative != nil {
		return e.evalBlockStatement(ie.Alternative)
	}

	return 0.0, nil
}

// boolToFloat converts a boolean value to a float64 representation,
// mapping true to 1.0 and false to 0.0.
func boolToFloat(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}

// isTruthy determines if a float64 value is considered true,
// which is any non-zero value.
func isTruthy(val float64) bool {
	return val != 0.0
}
