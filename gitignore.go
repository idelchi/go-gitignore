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
//	ignored := gi.Ignored("app.log", false) // true
//	ignored = gi.Ignored("important.log", false) // false
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
//	ignored := gi.Ignored("app.log", false) // true
//	ignored = gi.Ignored("important.log", false) // false (negated)
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

	pat := &pattern{
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
		pat.negated = true
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
		pat.dirOnly = true
		line = strings.TrimSuffix(line, "/")
	}

	// Check if pattern is rooted (starts with /)
	if strings.HasPrefix(line, "/") {
		pat.rooted = true
		line = strings.TrimPrefix(line, "/")
	}

	// Handle edge case: if pattern becomes empty after trimming "/" (i.e., the original was just "/")
	// This should be treated as a no-op pattern
	if line == "" {
		return nil
	}

	pat.pattern = line

	return pat
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

// Ignored determines whether a path should be ignored according to the gitignore patterns.
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
//	gi.Ignored("build/file.txt", false)  // true (parent excluded)
//	gi.Ignored("app.log", false)         // true
//	gi.Ignored("important.log", false)   // false (negated)
func (g *GitIgnore) Ignored(p string, isDir bool) bool {
	// Special cases
	if p == "" {
		return false
	}

	// Normalize leading "./" only (Git-compatible for check-ignore usage).
	for strings.HasPrefix(p, "./") {
		p = strings.TrimPrefix(p, "./")
		if p == "" {
			return false
		}
	}

	// No patterns means nothing is ignored
	if len(g.patterns) == 0 {
		return false
	}

	ignored := false

	// Track which directories are permanently excluded
	// Once a directory is excluded, its contents can NEVER be re-included
	excludedDirs := g.findExcludedParentDirectories(p)

	// Check if any parent directory is excluded (implements parent exclusion rule)
	parentExcluded := g.hasExcludedParent(p, excludedDirs)

	// Second pass: apply patterns to the target path
	for _, pat := range g.patterns {
		if matches(pat, p, isDir) {
			if pat.negated {
				// Git rule: cannot re-include file if parent directory is excluded
				if !parentExcluded {
					ignored = false
				}
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

// IgnoredStat behaves like Ignored but checks whether the path leads to a directory.
// Will fall back to "not dir" in case of errors.
func (g *GitIgnore) IgnoredStat(p string) bool {
	isDir := false

	if info, err := os.Stat(p); err == nil {
		isDir = info.IsDir()
	}

	return g.Ignored(p, isDir)
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
func matches(pat pattern, p string, isDir bool) bool {
	// Directory-only patterns match directories ONLY (not files).
	if pat.dirOnly {
		if isDir {
			return matchesDirectoryPath(pat, p)
		}
		return false
	}

	// Regular patterns
	return matchesFilePattern(pat, p, isDir)
}

// matchesDirectoryPath checks if a directory path matches a pattern.
// Handles both rooted patterns (anchored to repository root) and
// non-rooted patterns that can match at any directory level.
func matchesDirectoryPath(pat pattern, dirPath string) bool {
	// Special case: dir-only pattern that ends with "**/" (original form).
	// It should match directories strictly under the base, not the base itself.
	// Example: "abc/**/" matches "abc/x" and "abc/x/y", but NOT "abc".
	if pat.dirOnly && strings.HasSuffix(strings.TrimSuffix(pat.original, " "), "**/") {
		// The processed pattern will be ".../**" (we trimmed the final "/").
		baseGlob := strings.TrimSuffix(pat.pattern, "/**")

		// If baseGlob is empty, "**/" -> matches any directory.
		if baseGlob == "" {
			return true
		}

		// If the candidate is exactly the base, do not match.
		if m, _ := doublestar.Match(baseGlob, dirPath); m {
			return false
		}
		// Otherwise match any directory below the base.
		m, _ := doublestar.Match(baseGlob+"/**", dirPath)
		return m
	}

	// In our "single root .gitignore" world, any pattern containing a slash
	// is anchored to the repository root (Git behavior).
	if pat.rooted || strings.Contains(pat.pattern, "/") {
		return matchGlob(pat, dirPath)
	}

	// Patterns without slash can match at any directory level (basename).
	basename := path.Base(dirPath)
	return matchGlob(pat, basename)
}

// matchesFilePattern handles matching for regular patterns (not directory-only).
// Implements Git's complex rules for rooted vs non-rooted patterns,
// basename matching, and special handling for wildcard patterns.
func matchesFilePattern(pat pattern, filePath string, isDir bool) bool {
	// Special handling for patterns that end with "/**" (contents-only form).
	// These should NOT match the base directory entry itself. They match
	// files and directories strictly *under* the base.
	if strings.HasSuffix(pat.pattern, "/**") {
		baseGlob := strings.TrimSuffix(pat.pattern, "/**")

		if isDir {
			// If the directory equals the base, do not match.
			if m, _ := doublestar.Match(baseGlob, filePath); m {
				return false
			}
			// Directory strictly below the base is matched.
			if m, _ := doublestar.Match(baseGlob+"/**", filePath); m {
				return true
			}
			return false
		}

		// Files below the base are matched by the original pattern.
		if m, _ := doublestar.Match(pat.pattern, filePath); m {
			return true
		}
		// Also allow basename matching when the pattern has no slash.
		if !strings.Contains(pat.pattern, "/") {
			basename := path.Base(filePath)
			if m, _ := doublestar.Match(pat.pattern, basename); m {
				return true
			}
		}
		return false
	}

	// Special case: * pattern should match a single path component (including dotfiles).
	// If it's rooted (/*), it should only match at root level.
	if pat.pattern == "*" {
		if pat.rooted {
			// /* should only match top-level entries
			return !strings.Contains(filePath, "/")
		}
		// Unrooted * matches a single component at any depth (including ".")
		basename := path.Base(filePath)
		return basename != ""
	}

	if pat.rooted {
		// Rooted patterns match only from the repository root
		return matchGlob(pat, filePath)
	}

	// Non-rooted patterns can match at any level.

	// Try matching the full path
	if matchGlob(pat, filePath) {
		return true
	}

	// For patterns without slash, also try matching just the basename
	if !strings.Contains(pat.pattern, "/") {
		basename := path.Base(filePath)
		if matchGlob(pat, basename) {
			return true
		}

		// For directories: also check if any parent directory matches the pattern
		// For files: Git only matches the file's basename, not parent directory basenames
		if isDir {
			parts := strings.Split(filePath, "/")
			for i := 1; i < len(parts); i++ {
				parentPath := strings.Join(parts[:i], "/")
				parentBasename := path.Base(parentPath)
				if matchGlob(pat, parentBasename) {
					return true
				}
			}
		}

		return false
	}

	// For patterns with slash, they should be anchored to root (Git behavior)
	// Only match the full path since non-rooted slash patterns are treated as root-anchored
	return matchGlob(pat, filePath)
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

	// Normalize first-literal ']' inside character classes to avoid engine differences.
	glob = escapeFirstClosingBracketInCharClass(glob)

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
//nolint:gocognit // Complexity is acceptable.
func escapeBraces(p string) string {
	if p == "" {
		return p
	}

	const extra = 10
	result := make([]byte, 0, len(p)+extra)

	inCharClass := false
	charClassAtStart := false // true until we see the first "listed" char; '!' or '^' don't count

	for i := 0; i < len(p); i++ {
		c := p[i]

		switch c {
		case '[':
			// Only start a char class if this '[' is not escaped
			if i == 0 || p[i-1] != '\\' {
				inCharClass = true
				charClassAtStart = true
			}
		case ']':
			if (i == 0 || p[i-1] != '\\') && inCharClass {
				// If ']' appears as the first listed character (right after '[' or after a leading '!'/'^'),
				// it is a literal and should NOT end the class.
				if charClassAtStart {
					// remain in the class
				} else {
					inCharClass = false
				}
			}
		case '{', '}':
			// Only escape braces outside of character classes
			if !inCharClass {
				// Check if this brace is already escaped by counting preceding backslashes
				backslashCount := 0
				for j := i - 1; j >= 0 && p[j] == '\\'; j-- {
					backslashCount++
				}
				// If even number of backslashes (including 0), the brace is not escaped
				if backslashCount%2 == 0 {
					result = append(result, '\\')
				}
			}
		}

		// Once we're inside a class, after we append the first "listed" char,
		// we are no longer at the start (a leading '!' or '^' do not count).
		if inCharClass && charClassAtStart {
			switch c {
			case '!', '^':
				// still at start
			case '[':
				// already handled
			default:
				charClassAtStart = false
			}
		}

		result = append(result, c)
	}

	return string(result)
}

// escapeFirstClosingBracketInCharClass ensures that a ']' used as the first *listed* character
// inside a character class is escaped (e.g., "[]]" -> "[\\]]", "[!]]" -> "[!\\]]", "[^]]" -> "[^\\]]").
// This is only a normalization to make downstream globbing behavior consistent with Git across engines.
func escapeFirstClosingBracketInCharClass(p string) string {
	if p == "" {
		return p
	}

	var b strings.Builder
	b.Grow(len(p) + 8)

	inClass := false
	atStart := false // start of "listed" chars; ignores a leading '!' or '^'
	escaped := false

	for i := 0; i < len(p); i++ {
		c := p[i]

		if !inClass {
			if c == '[' && !escaped {
				inClass = true
				atStart = true
				escaped = false
				b.WriteByte(c)
				continue
			}
			if c == '\\' && !escaped {
				escaped = true
			} else {
				escaped = false
			}
			b.WriteByte(c)
			continue
		}

		// inClass == true
		if atStart {
			// Skip initial negator from counting
			if (c == '!' || c == '^') && !escaped {
				b.WriteByte(c)
				escaped = false
				continue
			}
			// First *listed* character
			if c == ']' && !escaped {
				// make it literal
				b.WriteByte('\\')
				b.WriteByte(']')
			} else {
				b.WriteByte(c)
			}
			atStart = false
			escaped = (c == '\\') && !escaped
			continue
		}

		// Normal inside-class handling
		if c == '\\' && !escaped {
			escaped = true
			b.WriteByte(c)
			continue
		}
		escaped = false

		if c == ']' {
			inClass = false
			b.WriteByte(c)
			continue
		}

		b.WriteByte(c)
	}

	return b.String()
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
	for _, pat := range g.patterns {
		for _, checkPath := range pathsToCheck {
			// Only check parent directories, not the target path itself
			if checkPath == targetPath {
				continue
			}

			// Check if this pattern EXPLICITLY excludes the directory itself.
			// Any pattern that matches a directory entry (with or without a trailing '/')
			// excludes that directory. This includes patterns like "foo/*" which match
			// "foo/bar" when "bar" is a directory, thereby excluding "foo/bar" and
			// everything beneath it.
			if dirMatches := g.patternExcludesDirectory(pat, checkPath); dirMatches {
				if pat.negated {
					// Allow explicit directory negation to clear exclusion
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
// The parent exclusion rule applies when a pattern matches the directory entry itself.
// Patterns like "foo/*" DO exclude "foo/bar" (a direct child directory of "foo").
// Patterns ending with "/**" match only the contents under a base and must NOT match the base entry.
func (g *GitIgnore) patternExcludesDirectory(pat pattern, dirPath string) bool {
	if pat.dirOnly {
		// Directory-only patterns (ending with /) explicitly exclude directories.
		// Special handling for "**/" form (match directories strictly below base).
		if strings.HasSuffix(strings.TrimSuffix(pat.original, " "), "**/") {
			baseGlob := strings.TrimSuffix(pat.pattern, "/**")
			if baseGlob == "" {
				// "**/" — any directory at any depth is excluded
				return true
			}
			// Do not exclude the base itself; exclude directories below it.
			if m, _ := doublestar.Match(baseGlob, dirPath); m {
				return false
			}
			m, _ := doublestar.Match(baseGlob+"/**", dirPath)
			return m
		}
		return matchesDirectoryPath(pat, dirPath)
	}

	// Non-dirOnly pattern with trailing "/**" is contents-only:
	// it must NOT exclude the base directory entry itself, but it DOES
	// exclude directories strictly below the base.
	if strings.HasSuffix(pat.pattern, "/**") {
		baseGlob := strings.TrimSuffix(pat.pattern, "/**")
		// Base entry must not be excluded
		if m, _ := doublestar.Match(baseGlob, dirPath); m {
			return false
		}
		// A directory strictly below the base should be excluded.
		m, _ := doublestar.Match(baseGlob+"/**", dirPath)
		return m
	}

	// Patterns that match a directory entry (basename or full path) exclude that directory.

	if pat.pattern == "*" {
		if pat.rooted {
			// "/*" excludes top-level directories it matches
			return !strings.Contains(dirPath, "/")
		}
		// "*" can exclude directories by matching their basename
		basename := path.Base(dirPath)
		return matchGlob(pat, basename)
	}

	if strings.Contains(pat.pattern, "**") {
		// "**" patterns can match directories at any level
		return matchGlob(pat, dirPath)
	}

	if strings.Contains(pat.pattern, "/") {
		// Pattern with slash - matches against the directory path
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
