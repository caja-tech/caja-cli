package script

import (
	"caja-cli/internal/environment"
	"caja-cli/internal/evaluator"
	"caja-cli/internal/lexer"
	"caja-cli/internal/semantic"
	"caja-cli/internal/syntax"
	"fmt"
)

// ParseWithDir parses a script and performs semantic analysis,
// returning the AST and the global environment with cached module ASTs.
func ParseWithDir(input string, baseDir string, filePath string) (*syntax.Program, *environment.Environment, error) {
	tknzr := lexer.New(input)
	parser := syntax.New(tknzr)
	prog := parser.Parse()
	parser.PrintErrors()
	if parser.HasErrors() {
		return nil, nil, fmt.Errorf("errors found while parsing input")
	}

	globalEnv := environment.NewEnvironment(baseDir, filePath, false)
	analyzer := semantic.New(globalEnv)
	analyzer.Run(prog)
	analyzer.PrintErrors()
	if analyzer.HasErrors() {
		return nil, nil, fmt.Errorf("errors found while performing semantical analysis on input")
	}

	return prog, globalEnv, nil
}

// Run evaluates a program using the global environment from semantic analysis.
// Returns the result, the global environment, and any error.
func Run(program *syntax.Program, globalEnv *environment.Environment) (environment.Object, error) {
	result, err := evaluator.Eval(program, globalEnv)
	if err != nil {
		return nil, err
	}
	return result, nil
}
