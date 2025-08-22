package gitignore_test

import (
	"testing"

	gitignore "github.com/idelchi/go-gitignore"
)

// TestGitIgnore contains the main test cases to validate parity with gitignore.
//
//nolint:maintidx		// The number of test cases are warranted.
func TestGitIgnore(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		Group        string
		Description  string
		Patterns     []string
		Path         string
		IsDir        bool
		ShouldIgnore bool
	}{
		// Basic patterns
		{
			Group:        "basic",
			Description:  "simple file pattern",
			Patterns:     []string{"one"},
			Path:         "one",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "basic",
			Description:  "simple file pattern in subdirectory",
			Patterns:     []string{"one"},
			Path:         "a/one",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "basic",
			Description:  "non-matching file",
			Patterns:     []string{"one"},
			Path:         "two",
			IsDir:        false,
			ShouldIgnore: false,
		},

		// Wildcard patterns
		{
			Group:        "wildcards",
			Description:  "star wildcard prefix",
			Patterns:     []string{"*.o"},
			Path:         "file.o",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "wildcards",
			Description:  "star wildcard prefix in subdirectory",
			Patterns:     []string{"*.o"},
			Path:         "src/internal.o",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "wildcards",
			Description:  "star wildcard suffix",
			Patterns:     []string{"ignored-*"},
			Path:         "ignored-file",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "wildcards",
			Description:  "star wildcard suffix in subdirectory",
			Patterns:     []string{"ignored-*"},
			Path:         "a/ignored-but-in-index",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "wildcards",
			Description:  "two star prefix matches",
			Patterns:     []string{"two*"},
			Path:         "a/b/twooo",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "wildcards",
			Description:  "star suffix matches",
			Patterns:     []string{"*three"},
			Path:         "a/3-three",
			IsDir:        false,
			ShouldIgnore: true,
		},

		// Directory patterns
		{
			Group:        "directories",
			Description:  "directory with trailing slash",
			Patterns:     []string{"top-level-dir/"},
			Path:         "top-level-dir",
			IsDir:        true,
			ShouldIgnore: true,
		},
		{
			Group:        "directories",
			Description:  "file not matched by directory pattern",
			Patterns:     []string{"top-level-dir/"},
			Path:         "top-level-dir",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "directories",
			Description:  "nested directory with trailing slash",
			Patterns:     []string{"ignored-dir/"},
			Path:         "a/b/ignored-dir",
			IsDir:        true,
			ShouldIgnore: true,
		},
		{
			Group:        "directories",
			Description:  "files inside ignored directory",
			Patterns:     []string{"ignored-dir/"},
			Path:         "a/b/ignored-dir/foo",
			IsDir:        false,
			ShouldIgnore: true,
		},

		// Negation patterns
		{
			Group:        "negation",
			Description:  "negated pattern",
			Patterns:     []string{"*", "!important.txt"},
			Path:         "important.txt",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "negation",
			Description:  "negated pattern with wildcards",
			Patterns:     []string{"*.html", "!foo.html"},
			Path:         "foo.html",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "negation",
			Description:  "other html file still ignored",
			Patterns:     []string{"*.html", "!foo.html"},
			Path:         "bar.html",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "negation",
			Description:  "negated directory pattern",
			Patterns:     []string{"/*", "!/foo", "/foo/*", "!/foo/bar"},
			Path:         "foo/bar",
			IsDir:        true,
			ShouldIgnore: false,
		},

		// Rooted patterns (with leading slash)
		{
			Group:        "rooted",
			Description:  "rooted pattern matches at root",
			Patterns:     []string{"/hello.txt"},
			Path:         "hello.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "rooted",
			Description:  "rooted pattern doesn't match in subdirectory",
			Patterns:     []string{"/hello.txt"},
			Path:         "a/hello.txt",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "rooted",
			Description:  "rooted wildcard pattern",
			Patterns:     []string{"/hello.*"},
			Path:         "hello.c",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "rooted",
			Description:  "rooted wildcard pattern doesn't match in subdirectory",
			Patterns:     []string{"/hello.*"},
			Path:         "a/hello.java",
			IsDir:        false,
			ShouldIgnore: false,
		},

		// Middle slash patterns
		{
			Group:        "middle-slash",
			Description:  "pattern with middle slash",
			Patterns:     []string{"doc/frotz"},
			Path:         "doc/frotz",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "middle-slash",
			Description:  "pattern with middle slash matches with leading slash",
			Patterns:     []string{"/doc/frotz"},
			Path:         "doc/frotz",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "middle-slash",
			Description:  "pattern with middle slash does NOT match in subdirectory (Git anchoring)",
			Patterns:     []string{"doc/frotz"},
			Path:         "a/doc/frotz",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "middle-slash",
			Description:  "foo/* matches file in foo",
			Patterns:     []string{"foo/*"},
			Path:         "foo/test.json",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "middle-slash",
			Description:  "foo/* matches directory in foo",
			Patterns:     []string{"foo/*"},
			Path:         "foo/bar",
			IsDir:        true,
			ShouldIgnore: true,
		},
		{
			Group:        "middle-slash",
			Description:  "foo/* does NOT match nested files (per Git spec)",
			Patterns:     []string{"foo/*"},
			Path:         "foo/bar/hello.c",
			IsDir:        false,
			ShouldIgnore: false,
		},

		// Git spec compliance tests for foo/* pattern
		{
			Group:        "git-spec-compliance",
			Description:  "foo/* matches direct file foo/test.json",
			Patterns:     []string{"foo/*"},
			Path:         "foo/test.json",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "git-spec-compliance",
			Description:  "foo/* matches direct directory foo/bar",
			Patterns:     []string{"foo/*"},
			Path:         "foo/bar",
			IsDir:        true,
			ShouldIgnore: true,
		},
		{
			Group:        "git-spec-compliance",
			Description:  "foo/* does NOT match nested foo/bar/hello.c",
			Patterns:     []string{"foo/*"},
			Path:         "foo/bar/hello.c",
			IsDir:        false,
			ShouldIgnore: false,
		},

		// Parent directory exclusion rule tests
		{
			Group:        "parent-exclusion",
			Description:  "Cannot re-include file if parent dir excluded",
			Patterns:     []string{"build/", "!build/important.txt"},
			Path:         "build/important.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "parent-exclusion",
			Description:  "Can re-include directory itself",
			Patterns:     []string{"build/", "!build/"},
			Path:         "build",
			IsDir:        true,
			ShouldIgnore: false,
		},

		// Double asterisk patterns
		{
			Group:        "double-asterisk",
			Description:  "**/foo matches foo anywhere",
			Patterns:     []string{"**/foo"},
			Path:         "foo",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "double-asterisk",
			Description:  "**/foo matches foo in subdirectory",
			Patterns:     []string{"**/foo"},
			Path:         "a/b/c/foo",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "double-asterisk",
			Description:  "**/foo/bar matches bar under any foo",
			Patterns:     []string{"**/foo/bar"},
			Path:         "foo/bar",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "double-asterisk",
			Description:  "**/foo/bar matches bar under any foo in subdirectory",
			Patterns:     []string{"**/foo/bar"},
			Path:         "a/b/foo/bar",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "double-asterisk",
			Description:  "abc/** matches everything inside abc",
			Patterns:     []string{"abc/**"},
			Path:         "abc/file.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "double-asterisk",
			Description:  "abc/** matches deeply nested file",
			Patterns:     []string{"abc/**"},
			Path:         "abc/x/y/z/file.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "double-asterisk",
			Description:  "a/**/b matches a/b",
			Patterns:     []string{"a/**/b"},
			Path:         "a/b",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "double-asterisk",
			Description:  "a/**/b matches a/x/b",
			Patterns:     []string{"a/**/b"},
			Path:         "a/x/b",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "double-asterisk",
			Description:  "a/**/b matches a/x/y/b",
			Patterns:     []string{"a/**/b"},
			Path:         "a/x/y/b",
			IsDir:        false,
			ShouldIgnore: true,
		},

		// Question mark patterns
		{
			Group:        "question-mark",
			Description:  "? matches single character",
			Patterns:     []string{"file.?"},
			Path:         "file.c",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "question-mark",
			Description:  "? doesn't match multiple characters",
			Patterns:     []string{"file.?"},
			Path:         "file.cc",
			IsDir:        false,
			ShouldIgnore: false,
		},

		// Range patterns
		{
			Group:        "range",
			Description:  "[a-z] matches lowercase letter",
			Patterns:     []string{"file[a-z].txt"},
			Path:         "filec.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "range",
			Description:  "[a-z] doesn't match uppercase",
			Patterns:     []string{"file[a-z].txt"},
			Path:         "fileC.txt",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "range",
			Description:  "[a-zA-Z] matches any letter",
			Patterns:     []string{"file[a-zA-Z].txt"},
			Path:         "fileC.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},

		// Complex patterns from test suite
		{
			Group:        "complex",
			Description:  "vmlinux* pattern",
			Patterns:     []string{"vmlinux*"},
			Path:         "arch/foo/kernel/vmlinux.lds.S",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "complex",
			Description:  "negation overrides globally with single root gitignore",
			Patterns:     []string{"vmlinux*", "!vmlinux*"},
			Path:         "arch/foo/kernel/vmlinux.lds.S",
			IsDir:        false,
			ShouldIgnore: false,
		},

		// Escaped patterns
		{
			Group:        "escaped",
			Description:  "escaped exclamation mark",
			Patterns:     []string{`\!important!.txt`},
			Path:         "!important!.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "escaped",
			Description:  "escaped hash",
			Patterns:     []string{`\#hashtag`},
			Path:         "#hashtag",
			IsDir:        false,
			ShouldIgnore: true,
		},

		// Comment and blank line handling
		{
			Group:        "special-lines",
			Description:  "pattern after comment",
			Patterns:     []string{"# this is a comment", "actual-pattern"},
			Path:         "actual-pattern",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "special-lines",
			Description:  "pattern after blank line",
			Patterns:     []string{"pattern1", "", "pattern2"},
			Path:         "pattern2",
			IsDir:        false,
			ShouldIgnore: true,
		},

		// Trailing whitespace
		{
			Group:        "whitespace",
			Description:  "trailing spaces are ignored",
			Patterns:     []string{"trailing   "},
			Path:         "trailing",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "whitespace",
			Description:  "escaped trailing spaces",
			Patterns:     []string{`trailing\ \ `},
			Path:         "trailing  ",
			IsDir:        false,
			ShouldIgnore: true,
		},

		// From test suite - nested includes with negation
		{
			Group:        "nested-negation",
			Description:  "multiple negation levels - on* pattern negates globally",
			Patterns:     []string{"four", "five", "six", "ignored-dir/", "!on*", "!two"},
			Path:         "a/b/one",
			IsDir:        false,
			ShouldIgnore: false,
		},

		// Edge cases
		{
			Group:        "edge-cases",
			Description:  "dot as path",
			Patterns:     []string{"*"},
			Path:         ".",
			IsDir:        true,
			ShouldIgnore: false,
		},
		{
			Group:        "edge-cases",
			Description:  "empty path",
			Patterns:     []string{"*"},
			Path:         "",
			IsDir:        false,
			ShouldIgnore: false,
		},

		// Exact prefix matching tests from suite
		{
			Group:        "prefix-matching",
			Description:  "git/ matches git directory",
			Patterns:     []string{"git/"},
			Path:         "a/git",
			IsDir:        true,
			ShouldIgnore: true,
		},
		{
			Group:        "prefix-matching",
			Description:  "git/ matches file in git directory",
			Patterns:     []string{"git/"},
			Path:         "a/git/foo",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "prefix-matching",
			Description:  "git/ doesn't match git-foo directory",
			Patterns:     []string{"git/"},
			Path:         "a/git-foo",
			IsDir:        true,
			ShouldIgnore: false,
		},
		{
			Group:        "prefix-matching",
			Description:  "git/ doesn't match file in git-foo",
			Patterns:     []string{"git/"},
			Path:         "a/git-foo/bar",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "prefix-matching",
			Description:  "/git/ matches git at root",
			Patterns:     []string{"/git/"},
			Path:         "git",
			IsDir:        true,
			ShouldIgnore: true,
		},
		{
			Group:        "prefix-matching",
			Description:  "/git/ doesn't match git in subdirectory",
			Patterns:     []string{"/git/"},
			Path:         "a/git",
			IsDir:        true,
			ShouldIgnore: false,
		},

		// Data/** pattern tests from suite
		{
			Group:        "complex-double-asterisk",
			Description:  "data/** ignores file",
			Patterns:     []string{"data/**", "!data/**/", "!data/**/*.txt"},
			Path:         "data/file",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "complex-double-asterisk",
			Description:  "data/** ignores nested file",
			Patterns:     []string{"data/**", "!data/**/", "!data/**/*.txt"},
			Path:         "data/data1/file1",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "complex-double-asterisk",
			Description:  "data/** allows .txt files",
			Patterns:     []string{"data/**", "!data/**/", "!data/**/*.txt"},
			Path:         "data/data1/file1.txt",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "complex-double-asterisk",
			Description:  "data/** allows nested .txt files",
			Patterns:     []string{"data/**", "!data/**/", "!data/**/*.txt"},
			Path:         "data/data2/file2.txt",
			IsDir:        false,
			ShouldIgnore: false,
		},

		// Extra tests
		// Ignore all except directories and specific extensions
		{
			Group:        "except-dirs-and-ext",
			Description:  "* !*/ !*.txt - hidden file matched by *",
			Patterns:     []string{"*", "!*/", "!*.txt"},
			Path:         ".file",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "except-dirs-and-ext",
			Description:  "* !*/ !*.txt - go file in subdir ignored",
			Patterns:     []string{"*", "!*/", "!*.txt"},
			Path:         "internal/file.go",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "except-dirs-and-ext",
			Description:  "* !*/ !*.txt - txt file in subdir allowed",
			Patterns:     []string{"*", "!*/", "!*.txt"},
			Path:         "internal/file.txt",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "except-dirs-and-ext",
			Description:  "* !*/ !*.txt - directory allowed",
			Patterns:     []string{"*", "!*/", "!*.txt"},
			Path:         "internal",
			IsDir:        true,
			ShouldIgnore: false,
		},
		{
			Group:        "except-dirs-and-ext",
			Description:  "* !*/ !*.txt - root txt file allowed",
			Patterns:     []string{"*", "!*/", "!*.txt"},
			Path:         "readme.txt",
			IsDir:        false,
			ShouldIgnore: false,
		},

		// Same but with specific directory ignored
		{
			Group:        "except-dirs-and-ext-with-ignored",
			Description:  "* !*/ !*.txt internal - hidden file ignored",
			Patterns:     []string{"*", "!*/", "!*.txt", "internal"},
			Path:         ".file",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "except-dirs-and-ext-with-ignored",
			Description:  "* !*/ !*.txt internal - go file in internal ignored",
			Patterns:     []string{"*", "!*/", "!*.txt", "internal"},
			Path:         "internal/file.go",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "except-dirs-and-ext-with-ignored",
			Description:  "* !*/ !*.txt internal - txt file in internal ignored",
			Patterns:     []string{"*", "!*/", "!*.txt", "internal"},
			Path:         "internal/file.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "except-dirs-and-ext-with-ignored",
			Description:  "* !*/ !*.txt internal - internal dir ignored",
			Patterns:     []string{"*", "!*/", "!*.txt", "internal"},
			Path:         "internal",
			IsDir:        true,
			ShouldIgnore: true,
		},
		{
			Group:        "except-dirs-and-ext-with-ignored",
			Description:  "* !*/ !*.txt internal - root txt file allowed",
			Patterns:     []string{"*", "!*/", "!*.txt", "internal"},
			Path:         "file.txt",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "except-dirs-and-ext-with-ignored",
			Description:  "* !*/ !*.txt internal - txt in other dir allowed",
			Patterns:     []string{"*", "!*/", "!*.txt", "internal"},
			Path:         "pkg/file.txt",
			IsDir:        false,
			ShouldIgnore: false,
		},

		// Multiple extensions allowed
		{
			Group:        "multiple-extensions",
			Description:  "* !*/ !*.go !*.mod !*.sum - go file allowed",
			Patterns:     []string{"*", "!*/", "!*.go", "!*.mod", "!*.sum"},
			Path:         "main.go",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "multiple-extensions",
			Description:  "* !*/ !*.go !*.mod !*.sum - mod file allowed",
			Patterns:     []string{"*", "!*/", "!*.go", "!*.mod", "!*.sum"},
			Path:         "go.mod",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "multiple-extensions",
			Description:  "* !*/ !*.go !*.mod !*.sum - nested go file allowed",
			Patterns:     []string{"*", "!*/", "!*.go", "!*.mod", "!*.sum"},
			Path:         "cmd/app/main.go",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "multiple-extensions",
			Description:  "* !*/ !*.go !*.mod !*.sum - txt file ignored",
			Patterns:     []string{"*", "!*/", "!*.go", "!*.mod", "!*.sum"},
			Path:         "readme.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "multiple-extensions",
			Description:  "* !*/ !*.go !*.mod !*.sum - binary ignored",
			Patterns:     []string{"*", "!*/", "!*.go", "!*.mod", "!*.sum"},
			Path:         "app",
			IsDir:        false,
			ShouldIgnore: true,
		},

		// Build artifacts ignored but source allowed
		{
			Group:        "build-artifacts",
			Description:  "* !*/ !*.go build/ - source in build ignored",
			Patterns:     []string{"*", "!*/", "!*.go", "build/"},
			Path:         "build/generated.go",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "build-artifacts",
			Description:  "* !*/ !*.go build/ - build dir ignored",
			Patterns:     []string{"*", "!*/", "!*.go", "build/"},
			Path:         "build",
			IsDir:        true,
			ShouldIgnore: true,
		},
		{
			Group:        "build-artifacts",
			Description:  "* !*/ !*.go build/ - source outside build allowed",
			Patterns:     []string{"*", "!*/", "!*.go", "build/"},
			Path:         "src/main.go",
			IsDir:        false,
			ShouldIgnore: false,
		},

		// Vendor exception pattern
		{
			Group:        "vendor-exception",
			Description:  "* !*/ !*.go vendor/**/*.go - vendor go file ignored",
			Patterns:     []string{"*", "!*/", "!*.go", "vendor/**/*.go"},
			Path:         "vendor/github.com/pkg/file.go",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "vendor-exception",
			Description:  "* !*/ !*.go vendor/**/*.go - non-vendor go allowed",
			Patterns:     []string{"*", "!*/", "!*.go", "vendor/**/*.go"},
			Path:         "internal/app.go",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "vendor-exception",
			Description:  "* !*/ !*.go vendor/**/*.go - vendor dir allowed",
			Patterns:     []string{"*", "!*/", "!*.go", "vendor/**/*.go"},
			Path:         "vendor",
			IsDir:        true,
			ShouldIgnore: false,
		},

		// Hidden files and directories special handling
		{
			Group:        "hidden-special",
			Description:  "* !*/ !.gitignore - .gitignore allowed",
			Patterns:     []string{"*", "!*/", "!.gitignore"},
			Path:         ".gitignore",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "hidden-special",
			Description:  "* !*/ !.gitignore - other hidden file matched by *",
			Patterns:     []string{"*", "!*/", "!.gitignore"},
			Path:         ".env",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "hidden-special",
			Description:  "* !*/ !.gitignore - nested .gitignore allowed",
			Patterns:     []string{"*", "!*/", "!.gitignore"},
			Path:         "subdir/.gitignore",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "hidden-special",
			Description:  "* !*/ !.git/ - .git dir allowed",
			Patterns:     []string{"*", "!*/", "!.git/"},
			Path:         ".git",
			IsDir:        true,
			ShouldIgnore: false,
		},

		// Test files special case
		{
			Group:        "test-files",
			Description:  "* !*/ !*_test.go - test file allowed",
			Patterns:     []string{"*", "!*/", "!*_test.go"},
			Path:         "main_test.go",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "test-files",
			Description:  "* !*/ !*_test.go - nested test file allowed",
			Patterns:     []string{"*", "!*/", "!*_test.go"},
			Path:         "pkg/utils/helper_test.go",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "test-files",
			Description:  "* !*/ !*_test.go - non-test go file ignored",
			Patterns:     []string{"*", "!*/", "!*_test.go"},
			Path:         "main.go",
			IsDir:        false,
			ShouldIgnore: true,
		},

		// Documentation files only
		{
			Group:        "docs-only",
			Description:  "* !*/ !*.md !*.txt !LICENSE - md file allowed",
			Patterns:     []string{"*", "!*/", "!*.md", "!*.txt", "!LICENSE"},
			Path:         "README.md",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "docs-only",
			Description:  "* !*/ !*.md !*.txt !LICENSE - LICENSE allowed",
			Patterns:     []string{"*", "!*/", "!*.md", "!*.txt", "!LICENSE"},
			Path:         "LICENSE",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "docs-only",
			Description:  "* !*/ !*.md !*.txt !LICENSE - nested md allowed",
			Patterns:     []string{"*", "!*/", "!*.md", "!*.txt", "!LICENSE"},
			Path:         "docs/api.md",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "docs-only",
			Description:  "* !*/ !*.md !*.txt !LICENSE - go file ignored",
			Patterns:     []string{"*", "!*/", "!*.md", "!*.txt", "!LICENSE"},
			Path:         "main.go",
			IsDir:        false,
			ShouldIgnore: true,
		},

		// Complex: ignore all except go, then re-ignore generated
		{
			Group:        "except-go-ignore-generated",
			Description:  "allow go but ignore generated - regular go allowed",
			Patterns:     []string{"*", "!*/", "!*.go", "*.generated.go", "*.pb.go"},
			Path:         "main.go",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "except-go-ignore-generated",
			Description:  "allow go but ignore generated - generated ignored",
			Patterns:     []string{"*", "!*/", "!*.go", "*.generated.go", "*.pb.go"},
			Path:         "models.generated.go",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "except-go-ignore-generated",
			Description:  "allow go but ignore generated - pb.go ignored",
			Patterns:     []string{"*", "!*/", "!*.go", "*.generated.go", "*.pb.go"},
			Path:         "api/service.pb.go",
			IsDir:        false,
			ShouldIgnore: true,
		},

		// Node modules style ignore with exceptions
		{
			Group:        "node-style",
			Description:  "* !*/ !package.json node_modules - package.json in node_modules ignored",
			Patterns:     []string{"*", "!*/", "!package.json", "node_modules"},
			Path:         "node_modules/package.json",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "node-style",
			Description:  "* !*/ !package.json node_modules - root package.json allowed",
			Patterns:     []string{"*", "!*/", "!package.json", "node_modules"},
			Path:         "package.json",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "node-style",
			Description:  "* !*/ !package.json node_modules - node_modules dir ignored",
			Patterns:     []string{"*", "!*/", "!package.json", "node_modules"},
			Path:         "node_modules",
			IsDir:        true,
			ShouldIgnore: true,
		},

		// Test to verify the special case code for foo/* is unnecessary
		{
			Group:        "foo-star-special-case",
			Description:  "foo/* should NOT match at nested level (Git anchoring)",
			Patterns:     []string{"foo/*"},
			Path:         "deep/nested/foo/bar",
			IsDir:        false,
			ShouldIgnore: false, // Git anchoring: foo/* only matches at root level
		},
		{
			Group:        "foo-star-special-case",
			Description:  "foo/* should not match foo/bar/baz at any level",
			Patterns:     []string{"foo/*"},
			Path:         "deep/nested/foo/bar/baz",
			IsDir:        false,
			ShouldIgnore: false,
		},

		// Tests to verify the directory matching logic handles these edge cases correctly
		{
			Group:        "dir-pattern-edge",
			Description:  "build pattern should match build directory at any level",
			Patterns:     []string{"build"},
			Path:         "src/build",
			IsDir:        true,
			ShouldIgnore: true,
		},
		{
			Group:        "dir-pattern-edge",
			Description:  "build pattern should match files inside build at any level",
			Patterns:     []string{"build"},
			Path:         "src/build/output.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "dir-pattern-edge",
			Description:  "build pattern should match deeply nested build dirs",
			Patterns:     []string{"build"},
			Path:         "a/b/c/d/build/e/f/g.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},

		// Tests to verify wildcard directory negation behavior
		{
			Group:        "wildcard-dir-negation",
			Description:  "!*/ should un-ignore directories only, not files",
			Patterns:     []string{"*", "!*/"},
			Path:         "file.txt",
			IsDir:        false,
			ShouldIgnore: true, // File should remain ignored
		},
		{
			Group:        "wildcard-dir-negation",
			Description:  "!*/ should un-ignore all directories",
			Patterns:     []string{"*", "!*/"},
			Path:         "somedir",
			IsDir:        true,
			ShouldIgnore: false, // Directory should be un-ignored
		},
		{
			Group:        "wildcard-dir-negation",
			Description:  "!**/ should work similarly for nested dirs",
			Patterns:     []string{"**/*", "!**/"},
			Path:         "deep/nested/dir",
			IsDir:        true,
			ShouldIgnore: false,
		},
		{
			Group:        "wildcard-dir-negation",
			Description:  "!**/ should not un-ignore files",
			Patterns:     []string{"**/*", "!**/"},
			Path:         "deep/nested/file.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "wildcard-dir-negation",
			Description:  "!*/ should un-ignore all directories but not files",
			Patterns:     []string{"*", "!*/"},
			Path:         "a/file.txt",
			IsDir:        false,
			ShouldIgnore: true, // File should remain ignored
		},
		{
			Group:        "wildcard-dir-negation",
			Description:  "!*/ should un-ignore all directories but only files explicitly given",
			Patterns:     []string{"*", "!*/", "!*.go"},
			Path:         "a/file.txt",
			IsDir:        false,
			ShouldIgnore: true, // File should remain ignored
		},
		{
			Group:        "wildcard-dir-negation",
			Description:  "!*/ should un-ignore all directories but only files explicitly given",
			Patterns:     []string{"*", "!*/", "!*.go"},
			Path:         "a/b/file.go",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "wildcard-dir-negation",
			Description:  "!*/ should un-ignore all directories but only files explicitly given",
			Patterns:     []string{"*", "!*/", "!*.go"},
			Path:         "file.go",
			IsDir:        false,
			ShouldIgnore: false,
		},

		{
			Group:        "parent-exclusion-non-dir",
			Description:  "cannot re-include file if parent dir excluded by non-slash pattern",
			Patterns:     []string{"build", "!build/important.txt"},
			Path:         "build/important.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "parent-exclusion-non-dir",
			Description:  "cannot re-include directory if parent dir excluded by non-slash pattern",
			Patterns:     []string{"build", "!build/sub/"},
			Path:         "build/sub",
			IsDir:        true,
			ShouldIgnore: true,
		},
		{
			Group:        "parent-exclusion-dironly",
			Description:  "cannot re-include subdirectory when parent directory excluded",
			Patterns:     []string{"build/", "!build/sub/"},
			Path:         "build/sub",
			IsDir:        true,
			ShouldIgnore: true,
		},
		{
			Group:        "negated-dir-does-not-include-files",
			Description:  "!build/ should not un-ignore files inside without explicit file rules",
			Patterns:     []string{"*", "!build/"},
			Path:         "build/file.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "negated-dir-does-not-include-files",
			Description:  "!/build/ should not un-ignore files inside without explicit file rules",
			Patterns:     []string{"/", "!/build/"},
			Path:         "build/file.txt",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "negated-dir-does-not-include-files",
			Description:  "!/build/ un-ignores directory and files inside (Git behavior)",
			Patterns:     []string{"/*", "!/build/"},
			Path:         "build/file.txt",
			IsDir:        false,
			ShouldIgnore: false, // Git actually allows this file
		},
		{
			Group:        "parent-exclusion-wildcard",
			Description:  "cannot re-include file if parent directory excluded by wildcard name pattern",
			Patterns:     []string{"tmp*", "!tmpcache/keep.txt"},
			Path:         "tmpcache/keep.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "parent-exclusion-non-dir-depth",
			Description:  "cannot re-include deep file if ancestor dir excluded by non-slash pattern",
			Patterns:     []string{"build", "!deep/build/important.txt"},
			Path:         "deep/build/important.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "parent-exclusion-dironly-wildcard",
			Description:  "cannot re-include nested directory under an ignored parent directory",
			Patterns:     []string{"foo*/", "!foobar/baz/"},
			Path:         "foobar/baz",
			IsDir:        true,
			ShouldIgnore: true,
		},

		// Anchoring tests (verify correct Git behavior)
		{
			Group:        "anchoring",
			Description:  "doc/frotz should match only at repo root",
			Patterns:     []string{"doc/frotz"},
			Path:         "doc/frotz",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "anchoring",
			Description:  "doc/frotz must NOT match in subdir",
			Patterns:     []string{"doc/frotz"},
			Path:         "a/doc/frotz",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "anchoring",
			Description:  "foo/bar should match only at repo root",
			Patterns:     []string{"foo/bar"},
			Path:         "foo/bar",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "anchoring",
			Description:  "foo/bar must NOT match at deeper levels",
			Patterns:     []string{"foo/bar"},
			Path:         "x/y/foo/bar",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "anchoring",
			Description:  "to match at any depth, use **/ prefix",
			Patterns:     []string{"**/doc/frotz"},
			Path:         "a/doc/frotz",
			IsDir:        false,
			ShouldIgnore: true,
		},

		// Fixed /* pattern tests
		{
			Group:        "anchored-star",
			Description:  "/* should ignore top-level dir 'folder' itself",
			Patterns:     []string{"/*"},
			Path:         "folder",
			IsDir:        true,
			ShouldIgnore: true,
		},
		{
			Group:        "anchored-star",
			Description:  "/* DOES ignore nested files via parent exclusion (Issue #3)",
			Patterns:     []string{"/*"},
			Path:         "folder/nested.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "anchored-star",
			Description:  "/* should ignore top-level file",
			Patterns:     []string{"/*"},
			Path:         "top.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},

		// Parent exclusion edge cases
		{
			Group:        "parent-exclusion-edges",
			Description:  "negating dir alone doesn't re-include files inside",
			Patterns:     []string{"build/", "!build/"},
			Path:         "build/file.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "parent-exclusion-edges",
			Description:  "non-slash parent exclusion blocks deep re-inclusion",
			Patterns:     []string{"tmp*", "!tmpcache/keep.txt"},
			Path:         "tmpcache/keep.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},

		// Dir-only vs non-dirs
		{
			Group:        "dironly-symlink-guard",
			Description:  "dir-only pattern shouldn't match non-dirs",
			Patterns:     []string{"symlinked-dir/"},
			Path:         "symlinked-dir",
			IsDir:        false, // simulate non-directory (e.g., symlink)
			ShouldIgnore: false,
		},

		// Leading space comments
		{
			Group:        "leading-space-comment",
			Description:  "leading spaces before # => literal pattern, not comment",
			Patterns:     []string{"  #notacomment"},
			Path:         "  #notacomment",
			IsDir:        false,
			ShouldIgnore: true, // matches the literal "  #notacomment"
		},
		{
			Group:        "leading-space-comment",
			Description:  "escaped # should match literal",
			Patterns:     []string{`\#hashtag`},
			Path:         "#hashtag",
			IsDir:        false,
			ShouldIgnore: true,
		},

		// Escape sequence tests
		{
			Group:        "escape-sequences",
			Description:  "literal backslash",
			Patterns:     []string{"file\\\\"},
			Path:         "file\\",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "escape-sequences",
			Description:  "escaped wildcard * should be literal",
			Patterns:     []string{"\\*.txt"},
			Path:         "*.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "escape-sequences",
			Description:  "escaped wildcard ? should be literal",
			Patterns:     []string{"file\\?.txt"},
			Path:         "file?.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "escape-sequences",
			Description:  "escaped bracket should be literal",
			Patterns:     []string{"\\[test\\]"},
			Path:         "[test]",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "escape-sequences",
			Description:  "escaped wildcard should NOT match as wildcard",
			Patterns:     []string{"\\*.txt"},
			Path:         "file.txt",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "escape-sequences",
			Description:  "escaped ? should NOT match as wildcard",
			Patterns:     []string{"file\\?.txt"},
			Path:         "filea.txt",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "escape-sequences",
			Description:  "escaped bracket should NOT match as range",
			Patterns:     []string{"\\[test\\]"},
			Path:         "atest]",
			IsDir:        false,
			ShouldIgnore: false,
		},

		// Trailing space escape tests
		{
			Group:        "trailing-spaces",
			Description:  "single escaped trailing space",
			Patterns:     []string{"file\\ "},
			Path:         "file ",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "trailing-spaces",
			Description:  "multiple escaped trailing spaces",
			Patterns:     []string{"file\\ \\ "},
			Path:         "file  ",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "trailing-spaces",
			Description:  "mixed escaped and unescaped trailing spaces",
			Patterns:     []string{"file\\ \\  "},
			Path:         "file  ",
			IsDir:        false,
			ShouldIgnore: true,
		},

		// Issue #1: Mixed literal and wildcard patterns
		{
			Group:        "mixed-literal-wildcard",
			Description:  "pattern with literal star followed by wildcard",
			Patterns:     []string{"ab\\*cd*.txt"},
			Path:         "ab*cd.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "mixed-literal-wildcard",
			Description:  "pattern with literal star followed by wildcard matching file",
			Patterns:     []string{"ab\\*cd*.txt"},
			Path:         "ab*cdHello.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "mixed-literal-wildcard",
			Description:  "pattern with literal star should not match without star",
			Patterns:     []string{"ab\\*cd*.txt"},
			Path:         "abcd.txt",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "mixed-literal-wildcard",
			Description:  "pattern should not match abZZcd.txt (no star in path)",
			Patterns:     []string{"ab\\*cd*.txt"},
			Path:         "abZZcd.txt",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "mixed-literal-wildcard",
			Description:  "pattern should match ab*cdZZ.txt (has literal star)",
			Patterns:     []string{"ab\\*cd*.txt"},
			Path:         "ab*cdZZ.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "mixed-literal-wildcard",
			Description:  "foo\\*bar pattern matches literal foo*bar",
			Patterns:     []string{"foo\\*bar"},
			Path:         "foo*bar",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "mixed-literal-wildcard",
			Description:  "foo\\*bar pattern does not match fooxbar",
			Patterns:     []string{"foo\\*bar"},
			Path:         "fooxbar",
			IsDir:        false,
			ShouldIgnore: false,
		},

		// Issue #2: Explicit directory re-inclusion
		{
			Group:        "dir-reinclusion",
			Description:  "explicit dir negation clears exclusion",
			Patterns:     []string{"build/", "!build/", "!build/keep.txt"},
			Path:         "build/keep.txt",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "dir-reinclusion",
			Description:  "explicit dir negation allows subdir negation",
			Patterns:     []string{"build", "!build/", "!build/foo"},
			Path:         "build/foo",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "dir-reinclusion",
			Description:  "dir stays ignored without explicit negation",
			Patterns:     []string{"build/", "!build/keep.txt"},
			Path:         "build/keep.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},

		// Issue #3: Anchored /* pattern with nested paths
		{
			Group:        "anchored-star-nested",
			Description:  "/* ignores subdir/file.txt via parent exclusion",
			Patterns:     []string{"/*"},
			Path:         "subdir/file.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "anchored-star-nested",
			Description:  "/* ignores deep/nested/path via parent exclusion",
			Patterns:     []string{"/*"},
			Path:         "deep/nested/path",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "anchored-star-nested",
			Description:  "/* still ignores top-level file",
			Patterns:     []string{"/*"},
			Path:         "file.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "anchored-star-nested",
			Description:  "/* still ignores top-level directory",
			Patterns:     []string{"/*"},
			Path:         "folder",
			IsDir:        true,
			ShouldIgnore: true,
		},

		// Issue #4: Curly brace expansion - FIXED for Git parity
		// Git treats braces as literal characters, not expansion syntax
		{
			Group:        "curly-braces",
			Description:  "Git treats braces literally - 'foo' NOT matched",
			Patterns:     []string{"{foo,bar}"},
			Path:         "foo",
			IsDir:        false,
			ShouldIgnore: false, // Git doesn't expand braces
		},
		{
			Group:        "curly-braces",
			Description:  "Git treats braces literally - 'bar' NOT matched",
			Patterns:     []string{"{foo,bar}"},
			Path:         "bar",
			IsDir:        false,
			ShouldIgnore: false, // Git doesn't expand braces
		},
		{
			Group:        "curly-braces",
			Description:  "literal {foo,bar} pattern matches exact string",
			Patterns:     []string{"{foo,bar}"},
			Path:         "{foo,bar}",
			IsDir:        false,
			ShouldIgnore: true, // Exact literal match
		},
		{
			Group:        "curly-braces",
			Description:  "complex pattern with braces treated literally",
			Patterns:     []string{"lib/{braces}.txt"},
			Path:         "lib/{braces}.txt",
			IsDir:        false,
			ShouldIgnore: true, // Exact match
		},
		{
			Group:        "curly-braces",
			Description:  "complex pattern does not expand braces",
			Patterns:     []string{"lib/{braces}.txt"},
			Path:         "lib/braces.txt",
			IsDir:        false,
			ShouldIgnore: false, // No expansion
		},
		{
			Group:        "curly-braces",
			Description:  "nested braces treated literally",
			Patterns:     []string{"config/{local,{prod,dev}}.json"},
			Path:         "config/{local,{prod,dev}}.json",
			IsDir:        false,
			ShouldIgnore: true, // Exact match
		},
		{
			Group:        "curly-braces",
			Description:  "nested braces don't match expanded forms",
			Patterns:     []string{"config/{local,{prod,dev}}.json"},
			Path:         "config/local.json",
			IsDir:        false,
			ShouldIgnore: false, // No expansion
		},
		{
			Group:        "curly-braces",
			Description:  "escaped braces in pattern",
			Patterns:     []string{"\\{escaped\\}.txt"},
			Path:         "{escaped}.txt",
			IsDir:        false,
			ShouldIgnore: true, // Escaped braces match literally
		},
		{
			Group:        "curly-braces",
			Description:  "mixed escaped and unescaped braces",
			Patterns:     []string{"\\{{a,b}\\}"},
			Path:         "{{a,b}}",
			IsDir:        false,
			ShouldIgnore: true, // Outer braces escaped, inner treated literally
		},

		// Hardening tests from input file for tight Git parity
		{
			Group:        "hardening-tests",
			Description:  "mixed literal + wildcard: ab*cd.txt should be ignored",
			Patterns:     []string{"ab\\*cd*.txt"},
			Path:         "ab*cd.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "hardening-tests",
			Description:  "mixed literal + wildcard: ab*cdZZ.txt should be ignored",
			Patterns:     []string{"ab\\*cd*.txt"},
			Path:         "ab*cdZZ.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "hardening-tests",
			Description:  "mixed literal + wildcard: abcd.txt should NOT be ignored",
			Patterns:     []string{"ab\\*cd*.txt"},
			Path:         "abcd.txt",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "hardening-tests",
			Description:  "mixed literal + wildcard: abZZcd.txt should NOT be ignored",
			Patterns:     []string{"ab\\*cd*.txt"},
			Path:         "abZZcd.txt",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "hardening-tests",
			Description:  "curly braces: foo NOT matched (Git treats literally)",
			Patterns:     []string{"{foo,bar}"},
			Path:         "foo",
			IsDir:        false,
			ShouldIgnore: false, // Git doesn't expand
		},
		{
			Group:        "hardening-tests",
			Description:  "curly braces: bar NOT matched (Git treats literally)",
			Patterns:     []string{"{foo,bar}"},
			Path:         "bar",
			IsDir:        false,
			ShouldIgnore: false, // Git doesn't expand
		},
		{
			Group:        "hardening-tests",
			Description:  "curly braces: literal {foo,bar} IS matched",
			Patterns:     []string{"{foo,bar}"},
			Path:         "{foo,bar}",
			IsDir:        false,
			ShouldIgnore: true, // Exact match
		},

		// Issue #5: Parent exclusion scenarios
		{
			Group:        "parent-exclusion-hierarchy",
			Description:  "cannot re-include nested path when parent excluded",
			Patterns:     []string{"foo/", "!foo/bar/", "!foo/bar/baz.txt"},
			Path:         "foo/bar/baz.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "parent-exclusion-hierarchy",
			Description:  "must unignore each level explicitly",
			Patterns:     []string{"foo/", "!foo/", "!foo/bar/", "!foo/bar/baz.txt"},
			Path:         "foo/bar/baz.txt",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "parent-exclusion-hierarchy",
			Description:  "parent exclusion applies to deeply nested paths",
			Patterns:     []string{"top/", "!top/a/b/c/file.txt"},
			Path:         "top/a/b/c/file.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		// Mid-pattern escaped spaces
		{
			Group:        "mid-pattern-escaped-spaces",
			Description:  "file with escaped space in middle",
			Patterns:     []string{"file\\ name.txt"},
			Path:         "file name.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "mid-pattern-escaped-spaces",
			Description:  "directory path with escaped space",
			Patterns:     []string{"dir/file\\ name.txt"},
			Path:         "dir/file name.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "mid-pattern-escaped-spaces",
			Description:  "multiple escaped spaces",
			Patterns:     []string{"my\\ \\ file.txt"},
			Path:         "my  file.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		// Cross-platform separator tests (protect against filepath)
		{
			Group:        "cross-platform-separators",
			Description:  "literal backslash in basename - matches basename",
			Patterns:     []string{"file\\\\"},
			Path:         "dir/file\\",
			IsDir:        false,
			ShouldIgnore: true, // Pattern "file\" matches basename "file\"
		},
		{
			Group:        "cross-platform-separators",
			Description:  "literal backslash in basename - should match exact",
			Patterns:     []string{"file\\\\"},
			Path:         "file\\",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "cross-platform-separators",
			Description:  "backslash in middle of pattern",
			Patterns:     []string{"foo\\\\bar"},
			Path:         "foo\\bar",
			IsDir:        false,
			ShouldIgnore: true,
		},
		// Edge ** forms
		{
			Group:        "edge-doublestar-forms",
			Description:  "** alone matches any file at any depth",
			Patterns:     []string{"**"},
			Path:         "a/b/c/file.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "edge-doublestar-forms",
			Description:  "** alone matches any directory at any depth",
			Patterns:     []string{"**"},
			Path:         "a/b/c",
			IsDir:        true,
			ShouldIgnore: true,
		},
		{
			Group:        "edge-doublestar-forms",
			Description:  "**/ matches only directories",
			Patterns:     []string{"**/"},
			Path:         "a/b",
			IsDir:        true,
			ShouldIgnore: true,
		},
		{
			Group:        "edge-doublestar-forms",
			Description:  "**/ matches files inside any directory",
			Patterns:     []string{"**/"},
			Path:         "a/b/file.txt",
			IsDir:        false,
			ShouldIgnore: true, // Git's **/ ignores files in any directory
		},
		{
			Group:        "edge-doublestar-forms",
			Description:  "!**/ un-ignores directories after **/*",
			Patterns:     []string{"**/*", "!**/"},
			Path:         "a/b/c",
			IsDir:        true,
			ShouldIgnore: false,
		},
		{
			Group:        "edge-doublestar-forms",
			Description:  "/** at end matches everything under prefix",
			Patterns:     []string{"foo/**"},
			Path:         "foo/bar/baz.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		// Brace escapes in character classes
		{
			Group:        "brace-in-character-class",
			Description:  "unescaped braces in character class - match { literally",
			Patterns:     []string{"file[{}].txt"},
			Path:         "file{.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "brace-in-character-class",
			Description:  "unescaped braces in character class - match } literally",
			Patterns:     []string{"file[{}].txt"},
			Path:         "file}.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "brace-in-character-class",
			Description:  "escaped braces in character class",
			Patterns:     []string{"file[\\{\\}].txt"},
			Path:         "file{.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "brace-in-character-class",
			Description:  "mix of braces in and out of character class",
			Patterns:     []string{"[{]foo,bar[}]"},
			Path:         "{foo,bar}",
			IsDir:        false,
			ShouldIgnore: true,
		},
		// Leading multi-backslash before !
		{
			Group:        "leading-multi-backslash",
			Description:  "double backslash before ! is literal, not negation",
			Patterns:     []string{"\\\\!keep.txt", "*"},
			Path:         "\\!keep.txt",
			IsDir:        false,
			ShouldIgnore: true, // First pattern doesn't negate, second ignores all
		},
		{
			Group:        "leading-multi-backslash",
			Description:  "single backslash before ! is literal, not negation",
			Patterns:     []string{"\\!keep.txt", "*"},
			Path:         "!keep.txt",
			IsDir:        false,
			ShouldIgnore: true, // Still literal, not negation
		},
		{
			Group:        "leading-multi-backslash",
			Description:  "triple backslash before ! - escapes backslash then !",
			Patterns:     []string{"\\\\\\!keep.txt"},
			Path:         "\\!keep.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		// Malformed glob patterns (should not panic)
		{
			Group:        "malformed-patterns",
			Description:  "unclosed character class",
			Patterns:     []string{"file["},
			Path:         "file[",
			IsDir:        false,
			ShouldIgnore: false, // Git doesn't match malformed patterns
		},
		{
			Group:        "malformed-patterns",
			Description:  "unclosed character class doesn't match other chars",
			Patterns:     []string{"file["},
			Path:         "filea",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "malformed-patterns",
			Description:  "unmatched closing bracket",
			Patterns:     []string{"file]"},
			Path:         "file]",
			IsDir:        false,
			ShouldIgnore: true,
		},
		// Backslash torture tests (from Git's suite)
		{
			Group:        "backslash-torture",
			Description:  "trailing space with single backslash escape",
			Patterns:     []string{"file\\ "},
			Path:         "file ",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "backslash-torture",
			Description:  "trailing space with double backslash - no escape",
			Patterns:     []string{"file\\\\ "},
			Path:         "file\\",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "backslash-torture",
			Description:  "trailing spaces with triple backslash - escapes backslash then space",
			Patterns:     []string{"file\\\\\\ "},
			Path:         "file\\ ",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "backslash-torture",
			Description:  "multiple trailing spaces with mixed escapes",
			Patterns:     []string{"file\\  \\ "},
			Path:         "file   ",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "backslash-torture",
			Description:  "quadruple backslash before space - two literal backslashes",
			Patterns:     []string{"file\\\\\\\\ "},
			Path:         "file\\\\",
			IsDir:        false,
			ShouldIgnore: true,
		},
		// Edge case: "/" pattern (should be no-op)
		{
			Group:        "edge-cases",
			Description:  "single slash pattern is no-op",
			Patterns:     []string{"/", "*.txt"},
			Path:         "file.txt",
			IsDir:        false,
			ShouldIgnore: true, // Only second pattern matches
		},
		{
			Group:        "edge-cases",
			Description:  "single slash pattern doesn't affect matching",
			Patterns:     []string{"/"},
			Path:         "anything",
			IsDir:        false,
			ShouldIgnore: false, // "/" is no-op, nothing ignored
		},
		// Negation non-dir parent (complex parent exclusion)
		{
			Group:        "negation-non-dir-parent",
			Description:  "non-dir !build should not by itself un-ignore child files when parent stays ignored",
			Patterns:     []string{"*", "build", "!*.txt"}, // directories still ignored by "*"
			Path:         "build/file.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "negation-non-dir-parent",
			Description:  "non-dir !build should not by itself un-ignore child files when parent stays ignored",
			Patterns:     []string{"*", "build", "!build", "!*.txt"}, // directories still ignored by "*"
			Path:         "build/file.txt",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "negation-non-dir-parent",
			Description:  "un-ignore build dir then allow .txt",
			Patterns:     []string{"build", "!build", "!*.txt"},
			Path:         "build/file.txt",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "negation-non-dir-parent",
			Description:  "un-ignore build dir then allow .txt",
			Patterns:     []string{"*", "!build", "!*.txt"},
			Path:         "build/file.py",
			IsDir:        false,
			ShouldIgnore: true,
		},
		// Doublestar zero-segment behavior
		{
			Group:        "doublestar-zero",
			Description:  "a/** should treat zero segments as valid and ignore a/ itself via parent rules",
			Patterns:     []string{"a/**"},
			Path:         "a",
			IsDir:        true,
			ShouldIgnore: true,
		},
		{
			Group:        "doublestar-zero",
			Description:  "negating a/**/ should re-allow the a/ directory itself",
			Patterns:     []string{"a/**", "!a/**/"},
			Path:         "a",
			IsDir:        true,
			ShouldIgnore: false,
		},
		// Braces inside character classes remain literal after escape pass
		{
			Group:        "brace-in-class-escape",
			Description:  "braces inside [] stay literal despite global brace escaping",
			Patterns:     []string{"file[{}].md"},
			Path:         "file{.md",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "brace-in-class-escape",
			Description:  "ensure '}' also literal in class",
			Patterns:     []string{"file[{}].md"},
			Path:         "file}.md",
			IsDir:        false,
			ShouldIgnore: true,
		},
		// Weird ** placements that Git traditionally treats as two *
		{
			Group:        "odd-doublestar",
			Description:  "ab**cd should behave like ab* *cd (Git-style), not cross slashes",
			Patterns:     []string{"ab**cd"},
			Path:         "ab/x/cd", // with slash
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "odd-doublestar",
			Description:  "ab**cd matches same-segment expansions",
			Patterns:     []string{"ab**cd"},
			Path:         "abZZcd",
			IsDir:        false,
			ShouldIgnore: true,
		},
		// Negation edge cases
		{
			Group:        "negation-edge",
			Description:  "bare ! line is ignored",
			Patterns:     []string{"!", "*.txt"},
			Path:         "x.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		// Non-directory pattern Git parity tests
		{
			Group:        "negation-parent-basename",
			Description:  "Non-dir !internal must NOT re-include files under internal/",
			Patterns:     []string{"*", "!*/", "!internal"},
			Path:         "internal/config/file/a.sh",
			IsDir:        false,
			ShouldIgnore: true, // Git ignores this
		},
		{
			Group:        "non-dir-direntry",
			Description:  "Pattern 'internal' ignores a directory entry named 'internal'",
			Patterns:     []string{"internal"},
			Path:         "internal",
			IsDir:        true,
			ShouldIgnore: true,
		},
		{
			Group:        "reinclude-needs-path",
			Description:  "Re-include files under internal only when pattern matches the file path",
			Patterns:     []string{"*", "!*/", "!internal/", "!internal/**"},
			Path:         "internal/config/file/a.sh",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "dir-negation-no-file",
			Description:  "Dir-only negation does not re-include files",
			Patterns:     []string{"*", "!*/", "!internal/"},
			Path:         "internal/config/file/a.sh",
			IsDir:        false,
			ShouldIgnore: true,
		},
		// A) Slash-containing directory patterns (the current gap)
		{
			Group:        "dir-pattern-mid-slash",
			Description:  "a/b/ ignores directory a/b",
			Patterns:     []string{"a/b/"},
			Path:         "a/b",
			IsDir:        true,
			ShouldIgnore: true,
		},
		{
			Group:        "dir-pattern-mid-slash",
			Description:  "a/b/ ignores a/b/c.txt under it",
			Patterns:     []string{"a/b/"},
			Path:         "a/b/c.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "dir-pattern-mid-slash",
			Description:  "a/b/ does not match a/b as a file",
			Patterns:     []string{"a/b/"},
			Path:         "a/b", // pretend file entry
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "dir-pattern-mid-slash",
			Description:  "negated !a/b/ re-allows the directory itself",
			Patterns:     []string{"a/b/", "!a/b/"},
			Path:         "a/b",
			IsDir:        true,
			ShouldIgnore: false,
		},
		{
			Group:        "dir-pattern-mid-slash-parent-excl",
			Description:  "a/ excluded; !a/b/ cannot re-include a/b",
			Patterns:     []string{"a/", "!a/b/"},
			Path:         "a/b",
			IsDir:        true,
			ShouldIgnore: true, // cannot hop into excluded parent
		},
		// B) !build re-opens directory, !*.txt re-includes files (the demo case)
		{
			Group:        "reopen-then-reinclude",
			Description:  "* build !build !*.txt => txt inside build is allowed",
			Patterns:     []string{"*", "build", "!build", "!*.txt"},
			Path:         "build/file.txt",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "reopen-then-reinclude",
			Description:  "* build !*.txt => still ignored because parent closed",
			Patterns:     []string{"*", "build", "!*.txt"},
			Path:         "build/file.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "reopen-then-reinclude",
			Description:  "* build/ !build/ !*.txt => txt inside build allowed",
			Patterns:     []string{"*", "build/", "!build/", "!*.txt"},
			Path:         "build/file.txt",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "qualified-exception",
			Description:  "build/ then !build/file.txt cannot re-include due to parent exclusion",
			Patterns:     []string{"build/", "!build/file.txt"},
			Path:         "build/file.txt",
			IsDir:        false,
			ShouldIgnore: true, // Parent exclusion: build/ is excluded, cannot re-include files inside
		},
		{
			Group:        "qualified-exception",
			Description:  "sibling remains ignored",
			Patterns:     []string{"build/", "!build/file.txt"},
			Path:         "build/other.bin",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "qualified-exception-correct",
			Description:  "build !build/file.txt cannot re-include due to parent exclusion",
			Patterns:     []string{"build", "!build/file.txt"},
			Path:         "build/file.txt",
			IsDir:        false,
			ShouldIgnore: true, // Parent exclusion: build excludes directory
		},
		{
			Group:        "qualified-exception-correct",
			Description:  "sibling also ignored due to parent exclusion",
			Patterns:     []string{"build", "!build/file.txt"},
			Path:         "build/other.bin",
			IsDir:        false,
			ShouldIgnore: true, // Parent exclusion applies
		},
		{
			Group:        "qualified-exception-with-reopen",
			Description:  "build !build !build/file.txt re-includes file",
			Patterns:     []string{"build", "!build", "!build/file.txt"},
			Path:         "build/file.txt",
			IsDir:        false,
			ShouldIgnore: false, // Directory re-opened, file explicitly included
		},
		{
			Group:        "qualified-exception-with-reopen",
			Description:  "sibling also included after directory reopen",
			Patterns:     []string{"build", "!build", "!build/file.txt"},
			Path:         "build/other.bin",
			IsDir:        false,
			ShouldIgnore: false, // !build re-opens entire directory
		},
		// C) Root-anchored star (/*) correctness
		{
			Group:        "root-star",
			Description:  "/* matches only top-level entries",
			Patterns:     []string{"/*"},
			Path:         "top",
			IsDir:        true,
			ShouldIgnore: true,
		},
		{
			Group:        "root-star",
			Description:  "/* causes parent exclusion for nested entries",
			Patterns:     []string{"/*"},
			Path:         "top/nested/file.txt",
			IsDir:        false,
			ShouldIgnore: true, // Parent exclusion: top is excluded by /*
		},
		// D) Character class and ? coverage (sanity vs doublestar)
		{
			Group:        "charclass-qmark",
			Description:  "log?.txt matches one digit only",
			Patterns:     []string{"log?.txt"},
			Path:         "log1.txt",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "charclass-qmark",
			Description:  "log10.txt does not match log?.txt",
			Patterns:     []string{"log?.txt"},
			Path:         "log10.txt",
			IsDir:        false,
			ShouldIgnore: false,
		},
		{
			Group:        "charclass-range",
			Description:  "img[0-2].png matches",
			Patterns:     []string{"img[0-2].png"},
			Path:         "img1.png",
			IsDir:        false,
			ShouldIgnore: true,
		},
		{
			Group:        "charclass-range",
			Description:  "img3.png does not match img[0-2].png",
			Patterns:     []string{"img[0-2].png"},
			Path:         "img3.png",
			IsDir:        false,
			ShouldIgnore: false,
		},
		// E) Directory pattern with mid slash + negation cascade (hierarchy)
		{
			Group:        "parent-exclusion-hierarchy",
			Description:  "foo/ blocks; !foo/bar/ alone cannot re-include",
			Patterns:     []string{"foo/", "!foo/bar/"},
			Path:         "foo/bar",
			IsDir:        true,
			ShouldIgnore: true,
		},
		{
			Group:        "parent-exclusion-hierarchy",
			Description:  "must unignore each level explicitly",
			Patterns:     []string{"foo/", "!foo/", "!foo/bar/"},
			Path:         "foo/bar",
			IsDir:        true,
			ShouldIgnore: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Group+"/"+tc.Description, func(t *testing.T) {
			t.Parallel()

			g := gitignore.New(tc.Patterns)
			isIgnored := g.IsIgnored(tc.Path, tc.IsDir)

			if isIgnored != tc.ShouldIgnore {
				t.Errorf("Ignore(patterns=%v, path=%q, isDir=%v) = %v, want %v",
					tc.Patterns, tc.Path, tc.IsDir, isIgnored, tc.ShouldIgnore)
			}
		})
	}
}
