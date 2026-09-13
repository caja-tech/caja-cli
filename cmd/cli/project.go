package main

import (
	"caja-cli/internal/project"
	"fmt"
	"io/fs"
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
// workDir, so relative paths passed to doc.write land next to the
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

// webAppAssetsDir is the folder a static-page project keeps the files
// its pages reference (images, documents, ...). It is mirrored into
// dist/<webAppAssetsDir> on every build so `assets/logo.svg` in a page
// resolves once dist/ is served.
const webAppAssetsDir = "assets"

// generateWebApp is the single static-page generation path — `caja
// build` and `caja serve` both call it, so the two can't drift apart the
// way they had when each inlined runBuiltBinaryOnce itself. It runs the
// freshly built generator binary once (its doc.write calls populate dist/)
// and then mirrors assets/ into dist/assets/.
//
// dist/assets is recreated from scratch each time so a deleted or renamed
// asset doesn't linger. Generated pages are deliberately NOT cleared: they
// are the script's own doc.write output, not this command's to manage.
// A project with no assets/ folder at all is left exactly as before.
func generateWebApp(outBin, projectDir string) error {
	if err := runBuiltBinaryOnce(outBin, projectDir); err != nil {
		return err
	}
	src := filepath.Join(projectDir, webAppAssetsDir)
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read %s: %w", src, err)
	}
	dst := filepath.Join(projectDir, project.OutputDir, webAppAssetsDir)
	if err := os.RemoveAll(dst); err != nil {
		return fmt.Errorf("failed to clear %s: %w", dst, err)
	}
	return copyDir(src, dst)
}

// copyDir recursively copies the directory tree at src to dst, creating
// dst and any intermediate directories as it goes. Files are copied by
// content (a symlink's target is what lands in dst), which is what a
// static file host would serve anyway.
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
	})
}
