package evaluator

import (
	"caja-cli/internal/environment"
	"caja-cli/internal/syntax"
	"fmt"
	"math"
	"strings"
	"time"
)

var (
	stackTraceLimit = 10000
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
	case *syntax.IndexAssignmentStatement:
		return evalIndexAssignmentStatement(node, env)
	case *syntax.NumberLiteral:
		return evalNumberLiteral(node)
	case *syntax.StringLiteral:
		return evalStringLiteral(node)
	case *syntax.DateLiteral:
		return evalDateLiteral(node)
	case *syntax.BooleanLiteral:
		return evalBooleanLiteral(node)
	case *syntax.ArrayLiteral:
		return evalArrayLiteral(node, env)
	case *syntax.IndexExpression:
		return evalIndexExpression(node, env)
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
	case *syntax.PrefixExpression:
		return evalPrefixExpressionNode(node, env)
	case *syntax.TypeAliasStatement:
		return nil, nil
	case *syntax.FunctionLiteral:
		return evalFunctionLiteral(node, env)
	case *syntax.CallExpression:
		return evalCallExpression(node, env)
	case *syntax.ImportStatement:
		return evalImportStatement(node, env)
	case *syntax.PropertyExpression:
		return evalPropertyExpression(node, env)
	}

	return nil, fmt.Errorf("unknown node type: %T", n)
}

// evalExpressionStatement evaluates the inner expression of an expression statement.
func evalExpressionStatement(node *syntax.ExpressionStatement, env *environment.Environment) (environment.Object, error) {
	return Eval(node.Expression, env)
}

// evalIndexAssignmentStatement evaluates the index, validates bounds, and updates the array element.
func evalIndexAssignmentStatement(node *syntax.IndexAssignmentStatement, env *environment.Environment) (environment.Object, error) {
	left, err := Eval(node.Left, env)
	if err != nil {
		return nil, err
	}

	arr, ok := left.(*environment.Array)
	if !ok {
		return nil, fmt.Errorf("type error: index assignment not supported for %s", left.Type())
	}

	indexObj, err := Eval(node.Index, env)
	if err != nil {
		return nil, err
	}

	indexNum, ok := indexObj.(*environment.Number)
	if !ok {
		return nil, fmt.Errorf("type error: array index must be NUMBER, got %s", indexObj.Type())
	}

	idx := int(indexNum.Value)
	if idx < 0 || idx >= len(arr.Elements) {
		return nil, fmt.Errorf("runtime error: array index out of bounds")
	}

	valObj, err := Eval(node.Value, env)
	if err != nil {
		return nil, err
	}

	arr.Elements[idx] = valObj
	return valObj, nil
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
	return environment.NativeBoolToBooleanObject(node.Value), nil
}

// evalArrayLiteral evaluates all expressions within an array literal and returns
// an Array object containing the evaluated elements.
func evalArrayLiteral(node *syntax.ArrayLiteral, env *environment.Environment) (environment.Object, error) {
	elements, err := evalExpressions(node.Elements, env)
	if err != nil {
		return nil, err
	}

	return &environment.Array{Elements: elements}, nil
}

// evalImportStatement evaluates an import by using the environment's ImportLoader.
func evalImportStatement(node *syntax.ImportStatement, env *environment.Environment) (environment.Object, error) {
	modPath := node.Path
	modName := node.Name.Value

	if cached, ok := env.ModuleCache[modPath]; ok {
		env.Set(modName, cached)
		return cached, nil
	}

	if stdMod := env.GetStandardModule(modPath); stdMod != nil {
		env.ModuleCache[modPath] = stdMod
		env.Set(modName, stdMod)
		return stdMod, nil
	}

	if env.Loading[modPath] {
		return nil, fmt.Errorf("circular import detected: %s", modPath)
	}

	env.Loading[modPath] = true
	defer func() { env.Loading[modPath] = false }()

	moduleObj, err := loadModule(modPath, env)
	if err != nil {
		return nil, err
	}

	env.ModuleCache[modPath] = moduleObj
	env.Set(modName, moduleObj)

	return moduleObj, nil
}

// evalPropertyExpression retrieves a property from a module object.
func evalPropertyExpression(node *syntax.PropertyExpression, env *environment.Environment) (environment.Object, error) {
	obj, err := Eval(node.Object, env)
	if err != nil {
		return nil, err
	}

	module, ok := obj.(*environment.Module)
	if !ok {
		return nil, fmt.Errorf("runtime error: property access is only supported on modules")
	}

	val, ok := module.Env.Get(node.Property.Value)
	if !ok {
		return nil, fmt.Errorf("runtime error: property '%s' not found in module '%s'", node.Property.Value, module.Name)
	}

	return val, nil
}

