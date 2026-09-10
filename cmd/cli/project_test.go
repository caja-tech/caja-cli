package main

import (
	"caja-cli/internal/project"
	"os"
	"path/filepath"
	"testing"
)

func chdir(t *testing.T, dir string) {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir to %s: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
}

func TestResolveProjectContext_ExplicitFileWinsOverManifest(t *testing.T) {
	dir := t.TempDir()
	manifest := &project.Manifest{Name: "demo", Type: project.TypeStaticPage, CajaVersion: "dev"}
	if err := project.Save(dir, manifest); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}
	chdir(t, dir)

	explicitPath := filepath.Join(dir, "other.caja")
	filePath, m, err := resolveProjectContext(explicitPath)
	if err != nil {
		t.Fatalf("resolveProjectContext() error = %v", err)
	}
	if filePath != explicitPath {
		t.Errorf("filePath = %q, want %q", filePath, explicitPath)
	}
	if m != nil {
		t.Errorf("manifest = %+v, want nil (explicit --file must bypass manifest lookup)", m)
	}
}

func TestResolveProjectContext_UsesManifestWhenFileOmitted(t *testing.T) {
	dir := t.TempDir()
	manifest := &project.Manifest{Name: "demo", Type: project.TypeWebApp, CajaVersion: "dev"}
	if err := project.Save(dir, manifest); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}
	chdir(t, dir)

	filePath, m, err := resolveProjectContext("")
	if err != nil {
		t.Fatalf("resolveProjectContext() error = %v", err)
	}
	// resolveProjectContext derives its path from os.Getwd(), which on
	// macOS resolves the /var -> /private/var symlink that t.TempDir()'s
	// raw path doesn't — resolve both sides before comparing so this isn't
	// a false failure on that platform.
	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("failed to resolve symlinks for %s: %v", dir, err)
	}
	wantPath := filepath.Join(resolvedDir, "main.caja")
	if filePath != wantPath {
		t.Errorf("filePath = %q, want %q", filePath, wantPath)
	}
	if m == nil || m.Type != project.TypeWebApp {
		t.Errorf("manifest = %+v, want Type=%q", m, project.TypeWebApp)
	}
}

func TestResolveProjectContext_NoManifestNoFile(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	filePath, m, err := resolveProjectContext("")
	if err != nil {
		t.Fatalf("resolveProjectContext() error = %v", err)
	}
	if filePath != "" {
		t.Errorf("filePath = %q, want empty", filePath)
	}
	if m != nil {
		t.Errorf("manifest = %+v, want nil", m)
	}
}
