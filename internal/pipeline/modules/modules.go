package modules

import (
	"caja-cli/internal/file"
	"caja-cli/internal/pipeline/ast"
	"caja-cli/internal/pipeline/lexer"
	"caja-cli/internal/pipeline/parser"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// resolveNodeModule attempts to find a module within node_modules directories,
// traversing up the directory tree from the baseDir.
func resolveNodeModule(baseDir string, moduleName string) (string, error) {
	currentDir := baseDir
	osPath := filepath.FromSlash(moduleName)

	for {
		nodeModulesPath := filepath.Join(currentDir, "node_modules", osPath)
		info, err := os.Stat(nodeModulesPath)
		if err == nil && info.IsDir() {
			// Check for package.json
			pkgJsonPath := filepath.Join(nodeModulesPath, "package.json")
			pkgJsonBytes, err := os.ReadFile(pkgJsonPath)
			mainFile := "index.caja"

			if err == nil {
				var pkg struct {
					Main string `json:"main"`
				}
				if json.Unmarshal(pkgJsonBytes, &pkg) == nil && pkg.Main != "" {
					mainFile = pkg.Main
				}
			}

			// Try to resolve the main file with and without the extension
			modPath := filepath.Join(nodeModulesPath, mainFile)
			if _, err := os.Stat(modPath); err == nil {
				return modPath, nil
			}

			modPathWithExt := modPath + file.EXTENSION
			if _, err := os.Stat(modPathWithExt); err == nil {
				return modPathWithExt, nil
			}

			return "", fmt.Errorf("module '%s' found in node_modules, but entry point '%s' could not be resolved", moduleName, mainFile)
		}

		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			// Reached the root of the file system
			break
		}
		currentDir = parentDir
	}

	return "", fmt.Errorf("module '%s' not found in local paths or node_modules", moduleName)
}

// Load reads and parses a module file, returning its abstract syntax tree (AST).
// It constructs the absolute file path by joining the baseDir, moduleName, and file.EXTENSION.
func Load(baseDir string, moduleName string) (*ast.Program, error) {
	osPath := filepath.FromSlash(moduleName)
	
	// 1. Try local file resolution first
	modPath := filepath.Join(baseDir, osPath+file.EXTENSION)
	binModcontent, err := os.ReadFile(modPath)
	
	if err != nil {
		// 2. Try node_modules resolution
		resolvedModPath, resolveErr := resolveNodeModule(baseDir, moduleName)
		if resolveErr != nil {
			return nil, resolveErr
		}
		modPath = resolvedModPath
		binModcontent, err = os.ReadFile(modPath)
		if err != nil {
			return nil, fmt.Errorf("unable to read module %s at %s: %s", moduleName, modPath, err)
		}
	}

	modContent := string(binModcontent)
	modTknzr := lexer.New(modContent)
	modParser := parser.New(modTknzr)
	modProgram := modParser.Parse()
	if modParser.HasErrors() {
		return nil, fmt.Errorf("failed to parse module %s: %v", moduleName, modParser.Errors())
	}

	return modProgram, nil
}
