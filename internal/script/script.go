package script

import (
	"caja-cli/internal/pipeline/analyzer"
	"caja-cli/internal/pipeline/ast"
	"caja-cli/internal/pipeline/environment"
	"caja-cli/internal/pipeline/evaluator"
	"caja-cli/internal/pipeline/lexer"
	"caja-cli/internal/pipeline/parser"
	"fmt"
)

// ParseWithDir parses a script and performs semantic analysis,
// returning the AST and the global environment with cached module ASTs.
func ParseWithDir(input string, baseDir string, filePath string) (*ast.Program, *environment.Environment, *analyzer.Analyzer, error) {
	tknzr := lexer.New(input)
	p := parser.New(tknzr)
	prog := p.Parse()
	p.PrintErrors()
	if p.HasErrors() {
		return nil, nil, nil, fmt.Errorf("errors found while parsing input")
	}

	globalEnv := environment.NewEnvironment(baseDir, filePath, false)
	a := analyzer.New(globalEnv)
	a.Run(prog)
	a.PrintErrors()
	if a.HasErrors() {
		return nil, nil, nil, fmt.Errorf("errors found while performing semantical analysis on input")
	}

	return prog, globalEnv, a, nil
}

// Run evaluates a program using the global environment from semantic analysis.
// Returns the result, the global environment, and any error.
func Run(program *ast.Program, globalEnv *environment.Environment) (environment.Object, error) {
	result, err := evaluator.Eval(program, globalEnv)
	if err != nil {
		return nil, err
	}
	return result, nil
}
