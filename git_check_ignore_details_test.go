package gitignore_test

import (
	"fmt"
	"strings"
	"testing"
)

// TestGitCheckIgnoreDetails validates YAML test specifications against actual Git check-ignore behavior,
// focusing only on the output of git check-ignore -v
//
//nolint:gocognit	// Long and complex setup is warranted.
func TestGitCheckIgnoreDetails(t *testing.T) {
	t.Parallel()

	filter := ParseFilter(*testFilter)

	files, err := YamlFiles("./tests/details", filter)
	if err != nil {
		t.Fatalf("scan test dir: %v", err)
	}

	if len(files) == 0 {
		t.Fatal("no test files found")
	}

	// Process each test file concurrently
	for _, file := range files {
		base := BaseNameWithoutExt(file)

		// Each test file runs as a separate subtest
		t.Run(base, func(t *testing.T) {
			t.Parallel()

			specs, err := LoadGitIgnoreSpecs(file)
			if err != nil {
				t.Fatalf("load specs from %s: %v", file, err)
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

					// Process each individual test case
					for _, c := range spec.Cases {
						if c.Details == nil {
							t.Skip("no details expected output specified")
						}

						// Format test name to clearly indicate directories
						testName := c.Path
						if c.Dir {
							testName += "/"
						}

						// Each test case runs as a separate subtest for precise failure reporting
						t.Run(testName, func(t *testing.T) {
							t.Parallel()

							result := runGitCheckIgnoreTest(t, spec, c, "-v")

							if !strings.Contains(result.Stdout, *c.Details) {
								// Create detailed error message with hierarchical context
								errorMsg := fmt.Sprintf("%s -> %s -> %s\n", base, spec.Name, testName)

								// Include descriptions from YAML for better context
								if spec.Description != "" {
									errorMsg += fmt.Sprintf("Group: %s\n", spec.Description)
								}

								if c.Description != "" {
									errorMsg += fmt.Sprintf("Case: %s\n", c.Description)
								}

								// Provide specific details about the Git validation failure
								errorMsg += fmt.Sprintf(
									"Git check-ignore validation failed:\n  path: %v\n  patterns: %v\n  expected: %v\n  got: %v\n",
									c.Path,
									strings.Split(spec.Gitignore, "\n"),
									*c.Details,
									result.Stdout,
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
