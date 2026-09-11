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
	// --skip-install: tests must not shell out to a real package manager
	// (slow, network-dependent, and not hermetic in CI) — a no-op for
	// project types with no dependencies to install anyway.
	cmd.SetArgs([]string{"--name", name, "--type", projectType, "--dir", dir, "--skip-install"})

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
	if !strings.Contains(string(mainCaja), `@caja/acerola`) {
		t.Errorf("expected main.caja to import @caja/acerola, got:\n%s", mainCaja)
	}
}

// TestInitCmd_HTTPAPIScaffoldsPackageJSON confirms the http-api scaffold
// declares @caja/acerola as a dependency, so a package manager install
// (real or manual) has something to fetch.
func TestInitCmd_HTTPAPIScaffoldsPackageJSON(t *testing.T) {
	dir := testInitScaffold(t, "http-api", "demo-api-pkg")

	pkgJSON, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		t.Fatalf("expected package.json to exist: %v", err)
	}
	if !strings.Contains(string(pkgJSON), `"@caja/acerola"`) {
		t.Errorf("expected package.json to declare @caja/acerola as a dependency, got:\n%s", pkgJSON)
	}
	if !strings.Contains(string(pkgJSON), `"demo-api-pkg"`) {
		t.Errorf("expected package.json to be rendered with the project name, got:\n%s", pkgJSON)
	}
}

// TestInitCmd_OtherProjectTypesHaveNoPackageJSON confirms package.json is
// http-api-only — static-page/web-app have no npm dependency to manage.
func TestInitCmd_OtherProjectTypesHaveNoPackageJSON(t *testing.T) {
	for _, projectType := range []string{"static-page", "web-app"} {
		dir := testInitScaffold(t, projectType, "demo-"+projectType)
		if _, err := os.Stat(filepath.Join(dir, "package.json")); !os.IsNotExist(err) {
			t.Errorf("expected no package.json for %s project, stat err: %v", projectType, err)
		}
	}
}

// TestInitCmd_SkipInstallLeavesNoNodeModules confirms --skip-install
// actually skips the install step (no node_modules ever gets created),
// rather than just suppressing output.
func TestInitCmd_SkipInstallLeavesNoNodeModules(t *testing.T) {
	dir := testInitScaffold(t, "http-api", "demo-skip-install")

	if _, err := os.Stat(filepath.Join(dir, "node_modules")); !os.IsNotExist(err) {
		t.Errorf("expected no node_modules with --skip-install, stat err: %v", err)
	}
}

// TestInitCmd_InstallWarnsWithoutFailingWhenNoPackageManagerFound confirms
// a missing npm/bun (or any other install failure) degrades to a warning
// rather than failing the whole init — the scaffolded project is still
// valid even if the dependency install didn't happen. Simulated here by
// pointing PATH somewhere with neither binary, rather than depending on the
// real npm registry's current state.
func TestInitCmd_InstallWarnsWithoutFailingWhenNoPackageManagerFound(t *testing.T) {
	emptyPathDir := t.TempDir()
	t.Setenv("PATH", emptyPathDir)

	dir := filepath.Join(t.TempDir(), "demo-no-pm")
	cmd, _ := NewInitCmd()
	bufOut := new(bytes.Buffer)
	cmd.SetOut(bufOut)
	cmd.SetArgs([]string{"--name", "demo-no-pm", "--type", "http-api", "--dir", dir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected init to still succeed without a package manager on PATH, got: %v\noutput:\n%s", err, bufOut.String())
	}
	if !strings.Contains(bufOut.String(), "no npm or bun found on PATH") {
		t.Errorf("expected a warning about the missing package manager, got output:\n%s", bufOut.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "main.caja")); err != nil {
		t.Errorf("expected the scaffold to still be created despite the install warning: %v", err)
	}
}
