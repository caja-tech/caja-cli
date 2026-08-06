package evaluator

import (
	"caja-cli/internal/environment"
	"caja-cli/internal/syntax"
	"fmt"
	"math"
	"time"
)

var (
	TRUE  = &environment.Boolean{Value: true}
	FALSE = &environment.Boolean{Value: false}
)

// Eval recursively evaluates a single AST node and returns its numeric
// value. It dispatches on the concrete node type: programs, statements,
// literals, identifiers, return statements, and infix expressions.
func Eval(n syntax.Node, env *environment.Environment) (environment.Object, error) {
	switch node := n.(type) {
	case *syntax.Program:
		return evalProgram(node, env)
	case *syntax.ExpressionStatement:
		return evalExpressionStatement(node, env)
	case *syntax.AssignStatement:
		return evalAssignStatement(node, env)
	case *syntax.NumberLiteral:
		return evalNumberLiteral(node)
	case *syntax.StringLiteral:
		return evalStringLiteral(node)
	case *syntax.DateLiteral:
		return evalDateLiteral(node)
	case *syntax.BooleanLiteral:
		return evalBooleanLiteral(node)
	case *syntax.Identifier:
		return evalIdentifier(node, env)
	case *syntax.ReturnStatement:
		return evalReturnStatement(node, env)
	case *syntax.LetStatement:
		return evalLetStatement(node, env)
	case *syntax.IfExpression:
		return evalIfExpression(node, env)
	case *syntax.BlockStatement:
		return evalBlockStatement(node, env)
	case *syntax.InfixExpression:
		return evalInfixExpressionNode(node, env)
	case *syntax.TypeAliasStatement:
		return nil, nil
	case *syntax.FunctionLiteral:
		return evalFunctionLiteral(node, env)
	case *syntax.CallExpression:
		return evalCallExpression(node, env)
	}

	return nil, fmt.Errorf("unknown node type: %T", n)
}

// evalExpressionStatement evaluates the inner expression of an expression statement.
func evalExpressionStatement(node *syntax.ExpressionStatement, env *environment.Environment) (environment.Object, error) {
	return Eval(node.Expression, env)
}

// evalAssignStatement evaluates the assigned value and updates the existing variable in the environment.
func evalAssignStatement(node *syntax.AssignStatement, env *environment.Environment) (environment.Object, error) {
	val, err := Eval(node.Value, env)
	if err != nil {
		return nil, err
	}
	env.Assign(node.Name.Value, val)
	return val, nil
}

// evalNumberLiteral returns a Number object representing the literal's value.
func evalNumberLiteral(node *syntax.NumberLiteral) (environment.Object, error) {
	return &environment.Number{Value: node.Value}, nil
}

// evalStringLiteral returns a String object representing the literal's value.
func evalStringLiteral(node *syntax.StringLiteral) (environment.Object, error) {
	return &environment.String{Value: node.Value}, nil
}

// evalDateLiteral parses the DateLiteral string value as 'YYYY-MM-DD' and returns
// a Date object representing the parsed time.
func evalDateLiteral(node *syntax.DateLiteral) (environment.Object, error) {
	parsedTime, _ := time.Parse("2006-01-02", node.Value)

	return &environment.Date{Value: parsedTime}, nil
}

// evalBooleanLiteral returns a Boolean object representing the literal's value.
func evalBooleanLiteral(node *syntax.BooleanLiteral) (environment.Object, error) {
	return nativeBoolToBooleanObject(node.Value), nil
}

// evalReturnStatement evaluates the returned expression and wraps it in a ReturnValue object.
func evalReturnStatement(node *syntax.ReturnStatement, env *environment.Environment) (environment.Object, error) {
	val, err := Eval(node.ReturnValue, env)
	if err != nil {
		return nil, err
	}
	return &environment.ReturnValue{Value: val}, nil
}

// evalLetStatement evaluates the assigned value and creates a new variable in the environment.
func evalLetStatement(node *syntax.LetStatement, env *environment.Environment) (environment.Object, error) {
	val, err := Eval(node.Value, env)
	if err != nil {
		return nil, err
	}
	env.Set(node.Name.Value, val)
	return val, nil
}

// evalInfixExpressionNode evaluates the left and right operands and applies the operator.
func evalInfixExpressionNode(node *syntax.InfixExpression, env *environment.Environment) (environment.Object, error) {
	left, err := Eval(node.Left, env)
	if err != nil {
		return nil, err
	}

	right, err := Eval(node.Right, env)
	if err != nil {
		return nil, err
	}

	return evalInfixExpression(node.Operator, left, right)
}

// evalFunctionLiteral returns a Function object capturing its parameters, body, and the current environment.
func evalFunctionLiteral(node *syntax.FunctionLiteral, env *environment.Environment) (environment.Object, error) {
	params := node.Parameters
	body := node.Body
	return &environment.Function{Parameters: params, Env: env, Body: body}, nil
}

