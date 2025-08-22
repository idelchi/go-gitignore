// Package gitignore_test provides unit tests for helper functions.
package gitignore_test

import (
	"reflect"
	"testing"
)

// TestFormatBool tests the formatBool helper function using table-driven tests.
func TestFormatBool(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    bool
		expected string
	}{
		{
			name:     "true value",
			input:    true,
			expected: "true",
		},
		{
			name:     "false value", 
			input:    false,
			expected: "false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := formatBool(tt.input)
			if result != tt.expected {
				t.Errorf("formatBool(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestParseFilter tests the ParseFilter helper function with various inputs.
func TestParseFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "single item",
			input:    "basic",
			expected: []string{"basic"},
		},
		{
			name:     "multiple items",
			input:    "basic,directories,complex",
			expected: []string{"basic", "directories", "complex"},
		},
		{
			name:     "items with spaces",
			input:    " basic , directories , complex ",
			expected: []string{"basic", "directories", "complex"},
		},
		{
			name:     "mixed spacing",
			input:    "basic,  directories,complex  ",
			expected: []string{"basic", "directories", "complex"},
		},
		{
			name:     "single item with spaces",
			input:    "  basic  ",
			expected: []string{"basic"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ParseFilter(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("ParseFilter(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestBaseNameWithoutExt tests the BaseNameWithoutExt helper function.
func TestBaseNameWithoutExt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple filename with extension",
			input:    "basic.yml",
			expected: "basic",
		},
		{
			name:     "filename with yaml extension",
			input:    "complex.yaml",
			expected: "complex",
		},
		{
			name:     "full path with extension",
			input:    "/path/to/file.yml",
			expected: "file",
		},
		{
			name:     "filename without extension",
			input:    "README",
			expected: "README",
		},
		{
			name:     "filename with multiple dots",
			input:    "test.spec.js",
			expected: "test.spec",
		},
		{
			name:     "hidden file with extension",
			input:    ".gitignore.yml",
			expected: ".gitignore",
		},
		{
			name:     "path with directory that has extension",
			input:    "/path/to/dir.ext/file.yml",
			expected: "file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := BaseNameWithoutExt(tt.input)
			if result != tt.expected {
				t.Errorf("BaseNameWithoutExt(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestShouldIncludeFile tests the ShouldIncludeFile helper function.
func TestShouldIncludeFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		filename string
		filter   []string
		expected bool
	}{
		{
			name:     "no filter - should include all",
			filename: "basic.yml",
			filter:   nil,
			expected: true,
		},
		{
			name:     "empty filter - should include all",
			filename: "basic.yml",
			filter:   []string{},
			expected: true,
		},
		{
			name:     "matching filter",
			filename: "basic.yml",
			filter:   []string{"basic", "complex"},
			expected: true,
		},
		{
			name:     "non-matching filter",
			filename: "basic.yml",
			filter:   []string{"complex", "advanced"},
			expected: false,
		},
		{
			name:     "single matching filter",
			filename: "directories.yaml",
			filter:   []string{"directories"},
			expected: true,
		},
		{
			name:     "case sensitive matching",
			filename: "Basic.yml",
			filter:   []string{"basic"},
			expected: false,
		},
		{
			name:     "path with directory",
			filename: "/tests/basic.yml",
			filter:   []string{"basic"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ShouldIncludeFile(tt.filename, tt.filter)
			if result != tt.expected {
				t.Errorf("ShouldIncludeFile(%q, %v) = %v, want %v", 
					tt.filename, tt.filter, result, tt.expected)
			}
		})
	}
}

// TestYamlFiles_EdgeCases tests YamlFiles function with edge cases.
func TestYamlFiles_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		dir         string
		expectError bool
	}{
		{
			name:        "nonexistent directory",
			dir:         "/nonexistent/path",
			expectError: true,
		},
		{
			name:        "empty string directory",
			dir:         "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := YamlFiles(tt.dir, nil)
			hasError := err != nil
			if hasError != tt.expectError {
				t.Errorf("YamlFiles(%q, nil) error = %v, expectError = %v", 
					tt.dir, err, tt.expectError)
			}
		})
	}
}