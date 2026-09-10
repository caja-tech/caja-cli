package main

import (
	"caja-cli/internal/project"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// resolveProjectContext determines which .caja file a command should act
// on. An explicit filePath always wins outright — cajaproj.yml is not even
// looked up — preserving standalone-script behavior byte for byte,
// regardless of whether a manifest happens to sit in the cwd. Only when
// filePath is empty does this look for cajaproj.yml in the current working
// directory; if none is found, filePath comes back empty too and it's up to
// the caller to raise its own existing "--file is required" error.
func resolveProjectContext(filePath string) (resolvedFilePath string, manifest *project.Manifest, err error) {
	if filePath != "" {
		return filePath, nil, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", nil, fmt.Errorf("failed to determine working directory: %w", err)
	}

	m, found, err := project.Load(cwd)
	if err != nil {
		return "", nil, err
	}
	if !found {
		return "", nil, nil
	}

	return filepath.Join(cwd, m.EntryOrDefault()), m, nil
}

// runBuiltBinaryOnce runs a just-compiled generator binary (a static-page
// project's build output) a single time with its working directory set to
// workDir, so relative paths passed to page.write land next to the
// project's manifest/entry file rather than wherever the caja binary itself
// happened to be invoked from.
func runBuiltBinaryOnce(outBin, workDir string) error {
	cmd := exec.Command(outBin)
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run %s: %w", outBin, err)
	}
	return nil
}