// evalIndexExpression evaluates an array index operation, ensuring the left side
// is an array and the index is a number within bounds, returning the requested element.
func evalIndexExpression(node *syntax.IndexExpression, env *environment.Environment) (environment.Object, error) {
	left, err := Eval(node.Left, env)
	if err != nil {
		return nil, err
	}

	index, err := Eval(node.Index, env)
	if err != nil {
		return nil, err
	}

	arrayObj, okLeft := left.(*environment.Array)
	indexObj, okIndex := index.(*environment.Number)

	if !okLeft || !okIndex {
		return nil, fmt.Errorf("runtime error: index operator not supported")
	}

	idx := int(indexObj.Value)
	if idx < 0 || idx >= len(arrayObj.Elements) {
		return nil, fmt.Errorf("runtime error: array index out of bounds")
	}

	return arrayObj.Elements[idx], nil
}

// evalReturnStatement evaluates the returned expression and wraps it in a ReturnValue object.
func evalReturnStatement(node *syntax.ReturnStatement, env *environment.Environment) (environment.Object, error) {
	currentFuncName := ""
	if len(env.CallStack) > 0 {
		currentFuncName = env.CallStack[len(env.CallStack)-1].FuncName
	}

	if callNode, ok := node.ReturnValue.(*syntax.CallExpression); ok {
		if ident, ok := callNode.Function.(*syntax.Identifier); ok && ident.Value == currentFuncName && currentFuncName != "" {
			args, err := evalExpressions(callNode.Arguments, env)
			if err != nil {
				return nil, err
			}
			fnObj, ok := env.Get(ident.Value)
			if !ok {
				return nil, fmt.Errorf("identifier not found: %s", ident.Value)
			}
			return &environment.TailCall{
				Function:  fnObj.(*environment.Function),
				Arguments: args,
			}, nil
		}
	}

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

	if node.Operator == "and" || node.Operator == "or" {
		leftBool, ok := left.(*environment.Boolean)
		if !ok {
			return nil, fmt.Errorf("type mismatch")
		}
		if node.Operator == "and" && !leftBool.Value {
			return left, nil
		}
		if node.Operator == "or" && leftBool.Value {
			return left, nil
		}
	}

	right, err := Eval(node.Right, env)
	if err != nil {
		return nil, err
	}

	if node.Operator == "and" || node.Operator == "or" || node.Operator == "xor" {
		return evalBooleanInfixExpression(node.Operator, left, right)
	}

	return evalInfixExpression(node.Operator, left, right)
}

func evalBooleanInfixExpression(operator string, left, right environment.Object) (environment.Object, error) {
	if left.Type() != environment.BOOLEAN_OBJ || right.Type() != environment.BOOLEAN_OBJ {
		return nil, fmt.Errorf("type mismatch")
	}

	leftVal := left.(*environment.Boolean).Value
	rightVal := right.(*environment.Boolean).Value

	switch operator {
	case "and":
		return environment.NativeBoolToBooleanObject(leftVal && rightVal), nil
	case "or":
		return environment.NativeBoolToBooleanObject(leftVal || rightVal), nil
	case "xor":
		return environment.NativeBoolToBooleanObject(leftVal != rightVal), nil
	default:
		return nil, fmt.Errorf("unknown operator: %s", operator)
	}
}

// evalPrefixExpressionNode evaluates the right operand and applies the prefix operator.
func evalPrefixExpressionNode(node *syntax.PrefixExpression, env *environment.Environment) (environment.Object, error) {
	right, err := Eval(node.Right, env)
	if err != nil {
		return nil, err
	}

	return evalPrefixExpression(node.Operator, right)
}

// evalPrefixExpression applies a prefix operator (!, -) to the right operand.
func evalPrefixExpression(operator string, right environment.Object) (environment.Object, error) {
	switch operator {
	case "!":
		return evalBangOperatorExpression(right)
	case "-":
		return evalMinusPrefixOperatorExpression(right)
	default:
		return nil, fmt.Errorf("unknown operator: %s", operator)
	}
}

// evalBangOperatorExpression applies the logical NOT operator (!) to the given object, returning the inverse boolean.
func evalBangOperatorExpression(right environment.Object) (environment.Object, error) {
	if right == environment.True() {
		return environment.False(), nil
	}
	if right == environment.False() {
		return environment.True(), nil
	}
	return nil, fmt.Errorf("type mismatch")
}

