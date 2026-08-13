package environment

import (
	"caja-cli/internal/pipeline/ast"
	"fmt"
	"strings"
)

// StackFrame represents a single frame in the runtime call stack,
// storing the function name, arguments, and source location.
type StackFrame struct {
	FuncName string
	Line     int
	Column   int
	Args     []string
}

// EnvConfig holds the configuration for an environment,
// such as its base directory and whether it represents a module.
type EnvConfig struct {
	BaseDir  string
	FileName string
	IsModule bool
}

// EnvRegistry maintains the runtime state of an environment,
// including variables, the call stack, and cached modules.
type EnvRegistry struct {
	store          map[string]Object
	CallStack      []StackFrame
	ModuleCache    map[string]*Module
	Loading        map[string]bool
	ModuleASTs     map[string]*ast.Program // ASTs parsed during semantic analysis
	ExportedValues *[]Object
	privates       map[string]bool
}

// Environment provides a symbol table to store and retrieve variables
// during evaluation. It supports variable shadowing and scope resolution
// through a reference to an outer enclosing
type Environment struct {
	outer *Environment
	EnvConfig
	EnvRegistry
}

// NewEnvironment creates and returns a new top-level environment with
// no outer scope.
func NewEnvironment(baseDir string, fileName string, isModule bool) *Environment {
	exportedValues := make([]Object, 0)
	return &Environment{
		outer: nil,
		EnvConfig: EnvConfig{
			BaseDir:  baseDir,
			FileName: fileName,
			IsModule: isModule,
		},
		EnvRegistry: EnvRegistry{
			store:          make(map[string]Object),
			CallStack:      []StackFrame{},
			ModuleCache:    make(map[string]*Module),
			Loading:        make(map[string]bool),
			ModuleASTs:     make(map[string]*ast.Program),
			ExportedValues: &exportedValues,
			privates:       make(map[string]bool),
		},
	}
}

// NewEnclosedEnvironment creates and returns a new environment that extends
// the given outer  This is typically used for block scopes.
func NewEnclosedEnvironment(outer *Environment) *Environment {
	env := &Environment{
		outer: outer,
		EnvRegistry: EnvRegistry{
			store: make(map[string]Object),
		},
	}
	if outer != nil {
		env.BaseDir = outer.BaseDir
		env.FileName = outer.FileName
		env.IsModule = outer.IsModule
		env.CallStack = outer.CallStack
		env.ModuleCache = outer.ModuleCache
		env.Loading = outer.Loading
		env.ModuleASTs = outer.ModuleASTs
		env.ExportedValues = outer.ExportedValues
		env.privates = outer.privates
	}
	return env
}

// Get retrieves the value of a variable by its name. It searches the
// current environment first, then recursively searches the outer environment
// if the variable is not found locally.
func (e *Environment) Get(key string) (Object, bool) {
	val, ok := e.store[key]
	if !ok && e.outer != nil {
		return e.outer.Get(key)
	}
	return val, ok
}

// Set defines a new variable or updates an existing one in the current
// environment and returns its value.
func (e *Environment) Set(name string, val Object) Object {
	e.store[name] = val
	return val
}

// MarkPrivate marks a variable as private to this module.
func (e *Environment) MarkPrivate(name string) {
	e.privates[name] = true
}

// IsPrivate returns true if the variable is marked as private in the current module's registry.
func (e *Environment) IsPrivate(name string) bool {
	return e.privates[name]
}

// Assign updates the value of an existing variable in the innermost scope
// where it is defined. It recursively searches outer environments to find
// the variable if it's not present in the current one.
func (e *Environment) Assign(name string, val Object) {
	if _, ok := e.store[name]; ok {
		e.store[name] = val
		return
	}

	if e.outer != nil {
		e.outer.Assign(name, val)
	}
}

// GetStackTrace generates a formatted string representing the current call stack,
// showing the function names, arguments, line numbers, and column positions
// of each active frame in the environment's execution history.
func (e *Environment) GetStackTrace() string {
	if len(e.CallStack) == 0 {
		return ""
	}

	trace := "\nStack trace:\n"
	for i := len(e.CallStack) - 1; i >= 0; i-- {
		frame := e.CallStack[i]
		argsJoined := strings.Join(frame.Args, ", ")

		trace += fmt.Sprintf("  at %s(%s) line %d, col %d\n",
			frame.FuncName,
			argsJoined,
			frame.Line,
			frame.Column)
	}
	return trace
}

// GetStandardModule retrieves and initializes a built-in standard module
// (e.g., array, date, string, math) by its name. It returns nil if the module is unknown.
func (e *Environment) GetStandardModule(moduleName string) *Module {
	switch moduleName {
	case "array":
		return e.newArrayModule()
	case "date":
		return e.newDateModule()
	case "string":
		return e.newStringModule()
	case "math":
		return e.newMathModule()
	case "log":
		return e.newLogModule()
	case "map":
		return e.newMapModule()
	default:
		return nil
	}
}
