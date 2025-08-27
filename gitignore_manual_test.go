package gitignore_test

import (
	"strings"
	"testing"

	gitignore "github.com/idelchi/go-gitignore"
)

// TestGitIgnoreBasic contains basic test cases for gitignore patterns.
func TestGitIgnoreBasic(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name         string
		gitignore    string
		path         string
		isDir        bool
		shouldIgnore bool
	}

	tcs := []testCase{
		// Basic wildcard patterns
		{
			name:         "simple wildcard match",
			gitignore:    "*.log",
			path:         "debug.log",
			isDir:        false,
			shouldIgnore: true,
		},
		{
			name:         "simple wildcard no match",
			gitignore:    "*.log",
			path:         "debug.txt",
			isDir:        false,
			shouldIgnore: false,
		},
		{
			name:         "nested wildcard match",
			gitignore:    "*.log",
			path:         "logs/app/debug.log",
			isDir:        false,
			shouldIgnore: true,
		},

		// Directory patterns
		{
			name:         "directory pattern file",
			gitignore:    "build/",
			path:         "build",
			isDir:        false,
			shouldIgnore: false,
		},
		{
			name:         "directory pattern dir",
			gitignore:    "build/",
			path:         "build",
			isDir:        true,
			shouldIgnore: true,
		},
		{
			name:         "directory pattern nested",
			gitignore:    "build/",
			path:         "src/build",
			isDir:        true,
			shouldIgnore: true,
		},

		// Negation patterns
		{
			name:         "negation basic",
			gitignore:    "*.log\n!important.log",
			path:         "important.log",
			isDir:        false,
			shouldIgnore: false,
		},
		{
			name:         "negation other file",
			gitignore:    "*.log\n!important.log",
			path:         "debug.log",
			isDir:        false,
			shouldIgnore: true,
		},
		{
			name:         "negation parent excluded",
			gitignore:    "logs/\n!logs/important.log",
			path:         "logs/important.log",
			isDir:        false,
			shouldIgnore: true, // Parent directory is excluded
		},

		// Rooted patterns
		{
			name:         "rooted pattern root",
			gitignore:    "/config",
			path:         "config",
			isDir:        false,
			shouldIgnore: true,
		},
		{
			name:         "rooted pattern nested",
			gitignore:    "/config",
			path:         "src/config",
			isDir:        false,
			shouldIgnore: false,
		},
		{
			name:         "rooted directory",
			gitignore:    "/tmp/",
			path:         "tmp",
			isDir:        true,
			shouldIgnore: true,
		},

		// Double star patterns
		{
			name:         "double star prefix",
			gitignore:    "**/cache",
			path:         "src/app/cache",
			isDir:        false,
			shouldIgnore: true,
		},
		{
			name:         "double star middle",
			gitignore:    "src/**/test.txt",
			path:         "src/a/b/c/test.txt",
			isDir:        false,
			shouldIgnore: true,
		},
		{
			name:         "double star suffix",
			gitignore:    "vendor/**",
			path:         "vendor/package/lib.go",
			isDir:        false,
			shouldIgnore: true,
		},
		{
			name:         "double star suffix base",
			gitignore:    "vendor/**",
			path:         "vendor",
			isDir:        true,
			shouldIgnore: false, // Base directory itself not ignored
		},

		// Complex patterns
		{
			name:         "node modules sandwich",
			gitignore:    "**/node_modules/**",
			path:         "project/node_modules/package/index.js",
			isDir:        false,
			shouldIgnore: true,
		},
		{
			name:         "node modules sandwich dir itself",
			gitignore:    "**/node_modules/**",
			path:         "project/node_modules",
			isDir:        true,
			shouldIgnore: false, // Directory itself not matched by sandwich pattern
		},
		{
			name:         "multiple patterns",
			gitignore:    "*.tmp\n*.cache\nbuild/\n!build/keep.txt\nnode_modules/",
			path:         "src/file.tmp",
			isDir:        false,
			shouldIgnore: true,
		},
		{
			name:         "escaped special characters",
			gitignore:    "\\#README\\#",
			path:         "#README#",
			isDir:        false,
			shouldIgnore: true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gi := gitignore.New(strings.Split(tc.gitignore, "\n")...)
			got := gi.Ignored(tc.path, tc.isDir)

			if got != tc.shouldIgnore {
				t.Errorf("Test %s failed:\n"+
					"  gitignore: %q\n"+
					"  path: %q (isDir: %v)\n"+
					"  expected ignored: %v\n"+
					"  got ignored: %v",
					tc.name, tc.gitignore, tc.path, tc.isDir, tc.shouldIgnore, got)
			}
		})
	}
}

func TestGitIgnoreEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("empty gitignore", func(t *testing.T) {
		t.Parallel()

		gi := gitignore.New()
		if gi.Ignored("anyfile.txt", false) {
			t.Error("Empty gitignore should not ignore any files")
		}
	})

	t.Run("comment lines", func(t *testing.T) {
		t.Parallel()

		gi := gitignore.New("# This is a comment", "*.log", "  # Another comment with spaces", "!important.log")

		if !gi.Ignored("debug.log", false) {
			t.Error("Should ignore .log files")
		}

		if gi.Ignored("important.log", false) {
			t.Error("Should not ignore negated important.log")
		}
	})

	t.Run("trailing spaces", func(t *testing.T) {
		t.Parallel()

		// Test with escaped trailing space
		gi := gitignore.New("file\\ ")
		if !gi.Ignored("file ", false) {
			t.Error("Should match file with trailing space when escaped")
		}

		if gi.Ignored("file", false) {
			t.Error("Should not match file without trailing space")
		}
	})

	t.Run("dot files", func(t *testing.T) {
		t.Parallel()

		gi := gitignore.New(".*")
		if !gi.Ignored(".gitignore", false) {
			t.Error("Should ignore dot files")
		}

		if !gi.Ignored(".config/settings", false) {
			t.Error("Should ignore paths starting with dot")
		}
	})

	t.Run("character classes", func(t *testing.T) {
		t.Parallel()

		gi := gitignore.New("test[0-9].txt")
		if !gi.Ignored("test5.txt", false) {
			t.Error("Should match test5.txt")
		}

		if gi.Ignored("testA.txt", false) {
			t.Error("Should not match testA.txt")
		}
	})
}
