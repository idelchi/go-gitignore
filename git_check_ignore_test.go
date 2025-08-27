//go:build !windows

package gitignore_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestGitCheckIgnore validates YAML test specifications against actual Git check-ignore behavior.
//
//nolint:gocognit	// Long and complex setup is warranted.
func TestGitCheckIgnore(t *testing.T) {
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
						// Format test name to clearly indicate directories
						testName := c.Path
						if c.Dir {
							testName += "/"
						}

						// Each test case runs as a separate subtest for precise failure reporting
						t.Run(testName, func(t *testing.T) {
							t.Parallel()

							result := runGitCheckIgnoreTest(t, spec, c)

							if !result.Pass {
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
									"Git check-ignore validation failed:\n  path: %v\n  patterns: %v\n  expected: %v\n  got: %v (exit=%d)\n",
									c.Path,
									strings.Split(spec.Gitignore, "\n"),
									boolToIgnored(result.Expected),
									boolToIgnored(result.Actual),
									result.ExitCode,
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

// runGitCheckIgnoreTest executes a single git check-ignore test case by creating
// a temporary git repository, writing the gitignore patterns, materializing the test path,
// and running the actual git check-ignore command to validate behavior.
func runGitCheckIgnoreTest(t *testing.T, spec GitIgnore, c Case, extraArgs ...string) validatorResult {
	t.Helper()

	// Fresh temp repo per case to avoid file/dir collisions across cases
	tmp := t.TempDir()

	// Init repo
	if out, err := runValidatorCmd(tmp, "git", "init", "-q"); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}

	// Write .gitignore for this test
	if err := os.WriteFile(filepath.Join(tmp, ".gitignore"), []byte(spec.Gitignore), 0o600); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}
	// Ensure repo-local excludes empty
	_ = os.WriteFile(filepath.Join(tmp, ".git", "info", "exclude"), []byte{}, 0o600)

	// Materialize the path under test
	target := filepath.Join(tmp, filepath.FromSlash(c.Path))
	if c.Dir {
		if err := os.MkdirAll(target, 0o750); err != nil {
			t.Fatalf("mkdir %q: %v", c.Path, err)
		}

		_ = os.WriteFile(filepath.Join(target, ".keep"), []byte{}, 0o600)
	} else {
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			t.Fatalf("mkdir parents for %q: %v", c.Path, err)
		}

		if err := os.WriteFile(target, []byte("x"), 0o600); err != nil {
			t.Fatalf("write file %q (test=%q): %v", target, c.Description, err)
		}
	}

	// Run: git check-ignore -q -- <path> to only get the exit code
	argPath := filepath.ToSlash(c.Path) // relative to repo root

	if len(extraArgs) == 0 {
		extraArgs = []string{"-q"}
	}

	args := []string{
		"-c", "core.excludesfile=/dev/null",
		"-c", "core.ignorecase=false",
		"check-ignore",
	}

	args = append(args, extraArgs...)

	args = append(args, "--", argPath)

	stdout, _, code := runValidatorGit(tmp, args...)

	// Infer match from exit code: 0 means ignored
	actualIgnored := code == 0

	return validatorResult{
		TestName:  spec.Name,
		TestDesc:  spec.Description,
		Gitignore: spec.Gitignore,
		Case:      c,
		ExitCode:  code,
		Actual:    actualIgnored,
		Expected:  c.Ignored,
		Pass:      actualIgnored == c.Ignored,
		Stdout:    stdout,
	}
}

// validatorResult holds the result of a git check-ignore validation test case.
type validatorResult struct {
	TestName  string // Name of the test group
	TestDesc  string // Description of the test group
	Gitignore string // The gitignore patterns being tested
	Case      Case   // The individual test case details
	ExitCode  int    // Exit code from git check-ignore command
	Actual    bool   // Actual result from git check-ignore
	Expected  bool   // Expected result from YAML specification
	Pass      bool   // Whether the test passed (actual == expected)
	Stdout    string // Captured stdout from git command (if any)
}

// runValidatorGit executes a git command in the specified working directory
// and returns stdout, stderr, and exit code. This is used specifically for
// running git check-ignore commands during validation.
func runValidatorGit(workingDir string, args ...string) (stdout, stderr string, exitCode int) {
	cmd := exec.CommandContext(context.Background(), "git", args...)

	cmd.Dir = workingDir

	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "PAGER=cat")

	var outBuf, errBuf bytes.Buffer

	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			exitCode = ee.ExitCode()
		} else {
			exitCode = 128
		}
	} else {
		exitCode = 0
	}

	return outBuf.String(), errBuf.String(), exitCode
}

// runValidatorCmd executes a generic command in the specified working directory.
// This is used for setup commands like git init during test preparation.
func runValidatorCmd(workingDir, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(context.Background(), name, args...)

	cmd.Dir = workingDir

	var out bytes.Buffer

	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()

	return out.String(), err
}
