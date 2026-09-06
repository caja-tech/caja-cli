package compiler

import (
	"caja-cli/internal/toolchain"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// CompileOptions controls the target platform Compile builds for.
type CompileOptions struct {
	// GOOS and GOARCH override the target OS/architecture for cross-
	// compilation, mirroring Go's own GOOS/GOARCH environment variables
	// (same names, same accepted values). Left empty, `go build` defaults to
	// the host platform — unchanged from before these options existed.
	GOOS   string
	GOARCH string
}

// Compile accepts the transpiled Go code and produces the executable binary.
func Compile(goSource string, outputBin string, opts CompileOptions) error {
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

	// 4. Invoke `go build`, optionally cross-compiling via GOOS/GOARCH
	cmdBuild := exec.Command(goBin, "build", "-o", outputBin, mainGoPath)
	cmdBuild.Dir = tmpDir
	cmdBuild.Stdout = os.Stdout
	cmdBuild.Stderr = os.Stderr
	cmdBuild.Env = append(os.Environ(), "GOROOT="+goroot)
	if opts.GOOS != "" {
		cmdBuild.Env = append(cmdBuild.Env, "GOOS="+opts.GOOS)
	}
	if opts.GOARCH != "" {
		cmdBuild.Env = append(cmdBuild.Env, "GOARCH="+opts.GOARCH)
	}

	if err := cmdBuild.Run(); err != nil {
		return fmt.Errorf("go build failed: %w", err)
	}

	return nil
}
