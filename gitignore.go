// Package gitignore implements Git-compatible gitignore pattern matching with precise parity to Git's behavior.
// This package provides sophisticated gitignore logic including pattern parsing with escape sequences,
// two-pass ignore checking with parent exclusion detection, complex negation rules that match Git's exact behavior,
// brace escaping to prevent expansion, and cross-platform path handling using forward slashes only.
//
// The core algorithm implements Git's parent exclusion rule: once a directory is excluded by a pattern,
// its contents cannot be re-included by negation patterns, maintaining strict compatibility with Git's behavior.
// The package uses a two-pass algorithm - first identifying excluded parent directories, then applying
// pattern matching to the target path.
//
// Key features:
//   - Precise Git compatibility for all gitignore edge cases
//   - Two-pass algorithm for parent exclusion detection
//   - Complex negation semantics matching Git's behavior
//   - Proper escape sequence handling for trailing spaces and special characters
//   - Cross-platform path handling with forward slash normalization
//   - Brace escaping to prevent unintended expansion
//
// Usage:
//
//	gi := gitignore.New([]string{"*.log", "build/", "!important.log"})
//	ignored := gi.IsIgnored("app.log", false) // true
//	ignored = gi.IsIgnored("important.log", false) // false
package gitignore

import (
	"os"
	"path"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// pattern represents a parsed gitignore pattern with its attributes.
// It stores both the original line and the processed pattern along with
// flags that determine how the pattern should be matched.
type pattern struct {
	// original is the raw line from the gitignore file before any processing
	original string
	// pattern is the processed pattern after parsing escape sequences and trimming
	pattern string
	// negated indicates this is a negation pattern (starts with !)
	negated bool
	// dirOnly indicates this pattern only matches directories (ends with /)
	dirOnly bool
	// rooted indicates this pattern is anchored to the repository root (starts with /)
	rooted bool
}

// GitIgnore represents a collection of gitignore patterns and provides
// methods to check if paths should be ignored. It maintains Git-compatible
// behavior for all pattern matching and exclusion rules.
type GitIgnore struct {
	// patterns holds the parsed gitignore patterns in the order they appear
	patterns []pattern
}

// New creates a GitIgnore instance from lines of a .gitignore file.
// Each line is parsed according to Git's gitignore specification, handling
// comments, blank lines, negation patterns, escape sequences, and directory-only patterns.
//
// Lines starting with # are treated as comments and ignored.
// Lines starting with ! are negation patterns that can re-include previously ignored files.
// Lines ending with / only match directories.
// Lines starting with / are anchored to the repository root.
// Trailing spaces are trimmed unless escaped with a backslash.
//
// Example:
//
//	lines := []string{"*.log", "build/", "!important.log", "# comment"}
//	gi := New(lines)
//	ignored := gi.IsIgnored("app.log", false) // true
//	ignored = gi.IsIgnored("important.log", false) // false (negated)
func New(lines []string) *GitIgnore {
	if len(lines) == 0 {
		return &GitIgnore{}
	}

	// Pre-allocate slice with capacity to avoid multiple allocations
	patterns := make([]pattern, 0, len(lines))

	for _, line := range lines {
		if p := parsePattern(line); p != nil {
			patterns = append(patterns, *p)
		}
	}

	return &GitIgnore{
		patterns: patterns,
	}
}

// parsePattern parses a single line from a gitignore file into a pattern struct.
// Returns nil for blank lines, comments, or invalid patterns.
//
// The parsing follows Git's gitignore specification:
//   - Blank lines are ignored
//   - Lines starting with # are comments (unless escaped with \#)
//   - Lines starting with ! are negation patterns (unless escaped with \!)
//   - Lines ending with / only match directories
//   - Lines starting with / are anchored to repository root
//   - Trailing spaces are trimmed unless escaped with backslash
//   - Escape sequences \! and \# at line start are converted to literal ! and #
//
// The function handles complex edge cases including escape sequence processing,
// trailing space handling, and pattern validation to ensure Git compatibility.
func parsePattern(line string) *pattern {
	// Blank lines are ignored
	if line == "" {
		return nil
	}

	// Comments start with # (unless escaped)
	if strings.HasPrefix(line, "#") {
		return nil
	}

	pattern := &pattern{
		original: line,
	}

	// Handle escaped characters at the beginning
	switch {
	case strings.HasPrefix(line, "\\!"):
		line = line[1:] // Remove the backslash, keep the !
	case strings.HasPrefix(line, "\\#"):
		line = line[1:] // Remove the backslash, keep the #
	case strings.HasPrefix(line, "!"):
		// Negation pattern
		pattern.negated = true
		line = line[1:]
	}

	// Trim trailing spaces unless escaped
	line = trimTrailingSpaces(line)

	// Empty pattern after trimming
	if len(line) == 0 {
		return nil
	}

	// Check if pattern matches directories only (trailing /)
	if strings.HasSuffix(line, "/") {
		pattern.dirOnly = true
		line = strings.TrimSuffix(line, "/")
	}

	// Check if pattern is rooted (starts with /)
	if strings.HasPrefix(line, "/") {
		pattern.rooted = true
		line = strings.TrimPrefix(line, "/")
	}

	// Handle edge case: if pattern becomes empty after trimming "/" (i.e., the original was just "/")
	// This should be treated as a no-op pattern
	if line == "" {
		return nil
	}

	pattern.pattern = line

	return pattern
}

// trimTrailingSpaces removes all unescaped trailing spaces from a gitignore pattern
// while preserving escaped trailing spaces according to Git's specification.
//
// Git treats trailing spaces specially: they are normally trimmed from patterns,
// but can be preserved by escaping with a backslash. The escape handling follows
// these rules:
//   - Odd number of backslashes before a space: space is escaped and kept
//   - Even number of backslashes before a space: space is not escaped and removed
//   - The escape backslash itself is removed when preserving an escaped space
//
// Examples:
//
//	"file   " -> "file"           (unescaped trailing spaces removed)
//	"file\\ " -> "file "          (escaped trailing space kept, escape removed)
//	"file\\\\ " -> "file\\\\"     (unescaped space after escaped backslash)
//	"file\\\\\\ " -> "file\\\\ "  (escaped space after escaped backslash)
//
// This precise behavior matches Git's exact implementation for gitignore pattern processing.
func trimTrailingSpaces(str string) string {
	// Git's behavior: trim trailing spaces unless they are escaped
	// An escaped space is a backslash followed by a space: "\ "
	// But we need to be careful with multiple backslashes
	if str == "" {
		return str
	}

	// Find where to trim by scanning backwards through trailing spaces
	trimEnd := len(str)
	index := len(str) - 1

	// Scan backwards through trailing spaces
	for index >= 0 && str[index] == ' ' {
		// Check if this space is escaped
		if index > 0 && str[index-1] == '\\' {
			// Count consecutive backslashes before this space
			backslashCount := 0

			for j := index - 1; j >= 0 && str[j] == '\\'; j-- {
				backslashCount++
			}
			// If odd number of backslashes, the space is escaped
			if backslashCount%2 == 1 {
				// This space is escaped, include it and the escape backslash
				// The escape backslash will be removed later
				trimEnd = index + 1

				break
			}
		}

		index--
	}

	// If we found trailing spaces (escaped or not)
	if index < len(str)-1 {
		trimEnd = index + 1
	}

	result := str[:trimEnd]

	// Handle escaped trailing spaces by removing the escape backslash
	// Only remove the backslash right before a trailing space
	if len(result) > 1 && result[len(result)-1] == ' ' && result[len(result)-2] == '\\' {
		// Remove the escape backslash before the trailing space
		result = result[:len(result)-2] + " "
	}

	// Return the pattern - all other escape sequences are preserved for doublestar
	return result
}

// IsIgnoredStat behaves like IsIgnored but checks whether the path leads to a directory.
// Will fall back to "not dir" in case of errors.
func (g *GitIgnore) IsIgnoredStat(path string) bool {
	isDir := false

	if info, err := os.Stat(path); err == nil {
		isDir = info.IsDir()
	}

	return g.IsIgnored(path, isDir)
}

// IsIgnored determines whether a path should be ignored according to the gitignore patterns.
// This method implements Git's complex two-pass algorithm with precise parent exclusion rules.
//
// The algorithm works in two passes:
//
// Pass 1 - Parent Exclusion Detection:
//
//	Identifies which parent directories are permanently excluded by patterns.
//	Once a directory is excluded, its contents cannot be re-included by negation patterns,
//	matching Git's strict parent exclusion rule.
//
// Pass 2 - Pattern Matching:
//
//	Applies all patterns to the target path, respecting the parent exclusion rule.
//	Negation patterns can only re-include files if no parent directory is excluded.
//
// Parameters:
//
//	path: The file or directory path to check (should use forward slashes)
//	isDir: True if the path represents a directory, false for files
//
// Returns true if the path should be ignored, false otherwise.
//
// Git Compatibility Notes:
//   - Implements exact Git parent exclusion semantics
//   - Handles complex negation pattern interactions
//   - Supports directory-only patterns (ending with /)
//   - Respects pattern precedence and ordering
//
// Examples:
//
//	gi := New([]string{"build/", "*.log", "!important.log"})
//	gi.IsIgnored("build/file.txt", false)  // true (parent excluded)
//	gi.IsIgnored("app.log", false)         // true
//	gi.IsIgnored("important.log", false)   // false (negated)
func (g *GitIgnore) IsIgnored(path string, isDir bool) bool {
	// Special cases
	if path == "" || path == "." {
		return false
	}

	// No patterns means nothing is ignored
	if len(g.patterns) == 0 {
		return false
	}

	// No path normalization - gitignore should work with paths exactly as provided
	// The caller is responsible for providing paths in the correct format
	// This preserves literal backslashes in filenames on all platforms

	ignored := false

	// Track which directories are permanently excluded
	// Once a directory is excluded, its contents can NEVER be re-included
	excludedDirs := g.findExcludedParentDirectories(path)

	// Check if any parent directory is excluded (implements parent exclusion rule)
	parentExcluded := g.hasExcludedParent(path, excludedDirs)

	// Second pass: apply patterns to the target path
	for _, p := range g.patterns {
		if matches(p, path, isDir) {
			if p.negated {
				// Git rule: cannot re-include file if parent directory is excluded
				if !parentExcluded {
					ignored = false
				}
				// Note: even directories cannot be re-included if parent is excluded
			} else {
				ignored = true
			}
		}
	}

	// Apply parent exclusion rule: if parent is excluded but no pattern matched this path,
	// the path should be ignored due to parent exclusion
	if parentExcluded && !ignored {
		ignored = true
	}

	return ignored
}

// Patterns returns the original pattern strings after parsing.
func (g *GitIgnore) Patterns() []string {
	if len(g.patterns) == 0 {
		return nil
	}

	patterns := make([]string, len(g.patterns))
	for i, p := range g.patterns {
		patterns[i] = p.original
	}

	return patterns
}

// matches determines if a pattern matches a given path, handling both directory-only
// and regular patterns according to Git's matching rules.
func matches(pattern pattern, path string, isDir bool) bool {
	// Special handling for directory-only patterns
	if pattern.dirOnly {
		// Directory patterns can match:
		// 1. The directory itself
		// 2. Files inside the directory
		return matchesDirectoryPattern(pattern, path, isDir)
	}

	// Regular patterns
	return matchesFilePattern(pattern, path, isDir)
}

// matchesDirectoryPattern handles matching for directory-only patterns (ending with /).
// These patterns have special semantics for positive vs negative patterns:
//   - Positive patterns match the directory and files inside it
//   - Negative patterns have complex rules for re-inclusion
func matchesDirectoryPattern(pattern pattern, path string, isDir bool) bool {
	// Directory patterns work differently for positive and negative patterns:
	// - Positive patterns (build/) match the directory AND files inside it
	// - Negative patterns have special cases:
	//   - Wildcard patterns like !*/ or !**/ only match directories
	//   - !build/ un-ignores the directory entry. It does not directly re-include child files;
	//     re-inclusion of files requires additional negations (e.g. !*.txt or !build/file.txt)
	if isDir {
		// Check if this directory matches the pattern directly
		if matchesDirectoryPath(pattern, path) {
			return true
		}

		// For positive patterns, also check if directory is inside a matching directory
		if !pattern.negated {
			if matchesParentDirectory(pattern, path) {
				return true
			}
		}

		return false
	}

	// For files:
	if pattern.negated {
		// CRITICAL: Negated directory patterns (!build/) should NEVER match files
		// They only match the directory itself for the purpose of un-ignoring the directory
		// but NOT for re-including files inside the directory
		return false
	}

	// For positive directory patterns, check if file is inside a matching directory
	return matchesParentDirectory(pattern, path)
}

// matchesParentDirectory checks if a pattern matches any parent directory of a given path.
func matchesParentDirectory(pattern pattern, path string) bool {
	parts := strings.Split(path, "/")
	for i := 1; i < len(parts); i++ {
		parentPath := strings.Join(parts[:i], "/")
		if matchesDirectoryPath(pattern, parentPath) {
			return true
		}
	}

	return false
}

// matchesDirectoryPath checks if a directory path matches a pattern.
// Handles both rooted patterns (anchored to repository root) and
// non-rooted patterns that can match at any directory level.
func matchesDirectoryPath(pattern pattern, dirPath string) bool {
	// In our "single root .gitignore" world, any pattern containing a slash
	// is anchored to the repository root (Git behavior)
	if pattern.rooted || strings.Contains(pattern.pattern, "/") {
		// Match against the full directory path
		return matchGlob(pattern, dirPath)
	}

	// Patterns without slash can match at any directory level
	// Compare against the basename only
	basename := path.Base(dirPath)
	return matchGlob(pattern, basename)
}

// matchesFilePattern handles matching for regular patterns (not directory-only).
// Implements Git's complex rules for rooted vs non-rooted patterns,
// basename matching, and special handling for wildcard patterns.
func matchesFilePattern(pattern pattern, filePath string, isDir bool) bool {
	// Special case: * pattern should only match files/dirs without slashes
	// BUT if it's rooted (/*), it should only match at root level
	if pattern.pattern == "*" {
		if pattern.rooted {
			// /* should only match top-level entries
			return !strings.Contains(filePath, "/")
		}
		// Unrooted * matches files/dirs without slashes at any depth
		basename := path.Base(filePath)

		return basename != "." && basename != "" // Don't match current dir or empty
	}

	if pattern.rooted {
		// Rooted patterns match only from the repository root
		return matchGlob(pattern, filePath)
	}

	// Non-rooted patterns can match at any level

	// Try matching the full path
	if matchGlob(pattern, filePath) {
		return true
	}

	// For patterns without slash, also try matching just the basename
	if !strings.Contains(pattern.pattern, "/") {
		basename := path.Base(filePath)
		if matchGlob(pattern, basename) {
			return true
		}

		// For directories: also check if any parent directory matches the pattern
		// For files: Git only matches the file's basename, not parent directory basenames
		if isDir {
			parts := strings.Split(filePath, "/")
			for i := 1; i < len(parts); i++ {
				parentPath := strings.Join(parts[:i], "/")

				parentBasename := path.Base(parentPath)
				if matchGlob(pattern, parentBasename) {
					return true
				}
			}
		}

		return false
	}

	// For patterns with slash, they should be anchored to root (Git behavior)
	// Only match the full path since non-rooted slash patterns are treated as root-anchored
	return matchGlob(pattern, filePath)
}

// matchGlob performs Git-compatible glob pattern matching using the doublestar library.
// Handles brace escaping to prevent unintended expansion since Git treats braces as
// literal characters rather than expansion syntax.
func matchGlob(p pattern, targetPath string) bool {
	// The pattern has already been processed by trimTrailingSpaces,
	// which handles escape sequences for trailing spaces.
	glob := p.pattern

	// Git does not support brace expansion, but doublestar does by default.
	// We need to escape unescaped braces to prevent expansion.
	glob = escapeBraces(glob)

	// Use doublestar for glob matching - it handles both escaped and unescaped chars
	// Ignore error since malformed patterns should not match (Git behavior)
	matched, _ := doublestar.Match(glob, targetPath)

	return matched
}

// escapeBraces escapes unescaped brace characters to prevent brace expansion.
// Git treats { and } as literal characters rather than expansion syntax,
// so this function ensures compatibility by escaping unescaped braces while
// preserving already-escaped ones and respecting character class boundaries.
//
// The function carefully tracks character class contexts ([...]) where braces
// should not be escaped, and counts preceding backslashes to determine if
// a brace is already escaped.
//
//nolint:gocognit		// Complexity is acceptable.
func escapeBraces(pattern string) string {
	if pattern == "" {
		return pattern
	}

	// Pre-allocate result slice with extra capacity for potential escape characters
	const extraCapacityForEscapes = 10

	result := make([]byte, 0, len(pattern)+extraCapacityForEscapes)
	inCharClass := false

	for index, char := range []byte(pattern) {
		// Track character class boundaries
		switch char {
		case '[':
			if index == 0 || pattern[index-1] != '\\' {
				inCharClass = true
			}
		case ']':
			if (index == 0 || pattern[index-1] != '\\') && inCharClass {
				inCharClass = false
			}
		case '{', '}':
			// Only escape braces outside of character classes
			if !inCharClass {
				// Check if this brace is already escaped by counting preceding backslashes
				backslashCount := 0

				for j := index - 1; j >= 0 && pattern[j] == '\\'; j-- {
					backslashCount++
				}
				// If even number of backslashes (including 0), the brace is not escaped
				if backslashCount%2 == 0 {
					// Add escape before the brace
					result = append(result, '\\')
				}
			}
		}

		result = append(result, char)
	}

	return string(result)
}

// findExcludedParentDirectories identifies which parent directories are permanently excluded.
// Returns a map of excluded directory paths for fast lookup.
func (g *GitIgnore) findExcludedParentDirectories(targetPath string) map[string]bool {
	excludedDirs := make(map[string]bool)

	// Build list of all parent paths to check
	parts := strings.Split(targetPath, "/")

	pathsToCheck := make([]string, 0, len(parts))
	for i := 1; i <= len(parts); i++ {
		pathsToCheck = append(pathsToCheck, strings.Join(parts[:i], "/"))
	}

	// First pass: determine which directories are excluded
	// Check ALL patterns (not just dirOnly) as they can exclude directories
	for _, pattern := range g.patterns {
		for _, checkPath := range pathsToCheck {
			// Only check parent directories, not the target path itself
			if checkPath == targetPath {
				continue
			}

			// Check if this pattern EXPLICITLY excludes the directory itself
			if dirMatches := g.patternExcludesDirectory(pattern, checkPath); dirMatches {
				if pattern.negated {
					// Issue #2 fix: Allow explicit directory negation to clear exclusion
					delete(excludedDirs, checkPath)
				} else {
					// Directory is excluded - mark it permanently
					excludedDirs[checkPath] = true
				}
			}
		}
	}

	return excludedDirs
}

// patternExcludesDirectory determines if a pattern explicitly excludes a directory.
// The parent exclusion rule only applies when a directory is explicitly excluded,
// not when patterns like "foo/*" match content inside the directory.
func (g *GitIgnore) patternExcludesDirectory(pat pattern, dirPath string) bool {
	if pat.dirOnly {
		// Directory-only patterns (ending with /) explicitly exclude directories
		return matchesDirectoryPath(pat, dirPath)
	}

	// For parent exclusion, only certain patterns actually exclude directories:
	// - Patterns that match the directory name directly (like "build")
	// - Rooted wildcard patterns like "/*" that match top-level directories
	// - NOT patterns like "foo/*" which match content, not the directory

	// Skip patterns that end with "/*" - these match content, not the directory
	if strings.HasSuffix(pat.pattern, "/*") {
		return false
	}

	if pat.pattern == "*" {
		if pat.rooted {
			// Issue #3 fix: "/*" pattern SHOULD cause parent exclusion
			// for top-level directories it matches
			return !strings.Contains(dirPath, "/")
		}
		// "*" pattern can exclude directories by matching their basename
		basename := path.Base(dirPath)

		return matchGlob(pat, basename)
	}

	if strings.Contains(pat.pattern, "**") {
		// "**" patterns can match directories at any level
		return matchGlob(pat, dirPath)
	}

	if strings.Contains(pat.pattern, "/") {
		// Pattern with slash - only matches if it specifically matches directory path
		// But NOT if it's a content pattern like "foo/*"
		return matchGlob(pat, dirPath)
	}

	// Pattern without slash - check if it matches directory basename
	// e.g., "build" pattern excludes directory named "build"
	basename := path.Base(dirPath)

	return matchGlob(pat, basename)
}

// hasExcludedParent checks if any parent directory is excluded.
// This applies to both files AND directories.
func (g *GitIgnore) hasExcludedParent(targetPath string, excludedDirs map[string]bool) bool {
	parts := strings.Split(targetPath, "/")
	if len(parts) <= 1 {
		return false // No parent directories
	}

	for pathIdx := 1; pathIdx < len(parts); pathIdx++ {
		parentPath := strings.Join(parts[:pathIdx], "/")
		if excludedDirs[parentPath] {
			return true
		}
	}

	return false
}
