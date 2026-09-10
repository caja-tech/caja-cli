package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_MissingFile(t *testing.T) {
	dir := t.TempDir()

	m, found, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if found {
		t.Fatalf("Load() found = true, want false")
	}
	if m != nil {
		t.Fatalf("Load() manifest = %+v, want nil", m)
	}
}

func TestLoad_ValidManifest(t *testing.T) {
	dir := t.TempDir()
	want := &Manifest{Name: "demo", Type: TypeStaticPage, CajaVersion: "0.1.0"}
	if err := Save(dir, want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, found, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if !found {
		t.Fatalf("Load() found = false, want true")
	}
	if got.Name != want.Name || got.Type != want.Type || got.CajaVersion != want.CajaVersion {
		t.Fatalf("Load() = %+v, want %+v", got, want)
	}
}

func TestLoad_InvalidType(t *testing.T) {
	dir := t.TempDir()
	content := "name: demo\ntype: not-a-real-type\ncajaVersion: dev\n"
	if err := os.WriteFile(filepath.Join(dir, ManifestFile), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	_, _, err := Load(dir)
	if err == nil {
		t.Fatalf("Load() error = nil, want an error for invalid type")
	}
}

func TestLoad_MalformedYAML(t *testing.T) {
	dir := t.TempDir()
	content := "name: [unterminated\n"
	if err := os.WriteFile(filepath.Join(dir, ManifestFile), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	_, _, err := Load(dir)
	if err == nil {
		t.Fatalf("Load() error = nil, want a YAML parse error")
	}
}

func TestManifest_EntryOrDefault(t *testing.T) {
	m := &Manifest{}
	if got := m.EntryOrDefault(); got != "main.caja" {
		t.Fatalf("EntryOrDefault() = %q, want %q", got, "main.caja")
	}

	m.Entry = "index.caja"
	if got := m.EntryOrDefault(); got != "index.caja" {
		t.Fatalf("EntryOrDefault() = %q, want %q", got, "index.caja")
	}
}

func TestType_Valid(t *testing.T) {
	valid := []Type{TypeStaticPage, TypeHTTPAPI, TypeWebApp}
	for _, ty := range valid {
		if !ty.Valid() {
			t.Errorf("Type(%q).Valid() = false, want true", ty)
		}
	}

	if Type("bogus").Valid() {
		t.Errorf(`Type("bogus").Valid() = true, want false`)
	}
}
