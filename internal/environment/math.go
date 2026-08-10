package environment

func (env *Environment) newMathModule() *Module {
	moduleName := "math"
	mathEnv := NewEnvironment(env.BaseDir, moduleName, true)

	return &Module{
		Name: moduleName,
		Env:  mathEnv,
	}
}
