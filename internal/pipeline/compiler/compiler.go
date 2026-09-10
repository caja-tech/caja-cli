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

// cajaGoEnv builds the environment a `go` subcommand invoked by Compile/Run
// should use: the host's own environment plus the resolved GOROOT and
// CGO_ENABLED=0.
//
// CGO_ENABLED=0: none of Caja's builtins need cgo (net/http's pure-Go
// resolver is fine for a server that only listens — it never resolves
// hostnames), and disabling it sidesteps a real toolchain-vs-host-linker
// incompatibility: a cgo-linked binary built with this pinned Go version
// against a newer macOS/Xcode `ld` can fail at process start with
// "dyld: missing LC_UUID load command" — a crash a Caja user would hit
// the moment they ran a compiled net/http-using binary, not something
// caught at build time. This also keeps native and cross-compiled builds
// consistent, since cross-compiling already forces CGO_ENABLED=0 whenever
// no matching C cross-compiler is configured.
func cajaGoEnv(goroot string) []string {
	return append(os.Environ(), "GOROOT="+goroot, "CGO_ENABLED=0")
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
	cmdBuild.Env = cajaGoEnv(goroot)
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
