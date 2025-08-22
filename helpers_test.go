package gitignore_test

import (
	"os"
	"path/filepath"
	"strings"

	yaml "github.com/goccy/go-yaml"
)

type Case struct {
	Path        string `yaml:"path"`
	IsDir       bool   `yaml:"isDir"`
	Ignored     bool   `yaml:"ignored"`
	Description string `yaml:"description"`
}

type GitIgnore struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Gitignore   string `yaml:"gitignore"`
	Cases       []Case `yaml:"cases"`
}

type GitIgnores []GitIgnore

// Helper functions

// formatBool returns a string representation of a boolean value
func formatBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// ParseFilter parses a comma-separated filter string into a slice of trimmed strings
func ParseFilter(filter string) []string {
	if filter == "" {
		return nil
	}
	parts := strings.Split(filter, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// BaseNameWithoutExt returns the base name of a file without its extension
func BaseNameWithoutExt(filename string) string {
	return strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
}

// ShouldIncludeFile checks if a file should be included based on the filter
func ShouldIncludeFile(filename string, filter []string) bool {
	if len(filter) == 0 {
		return true
	}
	baseName := BaseNameWithoutExt(filename)
	for _, f := range filter {
		if baseName == f {
			return true
		}
	}
	return false
}

func YamlFiles(dir string, filter []string) ([]string, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		switch strings.ToLower(filepath.Ext(e.Name())) {
		case ".yaml", ".yml":
			if ShouldIncludeFile(e.Name(), filter) {
				out = append(out, filepath.Join(dir, e.Name()))
			}
		}
	}
	return out, nil
}

func LoadGitIgnoreSpecs(path string) (GitIgnores, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var spec GitIgnores
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, err
	}
	return spec, nil
}
