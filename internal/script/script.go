package script

import (
	"caja-cli/internal/environment"
	"caja-cli/internal/evaluator"
	"caja-cli/internal/lexer"
	"caja-cli/internal/semantic"
	"caja-cli/internal/syntax"
	"fmt"
)

// Parse parses the given script input string into an abstract syntax tree and performs semantic analysis.
// It returns the parsed program or an error if any syntax or semantic errors are found.
func Parse(input string) (*syntax.Program, error) {
	tknzr := lexer.New(input)
	parser := syntax.New(tknzr)
	program := parser.Parse()

	if len(parser.Errors()) > 0 {
		fmt.Println("Parser errors found:")
		for _, msg := range parser.Errors() {
			fmt.Printf("\t- %s\n", msg)
		}
		return nil, fmt.Errorf("failed to parse the script")
	}

	analyzer := semantic.New()
	analyzer.Analyze(program)
	if len(analyzer.Errors()) > 0 {
		fmt.Println("Semantic errors found:")
		for _, msg := range analyzer.Errors() {
			fmt.Printf("\t- %s\n", msg)
		}
		return nil, fmt.Errorf("failed to parse the script")
	}

	return program, nil
}

// Run evaluates the given syntax program in a new environment and returns its computed result.
// It returns the result as an environment.Object or an error if the evaluation fails.
func Run(program *syntax.Program) (environment.Object, error) {
	env := environment.NewEnvironment()
	eval, err := evaluator.Eval(program, env)
	if err != nil {
		return nil, fmt.Errorf("evaluation error: %w", err)
	}

	return eval, nil
}