// evalCallExpression evaluates the function and its arguments, then executes the function body in an extended environment.
func evalCallExpression(node *syntax.CallExpression, env *environment.Environment) (environment.Object, error) {
	function, err := Eval(node.Function, env)
	if err != nil {
		return nil, err
	}

	var args []environment.Object
	for _, argNode := range node.Arguments {
		evaluated, err := Eval(argNode, env)
		if err != nil {
			return nil, err
		}
		args = append(args, evaluated)
	}

	fn, ok := function.(*environment.Function)
	if !ok {
		return nil, fmt.Errorf("not a function")
	}

	extendedEnv := environment.NewEnclosedEnvironment(fn.Env)
	for i, param := range fn.Parameters {
		extendedEnv.Set(param.Name, args[i])
	}

	evaluated, err := evalBlockStatement(fn.Body, extendedEnv)
	if err != nil {
		return nil, err
	}

	// Unwrap any early return values so the whole program doesn't halt
	if retVal, ok := evaluated.(*environment.ReturnValue); ok {
		return retVal.Value, nil
	}

	return evaluated, nil
}

// evalProgram evaluates every statement in the program sequentially. When a
// ReturnStatement is encountered its value is returned immediately. If the
// program ends without a return statement an error is produced.
func evalProgram(program *syntax.Program, env *environment.Environment) (environment.Object, error) {
	var result environment.Object
	var err error

	for _, statement := range program.Statements {
		result, err = Eval(statement, env)
		if err != nil {
			return nil, err
		}

		// UNWRAP IT: Halt the global script
		if retVal, ok := result.(*environment.ReturnValue); ok {
			return retVal.Value, nil
		}
	}

	return nil, fmt.Errorf("execution error: script finished without a return statement")
}

// evalIdentifier resolves an identifier to its value in the current
// environment. An error is returned if the identifier has not been assigned.
func evalIdentifier(node *syntax.Identifier, env *environment.Environment) (environment.Object, error) {
	val, ok := env.Get(node.Value)
	if !ok {
		return nil, fmt.Errorf("identifier not found: %s", node.Value)
	}
	return val, nil
}

// evalInfixExpression applies a binary arithmetic operator (+, -, *, /) to
// the left and right operands. Division by zero produces an error.
func evalInfixExpression(operator string, left, right environment.Object) (environment.Object, error) {
	if left.Type() != environment.NUMBER_OBJ || right.Type() != environment.NUMBER_OBJ {
		return nil, fmt.Errorf("type mismatch")
	}

	leftVal := left.(*environment.Number).Value
	rightVal := right.(*environment.Number).Value

	switch operator {
	case "+":
		return &environment.Number{Value: leftVal + rightVal}, nil
	case "-":
		return &environment.Number{Value: leftVal - rightVal}, nil
	case "*":
		return &environment.Number{Value: leftVal * rightVal}, nil
	case "/":
		if rightVal == 0 {
			return nil, fmt.Errorf("division by zero")
		}
		return &environment.Number{Value: leftVal / rightVal}, nil
	case "%":
		if rightVal == 0 {
			return nil, fmt.Errorf("modulo by zero")
		}
		return &environment.Number{Value: math.Mod(leftVal, rightVal)}, nil
	case "^":
		return &environment.Number{Value: math.Pow(leftVal, rightVal)}, nil
	case "<":
		return boolToPrimitive(leftVal < rightVal), nil
	case ">":
		return boolToPrimitive(leftVal > rightVal), nil
	case "<=":
		return boolToPrimitive(leftVal <= rightVal), nil
	case ">=":
		return boolToPrimitive(leftVal >= rightVal), nil
	case "==":
		return boolToPrimitive(leftVal == rightVal), nil
	case "!=":
		return boolToPrimitive(leftVal != rightVal), nil
	default:
		return nil, fmt.Errorf("unknown operator: %s", operator)
	}
}

// evalBlockStatement evaluates a sequence of statements in a block,
// returning the result of the last evaluated statement.
func evalBlockStatement(block *syntax.BlockStatement, env *environment.Environment) (environment.Object, error) {
	blockEnv := environment.NewEnclosedEnvironment(env)

	var result environment.Object
	var err error

	for _, statement := range block.Statements {
		result, err = Eval(statement, blockEnv)
		if err != nil {
			return nil, err
		}

		if result != nil && result.Type() == environment.RETURN_VALUE_OBJ {
			return result, nil
		}
	}

	return result, nil
}

// evalIfExpression evaluates an if-else conditional expression,
// executing the consequence block if the condition is truthy,
// and the alternative block otherwise.
func evalIfExpression(ie *syntax.IfExpression, env *environment.Environment) (environment.Object, error) {
	condition, err := Eval(ie.Condition, env)
	if err != nil {
		return nil, err
	}

	if isTruthy(condition) {
		return evalBlockStatement(ie.Consequence, env)
	}

	if ie.Alternative != nil {
		return evalBlockStatement(ie.Alternative, env)
	}

	return &environment.Number{Value: 0.0}, nil
}

// boolToPrimitive converts a boolean value to an environment.Object representation,
// mapping true to 1.0 and false to 0.0.
func boolToPrimitive(b bool) environment.Object {
	if b {
		return &environment.Number{Value: 1.0}
	}
	return &environment.Number{Value: 0.0}
}

// nativeBoolToBooleanObject converts a native Go boolean into its corresponding
// environment.Boolean object wrapper, returning either the TRUE or FALSE singleton.
func nativeBoolToBooleanObject(input bool) *environment.Boolean {
	if input {
		return TRUE
	}
	return FALSE
}

// isTruthy determines if a types.Object value is considered true,
// which is any non-zero value.
func isTruthy(val environment.Object) bool {
	if val.Type() == environment.NUMBER_OBJ {
		return val.(*environment.Number).Value != 0.0
	}
	return false
}
