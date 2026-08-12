package environment

import (
	"fmt"
	"time"
)

// newLogModule initializes and returns a builtin "log" module,
// populating it with standard logging functions.
func (e *Environment) newLogModule() *Module {
	moduleName := "log"
	logEnv := NewEnvironment(e.BaseDir, moduleName, true)

	createLogFunc := func(level string) *Builtin {
		return &Builtin{
			Fn: func(args ...Object) (Object, error) {
				if len(args) != 2 {
					return nil, fmt.Errorf("semantic error: wrong number of arguments for '%s'. got=%d, want=2", level, len(args))
				}

				msgObj, ok := args[0].(*String)
				if !ok {
					return nil, fmt.Errorf("semantic error: first argument to '%s' must be STRING, got %s", level, args[0].Type())
				}

				timestamp := time.Now().Format("2006-01-02 15:04:05.000")
				valStr := FormatObject(args[1])

				logPrefix := "Info"
				if level == "warn" {
					logPrefix = "Warning"
				} else if level == "error" {
					logPrefix = "Error"
				}

				output := fmt.Sprintf("%s [%s]: %s: %s", timestamp, logPrefix, msgObj.Value, valStr)
				fmt.Println(output)
				return &String{Value: output}, nil
			},
		}
	}

	logEnv.Set("info", createLogFunc("info"))
	logEnv.Set("warn", createLogFunc("warn"))
	logEnv.Set("error", createLogFunc("error"))

	logEnv.Set("export", &Builtin{
		Fn: func(args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("arity error: expected 1 argument for 'export', got %d", len(args))
			}
			if e.ExportedValues != nil {
				*e.ExportedValues = append(*e.ExportedValues, args[0])
			}
			return nil, nil
		},
	})

	return &Module{
		Name: moduleName,
		Env:  logEnv,
	}
}
