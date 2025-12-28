// Package metadata handles APG package metadata (metadata.json).
// NurOS 2026 - GPL 3.0
package metadata

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Metadata represents the APGv2 package metadata structure.
type Metadata struct {
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Type         string   `json:"type"`
	Architecture *string  `json:"architecture"`
	Description  string   `json:"description"`
	Maintainer   string   `json:"maintainer"`
	License      *string  `json:"license"`
	Tags         []string `json:"tags"`
	Homepage     string   `json:"homepage"`
	Dependencies []string `json:"dependencies"`
	Conflicts    []string `json:"conflicts"`
	Provides     []string `json:"provides"`
	Replaces     []string `json:"replaces"`
	Conf         []string `json:"conf"`
}

// New creates a new empty Metadata structure with initialized slices.
func New() *Metadata {
	return &Metadata{
		Tags:         make([]string, 0),
		Dependencies: make([]string, 0),
		Conflicts:    make([]string, 0),
		Provides:     make([]string, 0),
		Replaces:     make([]string, 0),
		Conf:         make([]string, 0),
	}
}

// Load reads metadata from a JSON file.
func Load(path string) (*Metadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata file: %w", err)
	}

	var meta Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	return &meta, nil
}

// Save writes metadata to a JSON file with pretty formatting.
func (m *Metadata) Save(path string) error {
	data, err := json.MarshalIndent(m, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to serialize metadata: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write metadata file: %w", err)
	}

	return nil
}

// Wizard provides interactive metadata creation.
type Wizard struct {
	reader *bufio.Reader
}

// NewWizard creates a new metadata creation wizard.
func NewWizard() *Wizard {
	return &Wizard{
		reader: bufio.NewReader(os.Stdin),
	}
}

func (w *Wizard) prompt(text string) string {
	fmt.Print(text)
	input, _ := w.reader.ReadString('\n')
	return strings.TrimSpace(input)
}

func (w *Wizard) promptOptional(text string) *string {
	result := w.prompt(text)
	if result == "" {
		return nil
	}
	return &result
}

func (w *Wizard) promptList(text string) []string {
	input := w.prompt(text)
	if input == "" {
		return []string{}
	}

	parts := strings.Split(input, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func (w *Wizard) promptYesNo(text string) bool {
	answer := strings.ToLower(w.prompt(text))
	return answer == "y" || answer == "yes"
}

func (w *Wizard) promptInt(text string) (int, error) {
	input := w.prompt(text)
	return strconv.Atoi(input)
}

// Run executes the interactive metadata creation wizard.
func (w *Wizard) Run() (*Metadata, error) {
	meta := New()

	fmt.Println("\033[1m\033[36mPackage Metadata Creation Wizard\033[0m")

	meta.Name = w.prompt("Package name: ")
	if meta.Name == "" {
		return nil, fmt.Errorf("package name is required")
	}

	meta.Version = w.prompt("Version: ")
	if meta.Version == "" {
		return nil, fmt.Errorf("version is required")
	}

	meta.Type = w.prompt("Type (misc/binary/source) [misc]: ")
	if meta.Type == "" {
		meta.Type = "misc"
	}

	meta.Architecture = w.promptOptional("Architecture (x86_64/aarch64/all/null) [null]: ")

	meta.Description = w.prompt("Description: ")
	meta.Maintainer = w.prompt("Maintainer: ")
	meta.License = w.promptOptional("License (MIT/GPL-3.0/etc): ")
	meta.Homepage = w.prompt("Homepage URL: ")

	meta.Tags = w.promptList("Tags (comma-separated): ")
	meta.Dependencies = w.promptList("Dependencies (comma-separated, e.g., 'lib-example >= 2.0.0'): ")
	meta.Conflicts = w.promptList("Conflicts (comma-separated): ")
	meta.Provides = w.promptList("Provides (comma-separated): ")
	meta.Replaces = w.promptList("Replaces (comma-separated): ")
	meta.Conf = w.promptList("Config files (comma-separated, e.g., '/etc/app.conf'): ")

	return meta, nil
}

// Validate checks if required fields are present.
func (m *Metadata) Validate() error {
	if m.Name == "" {
		return fmt.Errorf("name is required")
	}
	if m.Version == "" {
		return fmt.Errorf("version is required")
	}
	return nil
}
