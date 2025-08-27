// Package gitignore_test provides testing for the gitignore package.
//
// This package contains YAML-driven tests that verify Git-compatible
// gitignore pattern matching behavior across a range of edge cases and scenarios.
// It additionally validates the YAML tests against Git's actual check-ignore command.
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
	"fmt"
	"strings"
	"testing"

	gitignore "github.com/idelchi/go-gitignore"
)

// TestGitIgnored is the main test function that loads and executes all YAML-based tests.
//
//nolint:gocognit	// Long and complex setup is warranted.
func TestGitIgnored(t *testing.T) {
	t.Parallel()

	filter := ParseFilter(*testFilter)

	dir := "./tests/**/*.{yml,yaml}"

	files, err := Files(dir, filter)
	if err != nil {
		t.Fatalf("scan test dir %q: %v", dir, err)
	}

	if len(files) == 0 {
		t.Fatal("no test files found")
	}

	// Process each test file concurrently
	for _, f := range files {
		base := BaseNameWithoutExt(f)

		// Each test file runs as a separate subtest
		t.Run(base, func(t *testing.T) {
			t.Parallel()

			specs, err := LoadGitIgnoreSpecs(f)
			if err != nil {
				t.Fatalf("load specs from %s: %v", f, err)
			}

			if len(specs) == 0 {
				t.Fatal("no test specs found")
			}

			// Process each test group within the file
			for _, spec := range specs {
				// Each test group runs as a separate subtest
				t.Run(spec.Name, func(t *testing.T) {
					t.Parallel()

					if len(spec.Cases) == 0 {
						t.Fatal("no test cases found")
					}

					g := gitignore.New(strings.Split(spec.Gitignore, "\n")...)

					// Process each individual test case
					for _, tc := range spec.Cases {
						// Format test name to clearly indicate directories
						testName := tc.Path
						if tc.Dir {
							testName += "/"
						}

						// Each test case runs as a separate subtest for precise failure reporting
						t.Run(testName, func(t *testing.T) {
							t.Parallel()

							// Test the actual gitignore logic
							got := g.Ignored(tc.Path, tc.Dir)
							if got != tc.Ignored {
								// Create detailed error message with hierarchical context
								errorMsg := fmt.Sprintf("%s -> %s -> %s\n", base, spec.Name, testName)

								errorMsg += fmt.Sprintf("File: %s\n", f)

								// Include descriptions from YAML for better context
								if spec.Description != "" {
									errorMsg += fmt.Sprintf("Group: %s\n", spec.Description)
								}

								if tc.Description != "" {
									errorMsg += fmt.Sprintf("Case: %s\n", tc.Description)
								}

								errorMsg += fmt.Sprintf(
									"Ignored() check failed:\n  path: %v\n  dir: %v\n  patterns: %v\n  expected: %v\n  got: %v\n",
									tc.Path,
									tc.Dir,
									strings.Split(spec.Gitignore, "\n"),
									boolToIgnored(tc.Ignored),
									boolToIgnored(got),
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
