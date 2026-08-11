package environment

import (
	"fmt"
	"math"
)

// newMathModule initializes and returns a builtin "math" module,
// populating it with standard mathematical functions.
func (env *Environment) newMathModule() *Module {
	moduleName := "math"
	mathEnv := NewEnvironment(env.BaseDir, moduleName, true)

	mathEnv.Set("abs", &Builtin{
		Fn: func(args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'abs'. got=%d, want=1", len(args))
			}
			numObj, ok := args[0].(*Number)
			if !ok {
				return nil, fmt.Errorf("semantic error: argument to 'abs' must be NUMBER, got %s", args[0].Type())
			}
			return &Number{Value: math.Abs(numObj.Value)}, nil
		}})

	mathEnv.Set("sqrt", &Builtin{
		Fn: func(args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'sqrt'. got=%d, want=1", len(args))
			}
			numObj, ok := args[0].(*Number)
			if !ok {
				return nil, fmt.Errorf("semantic error: argument to 'sqrt' must be NUMBER, got %s", args[0].Type())
			}
			if numObj.Value < 0 {
				return nil, fmt.Errorf("runtime error: sqrt of negative number is undefined")
			}
			return &Number{Value: math.Sqrt(numObj.Value)}, nil
		}})

	mathEnv.Set("pow", &Builtin{
		Fn: func(args ...Object) (Object, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'pow'. got=%d, want=2", len(args))
			}
			baseObj, ok := args[0].(*Number)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'pow' must be NUMBER, got %s", args[0].Type())
			}
			expObj, ok := args[1].(*Number)
			if !ok {
				return nil, fmt.Errorf("semantic error: second argument to 'pow' must be NUMBER, got %s", args[1].Type())
			}
			return &Number{Value: math.Pow(baseObj.Value, expObj.Value)}, nil
		}})

	mathEnv.Set("floor", &Builtin{
		Fn: func(args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'floor'. got=%d, want=1", len(args))
			}
			numObj, ok := args[0].(*Number)
			if !ok {
				return nil, fmt.Errorf("semantic error: argument to 'floor' must be NUMBER, got %s", args[0].Type())
			}
			return &Number{Value: math.Floor(numObj.Value)}, nil
		}})

	mathEnv.Set("ceil", &Builtin{
		Fn: func(args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'ceil'. got=%d, want=1", len(args))
			}
			numObj, ok := args[0].(*Number)
			if !ok {
				return nil, fmt.Errorf("semantic error: argument to 'ceil' must be NUMBER, got %s", args[0].Type())
			}
			return &Number{Value: math.Ceil(numObj.Value)}, nil
		}})

	mathEnv.Set("round", &Builtin{
		Fn: func(args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'round'. got=%d, want=1", len(args))
			}
			numObj, ok := args[0].(*Number)
			if !ok {
				return nil, fmt.Errorf("semantic error: argument to 'round' must be NUMBER, got %s", args[0].Type())
			}
			return &Number{Value: math.Round(numObj.Value)}, nil
		}})

	mathEnv.Set("min", &Builtin{
		Fn: func(args ...Object) (Object, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'min'. got=%d, want=2", len(args))
			}
			num1Obj, ok := args[0].(*Number)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'min' must be NUMBER, got %s", args[0].Type())
			}
			num2Obj, ok := args[1].(*Number)
			if !ok {
				return nil, fmt.Errorf("semantic error: second argument to 'min' must be NUMBER, got %s", args[1].Type())
			}
			return &Number{Value: math.Min(num1Obj.Value, num2Obj.Value)}, nil
		}})

	mathEnv.Set("max", &Builtin{
		Fn: func(args ...Object) (Object, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'max'. got=%d, want=2", len(args))
			}
			num1Obj, ok := args[0].(*Number)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'max' must be NUMBER, got %s", args[0].Type())
			}
			num2Obj, ok := args[1].(*Number)
			if !ok {
				return nil, fmt.Errorf("semantic error: second argument to 'max' must be NUMBER, got %s", args[1].Type())
			}
			return &Number{Value: math.Max(num1Obj.Value, num2Obj.Value)}, nil
		}})

	mathEnv.Set("log", &Builtin{
		Fn: func(args ...Object) (Object, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'log'. got=%d, want=2", len(args))
			}
			nObj, ok := args[0].(*Number)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'log' must be NUMBER, got %s", args[0].Type())
			}
			bObj, ok := args[1].(*Number)
			if !ok {
				return nil, fmt.Errorf("semantic error: second argument to 'log' must be NUMBER, got %s", args[1].Type())
			}
			
			if nObj.Value <= 0 {
				return nil, fmt.Errorf("runtime error: log of non-positive number is undefined")
			}
			if bObj.Value <= 0 || bObj.Value == 1 {
				return nil, fmt.Errorf("runtime error: log base must be positive and not equal to 1")
			}
			
			return &Number{Value: math.Log(nObj.Value) / math.Log(bObj.Value)}, nil
		}})

	mathEnv.Set("PI", &Number{Value: math.Pi})
	mathEnv.Set("E", &Number{Value: math.E})
	mathEnv.Set("SQRT2", &Number{Value: math.Sqrt2})
	mathEnv.Set("LN2", &Number{Value: math.Ln2})
	mathEnv.Set("LN10", &Number{Value: math.Ln10})
	mathEnv.Set("LOG2E", &Number{Value: math.Log2E})
	mathEnv.Set("LOG10E", &Number{Value: math.Log10E})

	return &Module{
		Name: moduleName,
		Env:  mathEnv,
	}
}
