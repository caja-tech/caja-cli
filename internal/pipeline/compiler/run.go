package compiler

import (
	"caja-cli/internal/toolchain"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// Run accepts transpiled Go code and executes it directly via `go run`,
// mirroring how Compile shells out to `go build` — letting the Go toolchain
// own temp-file lifecycle and build caching instead of caja managing its own
// build-then-exec-then-cleanup dance. stdin/stdout/stderr are wired straight
// through so the executed program behaves like an in-process interpreter run.
func Run(goSource string, extraEnv []string, stdin io.Reader, stdout, stderr io.Writer) error {
	goBin, err := toolchain.EnsureToolchain()
	if err != nil {
		return fmt.Errorf("failed to get toolchain: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "caja-run-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	mainGoPath := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(mainGoPath, []byte(goSource), 0644); err != nil {
		return fmt.Errorf("failed to write main.go: %w", err)
	}

	goroot := filepath.Dir(filepath.Dir(goBin))
	// See Compile's cajaGoEnv for why CGO_ENABLED=0 is always set here too.
	env := append(cajaGoEnv(goroot), extraEnv...)

	cmdMod := exec.Command(goBin, "mod", "init", "caja_build")
	cmdMod.Dir = tmpDir
	cmdMod.Stdout = stdout
	cmdMod.Stderr = stderr
	cmdMod.Env = env
	if err := cmdMod.Run(); err != nil {
		return fmt.Errorf("go mod init failed: %w", err)
	}

	cmdRun := exec.Command(goBin, "run", "main.go")
	cmdRun.Dir = tmpDir
	cmdRun.Stdin = stdin
	cmdRun.Stdout = stdout
	cmdRun.Stderr = stderr
	cmdRun.Env = env

	if err := cmdRun.Run(); err != nil {
		return fmt.Errorf("go run failed: %w", err)
	}

	return nil
}
