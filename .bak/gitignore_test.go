// Package gitignore_test provides comprehensive testing for the gitignore package.
//
// This package contains YAML-driven integration tests that verify Git-compatible
// gitignore pattern matching behavior across a wide range of edge cases and scenarios.
// The test suite is designed to ensure perfect compatibility with Git's actual
// gitignore implementation.
//
// Test Structure:
//   - YAML test files in tests/ directory define test cases
//   - Each YAML file contains multiple test groups
//   - Each test group contains multiple test cases
//   - Command-line filtering allows running specific test files
//
// Usage:
//
//	go test                           # Run all tests
//	go test -f basic,directories      # Run specific test files
//	go test -v                        # Verbose output with hierarchical errors
package gitignore_test

import (
	"flag"
	"fmt"
	"strings"
	"testing"

	gitignore "github.com/idelchi/go-gitignore"
)

// testFilter allows filtering which test files to run via command line.
// Usage: go test -f "basic,directories" to run only basic.yml and directories.yml
var testFilter = flag.String("f", "", "Comma-separated list of test file names to run (without extension)")

// TestGitIgnore_YAML is the main test function that loads and executes all YAML-based tests.
// It provides comprehensive testing of gitignore pattern matching behavior by:
//  1. Loading test files from the tests/ directory (optionally filtered)
//  2. Parsing YAML test specifications
//  3. Creating gitignore instances with specified patterns
//  4. Running test cases and comparing results
//  5. Providing detailed error messages with hierarchical context
//
// The test uses t.Parallel() extensively for concurrent execution and better performance.
// Error messages follow the format: "test_file -> test_group -> test_case" with descriptions.
func TestGitIgnore_YAML(t *testing.T) {
	t.Parallel()

	filter := ParseFilter(*testFilter)
	files, err := YamlFiles("./tests", filter)
	if err != nil {
		t.Fatalf("scan test dir: %v", err)
	}

	// Process each test file concurrently
	for _, f := range files {
		f := f // capture range variable for closure
		base := BaseNameWithoutExt(f)

		// Each test file runs as a separate subtest
		t.Run(base, func(t *testing.T) {
			t.Parallel()

			specs, err := LoadGitIgnoreSpecs(f)
			if err != nil {
				t.Fatalf("load specs from %s: %v", f, err)
			}

			// Process each test group within the file
			for _, spec := range specs {
				// Each test group runs as a separate subtest
				t.Run(spec.Name, func(t *testing.T) {
					t.Parallel()

					g := gitignore.New(strings.Split(spec.Gitignore, "\n"))

					// Process each individual test case
					for _, tc := range spec.Cases {
						// Format test name to clearly indicate directories
						testName := tc.Path
						if tc.Dir {
							testName += "/"
						}

						// Each test case runs as a separate subtest for precise failure reporting
						t.Run(testName, func(t *testing.T) {
							// Test the actual gitignore logic
							got := g.Ignored(tc.Path, tc.Dir)
							if got != tc.Ignored {
								// Create detailed error message with hierarchical context
								errorMsg := fmt.Sprintf("%s -> %s -> %s\n", base, spec.Name, testName)

								errorMsg += fmt.Sprintf("Pattern: %v\n", g.Patterns())

								// Include descriptions from YAML for better context
								if spec.Description != "" {
									errorMsg += fmt.Sprintf("Group: %s\n", spec.Description)
								}

								if tc.Description != "" {
									errorMsg += fmt.Sprintf("Case: %s\n", tc.Description)
								}

								// Provide specific details about the failure
								errorMsg += fmt.Sprintf(
									"Expected Ignored(%q, isDir=%v) = %v, got %v",
									tc.Path, tc.Dir, tc.Ignored, got,
								)

								t.Error(errorMsg)
							}
						})
					}
				})
			}
		})
	}
}
