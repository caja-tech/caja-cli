package environment

import (
	"fmt"
	"strings"
)

var builtins = map[string]*Builtin{
	"len": {
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
		},
	},
	"append": {
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
		},
	},
	"head": {
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
		},
	},
	"tail": {
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
		},
	},
	"last": {
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
		},
	},
	"copy": {
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
		},
	},
	"slice": {
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
		},
	},
	"join": {
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
		},
	},
	"charAt": {
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
		},
	},
	"substring": {
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
		},
	},
	"concat": {
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
		},
	},
	"split": {
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
		},
	},
	"contains": {
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
		},
	},
	"startsWith": {
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
		},
	},
	"endsWith": {
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
		},
	},
	"replace": {
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
		},
	},
	"toUpper": {
		Fn: func(args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'toUpper'. got=%d, want=1", len(args))
			}
			strObj, ok := args[0].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'toUpper' must be STRING, got %s", args[0].Type())
			}

			return &String{Value: strings.ToUpper(strObj.Value)}, nil
		},
	},
	"toLower": {
		Fn: func(args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'toLower'. got=%d, want=1", len(args))
			}
			strObj, ok := args[0].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'toLower' must be STRING, got %s", args[0].Type())
			}

			return &String{Value: strings.ToLower(strObj.Value)}, nil
		},
	},
	"trim": {
		Fn: func(args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'trim'. got=%d, want=1", len(args))
			}
			strObj, ok := args[0].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'trim' must be STRING, got %s", args[0].Type())
			}

			return &String{Value: strings.TrimSpace(strObj.Value)}, nil
		},
	},
	"strlen": {
		Fn: func(args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'strlen'. got=%d, want=1", len(args))
			}
			strObj, ok := args[0].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'strlen' must be STRING, got %s", args[0].Type())
			}

			return &Number{Value: float64(len(strObj.Value))}, nil
		},
	},
}

// GetBuiltinFn retrieves a built-in function by its name, returning the Builtin object and a boolean indicating if it was found.
func GetBuiltinFn(name string) (*Builtin, bool) {
	builtin, ok := builtins[name]
	return builtin, ok
}