// evalMinusPrefixOperatorExpression applies the unary minus operator (-) to the given numeric object, negating its value.
func evalMinusPrefixOperatorExpression(right environment.Object) (environment.Object, error) {
	if right.Type() != environment.NUMBER_OBJ {
		return nil, fmt.Errorf("type mismatch")
	}

	value := right.(*environment.Number).Value
	return &environment.Number{Value: -value}, nil
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

	args, err := evalExpressions(node.Arguments, env)
	if err != nil {
		return nil, err
	}

	for {
		switch fn := function.(type) {
		case *environment.Function:
			funcName := ""
			if ident, ok := node.Function.(*syntax.Identifier); ok {
				funcName = ident.Value
			}

			var argContext []string
			for _, arg := range args {
				argContext = append(argContext, arg.Inspect())
			}

			frame := environment.StackFrame{
				FuncName: funcName,
				Line:     node.Token.Line,
				Column:   node.Token.Column,
				Args:     argContext,
			}

			extendedEnv := environment.NewEnclosedEnvironment(fn.Env)
			extendedEnv.CallStack = append(append([]environment.StackFrame(nil), env.CallStack...), frame)
			if len(extendedEnv.CallStack) > stackTraceLimit {
				return nil, fmt.Errorf("runtime error: stack overflow%s", extendedEnv.GetStackTrace())
			}

			for i, param := range fn.Parameters {
				extendedEnv.Set(param.Name, args[i])
			}

			evaluated, err := evalBlockStatement(fn.Body, extendedEnv)
			if err != nil {
				if !strings.Contains(err.Error(), "Stack trace:") {
					return nil, fmt.Errorf("%s%s", err.Error(), extendedEnv.GetStackTrace())
				}
				return nil, err
			}

			if tail, ok := evaluated.(*environment.TailCall); ok {
				function = tail.Function
				args = tail.Arguments
				continue
			}

			// Unwrap any early return values so the whole program doesn't halt
			if retVal, ok := evaluated.(*environment.ReturnValue); ok {
				return retVal.Value, nil
			}

			return evaluated, nil
		case *environment.Builtin:
			result, err := fn.Fn(args...)
			if err != nil {
				return nil, err
			}
			return result, nil

		default:
			return nil, fmt.Errorf("runtime error: not a function")
		}
	}
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

	if env.IsModule {
		return &environment.Number{Value: 0}, nil // or return some ANY value/nil
	}

	return nil, fmt.Errorf("execution error: script finished without a return statement")
}

// evalIdentifier resolves an identifier to its value in the current
// environment. An error is returned if the identifier has not been assigned.
func evalIdentifier(node *syntax.Identifier, env *environment.Environment) (environment.Object, error) {
	if val, ok := env.Get(node.Value); ok {
		return val, nil
	}

	return nil, fmt.Errorf("runtime error: identifier not found: %s", node.Value)

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
		return environment.NativeBoolToBooleanObject(leftVal < rightVal), nil
	case ">":
		return environment.NativeBoolToBooleanObject(leftVal > rightVal), nil
	case "<=":
		return environment.NativeBoolToBooleanObject(leftVal <= rightVal), nil
	case ">=":
		return environment.NativeBoolToBooleanObject(leftVal >= rightVal), nil
	case "==":
		return environment.NativeBoolToBooleanObject(leftVal == rightVal), nil
	case "!=":
		return environment.NativeBoolToBooleanObject(leftVal != rightVal), nil
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

		if result != nil && (result.Type() == environment.RETURN_VALUE_OBJ || result.Type() == environment.TAIL_CALL_OBJ) {
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

// evalExpressions evaluates a list of expressions left-to-right and returns a slice
// of their corresponding evaluated environment Objects.
func evalExpressions(exps []syntax.Expression, env *environment.Environment) ([]environment.Object, error) {
	var result []environment.Object

	for _, e := range exps {
		evaluated, err := Eval(e, env)
		if err != nil {
			return nil, err
		}
		result = append(result, evaluated)
	}

	return result, nil
}

// isTruthy determines if a types.Object value is considered true,
// which is any non-zero value or true boolean.
func isTruthy(val environment.Object) bool {
	if val.Type() == environment.BOOLEAN_OBJ {
		return val.(*environment.Boolean).Value
	}
	return false
}

// loadModule initializes a new environment for a module, evaluates its AST,
// and returns a Module object containing the evaluated environment.
func loadModule(moduleName string, env *environment.Environment) (*environment.Module, error) {
	modEnv := environment.NewEnvironment(env.BaseDir, moduleName, true)
	modEnv.ModuleASTs = env.ModuleASTs

	// Reuse AST cached during semantic analysis phase
	modProgram, ok := env.ModuleASTs[moduleName]
	if !ok {
		return nil, fmt.Errorf("module %s not found on cache", moduleName)
	}

	_, err := Eval(modProgram, modEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate module %s: %s", moduleName, err)
	}

	return &environment.Module{
		Name: moduleName,
		Env:  modEnv,
	}, nil
}
