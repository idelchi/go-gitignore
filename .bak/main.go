// verify_check_ignore.go
//
// Usage: go run tests/validator.go <tests.yml>
//
//	(expects the file under ./tests/<tests.yml>)
//
// Requires: git in PATH
package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	yaml "github.com/goccy/go-yaml"
)

type Case struct {
	Path        string `yaml:"path"`
	Description string `yaml:"description"`
	Ignored     *bool  `yaml:"ignored"`
	Dir         bool   `yaml:"dir"`
}

type Test struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Gitignore   string `yaml:"gitignore"`
	Cases       []Case `yaml:"cases"`
}

type result struct {
	TestIdx   int
	CaseIdx   int
	TestName  string
	TestDesc  string
	Gitignore string
	Case      Case

	CmdLine   string
	ExitCode  int
	Stdout    string
	Stderr    string
	Actual    bool
	Expected  *bool
	HasAssert bool // expected != nil
	Pass      bool // only meaningful if HasAssert
}

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("usage: %s <tests.yml>", os.Args[0])
	}
	yamlName := os.Args[1]

	data, err := os.ReadFile(filepath.Join("tests", yamlName))
	if err != nil {
		log.Fatalf("reading YAML: %v", err)
	}

	var tests []Test
	if err := yaml.Unmarshal(data, &tests); err != nil {
		log.Fatalf("parsing YAML: %v", err)
	}

	var results []result
	totalCases := 0

	for ti, t := range tests {
		if strings.TrimSpace(t.Gitignore) == "" {
			// Skip nodes without a gitignore payload
			// continue
		}
		for ci, c := range t.Cases {
			totalCases++

			// Fresh temp repo per case to avoid file/dir collisions across cases
			tmp, err := os.MkdirTemp("", "gitignore-verify-*")
			if err != nil {
				log.Fatalf("mktemp: %v", err)
			}
			func() {
				defer os.RemoveAll(tmp)

				// Init repo
				if out, err := runCmd(tmp, "git", "init", "-q"); err != nil {
					log.Fatalf("git init failed: %v\n%s", err, out)
				}

				// Write .gitignore for this test
				if err := os.WriteFile(filepath.Join(tmp, ".gitignore"), []byte(t.Gitignore), 0o644); err != nil {
					log.Fatalf("write .gitignore: %v", err)
				}
				// Ensure repo-local excludes empty
				_ = os.WriteFile(filepath.Join(tmp, ".git", "info", "exclude"), []byte{}, 0o644)

				// Materialize the path under test
				target := filepath.Join(tmp, filepath.FromSlash(c.Path))
				if c.Dir {
					if err := os.MkdirAll(target, 0o755); err != nil {
						log.Fatalf("mkdir %q: %v", c.Path, err)
					}
					_ = os.WriteFile(filepath.Join(target, ".keep"), []byte{}, 0o644)
				} else {
					if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
						log.Fatalf("mkdir parents for %q: %v", c.Path, err)
					}
					if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
						log.Fatalf("write file %q (test=%q): %v", target, c.Description, err)
					}
				}

				// Run: git -c core.excludesfile=/dev/null check-ignore -v -n -- <path>
				argPath := filepath.ToSlash(c.Path) // relative to repo root
				args := []string{
					"-c", "core.excludesfile=/dev/null",
					"-c", "core.ignorecase=false",
					"check-ignore", "-q", "--", argPath,
				}
				_, _, code := runGit(tmp, args...)

				// Infer match from verbose output: any line NOT starting with "::" indicates a match
				actualIgnored := code == 0

				r := result{
					TestIdx:   ti + 1,
					CaseIdx:   ci + 1,
					TestName:  t.Name,
					TestDesc:  t.Description,
					Gitignore: t.Gitignore,
					Case:      c,
					ExitCode:  code,
					Actual:    actualIgnored,
					Expected:  c.Ignored,
					HasAssert: c.Ignored != nil,
					Pass:      c.Ignored != nil && actualIgnored == *c.Ignored,
				}
				results = append(results, r)
			}()
		}
	}

	// Summarize
	var pass, fail, skip int
	for _, r := range results {
		if !r.HasAssert {
			skip++
			continue
		}
		if r.Pass {
			pass++
		} else {
			fail++
		}
	}

	// STDOUT: overall summary + per-pass concise lines
	fmt.Printf("=== Verification summary: %s ===\n", yamlName)
	fmt.Printf("tests: %d  cases: %d  asserting: %d  pass: %d  fail: %d  skip: %d\n",
		countTests(results), len(results), pass+fail, pass, fail, skip)

	for _, r := range results {
		if r.HasAssert && r.Pass {
			dirTag := ""
			if r.Case.Dir {
				dirTag = " [dir]"
			}
			desc := ""
			if s := strings.TrimSpace(r.Case.Description); s != "" {
				desc = " | " + s
			}
			fmt.Printf("PASS [T%d.C%d] %q path=%q%s expected=%v got=%v\n",
				r.TestIdx, r.CaseIdx, r.TestName, r.Case.Path, dirTag, *r.Expected, r.Actual)
			if desc != "" {
				fmt.Println("      ", strings.TrimPrefix(desc, " | "))
			}
		}
	}

	// STDERR: per-failure detailed diagnostics with full spec and git output
	for _, r := range results {
		if !r.HasAssert || r.Pass {
			continue
		}
		fmt.Fprintf(os.Stderr, "\nFAIL [T%d.C%d] %q\n", r.TestIdx, r.CaseIdx, r.TestName)
		if s := strings.TrimSpace(r.TestDesc); s != "" {
			fmt.Fprintf(os.Stderr, "  test.desc: %s\n", s)
		}
		fmt.Fprintf(os.Stderr, "  yaml.file: %s\n", yamlName)
		fmt.Fprintf(os.Stderr, "  .gitignore:\n%s\n", indentBlock(r.Gitignore, "    "))
		fmt.Fprintf(os.Stderr, "  case:\n")
		fmt.Fprintf(os.Stderr, "    path: %q\n", r.Case.Path)
		if r.Case.Dir {
			fmt.Fprintf(os.Stderr, "    dir: true\n")
		}
		if s := strings.TrimSpace(r.Case.Description); s != "" {
			fmt.Fprintf(os.Stderr, "    description: %s\n", s)
		}
		if r.Expected != nil {
			fmt.Fprintf(os.Stderr, "    expected_ignored: %v\n", *r.Expected)
		} else {
			fmt.Fprintf(os.Stderr, "    expected_ignored: (unspecified)\n")
		}
		fmt.Fprintf(os.Stderr, "    actual.ignored: %v (exit=%d)\n", r.Actual, r.ExitCode)
	}

	// Exit code: fail -> 1, otherwise 0
	if fail > 0 {
		os.Exit(1)
	}
}

func runGit(workingDir string, args ...string) (stdout, stderr string, exitCode int) {
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

func runCmd(workingDir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = workingDir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

func indentBlock(s, prefix string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n")
}

func countTests(rs []result) int {
	seen := map[int]struct{}{}
	for _, r := range rs {
		seen[r.TestIdx] = struct{}{}
	}
	return len(seen)
}
