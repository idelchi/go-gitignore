//go:build !windows

package gitignore_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestGitCheckIgnoreValidator validates YAML test specifications against actual git check-ignore behavior.
// It provides comprehensive validation of gitignore pattern matching by:
//  1. Loading test files from the tests/ directory (optionally filtered)
//  2. Parsing YAML test specifications using shared utilities
//  3. Creating temporary git repositories with specified .gitignore patterns
//  4. Running git check-ignore commands and comparing results with expected outcomes
//  5. Providing detailed error messages and validation summaries
//
// The test can be run in two modes:
//   - With -yaml flag: validates a specific YAML file (e.g., go test -yaml basic.yml)
//   - Without flag: validates all YAML files in ./tests directory
func TestGitCheckIgnore(t *testing.T) {
	t.Parallel()

	filter := ParseFilter(*testFilter)
	files, err := YamlFiles("./tests", filter)
	if err != nil {
		t.Fatalf("scan test dir: %v", err)
	}

	if len(files) == 0 {
		t.Skip("no test files found")
	}

	var totalPass, totalFail int

	for _, file := range files {
		file := file
		t.Run(BaseNameWithoutExt(file), func(t *testing.T) {
			t.Parallel()

			specs, err := LoadGitIgnoreSpecs(file)
			if err != nil {
				t.Fatalf("load specs from %s: %v", file, err)
			}

			for _, spec := range specs {
				spec := spec
				t.Run(spec.Name, func(t *testing.T) {
					t.Parallel()

					var results []validatorResult
					for ci, c := range spec.Cases {
						result := runGitCheckIgnoreTest(t, spec, c, ci+1)
						results = append(results, result)

						if result.Pass {
							totalPass++
						} else {
							totalFail++
							t.Errorf("path=%q expected=%v got=%v (exit=%d)",
								c.Path, c.Ignored, result.Actual, result.ExitCode)
						}
					}
				})
			}
		})
	}
}

// runGitCheckIgnoreTest executes a single git check-ignore test case
func runGitCheckIgnoreTest(t *testing.T, spec GitIgnore, c Case, caseIdx int) validatorResult {
	// Fresh temp repo per case to avoid file/dir collisions across cases
	tmp, err := os.MkdirTemp("", "gitignore-verify-*")
	if err != nil {
		t.Fatalf("mktemp: %v", err)
	}
	defer os.RemoveAll(tmp)

	// Init repo
	if out, err := runValidatorCmd(tmp, "git", "init", "-q"); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}

	// Write .gitignore for this test
	if err := os.WriteFile(filepath.Join(tmp, ".gitignore"), []byte(spec.Gitignore), 0o644); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}
	// Ensure repo-local excludes empty
	_ = os.WriteFile(filepath.Join(tmp, ".git", "info", "exclude"), []byte{}, 0o644)

	// Materialize the path under test
	target := filepath.Join(tmp, filepath.FromSlash(c.Path))
	if c.Dir {
		if err := os.MkdirAll(target, 0o755); err != nil {
			t.Fatalf("mkdir %q: %v", c.Path, err)
		}
		_ = os.WriteFile(filepath.Join(target, ".keep"), []byte{}, 0o644)
	} else {
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatalf("mkdir parents for %q: %v", c.Path, err)
		}
		if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
			t.Fatalf("write file %q (test=%q): %v", target, c.Description, err)
		}
	}

	// Run: git -c core.excludesfile=/dev/null check-ignore -q -- <path>
	argPath := filepath.ToSlash(c.Path) // relative to repo root
	args := []string{
		"-c", "core.excludesfile=/dev/null",
		"-c", "core.ignorecase=false",
		"check-ignore", "-q", "--", argPath,
	}
	_, _, code := runValidatorGit(tmp, args...)

	// Infer match from exit code: 0 means ignored
	actualIgnored := code == 0

	return validatorResult{
		CaseIdx:   caseIdx,
		TestName:  spec.Name,
		TestDesc:  spec.Description,
		Gitignore: spec.Gitignore,
		Case:      c,
		ExitCode:  code,
		Actual:    actualIgnored,
		Expected:  c.Ignored,
		Pass:      actualIgnored == c.Ignored,
	}
}

type validatorResult struct {
	CaseIdx   int
	TestName  string
	TestDesc  string
	Gitignore string
	Case      Case
	ExitCode  int
	Actual    bool
	Expected  bool
	Pass      bool
}

func runValidatorGit(workingDir string, args ...string) (stdout, stderr string, exitCode int) {
	cmd := exec.Command("git", args...)
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

func runValidatorCmd(workingDir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = workingDir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}
