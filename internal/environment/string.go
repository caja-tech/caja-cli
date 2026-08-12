package environment

import (
	"fmt"
	"strings"
)

// newStringModule initializes and returns a builtin "string" module,
// populating it with standard string manipulation functions.
func (e *Environment) newStringModule() *Module {
	moduleName := "string"
	stringEnv := NewEnvironment(e.BaseDir, moduleName, true)

	stringEnv.Set("charAt", &Builtin{
		Fn: func(args ...Object) (Object, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'charAt'. got=%d, want=2", len(args))
			}
			strObj, ok := args[0].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'charAt' must be STRING, got %s", args[0].Type())
			}
			idxObj, ok := args[1].(*Number)
			if !ok {
				return nil, fmt.Errorf("semantic error: second argument to 'charAt' must be NUMBER, got %s", args[1].Type())
			}

			str := strObj.Value
			idx := int(idxObj.Value)

			if idx < 0 || idx >= len(str) {
				return nil, fmt.Errorf("runtime error: charAt index %d out of bounds for string of length %d", idx, len(str))
			}

			return &String{Value: string(str[idx])}, nil
		}})

	stringEnv.Set("substring", &Builtin{
		Fn: func(args ...Object) (Object, error) {
			if len(args) != 3 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'substring'. got=%d, want=3", len(args))
			}
			strObj, ok := args[0].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'substring' must be STRING, got %s", args[0].Type())
			}
			startObj, ok := args[1].(*Number)
			if !ok {
				return nil, fmt.Errorf("semantic error: second argument to 'substring' must be NUMBER, got %s", args[1].Type())
			}
			endObj, ok := args[2].(*Number)
			if !ok {
				return nil, fmt.Errorf("semantic error: third argument to 'substring' must be NUMBER, got %s", args[2].Type())
			}

			str := strObj.Value
			start := int(startObj.Value)
			end := int(endObj.Value)

			if start < 0 || start > len(str) {
				return nil, fmt.Errorf("runtime error: substring start index %d out of bounds for string of length %d", start, len(str))
			}
			if end < 0 || end > len(str) {
				return nil, fmt.Errorf("runtime error: substring end index %d out of bounds for string of length %d", end, len(str))
			}
			if start > end {
				return nil, fmt.Errorf("runtime error: substring start index %d is greater than end index %d", start, end)
			}

			return &String{Value: str[start:end]}, nil
		}})

	stringEnv.Set("concat", &Builtin{
		Fn: func(args ...Object) (Object, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'concat'. got=%d, want=2", len(args))
			}
			str1Obj, ok := args[0].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'concat' must be STRING, got %s", args[0].Type())
			}
			str2Obj, ok := args[1].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: second argument to 'concat' must be STRING, got %s", args[1].Type())
			}

			return &String{Value: str1Obj.Value + str2Obj.Value}, nil
		}})

	stringEnv.Set("split", &Builtin{
		Fn: func(args ...Object) (Object, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'split'. got=%d, want=2", len(args))
			}
			strObj, ok := args[0].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'split' must be STRING, got %s", args[0].Type())
			}
			delimObj, ok := args[1].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: second argument to 'split' must be STRING, got %s", args[1].Type())
			}

			if len(delimObj.Value) != 1 {
				return nil, fmt.Errorf("runtime error: split delimiter must be a single character string")
			}

			parts := strings.Split(strObj.Value, delimObj.Value)
			elements := make([]Object, len(parts))
			for i, p := range parts {
				elements[i] = &String{Value: p}
			}
			return &Array{Elements: elements}, nil
		}})

	stringEnv.Set("contains", &Builtin{
		Fn: func(args ...Object) (Object, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'contains'. got=%d, want=2", len(args))
			}
			strObj, ok := args[0].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'contains' must be STRING, got %s", args[0].Type())
			}
			subObj, ok := args[1].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: second argument to 'contains' must be STRING, got %s", args[1].Type())
			}

			return &Boolean{Value: strings.Contains(strObj.Value, subObj.Value)}, nil
		}})

	stringEnv.Set("startsWith", &Builtin{
		Fn: func(args ...Object) (Object, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'startsWith'. got=%d, want=2", len(args))
			}
			strObj, ok := args[0].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'startsWith' must be STRING, got %s", args[0].Type())
			}
			prefixObj, ok := args[1].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: second argument to 'startsWith' must be STRING, got %s", args[1].Type())
			}

			return &Boolean{Value: strings.HasPrefix(strObj.Value, prefixObj.Value)}, nil
		}})

	stringEnv.Set("endsWith", &Builtin{
		Fn: func(args ...Object) (Object, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'endsWith'. got=%d, want=2", len(args))
			}
			strObj, ok := args[0].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'endsWith' must be STRING, got %s", args[0].Type())
			}
			suffixObj, ok := args[1].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: second argument to 'endsWith' must be STRING, got %s", args[1].Type())
			}

			return &Boolean{Value: strings.HasSuffix(strObj.Value, suffixObj.Value)}, nil
		}})

	stringEnv.Set("replace", &Builtin{
		Fn: func(args ...Object) (Object, error) {
			if len(args) != 3 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'replace'. got=%d, want=3", len(args))
			}
			strObj, ok := args[0].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'replace' must be STRING, got %s", args[0].Type())
			}
			oldObj, ok := args[1].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: second argument to 'replace' must be STRING, got %s", args[1].Type())
			}
			newObj, ok := args[2].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: third argument to 'replace' must be STRING, got %s", args[2].Type())
			}

			return &String{Value: strings.ReplaceAll(strObj.Value, oldObj.Value, newObj.Value)}, nil
		}})

	stringEnv.Set("toUpper", &Builtin{
		Fn: func(args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'toUpper'. got=%d, want=1", len(args))
			}
			strObj, ok := args[0].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'toUpper' must be STRING, got %s", args[0].Type())
			}

			return &String{Value: strings.ToUpper(strObj.Value)}, nil
		}})

	stringEnv.Set("toLower", &Builtin{
		Fn: func(args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'toLower'. got=%d, want=1", len(args))
			}
			strObj, ok := args[0].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'toLower' must be STRING, got %s", args[0].Type())
			}

			return &String{Value: strings.ToLower(strObj.Value)}, nil
		}})

	stringEnv.Set("trim", &Builtin{
		Fn: func(args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'trim'. got=%d, want=1", len(args))
			}
			strObj, ok := args[0].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'trim' must be STRING, got %s", args[0].Type())
			}

			return &String{Value: strings.TrimSpace(strObj.Value)}, nil
		}})

	stringEnv.Set("len", &Builtin{
		Fn: func(args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'strlen'. got=%d, want=1", len(args))
			}
			strObj, ok := args[0].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'strlen' must be STRING, got %s", args[0].Type())
			}

			return &Number{Value: float64(len(strObj.Value))}, nil
		}})

	return &Module{
		Name: moduleName,
		Env:  stringEnv,
	}
}
