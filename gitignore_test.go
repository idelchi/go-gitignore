package gitignore_test

import (
	"flag"
	"strings"
	"testing"

	gitignore "github.com/idelchi/go-gitignore"
)

var testFilter = flag.String("f", "", "Comma-separated list of test file names to run (without extension)")

func TestGitIgnore_YAML(t *testing.T) {
	t.Parallel()

	filter := ParseFilter(*testFilter)
	files, err := YamlFiles("./tests", filter)
	if err != nil {
		t.Fatalf("scan test dir: %v", err)
	}

	for _, f := range files {
		f := f // capture range variable
		base := BaseNameWithoutExt(f)

		t.Run(base, func(t *testing.T) {
			t.Parallel()

			specs, err := LoadGitIgnoreSpecs(f)
			if err != nil {
				t.Fatalf("load specs from %s: %v", f, err)
			}

			for _, spec := range specs {
				spec := spec // capture range variable
				t.Run(spec.Name, func(t *testing.T) {
					t.Parallel()

					g := gitignore.New(strings.Split(spec.Gitignore, "\n"))

					for _, tc := range spec.Cases {
						tc := tc // capture range variable
						testName := tc.Path
						if tc.IsDir {
							testName += "/"
						}

						t.Run(testName, func(t *testing.T) {
							got := g.Ignored(tc.Path, tc.IsDir)
							if got != tc.Ignored {
								// Format: test file -> test group -> test case
								errorMsg := base + " -> " + spec.Name + " -> " + testName + "\n"
								
								// Add descriptions if available
								if spec.Description != "" {
									errorMsg += "Group: " + spec.Description + "\n"
								}
								if tc.Description != "" {
									errorMsg += "Case: " + tc.Description + "\n"
								}
								
								// Add failure specifics
								errorMsg += "Expected Ignored(\"" + tc.Path + "\", isDir=" + 
									formatBool(tc.IsDir) + ") = " + formatBool(tc.Ignored) + 
									", got " + formatBool(got)
								
								t.Error(errorMsg)
							}
						})
					}
				})
			}
		})
	}
}
