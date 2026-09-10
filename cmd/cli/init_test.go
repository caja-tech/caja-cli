package main

import (
	"bytes"
	"caja-cli/internal/project"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCmd_MissingName(t *testing.T) {
	cmd, _ := NewInitCmd()
	bufOut := new(bytes.Buffer)
	cmd.SetOut(bufOut)
	cmd.SetArgs([]string{"--type", "static-page"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error due to missing --name flag")
	}
	if err.Error() != "the --name flag is required" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestInitCmd_MissingType(t *testing.T) {
	cmd, _ := NewInitCmd()
	bufOut := new(bytes.Buffer)
	cmd.SetOut(bufOut)
	cmd.SetArgs([]string{"--name", "demo", "--dir", t.TempDir()})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error due to missing --type flag")
	}
	if err.Error() != "the --type flag is required" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestInitCmd_InvalidType(t *testing.T) {
	cmd, _ := NewInitCmd()
	cmd.SetArgs([]string{"--name", "demo", "--type", "bogus", "--dir", t.TempDir()})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error due to invalid --type flag")
	}
	if !strings.Contains(err.Error(), `invalid --type "bogus"`) {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestInitCmd_RefusesNonEmptyDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("hi"), 0644); err != nil {
		t.Fatalf("failed to seed existing file: %v", err)
	}

	cmd, _ := NewInitCmd()
	cmd.SetArgs([]string{"--name", "demo", "--type", "static-page", "--dir", dir})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error due to non-empty target directory")
	}
	if !strings.Contains(err.Error(), "already exists and is not empty") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestInitCmd_DefaultDirDerivedFromName(t *testing.T) {
	base := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	if err := os.Chdir(base); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	cmd, _ := NewInitCmd()
	cmd.SetArgs([]string{"--name", "my-app", "--type", "static-page"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected init to succeed, got: %v", err)
	}

	if _, err := os.Stat(filepath.Join(base, "my-app", "cajaproj.yml")); err != nil {
		t.Errorf("expected ./my-app/cajaproj.yml to exist: %v", err)
	}
}

func testInitScaffold(t *testing.T, projectType, name string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)

	cmd, _ := NewInitCmd()
	bufOut := new(bytes.Buffer)
	cmd.SetOut(bufOut)
	cmd.SetArgs([]string{"--name", name, "--type", projectType, "--dir", dir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected init --type %s to succeed, got: %v\noutput:\n%s", projectType, err, bufOut.String())
	}

	manifest, found, err := project.Load(dir)
	if err != nil {
		t.Fatalf("expected a parseable cajaproj.yml, got error: %v", err)
	}
	if !found {
		t.Fatalf("expected cajaproj.yml to exist in %s", dir)
	}
	if manifest.Name != name {
		t.Errorf("manifest.Name = %q, want %q", manifest.Name, name)
	}
	if string(manifest.Type) != projectType {
		t.Errorf("manifest.Type = %q, want %q", manifest.Type, projectType)
	}
	if manifest.CajaVersion == "" {
		t.Errorf("expected manifest.CajaVersion to be set")
	}

	if _, err := os.Stat(filepath.Join(dir, "main.caja")); err != nil {
		t.Errorf("expected main.caja to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "README.md")); err != nil {
		t.Errorf("expected README.md to exist: %v", err)
	}

	return dir
}

func TestInitCmd_ScaffoldsStaticPage(t *testing.T) {
	dir := testInitScaffold(t, "static-page", "demo-site")

	if _, err := os.Stat(filepath.Join(dir, ".gitignore")); err != nil {
		t.Errorf("expected .gitignore to exist: %v", err)
	}

	mainCaja, err := os.ReadFile(filepath.Join(dir, "main.caja"))
	if err != nil {
		t.Fatalf("failed to read main.caja: %v", err)
	}
	if !strings.Contains(string(mainCaja), "import page") {
		t.Errorf("expected main.caja to import the page module, got:\n%s", mainCaja)
	}
	if !strings.Contains(string(mainCaja), "demo-site") {
		t.Errorf("expected main.caja to be rendered with the project name, got:\n%s", mainCaja)
	}
}

func TestInitCmd_ScaffoldsWebApp(t *testing.T) {
	dir := testInitScaffold(t, "web-app", "demo-app")

	mainCaja, err := os.ReadFile(filepath.Join(dir, "main.caja"))
	if err != nil {
		t.Fatalf("failed to read main.caja: %v", err)
	}
	if !strings.Contains(string(mainCaja), "import browser") {
		t.Errorf("expected main.caja to import the browser module, got:\n%s", mainCaja)
	}
}

func TestInitCmd_ScaffoldsHTTPAPI(t *testing.T) {
	dir := testInitScaffold(t, "http-api", "demo-api")

	mainCaja, err := os.ReadFile(filepath.Join(dir, "main.caja"))
	if err != nil {
		t.Fatalf("failed to read main.caja: %v", err)
	}
	if !strings.Contains(string(mainCaja), `import "http" as http`) {
		t.Errorf("expected main.caja to import the http module, got:\n%s", mainCaja)
	}
}
