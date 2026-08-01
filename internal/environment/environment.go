package environment

type Environment struct {
	store map[string]float64
}

func New() *Environment {
	return &Environment{store: make(map[string]float64)}
}

func (env *Environment) Get(key string) (float64, bool) {
	val, ok := env.store[key]
	return val, ok
}

func (env *Environment) Set(key string, val float64) float64 {
	env.store[key] = val
	return val
}
