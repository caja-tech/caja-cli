package compiler_test

import (
	"caja-cli/internal/pipeline/compiler"
	"caja-cli/internal/script"
	"os"
	"path/filepath"
	"testing"
)

func TestSamplesCompilation(t *testing.T) {
	samplesDir := "samples"
	
	entries, err := os.ReadDir(samplesDir)
	if err != nil {
		t.Fatalf("failed to read samples directory: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirName := entry.Name()
		t.Run(dirName, func(t *testing.T) {
			filePath := filepath.Join(samplesDir, dirName, dirName+".caja")
			
			sourceCode, err := os.ReadFile(filePath)
			if err != nil {
				t.Fatalf("failed to read file '%s': %v", filePath, err)
			}

			baseDir, _ := filepath.Abs(filepath.Dir(filePath))

			// Parse the script to get the AST
			program, _, a, err := script.ParseWithDir(string(sourceCode), baseDir, filePath)
			if err != nil {
				t.Fatalf("failed to parse script: %v", err)
			}

			// Transpile to Go source
			goCode, err := compiler.Transpile(program, a)
			if err != nil {
				t.Fatalf("transpilation failed: %v", err)
			}

			outBin := filepath.Join(baseDir, dirName)
			
			// Test compilation
			err = compiler.Compile(goCode, outBin)
			if err != nil {
				t.Fatalf("compilation failed: %v", err)
			}
			
			// Verify binary exists
			if _, err := os.Stat(outBin); os.IsNotExist(err) {
				t.Fatalf("expected binary %s to be generated, but it was not", outBin)
			}
			
			// Clean up binary
			os.Remove(outBin)
		})
	}
}
