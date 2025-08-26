package gitignore_test

import (
	"strings"
	"testing"

	gitignore "github.com/idelchi/go-gitignore"
)

// tc is a test case for GitIgnore details.
type tc struct {
	name      string
	gitignore string
	path      string
	isDir     bool
	ignored   bool
	details   string
}

// gi is a simple helper to return an initialize GitIgnorer.
func gi(lines string) *gitignore.GitIgnore {
	// Trim leading/trailing newlines and split to lines for New(...)
	trimmed := strings.Trim(lines, "\n")
	if trimmed == "" {
		return gitignore.New()
	}

	return gitignore.New(strings.Split(trimmed, "\n")...)
}

// TestGitDetails performs some simple tests to check the sanity of the details representation.
func TestGitDetails(t *testing.T) {
	t.Parallel()

	tcs := []tc{
		{
			name:      "01 simple star exclude",
			gitignore: "*\n",
			path:      "a.log",
			ignored:   true,
			details:   "*",
		},
		{
			name:      "02 star then rescue file",
			gitignore: "*\n!a.log\n",
			path:      "a.log",
			ignored:   false,
			details:   "!a.log",
		},
		{
			name:      "03 parent exclusion by dir slash",
			gitignore: "build/\n",
			path:      "build/file.txt",
			ignored:   true,
			details:   "build/",
		},
		{
			name:      "04 contents-only does not exclude base dir",
			gitignore: "build/**\n",
			path:      "build",
			isDir:     true,
			ignored:   false,
			details:   "",
		},
		{
			name:      "05 contents-only excludes child",
			gitignore: "build/**\n",
			path:      "build/x.txt",
			ignored:   true,
			details:   "build/**",
		},
		{
			name:      "06 build star rescue specific file",
			gitignore: "build/*\n!build/keep.txt\n",
			path:      "build/keep.txt",
			ignored:   false,
			details:   "!build/keep.txt",
		},
		{
			name:      "07 directory-glob-only (/**/)",
			gitignore: "dir/**/\n",
			path:      "dir/x",
			isDir:     true,
			ignored:   true,
			details:   "dir/**/",
		},
		{
			name:      "08 ignore all, unignore dirs only",
			gitignore: "*\n!*/\n",
			path:      "x",
			isDir:     true,
			ignored:   false,
			details:   "!*/",
		},
		{
			name:      "09 allow md under any dir",
			gitignore: "*\n!*/\n!*.md\n",
			path:      "docs/readme.md",
			ignored:   false,
			details:   "!*.md",
		},
		{
			name:      "10 star leaves non-md ignored",
			gitignore: "*\n!*/\n!*.md\n",
			path:      "readme.txt",
			ignored:   true,
			details:   "*",
		},
		{
			name:      "11 escaped braces literal",
			gitignore: `\{a\}` + "\n",
			path:      "{a}",
			ignored:   true,
			details:   `\{a\}`,
		},
		{
			name:      "12 escaped star literal",
			gitignore: `\*.txt` + "\n",
			path:      "*.txt",
			ignored:   true,
			details:   `\*.txt`,
		},
		{
			name:      "13 rooted file",
			gitignore: "/foo\n",
			path:      "foo",
			ignored:   true,
			details:   "/foo",
		},
		{
			name:      "14 rooted does not hit deeper",
			gitignore: "/foo\n",
			path:      "a/foo",
			ignored:   false,
			details:   "",
		},
		{
			name:      "15 segment exactness",
			gitignore: "foo/bar\n",
			path:      "foo/barbaz",
			ignored:   false,
			details:   "",
		},
		{
			name:      "16 dstar any depth",
			gitignore: "**/node_modules/**\n",
			path:      "src/node_modules/file.js",
			ignored:   true,
			details:   "**/node_modules/**",
		},
		{
			name:      "17 dstar base entry not excluded",
			gitignore: "**/node_modules/**\n",
			path:      "node_modules",
			isDir:     true,
			ignored:   false,
			details:   "",
		},
		{
			name:      "18 blocked negation inside excluded dir",
			gitignore: "build/\n!build/keep.txt\n",
			path:      "build/keep.txt",
			ignored:   true,
			details:   "build/",
		},
		{
			name:      "19 bang-dot cannot rescue root",
			gitignore: "*\n!.\n",
			path:      ".",
			isDir:     true,
			ignored:   true,
			details:   "*",
		},
		{
			name:      "20 dot-slash normalization rescue",
			gitignore: "*\n!x.txt\n",
			path:      "./x.txt",
			ignored:   false,
			details:   "!x.txt",
		},
		{
			name:      "21 double-slash POSIX special never ignored",
			gitignore: "*\n",
			path:      "//share",
			ignored:   false,
			details:   "",
		},
		{
			name:      "22 char class negation",
			gitignore: "file[!a].txt\n",
			path:      "fileb.txt",
			ignored:   true,
			details:   "file[!a].txt",
		},
		{
			name:      "23 escaped question literal",
			gitignore: `file\?.txt` + "\n",
			path:      "file?.txt",
			ignored:   true,
			details:   `file\?.txt`,
		},
		{
			name:      "24 bare name excludes dir",
			gitignore: "foo\n",
			path:      "foo",
			isDir:     true,
			ignored:   true,
			details:   "foo",
		},
		{
			name:      "25 child of bare-name-excluded dir",
			gitignore: "foo\n",
			path:      "foo/bar.txt",
			ignored:   true,
			details:   "foo",
		},
		{
			name:      "26 negation rescues specific log",
			gitignore: "*.log\n!important.log\n",
			path:      "important.log",
			ignored:   false,
			details:   "!important.log",
		},
		{
			name:      "27 empty gitignore means include",
			gitignore: "",
			path:      "a/b",
			ignored:   false,
			details:   "",
		},
		{
			name:      "28 slash no-op then txt exclude",
			gitignore: "/\n*.txt\n",
			path:      "file.txt",
			ignored:   true,
			details:   "*.txt",
		},
		{
			name:      "29 bare double-star matches all",
			gitignore: "**\n",
			path:      "a/b",
			ignored:   true,
			details:   "**",
		},
		{
			name:      "30 abc dstar does not exclude base dir",
			gitignore: "abc/**\n",
			path:      "abc",
			isDir:     true,
			ignored:   false,
			details:   "",
		},
	}

	for _, c := range tcs {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			gi := gi(c.gitignore)
			got := gi.Decide(c.path, c.isDir)

			if got.Ignored != c.ignored {
				t.Fatalf("Ignored mismatch: path=%q dir=%v got=%v want=%v (details got=%q)",
					c.path, c.isDir, got.Ignored, c.ignored, got.Details)
			}

			if got.Details != c.details {
				t.Fatalf("Details mismatch: path=%q dir=%v got=%q want=%q (ignored got=%v)",
					c.path, c.isDir, got.Details, c.details, got.Ignored)
			}
		})
	}
}
