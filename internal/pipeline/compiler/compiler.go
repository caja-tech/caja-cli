package compiler

import (
	"caja-cli/internal/toolchain"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Compile accepts the transpiled Go code and produces the executable binary.
func Compile(goSource string, outputBin string) error {
	// 1. Obtain toolchain
	goBin, err := toolchain.EnsureToolchain()
	if err != nil {
		return fmt.Errorf("failed to get toolchain: %w", err)
	}

	// 2. Create hidden temp directory for the workspace
	tmpDir, err := os.MkdirTemp("", "caja-build-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// 3. Write main.go
	mainGoPath := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(mainGoPath, []byte(goSource), 0644); err != nil {
		return fmt.Errorf("failed to write main.go: %w", err)
	}

	goroot := filepath.Dir(filepath.Dir(goBin))
	
	// Initialize go module to avoid build errors outside GOPATH/workspace
	cmdMod := exec.Command(goBin, "mod", "init", "caja_build")
	cmdMod.Dir = tmpDir
	cmdMod.Stdout = os.Stdout
	cmdMod.Stderr = os.Stderr
	cmdMod.Env = append(os.Environ(), "GOROOT="+goroot)
	if err := cmdMod.Run(); err != nil {
		return fmt.Errorf("go mod init failed: %w", err)
	}

	// 4. Invoke `go build` targeting host architecture
	cmdBuild := exec.Command(goBin, "build", "-o", outputBin, mainGoPath)
	cmdBuild.Dir = tmpDir
	cmdBuild.Stdout = os.Stdout
	cmdBuild.Stderr = os.Stderr
	cmdBuild.Env = append(os.Environ(), "GOROOT="+goroot)

	if err := cmdBuild.Run(); err != nil {
		return fmt.Errorf("go build failed: %w", err)
	}

	return nil
}
