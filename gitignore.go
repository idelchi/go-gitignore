// Package gitignore implements Git-compatible gitignore pattern matching with the aim to reach parity to Git's
// behavior.
// This package provides gitignore logic including pattern parsing with escape sequences,
// two-pass ignore checking with parent exclusion detection, complex negation rules that match Git's exact behavior,
// brace escaping to prevent expansion, and cross-platform path handling using forward slashes only.
//
// The core algorithm implements Git's parent exclusion rule: once a directory is excluded by a pattern,
// its contents cannot be re-included by negation patterns, maintaining strict compatibility with Git's behavior.
// The package uses a two-pass algorithm - first identifying excluded parent directories, then applying
// pattern matching to the target path.
//
// Key features:
//   - Git compatibility for a large amount of gitignore edge cases
//   - Two-pass algorithm for parent exclusion detection
//   - Negation semantics matching Git's behavior
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

// Constants for pattern matching.
const (
	doubleStarSlash = "/**"
	doubleSlash     = "//"
	tripleSlash     = "///"
	dotSlash        = "./"
	wildcard        = "*"
	doubleStar      = "**"
)

// normalizeCandidatePath collapses runs of '/' and cleans dot segments like Git does.
func normalizeCandidatePath(p string) string {
	if p == "" || p == "." {
		return p
	}

	// Special case: preserve leading double slash (POSIX behavior)
	preserveDoubleSlash := strings.HasPrefix(p, doubleSlash) && !strings.HasPrefix(p, tripleSlash)
	if preserveDoubleSlash {
		p = doubleSlash + p[2:]
	}

	// Collapse all runs of '/'
	for strings.Contains(p, doubleSlash) {
		p = strings.ReplaceAll(p, doubleSlash, "/")
	}

	// Restore leading double slash if needed
	if preserveDoubleSlash && !strings.HasPrefix(p, doubleSlash) {
		p = "/" + p
	}

	// Clean dot segments
	return path.Clean(p)
}

// normalizeMetaEscapes ensures that * and ? remain meta even if preceded by odd number of backslashes.
func normalizeMetaEscapes(glob string) string {
	if glob == "" {
		return glob
	}

	var b strings.Builder
	b.Grow(len(glob) + 8)

	inClass := false

	for i := 0; i < len(glob); i++ {
		ch := glob[i]

		// Track character class boundaries
		if ch == '[' && !inClass {
			inClass = true
		} else if ch == ']' && inClass {
			inClass = false
		} else if ch == '\\' && i+1 < len(glob) {
			// Count consecutive backslashes
			runStart := i
			for i < len(glob) && glob[i] == '\\' {
				i++
			}

			runLen := i - runStart

			// Check if next character is a meta character
			if i < len(glob) && !inClass && (glob[i] == '*' || glob[i] == '?') {
				// Write original backslashes
				for range runLen {
					b.WriteByte('\\')
				}
				// Add extra backslash if odd number (to keep meta unescaped)
				if runLen%2 == 1 {
					b.WriteByte('\\')
				}
			} else {
				// Write backslashes as-is
				for range runLen {
					b.WriteByte('\\')
				}
			}

			i-- // Back up since outer loop will increment

			continue
		}

		b.WriteByte(ch)
	}

	return b.String()
}

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

