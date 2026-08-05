package evaluator

import (
	"caja-cli/internal/syntax"
	"fmt"
	"math"
)

// Eval recursively evaluates a single AST node and returns its numeric
// value. It dispatches on the concrete node type: programs, statements,
// literals, identifiers, return statements, and infix expressions.
func Eval(n syntax.Node, env *Environment) (float64, error) {
	switch node := n.(type) {
	case *syntax.Program:
		return evalProgram(node, env)
	case *syntax.ExpressionStatement:
		return Eval(node.Expression, env)
	case *syntax.AssignStatement:
		val, err := Eval(node.Value, env)
		if err != nil {
			return 0, err
		}
		env.Assign(node.Name.Value, val)
		return val, nil
	case *syntax.NumberLiteral:
		return node.Value, nil
	case *syntax.Identifier:
		return evalIdentifier(node, env)
	case *syntax.ReturnStatement:
		return Eval(node.ReturnValue, env)
	case *syntax.LetStatement:
		val, err := Eval(node.Value, env)
		if err != nil {
			return 0, err
		}
		env.Set(node.Name.Value, val)
		return val, nil
	case *syntax.IfExpression:
		return evalIfExpression(node, env)
	case *syntax.BlockStatement:
		return evalBlockStatement(node, env)
	case *syntax.InfixExpression:
		left, err := Eval(node.Left, env)
		if err != nil {
			return 0, err
		}

		right, err := Eval(node.Right, env)
		if err != nil {
			return 0, err
		}

		return evalInfixExpression(node.Operator, left, right)
	}

	return 0, fmt.Errorf("unknown node type: %T", n)
}

// evalProgram evaluates every statement in the program sequentially. When a
// ReturnStatement is encountered its value is returned immediately. If the
// program ends without a return statement an error is produced.
func evalProgram(program *syntax.Program, env *Environment) (float64, error) {
	var result float64
	var err error

	for _, statement := range program.Statements {
		result, err = Eval(statement, env)
		if err != nil {
			return 0, err
		}

		if _, ok := statement.(*syntax.ReturnStatement); ok {
			return result, nil
		}
	}

	return 0, fmt.Errorf("execution error: script finished without a return statement")
}

// evalIdentifier resolves an identifier to its value in the current
// environment. An error is returned if the identifier has not been assigned.
func evalIdentifier(node *syntax.Identifier, env *Environment) (float64, error) {
	val, ok := env.Get(node.Value)
	if !ok {
		return 0, fmt.Errorf("identifier not found: %s", node.Value)
	}
	return val, nil
}

// evalInfixExpression applies a binary arithmetic operator (+, -, *, /) to
// the left and right operands. Division by zero produces an error.
func evalInfixExpression(operator string, left, right float64) (float64, error) {
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
func evalBlockStatement(block *syntax.BlockStatement, env *Environment) (float64, error) {
	blockEnv := NewEnclosedEnvironment(env)

	var result float64
	var err error

	for _, statement := range block.Statements {
		result, err = Eval(statement, blockEnv)
		if err != nil {
			return 0, err
		}
	}

	return result, nil
}

// evalIfExpression evaluates an if-else conditional expression,
// executing the consequence block if the condition is truthy,
// and the alternative block otherwise.
func evalIfExpression(ie *syntax.IfExpression, env *Environment) (float64, error) {
	condition, err := Eval(ie.Condition, env)
	if err != nil {
		return 0, err
	}

	if isTruthy(condition) {
		return evalBlockStatement(ie.Consequence, env)
	}

	if ie.Alternative != nil {
		return evalBlockStatement(ie.Alternative, env)
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
