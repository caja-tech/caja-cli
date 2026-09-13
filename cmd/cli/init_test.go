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
	cmd.SetArgs([]string{"--type", "web-app"})

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
	cmd.SetArgs([]string{"--name", "demo", "--type", "web-app", "--dir", dir})

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
	cmd.SetArgs([]string{"--name", "my-app", "--type", "web-app"})

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

func TestInitCmd_ScaffoldsWebApp(t *testing.T) {
	dir := testInitScaffold(t, "web-app", "demo-site")

	if _, err := os.Stat(filepath.Join(dir, ".gitignore")); err != nil {
		t.Errorf("expected .gitignore to exist: %v", err)
	}

	mainCaja, err := os.ReadFile(filepath.Join(dir, "main.caja"))
	if err != nil {
		t.Fatalf("failed to read main.caja: %v", err)
	}
	if !strings.Contains(string(mainCaja), "import doc") {
		t.Errorf("expected main.caja to import the doc module, got:\n%s", mainCaja)
	}
	// Both UI packages, not just the theme: Page/definePage and the event
	// handlers live in @caja/ui, and @caja/siriguela does not re-export
	// them (it reaches ui through a wildcard import, which is deliberately
	// not forwarded to a theme's own consumers).
	for _, imp := range []string{`import * from "@caja/siriguela"`, `import * from "@caja/ui"`} {
		if !strings.Contains(string(mainCaja), imp) {
			t.Errorf("expected main.caja to contain %q, got:\n%s", imp, mainCaja)
		}
	}
	if !strings.Contains(string(mainCaja), "demo-site") {
		t.Errorf("expected main.caja to be rendered with the project name, got:\n%s", mainCaja)
	}

	// The web-app scaffold is a small multi-page site: pages/ holds one
	// module per page (rendered through text/template, so the project name
	// must reach them too) and assets/ ships a real file — both live in
	// NESTED template directories, which renderProjectTemplates has to
	// create before it can write into them.
	for _, rel := range []string{
		filepath.Join("assets", "siriguela.svg"),
		filepath.Join("pages", "index.caja"),
		filepath.Join("pages", "about.caja"),
	} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Errorf("expected %s to be scaffolded: %v", rel, err)
		}
	}
	for _, rel := range []string{filepath.Join("pages", "index.caja"), filepath.Join("pages", "about.caja")} {
		content, err := os.ReadFile(filepath.Join(dir, rel))
		if err != nil {
			continue // already reported above
		}
		if !strings.Contains(string(content), "demo-site") {
			t.Errorf("expected %s to be rendered with the project name", rel)
		}
		if strings.Contains(string(content), "{{") {
			t.Errorf("expected %s to have no unrendered template actions left, got one", rel)
		}
	}
	if svg, err := os.ReadFile(filepath.Join(dir, "assets", "siriguela.svg")); err == nil && !strings.HasPrefix(string(svg), "<svg") {
		t.Errorf("expected assets/siriguela.svg to be copied verbatim, got:\n%s", svg)
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

// assertScaffoldPackageJSON reads the package.json a scaffold rendered and
// checks it names the project and declares every dependency listed. Shared
// by the per-project-type tests below, which differ only in that list — the
// interesting part of each is which packages it expects, not the reading.
func assertScaffoldPackageJSON(t *testing.T, dir, name string, deps ...string) {
	t.Helper()
	pkgJSON, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		t.Fatalf("expected package.json to exist: %v", err)
	}
	for _, dep := range deps {
		if !strings.Contains(string(pkgJSON), dep) {
			t.Errorf("expected package.json to declare %s as a dependency, got:\n%s", dep, pkgJSON)
		}
	}
	if !strings.Contains(string(pkgJSON), `"`+name+`"`) {
		t.Errorf("expected package.json to be rendered with the project name, got:\n%s", pkgJSON)
	}
}

// TestInitCmd_HTTPAPIScaffoldsPackageJSON confirms the http-api scaffold
// declares @caja/acerola as a dependency, so a package manager install
// (real or manual) has something to fetch.
func TestInitCmd_HTTPAPIScaffoldsPackageJSON(t *testing.T) {
	dir := testInitScaffold(t, "http-api", "demo-api-pkg")
	assertScaffoldPackageJSON(t, dir, "demo-api-pkg", `"@caja/acerola"`)
}

