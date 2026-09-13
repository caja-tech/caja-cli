// Package project reads and writes cajaproj.yml, the manifest that declares
// a Caja project's type (static-page, http-api, web-app), name, and entry
// file. It's the single source of truth for the manifest's shape and
// filename, mirroring internal/file's role for the .caja extension/main.caja
// convention.
package project

import (
	"caja-cli/internal/file"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Type is a project's declared kind, controlling how `caja build`/`run`/
// `serve` treat its entry file.
type Type string

const (
	// TypeWebApp builds a progressive web app: HTML, CSS and JavaScript
	// generated at build time into OutputDir, installable and offline-capable.
	// It was called "static-page" until the wasm target was removed; there is
	// no compatibility alias, so an older manifest must be updated by hand.
	TypeWebApp  Type = "web-app"
	TypeHTTPAPI Type = "http-api"
)

// ManifestFile is the conventional filename `caja init` writes and
// resolveProjectContext-style lookups read, analogous to file.MAIN_FILE.
const ManifestFile = "cajaproj.yml"

// OutputDir is the directory a web-app project's generator binary writes
// into, relative to the project's entry file. Not yet configurable via the
// manifest.
const OutputDir = "dist"

// Manifest is cajaproj.yml's decoded shape.
type Manifest struct {
	Name        string `yaml:"name"`
	Type        Type   `yaml:"type"`
	CajaVersion string `yaml:"cajaVersion"`
	// Entry is the project's entry .caja file, relative to the project
	// directory. Empty means "use file.MAIN_FILE" — see EntryOrDefault.
	Entry string `yaml:"entry,omitempty"`
}

// Valid reports whether t is one of the project types this CLI knows about.
func (t Type) Valid() bool {
	switch t {
	case TypeWebApp, TypeHTTPAPI:
		return true
	default:
		return false
	}
}

// EntryOrDefault returns m.Entry, or file.MAIN_FILE when m.Entry is empty.
func (m *Manifest) EntryOrDefault() string {
	if m.Entry == "" {
		return file.MAIN_FILE
	}
	return m.Entry
}

// Load reads dir/cajaproj.yml. A missing manifest is not an error: it
// returns (nil, false, nil) so callers can fall back to requiring an
// explicit --file. A present-but-invalid manifest (malformed YAML, or an
// unrecognized type) is an error.
func Load(dir string) (manifest *Manifest, found bool, err error) {
	manifestPath := filepath.Join(dir, ManifestFile)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("failed to read %s: %w", manifestPath, err)
	}

	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, false, fmt.Errorf("failed to parse %s: %w", manifestPath, err)
	}

	if !m.Type.Valid() {
		return nil, false, fmt.Errorf("%s: invalid type %q (must be %q or %q)", manifestPath, m.Type, TypeWebApp, TypeHTTPAPI)
	}

	return &m, true, nil
}

// Save writes m to dir/cajaproj.yml as YAML.
func Save(dir string, m *Manifest) error {
	data, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}
	manifestPath := filepath.Join(dir, ManifestFile)
	if err := os.WriteFile(manifestPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", manifestPath, err)
	}
	return nil
}
