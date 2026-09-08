package environment

import (
	"caja-cli/internal/pipeline/ast"
)

// EnvConfig holds the configuration for an environment,
// such as its base directory and whether it represents a module.
type EnvConfig struct {
	BaseDir  string
	FileName string
	IsModule bool
}

// EnvRegistry tracks module resolution state shared across the analyzer and
// compiler passes for a single top-level script and everything it imports.
type EnvRegistry struct {
	ModuleASTs      map[string]*ast.Program // ASTs parsed during semantic analysis
	ModuleAnalyzers map[string]interface{}
	ModuleFilePaths map[string]string // resolved on-disk path per import specifier, for source-location reporting
}

// Environment carries a script or module's base directory, file identity,
// and the module-resolution registries the analyzer and compiler populate
// and read as imports are processed.
type Environment struct {
	EnvConfig
	EnvRegistry
}

// NewEnvironment creates and returns a new top-level environment with
// no outer scope.
func NewEnvironment(baseDir string, fileName string, isModule bool) *Environment {
	return &Environment{
		EnvConfig: EnvConfig{
			BaseDir:  baseDir,
			FileName: fileName,
			IsModule: isModule,
		},
		EnvRegistry: EnvRegistry{
			ModuleASTs:      make(map[string]*ast.Program),
			ModuleAnalyzers: make(map[string]interface{}),
			ModuleFilePaths: make(map[string]string),
		},
	}
}
