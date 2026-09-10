package main

import (
	"caja-cli/internal/project"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
)

// templateData is the substitution set every scaffold template is rendered
// against. Deliberately minimal (just Name) — the templates don't need
// anything else yet, and this is easy to grow later.
type templateData struct {
	Name string
}

// ensureEmptyDir creates dir (and any missing parents) if it doesn't exist
// yet, or confirms it's empty if it does. Refusing to scaffold into a
// non-empty directory (no --force/overwrite escape hatch in v1) avoids
// silently clobbering whatever's already there, matching common scaffolding
// tool UX (create-react-app, cargo new, ...).
func ensureEmptyDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return os.MkdirAll(dir, 0755)
		}
		return fmt.Errorf("failed to inspect directory %q: %w", dir, err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("directory %q already exists and is not empty", dir)
	}
	return nil
}

// renderProjectTemplates walks templatesFS's templates/<projectType>
// directory and renders every *.tmpl file it finds into targetDir (stripping
// the .tmpl suffix from the destination filename) via text/template against
// data. Non-.tmpl files, if any are ever added, are copied verbatim.
func renderProjectTemplates(projectType project.Type, targetDir string, data templateData) error {
	srcDir := filepath.Join("templates", string(projectType))

	return fs.WalkDir(templatesFS, srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		content, err := templatesFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read template %q: %w", path, err)
		}

		destName := strings.TrimSuffix(rel, ".tmpl")
		destPath := filepath.Join(targetDir, destName)

		if !strings.HasSuffix(rel, ".tmpl") {
			return os.WriteFile(destPath, content, 0644)
		}

		tmpl, err := template.New(rel).Parse(string(content))
		if err != nil {
			return fmt.Errorf("failed to parse template %q: %w", path, err)
		}

		f, err := os.Create(destPath)
		if err != nil {
			return fmt.Errorf("failed to create %q: %w", destPath, err)
		}
		defer f.Close()

		if err := tmpl.Execute(f, data); err != nil {
			return fmt.Errorf("failed to render template %q: %w", path, err)
		}
		return nil
	})
}

// NewInitCmd creates and returns the 'init' command: scaffolds a new Caja
// project (a cajaproj.yml manifest plus a per-type main.caja and README) so
// `caja build`/`run`/`serve` can auto-detect the project's type afterward
// via resolveProjectContext.
func NewInitCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Scaffold a new caja project",
		Long:  "Create a new caja project directory containing a cajaproj.yml manifest and a main.caja entry point for the given project type.",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := cmd.Flags().GetString("name")
			if err != nil {
				return fmt.Errorf("failed to retrieve 'name' flag: %w", err)
			}
			if name == "" {
				_ = cmd.Help()
				return fmt.Errorf("the --name flag is required")
			}
			if strings.ContainsAny(name, "/\\") {
				return fmt.Errorf("invalid --name %q: must not contain path separators", name)
			}

			typeFlag, err := cmd.Flags().GetString("type")
			if err != nil {
				return fmt.Errorf("failed to retrieve 'type' flag: %w", err)
			}
			if typeFlag == "" {
				_ = cmd.Help()
				return fmt.Errorf("the --type flag is required")
			}
			projectType := project.Type(typeFlag)
			if !projectType.Valid() {
				return fmt.Errorf("invalid --type %q: must be one of %q, %q, %q", typeFlag, project.TypeStaticPage, project.TypeHTTPAPI, project.TypeWebApp)
			}

			targetDir, err := cmd.Flags().GetString("dir")
			if err != nil {
				return fmt.Errorf("failed to retrieve 'dir' flag: %w", err)
			}
			if targetDir == "" {
				targetDir = name
			}

			if err := ensureEmptyDir(targetDir); err != nil {
				return err
			}

			if err := renderProjectTemplates(projectType, targetDir, templateData{Name: name}); err != nil {
				return err
			}

			manifest := &project.Manifest{
				Name:        name,
				Type:        projectType,
				CajaVersion: Version,
			}
			if err := project.Save(targetDir, manifest); err != nil {
				return err
			}

			fmt.Printf("Created %s project %q in %s\n", projectType, name, targetDir)

			return nil
		},
	}

	cmd.Flags().String("name", "", "Name of the project to create")
	cmd.Flags().String("type", "", "Project type: static-page, http-api, or web-app")
	cmd.Flags().String("dir", "", "Directory to create the project in (defaults to ./<name>)")

	return cmd, nil
}
