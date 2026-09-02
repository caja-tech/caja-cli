package evaluator

import (
	"caja-cli/internal/pipeline/ast"
	"caja-cli/internal/pipeline/environment"
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
func Eval(n ast.Node, env *environment.Environment) (environment.Object, error) {
	switch node := n.(type) {
	case *ast.Program:
		return evalProgram(node, env)
	case *ast.ExpressionStatement:
		return evalExpressionStatement(node, env)
	case *ast.AssignStatement:
		return evalAssignStatement(node, env)
	case *ast.IndexAssignmentStatement:
		return evalIndexAssignmentStatement(node, env)
	case *ast.PropertyAssignmentStatement:
		return evalPropertyAssignmentStatement(node, env)
	case *ast.NumberLiteral:
		return evalNumberLiteral(node)
	case *ast.StringLiteral:
		return evalStringLiteral(node)
	case *ast.DateLiteral:
		return evalDateLiteral(node)
	case *ast.BooleanLiteral:
		return evalBooleanLiteral(node)
	case *ast.NilLiteral:
		return environment.NullObj, nil
	case *ast.ArrayLiteral:
		return evalArrayLiteral(node, env)
	case *ast.MapLiteral:
		return evalMapLiteral(node, env)
	case *ast.IndexExpression:
		return evalIndexExpression(node, env)
	case *ast.Identifier:
		return evalIdentifier(node, env)
	case *ast.ReturnStatement:
		return evalReturnStatement(node, env)
	case *ast.LetStatement:
		return evalLetStatement(node, env)
	case *ast.ConstStatement:
		return evalConstStatement(node, env)
	case *ast.IfExpression:
		return evalIfExpression(node, env)
	case *ast.BlockStatement:
		return evalBlockStatement(node, env)
	case *ast.InfixExpression:
		return evalInfixExpressionNode(node, env)
	case *ast.PrefixExpression:
		return evalPrefixExpressionNode(node, env)
	case *ast.TypeAliasStatement:
		return nil, nil
	case *ast.TypeConstraintStatement:
		return evalTypeConstraintStatement(node, env)
	case *ast.FunctionLiteral:
		return evalFunctionLiteral(node, env)
	case *ast.SafePipeExpression:
		left, err := Eval(node.Left, env)
		if err != nil { return nil, err }
		if left == nil || left.Type() == environment.NULL_OBJ {
			return environment.NullObj, nil
		}
		return evalSafePipeCall(node, left, env)
	case *ast.CallExpression:
		return evalCallExpression(node, env)
	case *ast.ImportStatement:
		return evalImportStatement(node, env)
	case *ast.PropertyExpression:
		return evalPropertyExpression(node, env)
	case *ast.StructLiteral:
		return evalStructLiteral(node, env)
	}

	return nil, fmt.Errorf("unknown node type: %T", n)
}

// evalExpressionStatement evaluates the inner expression of an expression statement.
func evalExpressionStatement(node *ast.ExpressionStatement, env *environment.Environment) (environment.Object, error) {
	return Eval(node.Expression, env)
}

// evalIndexAssignmentStatement evaluates the index, validates bounds, and updates the array element.
func evalIndexAssignmentStatement(node *ast.IndexAssignmentStatement, env *environment.Environment) (environment.Object, error) {
	left, err := Eval(node.Left, env)
	if err != nil {
		return nil, err
	}

	idxObj, err := Eval(node.Index, env)
	if err != nil {
		return nil, err
	}

	val, err := Eval(node.Value, env)
	if err != nil {
		return nil, err
	}

	if left.Type() == environment.NULL_OBJ {
		return nil, fmt.Errorf("null pointer exception: cannot assign to index of nil")
	}

	if arr, ok := left.(*environment.Array); ok {
		if idxObj.Type() != environment.NUMBER_OBJ {
			return nil, fmt.Errorf("array index must be NUMBER, got %s", idxObj.Type())
		}
		idx := int(idxObj.(*environment.Number).Value)

		if idx < 0 || idx >= len(arr.Elements) {
			return nil, fmt.Errorf("runtime error: array index out of bounds")
		}

		arr.Elements[idx] = val
		return val, nil
	}

	if mapObj, ok := left.(*environment.Map); ok {
		hash, err := hashKey(idxObj)
		if err != nil {
			return nil, err
		}

		mapObj.Pairs[hash] = environment.MapPair{Key: idxObj, Value: val}
		return val, nil
	}

	return nil, fmt.Errorf("index assignment not supported for %s", left.Type())
}

// evalPropertyAssignmentStatement evaluates the property assignment, updates the object, and returns the assigned value.
func evalPropertyAssignmentStatement(node *ast.PropertyAssignmentStatement, env *environment.Environment) (environment.Object, error) {
	left, err := Eval(node.Object, env)
	if err != nil {
		return nil, err
	}

	valObj, err := Eval(node.Value, env)
	if err != nil {
		return nil, err
	}

	if left.Type() == environment.NULL_OBJ {
		if node.Safe {
			return environment.NullObj, nil
		}
		return nil, fmt.Errorf("null pointer exception: cannot assign property '%s' of nil", node.Property.Value)
	}

	if mod, ok := left.(*environment.Module); ok {
		mod.Env.Set(node.Property.Value, valObj)
		return valObj, nil
	} else if structObj, ok := left.(*environment.StructObject); ok {
		structObj.Fields[node.Property.Value] = valObj
		return valObj, nil
	}

	return nil, fmt.Errorf("type error: property assignment not supported for %s", left.Type())
}

// evalAssignStatement evaluates the assigned value and updates the existing variable in the environment.
func evalAssignStatement(node *ast.AssignStatement, env *environment.Environment) (environment.Object, error) {
	val, err := Eval(node.Value, env)
	if err != nil {
		return nil, err
	}
	env.Assign(node.Name.Value, val)
	return val, nil
}

// evalNumberLiteral returns a Number object representing the literal's value.
func evalNumberLiteral(node *ast.NumberLiteral) (environment.Object, error) {
	return &environment.Number{Value: node.Value}, nil
}

// evalStringLiteral returns a String object representing the literal's value.
func evalStringLiteral(node *ast.StringLiteral) (environment.Object, error) {
	return &environment.String{Value: node.Value}, nil
}

// evalDateLiteral parses the DateLiteral string value as 'YYYY-MM-DD' and returns
// a Date object representing the parsed time.
func evalDateLiteral(node *ast.DateLiteral) (environment.Object, error) {
	parsedTime, _ := time.Parse("2006-01-02", node.Value)

	return &environment.Date{Value: parsedTime}, nil
}

// evalBooleanLiteral returns a Boolean object representing the literal's value.
func evalBooleanLiteral(node *ast.BooleanLiteral) (environment.Object, error) {
	return environment.NativeBoolToBooleanObject(node.Value), nil
}

// evalArrayLiteral evaluates all expressions within an array literal and returns
// an Array object containing the evaluated elements.
func evalArrayLiteral(node *ast.ArrayLiteral, env *environment.Environment) (environment.Object, error) {
	elements, err := evalExpressions(node.Elements, env)
	if err != nil {
		return nil, err
	}

	return &environment.Array{Elements: elements}, nil
}

// evalImportStatement evaluates an import by using the environment's ImportLoader.
func evalImportStatement(node *ast.ImportStatement, env *environment.Environment) (environment.Object, error) {
	modPath := node.Path
	modName := node.Name.Value

	var moduleObj environment.Object

	if cached, ok := env.ModuleCache[modPath]; ok {
		moduleObj = cached
	} else if stdMod := env.GetStandardModule(modPath); stdMod != nil {
		env.ModuleCache[modPath] = stdMod
		moduleObj = stdMod
	} else {
		if env.Loading[modPath] {
			return nil, fmt.Errorf("circular import detected: %s", modPath)
		}

		env.Loading[modPath] = true
		defer func() { env.Loading[modPath] = false }()

		loadedMod, err := loadModule(modPath, env)
		if err != nil {
			return nil, err
		}

		env.ModuleCache[modPath] = loadedMod
		moduleObj = loadedMod
	}

	env.Set(modName, moduleObj)

	if len(node.NamedImports) > 0 {
		if modMap, ok := moduleObj.(*environment.Module); ok {
			for _, named := range node.NamedImports {
				if val, exists := modMap.Env.Get(named.Value); exists {
					env.Set(named.Value, val)
				}
			}
		}
	}

	return moduleObj, nil
}

// evalPropertyExpression retrieves a property from a module object.
func evalPropertyExpression(node *ast.PropertyExpression, env *environment.Environment) (environment.Object, error) {
	obj, err := Eval(node.Object, env)
	if err != nil {
		return nil, err
	}

	if obj.Type() == environment.NULL_OBJ {
		if node.Safe {
			return environment.NullObj, nil
		}
		return nil, fmt.Errorf("null pointer exception: cannot read property '%s' of nil", node.Property.Value)
	}

	if module, ok := obj.(*environment.Module); ok {
		val, ok := module.Env.Get(node.Property.Value)
		if !ok {
			return nil, fmt.Errorf("runtime error: property '%s' not found in module '%s'", node.Property.Value, module.Name)
		}

		if module.Env.IsPrivate(node.Property.Value) {
			return nil, fmt.Errorf("runtime error: property '%s' is private and cannot be accessed from outside module '%s'", node.Property.Value, module.Name)
		}

		return val, nil
	} else if structObj, ok := obj.(*environment.StructObject); ok {
		val, ok := structObj.Fields[node.Property.Value]
		if !ok {
			return nil, fmt.Errorf("runtime error: property '%s' not found in struct '%s'", node.Property.Value, structObj.StructName)
		}
		return val, nil
	}

	return nil, fmt.Errorf("runtime error: property access is only supported on modules and structs")
}

func hashKey(obj environment.Object) (string, error) {
	if obj.Type() == environment.STRING_OBJ {
		return obj.(*environment.String).Value, nil
	}
	if obj.Type() == environment.NUMBER_OBJ {
		return fmt.Sprintf("%f", obj.(*environment.Number).Value), nil
	}
	if structObj, ok := obj.(*environment.StructObject); ok {
		if keyObj, exists := structObj.Fields["key"]; exists {
			if fn, ok := keyObj.(*environment.Function); ok {
				extendedEnv := environment.NewEnclosedEnvironment(fn.Env)
				evaluated, err := Eval(fn.Body, extendedEnv)
				if err != nil {
					return "", err
				}
				if rv, ok := evaluated.(*environment.ReturnValue); ok {
					evaluated = rv.Value
				}
				if strRes, ok := evaluated.(*environment.String); ok {
					return strRes.Value, nil
				}
			}
		}
	}
	return "", fmt.Errorf("unusable as map key: %s", obj.Type())
}

func evalMapLiteral(node *ast.MapLiteral, env *environment.Environment) (environment.Object, error) {
	pairs := make(map[string]environment.MapPair)

	for keyNode, valueNode := range node.Pairs {
		keyObj, err := Eval(keyNode, env)
		if err != nil {
			return nil, err
		}

		hash, err := hashKey(keyObj)
		if err != nil {
			return nil, err
		}

		valObj, err := Eval(valueNode, env)
		if err != nil {
			return nil, err
		}

		pairs[hash] = environment.MapPair{Key: keyObj, Value: valObj}
	}

	return &environment.Map{Pairs: pairs}, nil
}

// evalIndexExpression evaluates an array index operation, ensuring the left side
// is an array and the index is a number within bounds, returning the requested element.
func evalIndexExpression(node *ast.IndexExpression, env *environment.Environment) (environment.Object, error) {
	left, err := Eval(node.Left, env)
	if err != nil {
		return nil, err
	}

	index, err := Eval(node.Index, env)
	if err != nil {
		return nil, err
	}

	if left.Type() == environment.NULL_OBJ {
		return nil, fmt.Errorf("null pointer exception: cannot read index of nil")
	}

	if arrayObj, ok := left.(*environment.Array); ok {
		indexObj, okIndex := index.(*environment.Number)
		if !okIndex {
			return nil, fmt.Errorf("runtime error: array index must be NUMBER")
		}
		idx := int(indexObj.Value)
		if idx < 0 || idx >= len(arrayObj.Elements) {
			return nil, fmt.Errorf("runtime error: array index out of bounds")
		}
		return arrayObj.Elements[idx], nil
	}

	if mapObj, ok := left.(*environment.Map); ok {
		hash, err := hashKey(index)
		if err != nil {
			return nil, err
		}
		pair, exists := mapObj.Pairs[hash]
		if !exists {
			return environment.NullObj, nil
		}
		return pair.Value, nil
	}

	return nil, fmt.Errorf("runtime error: index operator not supported for type %s", left.Type())
}

// evalReturnStatement evaluates the returned expression and wraps it in a ReturnValue object.
func evalReturnStatement(node *ast.ReturnStatement, env *environment.Environment) (environment.Object, error) {
	currentFuncName := ""
	if len(env.CallStack) > 0 {
		currentFuncName = env.CallStack[len(env.CallStack)-1].FuncName
	}

	if callNode, ok := node.ReturnValue.(*ast.CallExpression); ok {
		if ident, ok := callNode.Function.(*ast.Identifier); ok && ident.Value == currentFuncName && currentFuncName != "" {
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

	if node.ReturnValue == nil {
		return &environment.ReturnValue{Value: &environment.StructObject{StructName: "Nothing", Fields: make(map[string]environment.Object)}}, nil
	}

	val, err := Eval(node.ReturnValue, env)
	if err != nil {
		return nil, err
	}
	return &environment.ReturnValue{Value: val}, nil
}

// evalLetStatement evaluates the assigned value and creates a new variable in the environment.
func evalLetStatement(node *ast.LetStatement, env *environment.Environment) (environment.Object, error) {
	val, err := Eval(node.Value, env)
	if err != nil {
		return nil, err
	}

	if node.ValueType != "" {
		if err := checkTypeMatches(node.ValueType, val); err != nil {
			return nil, err
		}
		val, err = enforceTypeConstraint(node.ValueType, val, env)
		if err != nil {
			return nil, err
		}
	}

	env.Set(node.Name.Value, val)
	if node.IsPrivate {
		env.MarkPrivate(node.Name.Value)
	}
	return val, nil
}

// evalConstStatement evaluates the assigned value and creates a new constant in the environment.
func evalConstStatement(node *ast.ConstStatement, env *environment.Environment) (environment.Object, error) {
	val, err := Eval(node.Value, env)
	if err != nil {
		return nil, err
	}

	if node.ValueType != "" {
		if err := checkTypeMatches(node.ValueType, val); err != nil {
			return nil, err
		}
	}

	env.Set(node.Name.Value, val)
	if node.IsPrivate {
		env.MarkPrivate(node.Name.Value)
	}
	return val, nil
}

func checkTypeMatches(expected string, obj environment.Object) error {
	if expected == "" {
		return nil
	}
	if obj.Type() == environment.NULL_OBJ {
		return nil
	}

	var expectedObjType environment.ObjectType
	switch expected {
	case "Number":
		expectedObjType = environment.NUMBER_OBJ
	case "String":
		expectedObjType = environment.STRING_OBJ
	case "Boolean":
		expectedObjType = environment.BOOLEAN_OBJ
	case "Date":
		expectedObjType = environment.DATE_OBJ
	case "Function":
		expectedObjType = environment.FUNCTION_OBJ
	default:
		if len(expected) > 0 && expected[0] == '[' && expected[len(expected)-1] == ']' {
			expectedObjType = environment.ARRAY_OBJ
		} else {
			// Other custom types or modules
			return nil
		}
	}

	if obj.Type() != expectedObjType {
		return fmt.Errorf("type mismatch: expected %s, got %s", expected, obj.Type())
	}
	return nil
}

// evalInfixExpressionNode evaluates the left and right operands and applies the operator.
func evalInfixExpressionNode(node *ast.InfixExpression, env *environment.Environment) (environment.Object, error) {
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

	if node.Operator == "==" || node.Operator == "!=" {
		var isEqual bool
		if left == nil && right == nil {
			isEqual = true
		} else if left == nil || right == nil {
			isEqual = false
		} else if left.Type() == right.Type() {
			switch left.Type() {
			case environment.NUMBER_OBJ:
				isEqual = left.(*environment.Number).Value == right.(*environment.Number).Value
			case environment.STRING_OBJ:
				isEqual = left.(*environment.String).Value == right.(*environment.String).Value
			case environment.BOOLEAN_OBJ:
				isEqual = left.(*environment.Boolean).Value == right.(*environment.Boolean).Value
			case environment.NULL_OBJ:
				isEqual = true
			default:
				isEqual = left == right
			}
		} else {
			// different types
			if left.Type() == environment.NULL_OBJ && right.Type() == environment.NULL_OBJ {
				isEqual = true
			} else {
				isEqual = false
			}
		}

		if node.Operator == "==" {
			return environment.NativeBoolToBooleanObject(isEqual), nil
		}
		return environment.NativeBoolToBooleanObject(!isEqual), nil
	}

	return evalInfixExpression(node.Operator, left, right)
}

// evalBooleanInfixExpression applies a logical operator (and, or, xor) to
// the left and right boolean operands, returning the resulting boolean.
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
func evalPrefixExpressionNode(node *ast.PrefixExpression, env *environment.Environment) (environment.Object, error) {
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
func evalFunctionLiteral(node *ast.FunctionLiteral, env *environment.Environment) (environment.Object, error) {
	params := node.Parameters
	body := node.Body
	return &environment.Function{Parameters: params, Env: env, Body: body, ReturnType: node.ReturnType}, nil
}

// evalCallExpression evaluates the function and its arguments, then executes the function body in an extended environment.
func evalCallExpression(node *ast.CallExpression, env *environment.Environment) (environment.Object, error) {
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
			if ident, ok := node.Function.(*ast.Identifier); ok {
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

			if fn.ReturnType == "Nothing" {
				return &environment.StructObject{StructName: "Nothing", Fields: make(map[string]environment.Object)}, nil
			}

			return evaluated, nil
		case *environment.Builtin:
			if fn.Name == "map.containsKey" {
				if len(args) != 2 {
					return nil, fmt.Errorf("arity error: expected 2 arguments for 'containsKey', got %d", len(args))
				}
				mapObj, ok := args[0].(*environment.Map)
				if !ok {
					return nil, fmt.Errorf("type error: first argument to 'containsKey' must be MAP, got %s", args[0].Type())
				}
				keyObj := args[1]
				hash, err := hashKey(keyObj)
				if err != nil {
					return nil, err
				}
				_, exists := mapObj.Pairs[hash]
				return &environment.Boolean{Value: exists}, nil
			}

			if fn.Name == "map.delete" {
				if len(args) != 2 {
					return nil, fmt.Errorf("arity error: expected 2 arguments for 'delete', got %d", len(args))
				}
				mapObj, ok := args[0].(*environment.Map)
				if !ok {
					return nil, fmt.Errorf("type error: first argument to 'delete' must be MAP, got %s", args[0].Type())
				}
				keyObj := args[1]
				hash, err := hashKey(keyObj)
				if err != nil {
					return nil, err
				}
				
				newPairs := make(map[string]environment.MapPair)
				for k, v := range mapObj.Pairs {
					newPairs[k] = v
				}
				delete(newPairs, hash)
				
				return &environment.Map{Pairs: newPairs}, nil
			}

			result, err := fn.Fn(env, args...)
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
func evalProgram(program *ast.Program, env *environment.Environment) (environment.Object, error) {
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
func evalIdentifier(node *ast.Identifier, env *environment.Environment) (environment.Object, error) {
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
func evalBlockStatement(block *ast.BlockStatement, env *environment.Environment) (environment.Object, error) {
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
func evalIfExpression(ie *ast.IfExpression, env *environment.Environment) (environment.Object, error) {
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
func evalExpressions(exps []ast.Expression, env *environment.Environment) ([]environment.Object, error) {
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

// evalStructLiteral evaluates all field expressions within a struct literal
// and constructs a StructObject holding the evaluated field values.
func evalStructLiteral(node *ast.StructLiteral, env *environment.Environment) (environment.Object, error) {
	fields := make(map[string]environment.Object)

	for name, expr := range node.Fields {
		val, err := Eval(expr, env)
		if err != nil {
			return nil, err
		}
		fields[name] = val
	}

	return &environment.StructObject{
		StructName: node.StructName,
		Fields:     fields,
	}, nil
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

func evalTypeConstraintStatement(node *ast.TypeConstraintStatement, env *environment.Environment) (environment.Object, error) {
	predicateObj, err := Eval(node.Predicate, env)
	if err != nil {
		return nil, err
	}
	env.SetTypeConstraint(node.Name.Value, predicateObj)
	return nil, nil
}

func enforceTypeConstraint(typeName string, val environment.Object, env *environment.Environment) (environment.Object, error) {
	if val == nil || val.Type() == environment.NULL_OBJ {
		return val, nil
	}
	isNullable := false
	if len(typeName) > 0 && typeName[len(typeName)-1] == '?' {
		isNullable = true
		typeName = typeName[:len(typeName)-1]
	}
	
	predicate, ok := env.GetTypeConstraint(typeName)
	if !ok {
		return val, nil
	}
	
	var res environment.Object
	
	switch fn := predicate.(type) {
	case *environment.Function:
		extendedEnv := environment.NewEnclosedEnvironment(fn.Env)
		extendedEnv.Set(fn.Parameters[0].Name, val)
		var err error
		res, err = Eval(fn.Body, extendedEnv)
		if err != nil {
			return nil, err
		}
		// Extract return value if it's a ReturnValue object
		if returnValue, ok := res.(*environment.ReturnValue); ok {
			res = returnValue.Value
		}
	default:
		return nil, fmt.Errorf("type error: constraint predicate for '%s' is not a valid function", typeName)
	}
	
	if boolRes, ok := res.(*environment.Boolean); ok {
		if !boolRes.Value {
			if isNullable {
				return environment.NullObj, nil
			}
			return nil, fmt.Errorf("type constraint error: value does not satisfy the constraint for type '%s'", typeName)
		}
	} else {
		return nil, fmt.Errorf("type error: constraint predicate for '%s' did not return a Boolean", typeName)
	}
	
	return val, nil
}

func evalSafePipeCall(node *ast.SafePipeExpression, leftObj environment.Object, env *environment.Environment) (environment.Object, error) {
	dummyName := "___safe_pipe_dummy___"
	env.Set(dummyName, leftObj)
	orig := node.Call.Arguments[0]
	node.Call.Arguments[0] = &ast.Identifier{Value: dummyName}
	res, err := evalCallExpression(node.Call, env)
	node.Call.Arguments[0] = orig
	return res, err
}
