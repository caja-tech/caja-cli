package environment

import "fmt"

// newArrayModule initializes and returns a builtin "array" module,
// populating it with standard array manipulation functions.
func (env *Environment) newArrayModule() *Module {
	moduleName := "array"
	arrayEnv := NewEnvironment(env.BaseDir, moduleName, true)

	arrayEnv.Set("len", &Builtin{
		Fn: func(args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for len. got=%d, want=1", len(args))
			}

			switch arg := args[0].(type) {
			case *Array:
				return &Number{Value: float64(len(arg.Elements))}, nil
			default:
				return nil, fmt.Errorf("semantic error: argument to 'len' not supported, got %s", args[0].Type())
			}
		}})

	arrayEnv.Set("append", &Builtin{
		Fn: func(args ...Object) (Object, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for append. got=%d, want=2", len(args))
			}
			arr, ok := args[0].(*Array)
			if !ok {
				return nil, fmt.Errorf("semantic error: argument to 'append' must be ARRAY, got %s", args[0].Type())
			}
			newElements := make([]Object, len(arr.Elements), len(arr.Elements)+1)
			copy(newElements, arr.Elements)
			newElements = append(newElements, args[1])
			return &Array{Elements: newElements}, nil
		}})

	arrayEnv.Set("head", &Builtin{
		Fn: func(args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for head. got=%d, want=1", len(args))
			}
			arr, ok := args[0].(*Array)
			if !ok {
				return nil, fmt.Errorf("semantic error: argument to 'head' must be ARRAY, got %s", args[0].Type())
			}
			if len(arr.Elements) > 0 {
				return arr.Elements[0], nil
			}
			return nil, fmt.Errorf("runtime error: head on empty array")
		}})

	arrayEnv.Set("tail", &Builtin{
		Fn: func(args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for tail. got=%d, want=1", len(args))
			}
			arr, ok := args[0].(*Array)
			if !ok {
				return nil, fmt.Errorf("semantic error: argument to 'tail' must be ARRAY, got %s", args[0].Type())
			}
			if len(arr.Elements) > 0 {
				newElements := make([]Object, len(arr.Elements)-1)
				copy(newElements, arr.Elements[1:])
				return &Array{Elements: newElements}, nil
			}
			return &Array{Elements: []Object{}}, nil
		}})

	arrayEnv.Set("last", &Builtin{
		Fn: func(args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for last. got=%d, want=1", len(args))
			}
			arr, ok := args[0].(*Array)
			if !ok {
				return nil, fmt.Errorf("semantic error: argument to 'last' must be ARRAY, got %s", args[0].Type())
			}
			if len(arr.Elements) > 0 {
				return arr.Elements[len(arr.Elements)-1], nil
			}
			return nil, fmt.Errorf("runtime error: last on empty array")
		}})

	arrayEnv.Set("copy", &Builtin{
		Fn: func(args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for copy. got=%d, want=1", len(args))
			}
			arr, ok := args[0].(*Array)
			if !ok {
				return nil, fmt.Errorf("semantic error: argument to 'copy' must be ARRAY, got %s", args[0].Type())
			}
			newElements := make([]Object, len(arr.Elements))
			copy(newElements, arr.Elements)
			return &Array{Elements: newElements}, nil
		}})

	arrayEnv.Set("slice", &Builtin{
		Fn: func(args ...Object) (Object, error) {
			if len(args) != 3 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for slice. got=%d, want=3", len(args))
			}
			arr, ok := args[0].(*Array)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'slice' must be ARRAY, got %s", args[0].Type())
			}
			startNum, ok := args[1].(*Number)
			if !ok {
				return nil, fmt.Errorf("semantic error: second argument to 'slice' must be NUMBER, got %s", args[1].Type())
			}
			endNum, ok := args[2].(*Number)
			if !ok {
				return nil, fmt.Errorf("semantic error: third argument to 'slice' must be NUMBER, got %s", args[2].Type())
			}

			start := int(startNum.Value)
			end := int(endNum.Value)

			if start < 0 || start > len(arr.Elements) {
				return nil, fmt.Errorf("runtime error: slice start index %d out of bounds for array of length %d", start, len(arr.Elements))
			}
			if end < 0 || end > len(arr.Elements) {
				return nil, fmt.Errorf("runtime error: slice end index %d out of bounds for array of length %d", end, len(arr.Elements))
			}
			if start > end {
				return nil, fmt.Errorf("runtime error: slice start index %d is greater than end index %d", start, end)
			}

			newElements := make([]Object, end-start)
			copy(newElements, arr.Elements[start:end])
			return &Array{Elements: newElements}, nil
		}})

	arrayEnv.Set("join", &Builtin{
		Fn: func(args ...Object) (Object, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'join'. got=%d, want=2", len(args))
			}
			arr1, ok := args[0].(*Array)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'join' must be ARRAY, got %s", args[0].Type())
			}
			arr2, ok := args[1].(*Array)
			if !ok {
				return nil, fmt.Errorf("semantic error: second argument to 'join' must be ARRAY, got %s", args[1].Type())
			}

			newElements := make([]Object, len(arr1.Elements)+len(arr2.Elements))
			copy(newElements, arr1.Elements)
			copy(newElements[len(arr1.Elements):], arr2.Elements)
			return &Array{Elements: newElements}, nil
		}})

	return &Module{
		Name: moduleName,
		Env:  arrayEnv,
	}
}
