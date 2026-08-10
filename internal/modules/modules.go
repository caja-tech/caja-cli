package modules

import (
	"caja-cli/internal/file"
	"caja-cli/internal/lexer"
	"caja-cli/internal/syntax"
	"fmt"
	"os"
	"path/filepath"
)

// Load reads and parses a module file, returning its abstract syntax tree (AST).
// It constructs the absolute file path by joining the baseDir, moduleName, and file.EXTENSION.
func Load(baseDir string, moduleName string) (*syntax.Program, error) {
	osPath := filepath.FromSlash(moduleName)
	modPath := filepath.Join(baseDir, osPath+file.EXTENSION)
	binModcontent, err := os.ReadFile(modPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read module %s: %s\n", moduleName, err)
	}

	modContent := string(binModcontent)
	modTknzr := lexer.New(modContent)
	modParser := syntax.New(modTknzr)
	modProgram := modParser.Parse()
	if modParser.HasErrors() {
		return nil, fmt.Errorf("failed to parse module %s: %s", moduleName, err)
	}

	return modProgram, nil
}
