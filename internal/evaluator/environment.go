package evaluator

// Environment provides a symbol table to store and retrieve variables
// during evaluation. It supports variable shadowing and scope resolution
// through a reference to an outer enclosing environment.
type Environment struct {
	store map[string]float64
	outer *Environment
}

// NewEnvironment creates and returns a new top-level environment with
// no outer scope.
func NewEnvironment() *Environment {
	return &Environment{store: make(map[string]float64), outer: nil}
}

// NewEnclosedEnvironment creates and returns a new environment that extends
// the given outer environment. This is typically used for block scopes.
func NewEnclosedEnvironment(outer *Environment) *Environment {
	env := NewEnvironment()
	env.outer = outer
	return env
}

// Get retrieves the value of a variable by its name. It searches the
// current environment first, then recursively searches the outer environment
// if the variable is not found locally.
func (env *Environment) Get(key string) (float64, bool) {
	val, ok := env.store[key]
	if !ok && env.outer != nil {
		return env.outer.Get(key)
	}
	return val, ok
}

// Set defines a new variable or updates an existing one in the current
// environment and returns its value.
func (env *Environment) Set(key string, val float64) float64 {
	env.store[key] = val
	return val
}

// Assign updates the value of an existing variable in the innermost scope
// where it is defined. It recursively searches outer environments to find
// the variable if it's not present in the current one.
func (env *Environment) Assign(name string, val float64) {
	if _, ok := env.store[name]; ok {
		env.store[name] = val
		return
	}

	if env.outer != nil {
		env.outer.Assign(name, val)
	}
}
