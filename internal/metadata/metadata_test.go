package metadata

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNew(t *testing.T) {
	meta := New()

	if meta == nil {
		t.Fatal("New() returned nil")
	}

	if meta.Tags == nil {
		t.Error("Tags should be initialized")
	}
	if meta.Dependencies == nil {
		t.Error("Dependencies should be initialized")
	}
	if meta.Conflicts == nil {
		t.Error("Conflicts should be initialized")
	}
	if meta.Provides == nil {
		t.Error("Provides should be initialized")
	}
	if meta.Replaces == nil {
		t.Error("Replaces should be initialized")
	}
	if meta.Conf == nil {
		t.Error("Conf should be initialized")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		meta    *Metadata
		wantErr bool
	}{
		{
			name: "valid metadata",
			meta: &Metadata{
				Name:    "test-package",
				Version: "1.0.0",
			},
			wantErr: false,
		},
		{
			name: "missing name",
			meta: &Metadata{
				Version: "1.0.0",
			},
			wantErr: true,
		},
		{
			name: "missing version",
			meta: &Metadata{
				Name: "test-package",
			},
			wantErr: true,
		},
		{
			name:    "empty metadata",
			meta:    &Metadata{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.meta.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	metaPath := filepath.Join(tmpDir, "metadata.json")

	// Create test metadata
	original := &Metadata{
		Name:         "test-package",
		Version:      "1.0.0",
		Type:         "binary",
		Description:  "Test package",
		Maintainer:   "Test Author",
		Homepage:     "https://example.com",
		Tags:         []string{"test", "example"},
		Dependencies: []string{"libc >= 2.0"},
		Conflicts:    []string{},
		Provides:     []string{},
		Replaces:     []string{},
		Conf:         []string{"/etc/test.conf"},
	}

	arch := "x86_64"
	original.Architecture = &arch

	license := "GPL-3.0"
	original.License = &license

	// Save
	if err := original.Save(metaPath); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		t.Fatal("metadata.json was not created")
	}

	// Load
	loaded, err := Load(metaPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Compare fields
	if loaded.Name != original.Name {
		t.Errorf("Name mismatch: got %q, want %q", loaded.Name, original.Name)
	}
	if loaded.Version != original.Version {
		t.Errorf("Version mismatch: got %q, want %q", loaded.Version, original.Version)
	}
	if loaded.Type != original.Type {
		t.Errorf("Type mismatch: got %q, want %q", loaded.Type, original.Type)
	}
	if loaded.Description != original.Description {
		t.Errorf("Description mismatch: got %q, want %q", loaded.Description, original.Description)
	}
	if loaded.Architecture == nil || *loaded.Architecture != *original.Architecture {
		t.Error("Architecture mismatch")
	}
	if loaded.License == nil || *loaded.License != *original.License {
		t.Error("License mismatch")
	}
	if len(loaded.Tags) != len(original.Tags) {
		t.Errorf("Tags count mismatch: got %d, want %d", len(loaded.Tags), len(original.Tags))
	}
	if len(loaded.Dependencies) != len(original.Dependencies) {
		t.Errorf("Dependencies count mismatch: got %d, want %d", len(loaded.Dependencies), len(original.Dependencies))
	}
}

func TestLoad_NonExistent(t *testing.T) {
	_, err := Load("/nonexistent/metadata.json")
	if err == nil {
		t.Error("Load should fail for non-existent file")
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	metaPath := filepath.Join(tmpDir, "metadata.json")

	// Write invalid JSON
	if err := os.WriteFile(metaPath, []byte("not valid json"), 0644); err != nil {
		t.Fatalf("Failed to create invalid file: %v", err)
	}

	_, err := Load(metaPath)
	if err == nil {
		t.Error("Load should fail for invalid JSON")
	}
}

func TestJSONSerialization(t *testing.T) {
	meta := &Metadata{
		Name:         "test",
		Version:      "1.0.0",
		Type:         "misc",
		Architecture: nil,
		Description:  "Test",
		Maintainer:   "Author",
		License:      nil,
		Tags:         []string{},
		Homepage:     "",
		Dependencies: []string{},
		Conflicts:    []string{},
		Provides:     []string{},
		Replaces:     []string{},
		Conf:         []string{},
	}

	data, err := json.MarshalIndent(meta, "", "    ")
	if err != nil {
		t.Fatalf("JSON marshaling failed: %v", err)
	}

	// Verify null fields are serialized correctly
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("JSON unmarshaling failed: %v", err)
	}

	if parsed["architecture"] != nil {
		t.Error("Architecture should be null")
	}
	if parsed["license"] != nil {
		t.Error("License should be null")
	}
}
