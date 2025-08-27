package gitignore_test

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"

	yaml "github.com/goccy/go-yaml"
)

// testFilter allows filtering which test files to run via command line.
// Usage: go test -f "basic,directories" to run only basic.yml and directories.yml.
//
//nolint:gochecknoglobals	// Test flag needs to be global for reuse.
var testFilter = flag.String("f", "", "YAML file to validate (e.g. 'basic.yml')")

// Case represents a single test case within a gitignore test group.
// Each case tests a specific path against the gitignore patterns to verify
// whether it should be ignored or not.
type Case struct {
	// Path is the file or directory path to test against gitignore patterns
	Path string `yaml:"path"`
	// Dir indicates whether this path represents a directory (true) or file (false)
	Dir bool `yaml:"dir"`
	// Ignored is the expected result - whether this path should be ignored
	Ignored bool `yaml:"ignored"`
	// Description provides human-readable context for this test case
	Description string `yaml:"description"`
	// Details holds an optional value detailing the expected output of `git check-ignore -v`
	Details *string `yaml:"details,omitempty"`
}

// GitIgnore represents a test group with a specific set of gitignore patterns
// and associated test cases. This corresponds to a single test scenario
// within a YAML test file.
type GitIgnore struct {
	// Name is the identifier for this test group
	Name string `yaml:"name"`
	// Description provides context about what this test group validates
	Description string `yaml:"description"`
	// Gitignore contains the raw gitignore patterns (newline-separated)
	Gitignore string `yaml:"gitignore"`
	// Cases contains all test cases for this gitignore pattern set
	Cases []Case `yaml:"cases"`
}

// GitIgnores represents a collection of GitIgnore test groups,
// typically loaded from a single YAML test file.
type GitIgnores []GitIgnore

// ParseFilter parses a comma-separated filter string into a slice of trimmed strings.
// This enables command-line filtering of test files using the -f flag.
// Empty strings are filtered out, and whitespace is trimmed from each part.
//
// Example:
//
//	ParseFilter("basic, directories ") returns ["basic", "directories"]
//	ParseFilter("") returns nil
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

// BaseNameWithoutExt extracts the base filename without its extension.
// This is used to convert test file paths into test names.
//
// Example:
//
//	BaseNameWithoutExt("/path/to/basic.yml") returns "basic"
//	BaseNameWithoutExt("complex.yaml") returns "complex"
func BaseNameWithoutExt(filename string) string {
	return strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
}

// ShouldIncludeFile determines whether a test file should be included based on
// the provided filter criteria. If no filter is provided, all files are included.
// Otherwise, only files whose base name (without extension) matches one of the
// filter entries will be included.
//
// This enables selective test execution via the -f command-line flag.
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

// YamlFiles discovers and returns paths to YAML test files in the specified directory.
// Only files with .yml or .yaml extensions are included. The optional filter parameter
// allows selective inclusion of files based on their base names.
//
// This function is the entry point for test file discovery and supports the command-line
// filtering functionality.
//
// Parameters:
//
//	dir: Directory path to search for YAML files
//	filter: Optional list of base names to include (nil means include all)
//
// Returns:
//
//	Slice of full file paths to matching YAML files
//	Error if directory cannot be read
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

	if len(out) == 0 {
		return nil, errors.New("no YAML files found")
	}

	return out, nil
}

// LoadGitIgnoreSpecs reads and parses a YAML test file into GitIgnore test specifications.
// Each YAML file contains an array of test groups, where each group defines gitignore
// patterns and associated test cases.
//
// The YAML structure expected:
//   - name: "test group name"
//     description: "what this tests"
//     gitignore: |
//     pattern1
//     pattern2
//     cases:
//   - path: "file/path"
//     ignored: true
//     description: "why this should be ignored"
//
// Parameters:
//
//	path: Full path to the YAML test file
//
// Returns:
//
//	Parsed GitIgnores test specifications
//	Error if file cannot be read or parsed
func LoadGitIgnoreSpecs(path string) (GitIgnores, error) {
	data, err := os.ReadFile(path) //nolint:gosec	// OK to include file for test purposes.
	if err != nil {
		return nil, err
	}

	var spec GitIgnores
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, err
	}

	return spec, nil
}

// boolToIgnored converts a boolean value to its string representation for gitignore status.
func boolToIgnored(ign bool) string {
	if ign {
		return "ignored"
	}

	return "not ignored"
}