// TestInitCmd_WebAppScaffoldsPackageJSON confirms the static-page
// scaffold declares every package its templates import — the theme, the
// structural layer underneath it, and the JS interop layer. The scaffold
// reaches into @caja/ui directly (for Page/definePage and the event
// handlers), so it is a real direct dependency, not one left to npm to
// resolve transitively through @caja/siriguela's own manifest. @caja/js is
// imported by pages/index.caja rather than main.caja, which is exactly why
// it is easy to drop from the manifest by accident: the resulting project
// scaffolds and installs cleanly and only fails when that page is built.
func TestInitCmd_WebAppScaffoldsPackageJSON(t *testing.T) {
	dir := testInitScaffold(t, "web-app", "demo-site-pkg")
	assertScaffoldPackageJSON(t, dir, "demo-site-pkg", `"@caja/siriguela"`, `"@caja/ui"`, `"@caja/js"`)
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

// testInitScaffoldWithInstall scaffolds a project with the install step
// ENABLED (unlike testInitScaffold's --skip-install), but with PATH pointed
// at an empty directory so installDependencies is guaranteed to find no
// package manager. That keeps the run hermetic — no shelling out to a real
// npm, no network, no dependency on the registry's current state — while
// still proving the install path was entered at all, since its "no npm or
// bun found on PATH" warning is unreachable otherwise. Returns the project
// directory and everything init wrote to its output.
func testInitScaffoldWithInstall(t *testing.T, projectType, name string) (string, string) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())

	dir := filepath.Join(t.TempDir(), name)
	cmd, _ := NewInitCmd()
	bufOut := new(bytes.Buffer)
	cmd.SetOut(bufOut)
	cmd.SetArgs([]string{"--name", name, "--type", projectType, "--dir", dir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected init --type %s to still succeed without a package manager on PATH, got: %v\noutput:\n%s", projectType, err, bufOut.String())
	}
	return dir, bufOut.String()
}

// TestInitCmd_InstallWarnsWithoutFailingWhenNoPackageManagerFound confirms
// a missing npm/bun (or any other install failure) degrades to a warning
// rather than failing the whole init — the scaffolded project is still
// valid even if the dependency install didn't happen. Simulated here by
// pointing PATH somewhere with neither binary, rather than depending on the
// real npm registry's current state.
func TestInitCmd_InstallWarnsWithoutFailingWhenNoPackageManagerFound(t *testing.T) {
	dir, out := testInitScaffoldWithInstall(t, "http-api", "demo-no-pm")

	if !strings.Contains(out, "no npm or bun found on PATH") {
		t.Errorf("expected a warning about the missing package manager, got output:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "main.caja")); err != nil {
		t.Errorf("expected the scaffold to still be created despite the install warning: %v", err)
	}
}

// TestScaffoldHasPackageJSON unit-tests the install gate itself. It replaced
// a hardcoded `projectType == TypeHTTPAPI` check, so the property worth
// pinning is that it reads the rendered scaffold and nothing else — that is
// what makes "add npm deps to a project type" a pure template change.
func TestScaffoldHasPackageJSON(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T) string
		want  bool
	}{
		{
			name: "dir containing a package.json",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0644); err != nil {
					t.Fatalf("failed to write package.json: %v", err)
				}
				return dir
			},
			want: true,
		},
		{
			name:  "empty dir",
			setup: func(t *testing.T) string { return t.TempDir() },
			want:  false,
		},
		{
			name: "dir with other files but no package.json",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				if err := os.WriteFile(filepath.Join(dir, "main.caja"), []byte("return 0\n"), 0644); err != nil {
					t.Fatalf("failed to write main.caja: %v", err)
				}
				return dir
			},
			want: false,
		},
		{
			// A path that doesn't exist must be false, not a panic: this
			// runs unconditionally right after scaffolding, on whatever
			// --dir the user gave.
			name: "nonexistent dir",
			setup: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "does-not-exist")
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scaffoldHasPackageJSON(tt.setup(t)); got != tt.want {
				t.Errorf("scaffoldHasPackageJSON() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestInitCmd_InstallGateFollowsScaffoldNotProjectType is the behavioural
// half: static-page and web-app now ship a package.json.tmpl, so init must
// try to install for them too. Under the previous http-api-only condition
// their dependencies would have been scaffolded and then never fetched,
// leaving a project whose own main.caja imports don't resolve.
//
// Asserted via the missing-package-manager warning (PATH pointed at an
// empty dir) rather than a real install, so this neither touches the
// network nor depends on the npm registry's current state — the warning is
// only reachable if installDependencies ran at all.
func TestInitCmd_InstallGateFollowsScaffoldNotProjectType(t *testing.T) {
	for _, projectType := range []string{"web-app", "http-api"} {
		t.Run(projectType, func(t *testing.T) {
			dir, out := testInitScaffoldWithInstall(t, projectType, "demo-gate-"+projectType)

			if !scaffoldHasPackageJSON(dir) {
				t.Fatalf("expected the %s scaffold to contain a package.json", projectType)
			}
			if !strings.Contains(out, "no npm or bun found on PATH") {
				t.Errorf("expected init to attempt an install for %s (the scaffold declares dependencies), got output:\n%s", projectType, out)
			}
		})
	}
}

// TestInitCmd_SkipInstallSuppressesTheGate confirms --skip-install still
// wins over the scaffold check: the package.json is written, but no install
// is attempted. Without this the new gate would have made --skip-install
// reachable only for project types that happen to have no dependencies.
func TestInitCmd_SkipInstallSuppressesTheGate(t *testing.T) {
	emptyPathDir := t.TempDir()
	t.Setenv("PATH", emptyPathDir)

	dir := testInitScaffold(t, "web-app", "demo-skip-gate")

	if !scaffoldHasPackageJSON(dir) {
		t.Fatalf("expected the scaffold to contain a package.json, so this test isn't passing vacuously")
	}
	if _, err := os.Stat(filepath.Join(dir, "node_modules")); !os.IsNotExist(err) {
		t.Errorf("expected no node_modules with --skip-install, stat err: %v", err)
	}
}