// Ignored determines whether a path should be ignored according to the gitignore patterns.
// This method implements a two-pass algorithm with parent exclusion rules.
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
// Returns true if the path should be ignored, false otherwise.
//
// Git Compatibility Notes:
//   - Implements Git parent exclusion semantics
//   - Handles negation pattern interactions
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

	// Git quirk: paths starting with "//" are never ignored
	// Leading double slash has special meaning in POSIX and Git doesn't match them
	if strings.HasPrefix(p, doubleSlash) && !strings.HasPrefix(p, tripleSlash) {
		return false
	}

	// Normalize leading "./" only (Git-compatible for check-ignore usage).
	// Special case: "./" for directory should become "."
	if p == dotSlash && isDir {
		p = "."
	} else {
		for strings.HasPrefix(p, dotSlash) {
			p = strings.TrimPrefix(p, dotSlash)
			if p == "" {
				return false
			}
		}
	}

	// NEW: collapse redundant slashes & clean dot segments
	p = normalizeCandidatePath(p)

	// No patterns means nothing is ignored
	if len(g.patterns) == 0 {
		return false
	}

	ignored := false

	// Track which directories are excluded after considering all patterns and negations
	// Files under excluded directories can only be re-included if all parent directories
	// on the path have been re-included
	excludedDirs := g.findExcludedParentDirectories(p)

	// Check if any parent directory is excluded (implements parent exclusion rule)
	parentExcluded := hasExcludedParent(p, excludedDirs)

	// Second pass: apply patterns to the target path
	for _, pat := range g.patterns {
		if matches(pat, p, isDir) {
			if pat.negated {
				// Git rule: cannot re-include file if parent directory is excluded
				if !parentExcluded {
					// GITIGNORE QUIRK: negation patterns for "." don't work in Git
					// (verified against actual Git behavior)
					if pat.pattern == "." && p == "." {
						// Keep ignored = true (don't set to false)
					} else {
						ignored = false
					}
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

// findExcludedParentDirectories identifies which parent directories are permanently excluded.
// Returns a map of excluded directory paths for fast lookup.
func (g *GitIgnore) findExcludedParentDirectories(targetPath string) map[string]bool {
	excludedDirs := make(map[string]bool)

	// Build list of all parent paths to check
	parts := strings.Split(targetPath, "/")

	pathsToCheck := make([]string, 0, len(parts))
	for i := 1; i <= len(parts); i++ {
		checkPath := strings.Join(parts[:i], "/")

		checkPath = normalizeCandidatePath(checkPath)
		pathsToCheck = append(pathsToCheck, checkPath)
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
			// excludes that directory. This includes patterns like "foo/*" and "**/"
			// which may match directories at various depths.
			if dirMatches := patternExcludesDirectory(pat, checkPath); dirMatches {
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

// isGitIgnoreQuirk detects patterns with special Git behaviors that need handling
func isGitIgnoreQuirk(pat pattern, path string, isDir bool) (bool, bool) {
	// GITIGNORE QUIRK: Patterns ending with /** are "contents-only"
	// They match everything under the base but NOT the base itself
	// Reference: https://git-scm.com/docs/gitignore#_pattern_format
	if hasContentsOnlyQuirk(pat.pattern, pat.dirOnly) {
		if isBaseOfPattern(pat, path, isDir) {
			return true, false // Don't match the base
		}
	}
	return false, false
}

func hasContentsOnlyQuirk(pattern string, dirOnly bool) bool {
	return strings.HasSuffix(pattern, "/**")
}

func isBaseOfPattern(pat pattern, path string, isDir bool) bool {
	base := extractBase(pat.pattern)
	if base == "" || base == "**" || strings.HasSuffix(base, "**") {
		return false
	}

	basePattern := pattern{
		pattern: base,
		rooted:  pat.rooted,
		negated: false,
		dirOnly: false,
	}
	
	// For directory-only patterns, only check directories
	if pat.dirOnly && !isDir {
		return false
	}
	
	return matchesSimple(basePattern, path, isDir)
}

func extractBase(pattern string) string {
	// Strip all trailing /** groups to find the base
	base := pattern
	for strings.HasSuffix(base, "/**") {
		base = strings.TrimSuffix(base, "/**")
	}
	return base
}

// matches determines if a pattern matches a given path using a unified approach
func matches(pat pattern, p string, isDir bool) bool {
	// Directory-only patterns match directories ONLY (not files)
	if pat.dirOnly && !isDir {
		return false
	}
	
	// Check for Git quirks first
	if hasQuirk, quirkResult := isGitIgnoreQuirk(pat, p, isDir); hasQuirk {
		return quirkResult
	}
	
	// Use the simple, unified matching
	return matchesSimple(pat, p, isDir)
}

// matchesSimple implements a unified, simple approach to pattern matching
func matchesSimple(pat pattern, p string, isDir bool) bool {
	glob := pat.pattern
	
	// Determine the target path to match against
	matchPath := p
	if !pat.rooted && !strings.Contains(glob, "/") {
		// Non-rooted patterns without slash match only the basename
		matchPath = path.Base(p)
	}
	
	// Let the glob library handle the matching
	return matchGlob(pat, matchPath)
}

// matchGlob performs Git-compatible glob pattern matching using the doublestar library.
// Handles brace escaping to prevent unintended expansion since Git treats braces as
// literal characters rather than expansion syntax.
func matchGlob(p pattern, targetPath string) bool {
	// The pattern has already been processed by trimTrailingSpaces,
	// which handles escape sequences for trailing spaces.
	glob := p.pattern

	// Check if this pattern has no wildcards (literal matching)
	if !hasUnescapedWildcards(glob) {
		// Process escapes for literal matching - remove all escape backslashes
		literal := processEscapes(glob, true)

		return literal == targetPath
	}

	// Process escape sequences before glob matching
	originalGlob := glob

	glob = processEscapes(glob, false)

	// Git does not support brace expansion, but doublestar does by default.
	// We need to escape unescaped braces to prevent expansion.
	glob = escapeBraces(glob)

	// Normalize first-literal ']' inside character classes to avoid engine differences.
	glob = escapeFirstClosingBracketInCharClass(glob)

	// Only apply normalizeMetaEscapes if we haven't explicitly escaped wildcards
	// Check if the original pattern had escaped wildcards that we want to keep literal
	hasEscapedWildcards := strings.Contains(originalGlob, "\\*") || strings.Contains(originalGlob, "\\?") ||
		strings.Contains(originalGlob, "\\[")
	if !hasEscapedWildcards {
		glob = normalizeMetaEscapes(glob)
	}

	matched, _ := doublestar.Match(glob, targetPath)

	return matched
}

// matchRawGlob is a convenience wrapper for matchGlob with a raw pattern string.
func matchRawGlob(glob, targetPath string) bool {
	return matchGlob(pattern{pattern: glob}, targetPath)
}

// stripTrailingSuffix removes trailing "/**" segments based on mode.
func stripTrailingSuffix(glob string, allowDoubleSlash bool) string {
	for strings.HasSuffix(glob, doubleStarSlash) {
		if !allowDoubleSlash || !strings.HasSuffix(glob, "/**/") {
			glob = strings.TrimSuffix(glob, doubleStarSlash)
		} else {
			break
		}
	}

	return glob
}

// endsWithDoubleStarSegment reports whether glob's last path segment is exactly "**".
func endsWithDoubleStarSegment(glob string) bool {
	if glob == doubleStar {
		return true
	}

	return strings.HasSuffix(glob, doubleStarSlash) && strings.TrimSuffix(glob, doubleStarSlash) != ""
}

// escapeBraces escapes unescaped brace characters to prevent brace expansion.
func escapeBraces(p string) string {
	if p == "" || (!strings.Contains(p, "{") && !strings.Contains(p, "}")) {
		return p
	}

	var result strings.Builder
	result.Grow(len(p) + 10)

	inCharClass := false

	for i := range len(p) {
		c := p[i]

		// Track character class boundaries
		if c == '[' && (i == 0 || p[i-1] != '\\') {
			inCharClass = true
		} else if c == ']' && inCharClass && (i == 0 || p[i-1] != '\\') {
			inCharClass = false
		} else if (c == '{' || c == '}') && !inCharClass {
			// Count preceding backslashes
			backslashes := 0

			for j := i - 1; j >= 0 && p[j] == '\\'; j-- {
				backslashes++
			}
			// Escape if not already escaped (even number of backslashes)
			if backslashes%2 == 0 {
				result.WriteByte('\\')
			}
		}

		result.WriteByte(c)
	}

	return result.String()
}

// escapeFirstClosingBracketInCharClass ensures that a ']' used as the first character
// inside a character class is escaped for consistent glob matching behavior.
func escapeFirstClosingBracketInCharClass(p string) string {
	if p == "" || !strings.Contains(p, "[") {
		return p
	}

	var b strings.Builder
	b.Grow(len(p) + 8)

	i := 0
	for i < len(p) {
		if p[i] != '[' || (i > 0 && p[i-1] == '\\') {
			b.WriteByte(p[i])

			i++

			continue
		}

		// Found unescaped '[', start of character class
		b.WriteByte('[')

		i++

		// Skip negation characters
		if i < len(p) && (p[i] == '!' || p[i] == '^') {
			b.WriteByte(p[i])

			i++
		}

		// Check if first listed character is ']'
		if i < len(p) && p[i] == ']' {
			b.WriteByte('\\')
			b.WriteByte(']')

			i++
		}

		// Copy until we find the closing ']'
		for i < len(p) {
			if p[i] == ']' && (i == 0 || p[i-1] != '\\') {
				b.WriteByte(']')

				i++

				break
			}

			b.WriteByte(p[i])

			i++
		}
	}

	return b.String()
}

// hasExcludedParent checks if any parent directory is excluded.
func hasExcludedParent(targetPath string, excludedDirs map[string]bool) bool {
	parts := strings.Split(targetPath, "/")
	if len(parts) <= 1 {
		return false
	}

	for pathIdx := 1; pathIdx < len(parts); pathIdx++ {
		parentPath := strings.Join(parts[:pathIdx], "/")
		if excludedDirs[parentPath] {
			return true
		}
	}

	return false
}

// hasUnescapedWildcards checks if pattern has unescaped wildcards.
func hasUnescapedWildcards(pattern string) bool {
	for i := 0; i < len(pattern); i++ {
		if pattern[i] == '\\' && i+1 < len(pattern) {
			// Skip escaped character
			i++

			continue
		}
		// Check for unescaped wildcards
		if pattern[i] == '*' || pattern[i] == '?' || pattern[i] == '[' {
			return true
		}
	}

	return false
}

// processEscapes handles escape sequences in patterns with different modes.
func processEscapes(pattern string, forLiteral bool) string {
	if pattern == "" {
		return pattern
	}

	var result strings.Builder
	result.Grow(len(pattern) + 10)

	inCharClass := false

	for i := 0; i < len(pattern); i++ {
		char := pattern[i]

		// Track character class boundaries
		if char == '[' && (i == 0 || pattern[i-1] != '\\') {
			inCharClass = true

			result.WriteByte(char)
		} else if char == ']' && inCharClass && (i == 0 || pattern[i-1] != '\\') {
			inCharClass = false

			result.WriteByte(char)
		} else if char == '\\' && i+1 < len(pattern) {
			next := pattern[i+1]

			// Inside character classes, preserve backslashes as-is for doublestar
			if inCharClass {
				result.WriteByte('\\')
				result.WriteByte(next)

				i++ // Skip the next character
			} else {
				// Outside character classes, use existing logic
				switch next {
				case '*', '?', '[', ']':
					if forLiteral {
						// For literal matching, remove escape backslash
						result.WriteByte(next)
					} else {
						// For glob matching, keep escaped for doublestar
						result.WriteByte('\\')
						result.WriteByte(next)
					}

					i++ // Skip the next character
				case '#', '{', '}', '!':
					// Always remove backslash for these special chars
					result.WriteByte(next)

					i++ // Skip the next character
				case '\\':
					if !forLiteral && i+2 < len(pattern) && (pattern[i+2] == '*' || pattern[i+2] == '?' || pattern[i+2] == '[') {
						// For glob matching with \\* or \\? or \\[ - preserve for doublestar
						result.WriteByte('\\')
						result.WriteByte('\\')

						i++ // Skip the second backslash
					} else {
						// Regular double backslash becomes single
						result.WriteByte('\\')

						i++
					}
				default:
					// Keep backslash for other cases
					result.WriteByte(char)
				}
			}
		} else {
			result.WriteByte(char)
		}
	}

	return result.String()
}

// trimTrailingSpaces removes unescaped trailing spaces and processes escape sequences.
func trimTrailingSpaces(str string) string {
	if str == "" {
		return str
	}

	// Trim unescaped trailing spaces
	for len(str) > 0 && str[len(str)-1] == ' ' {
		backslashes := 0

		for i := len(str) - 2; i >= 0 && str[i] == '\\'; i-- {
			backslashes++
		}

		if backslashes%2 == 1 {
			break // Space is escaped
		}

		str = str[:len(str)-1]
	}

	// Process \<space> escapes
	var result strings.Builder
	result.Grow(len(str))

	for i := 0; i < len(str); i++ {
		if i < len(str)-1 && str[i] == '\\' && str[i+1] == ' ' {
			result.WriteByte(' ')

			i++
		} else {
			result.WriteByte(str[i])
		}
	}

	return result.String()
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


// patternExcludesDirectory determines if a pattern explicitly excludes a directory.
// Uses the same unified matching approach as regular pattern matching.
func patternExcludesDirectory(pat pattern, dirPath string) bool {
	// Simply use the same matching logic, treating the directory as a directory
	return matches(pat, dirPath, true)
}
