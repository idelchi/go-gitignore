package gitignore_test

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
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
func ParseFilter(filter string) []string {
	if filter == "" {
		return nil
	}

	return strings.Split(strings.TrimSpace(filter), ",")
}

// BaseNameWithoutExt extracts the base filename without its extension.
func BaseNameWithoutExt(filename string) string {
	return strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
}

// ShouldIncludeFile determines whether a test file should be included based on
// the provided filter criteria. If no filter is provided, all files are included.
func ShouldIncludeFile(filename string, filter []string) bool {
	if len(filter) == 0 {
		return true
	}

	return slices.Contains(filter, BaseNameWithoutExt(filename))
}

// Files discovers and returns paths to test files in the specified directory.
func Files(dir string, filter []string) ([]string, error) {
	base, pattern := doublestar.SplitPattern(dir)

	files, err := doublestar.Glob(os.DirFS(base), pattern, doublestar.WithFilesOnly())
	if err != nil {
		return nil, err
	}

	var out []string

	for _, file := range files {
		if ShouldIncludeFile(file, filter) {
			out = append(out, filepath.Join(base, file))
		}
	}

	if len(out) == 0 {
		return nil, errors.New("no files found")
	}

	return out, nil
}

// LoadGitIgnoreSpecs reads and parses a YAML test file into GitIgnore test specifications.
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
