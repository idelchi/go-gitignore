// Package gitignore implements Git-compatible gitignore pattern matching with the aim to reach parity with Git's
// ignore behavior. This package provides gitignore semantics including pattern parsing with escape sequences,
// two-pass ignore checking with parent exclusion detection, negation rules that attempt to match Git's behavior,
// brace escaping to prevent expansion, and cross-platform path handling using forward slashes only.
//
// The implementation uses a two-pass algorithm that attempts to mirror Git's internal gitignore processing:
//
//	Pass 1 - Parent Exclusion Detection:
//	  Identifies which parent directories are permanently excluded by patterns.
//
//	Pass 2 - Pattern Matching:
//	  Applies all patterns to the target path, respecting the parent exclusion rule.
//	  Negation patterns can only re-include files if no parent directory is excluded.
//
// Usage:
//
//	gi := gitignore.New("*.log", "build/", "!important.log")
//	ignored := gi.Ignored("app.log", false)     // true (matches *.log)
//	ignored = gi.Ignored("important.log", false) // false (negated by !important.log)
//	ignored = gi.Ignored("build/file.txt", false) // true (parent directory build/ is excluded)
package gitignore

import (
	"path"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// Pattern matching constants used throughout gitignore processing.
// These represent common pattern elements and path prefixes that require special handling.
const (
	// Double star pattern (matches any file or directory recursively).
	doubleStar = "**"
	// Contents-only pattern suffix (matches everything under a directory but not the directory itself).
	doubleStarSlash = "/**"
	// Double slash prefix (has special POSIX meaning, never ignored by Git).
	doubleSlash = "//"
	// Triple slash prefix (treated as regular path by Git).
	tripleSlash = "///"
	// Current directory prefix (normalized away during processing).
	dotSlash = "./"
)

// Buffer growth constants for string building operations.
const (
	// Small buffer growth for short patterns.
	smallBufferGrowth = 8
	// Medium buffer growth for typical patterns.
	mediumBufferGrowth = 10
	// Offset for backslash counting operations.
	backslashOffset = 2
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
//	gi := New("*.log", "build/", "!important.log", "# comment")
//	ignored := gi.Ignored("app.log", false) // true
//	ignored = gi.Ignored("important.log", false) // false (negated)
func New(lines ...string) *GitIgnore {
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
// This method implements Git's exact two-pass algorithm with parent exclusion rules.
//
// Algorithm Overview:
//
// The implementation mirrors Git's internal gitignore processing with two distinct passes:
//
// Pass 1 - Parent Exclusion Detection:
//   - Builds a list of all parent directories in the target path
//   - Applies all patterns to each parent directory to determine which are excluded
//   - Creates a map of permanently excluded directories
//
// Pass 2 - Pattern Matching:
//   - Applies all patterns to the target path in the order they appear
//   - For negation patterns (!pattern), checks if parent exclusion rule prevents re-inclusion
//   - Implements Git's specific quirks (like dot negation: "!." cannot un-ignore repository root)
//   - Final decision respects both pattern matches and parent exclusion status
//
// Git Compatibility Features:
//   - Exact implementation of Git's parent exclusion semantics
//   - Handles all documented Git quirks and edge cases
//   - Supports directory-only patterns (trailing /) that match directories but not files
//   - Processes patterns in order with proper negation precedence
//   - Normalizes paths using Git's path cleaning rules
//   - Handles special path prefixes ("//" paths are never ignored)
//
// Parameters:
//   - p: The path to check (normalized to use forward slashes)
//   - isDir: Whether the path represents a directory (affects directory-only pattern matching)
//
// Returns:
//   - true if the path should be ignored according to Git's gitignore rules
//   - false if the path should be included
//
// Examples:
//
//	gi := New("build/", "*.log", "!important.log")
//	gi.Ignored("build/file.txt", false)    // true (parent directory build/ is excluded)
//	gi.Ignored("app.log", false)           // true (matches *.log pattern)
//	gi.Ignored("important.log", false)     // false (re-included by !important.log)
//	gi.Ignored("src/important.log", false) // depends on whether src/ is excluded
//
//nolint:gocognit	// Function is complex by design.
func (g *GitIgnore) Ignored(inputPath string, isDir bool) bool {
	// === PREPROCESSING PHASE ===
	// Handle edge cases and normalize the input path according to Git's rules

	// Empty paths are never ignored (Git behavior)
	if inputPath == "" {
		return false
	}

	// GITIGNORE QUIRK: Double-slash paths are never ignored
	// POSIX systems treat "//" specially, and Git respects this by never ignoring such paths
	// However, "///" and beyond are treated as regular paths
	if strings.HasPrefix(inputPath, doubleSlash) && !strings.HasPrefix(inputPath, tripleSlash) {
		return false
	}

	// Normalize leading "./" prefixes (Git strips these during processing)
	// Special handling: "./" as a directory becomes "." (current directory)
	normalizedPath := inputPath
	if normalizedPath == dotSlash && isDir {
		normalizedPath = "."
	} else {
		// Strip all leading "./" prefixes until none remain
		for strings.HasPrefix(normalizedPath, dotSlash) {
			normalizedPath = strings.TrimPrefix(normalizedPath, dotSlash)
			if normalizedPath == "" {
				return false // Path becomes empty after normalization
			}
		}
	}

	// Apply Git's path normalization: collapse "//" to "/" and clean dot segments
	// This ensures consistent path representation for pattern matching
	normalizedPath = normalizePathForMatching(normalizedPath)

	// Early exit: no patterns means nothing is ignored
	if len(g.patterns) == 0 {
		return false
	}

	// === TWO-PASS ALGORITHM ===

	// Initialize the ignore status (will be modified by pattern matching)
	ignored := false

	// === PASS 1: PARENT EXCLUSION DETECTION ===
	// Identify which parent directories are permanently excluded by any pattern.
	excludedDirs := g.findExcludedParentDirectories(normalizedPath)

	// Determine if any parent directory is excluded (enforces parent exclusion rule)
	// If true, negation patterns cannot re-include this path
	parentExcluded := hasExcludedParentDirectory(normalizedPath, excludedDirs)

	// === PASS 2: PATTERN MATCHING ===
	// Apply all patterns to the target path in order, respecting parent exclusion
	for _, pat := range g.patterns {
		if matchesPattern(pat, normalizedPath, isDir) { //nolint:nestif		// Function is complex by design.
			if pat.negated {
				// NEGATION PATTERN: Attempts to re-include a previously ignored path
				// Git rule: negation only works if no parent directory is excluded
				if !parentExcluded {
					// GITIGNORE QUIRK: Repository root "." cannot be un-ignored
					// Reference: Verified with git version 2.34.1
					// Behavior: Pattern "!." does not un-ignore the repository root
					// This prevents the repository root from being accidentally excluded
					if pat.pattern != "." || normalizedPath != "." {
						ignored = false // Successfully re-include the path
					}
				}
				// If parentExcluded is true, negation is ignored (Git's parent exclusion rule)
			} else {
				// EXCLUSION PATTERN: Mark the path as ignored
				ignored = true
			}
		}
	}

	// === FINAL PARENT EXCLUSION CHECK ===
	// Even if no pattern directly matched this path, it may still be ignored
	// due to parent exclusion (contents of excluded directories are always ignored)
	if parentExcluded && !ignored {
		ignored = true
	}

	return ignored
}

// Patterns returns a copy of the original pattern strings used to create this GitIgnore instance.
// This method provides access to the raw gitignore patterns as they appeared in the input,
// before any parsing or processing occurred.
//
// The returned slice is a copy, so modifications to it will not affect the GitIgnore instance.
// Empty lines, comments, and invalid patterns that were skipped during parsing are not included.
//
// This is useful for debugging, logging, or recreating gitignore files from a GitIgnore instance.
//
// Returns:
//   - A slice containing the original pattern strings in the order they were processed
//   - Empty slice if no valid patterns were found during parsing
func (g *GitIgnore) Patterns() []string {
	// Create a copy to prevent external modification of our internal state
	patterns := make([]string, len(g.patterns))
	for i, p := range g.patterns {
		patterns[i] = p.original
	}

	return patterns
}

// findExcludedParentDirectories identifies which parent directories are permanently excluded.
// This function implements Pass 1 of Git's gitignore algorithm by checking all parent directories
// against all patterns to determine which directories are excluded.
//
// The function builds a list of all parent directory paths from the target path, then applies
// every pattern to each parent directory. A directory becomes permanently excluded if any
// non-negation pattern matches it, and can only be re-included by a negation pattern that
// specifically matches the directory itself.
//
// This is crucial for implementing Git's parent exclusion rule: once a directory is marked
// as excluded here, its contents cannot be re-included by negation patterns during Pass 2.
//
// Returns a map of excluded directory paths for fast O(1) lookup during Pass 2.
//
// Example:
//
//	For path "src/main/java/App.java" with pattern "src/"
//	- Creates parent paths: ["src", "src/main", "src/main/java"]
//	- Applies pattern "src/" to each parent
//	- "src" matches and becomes excluded
//	- Returns map: {"src": true}
//	- Later, "src/main/java/App.java" will be ignored due to parent exclusion
func (g *GitIgnore) findExcludedParentDirectories(targetPath string) map[string]bool {
	// Map to track which directories are excluded (key = directory path, value = true if excluded)
	excludedDirs := make(map[string]bool)

	// === STEP 1: Build list of all parent directory paths ===
	// Split target path into components to construct all possible parent paths
	// Example: "src/main/java/App.java" → ["src", "main", "java", "App.java"]
	parts := strings.Split(targetPath, "/")

	// Pre-allocate slice for efficiency (we'll have at most len(parts) parent paths)
	pathsToCheck := make([]string, 0, len(parts))

	// Construct each parent path: "src", "src/main", "src/main/java"
	// Note: we exclude the final component (the target file/directory itself)
	for i := 1; i <= len(parts); i++ {
		checkPath := strings.Join(parts[:i], "/")

		// Apply same normalization as main path for consistency
		checkPath = normalizePathForMatching(checkPath)
		pathsToCheck = append(pathsToCheck, checkPath)
	}

	// === STEP 2: Apply all patterns to all parent directories ===
	// This is the core of Pass 1: determine which directories are excluded
	// We check ALL patterns (not just directory-only patterns) because any pattern
	// can potentially exclude a directory
	for _, pat := range g.patterns {
		for _, checkPath := range pathsToCheck {
			// Skip checking the target path itself (that's handled in Pass 2)
			if checkPath == targetPath {
				continue
			}

			// Apply this pattern to this parent directory
			// We always pass isDir=true because we're checking directory parents
			// Any pattern that matches a directory path excludes that directory
			// Examples: "foo/" matches "foo", "foo/*" matches "foo", "**/" matches any directory
			if matchesPattern(pat, checkPath, true) {
				if pat.negated {
					// NEGATION PATTERN: Remove directory from exclusion list
					// This allows negation patterns to re-include directories that were
					// previously excluded by earlier patterns in the same .gitignore file
					delete(excludedDirs, checkPath)
				} else {
					// EXCLUSION PATTERN: Mark directory as permanently excluded
					// Once marked here, this directory's contents cannot be re-included
					// by negation patterns during Pass 2 (Git's parent exclusion rule)
					excludedDirs[checkPath] = true
				}
			}
		}
	}

	return excludedDirs
}

// detectGitPatternQuirk detects patterns with special Git behaviors that require custom handling.
// Returns whether a quirk was detected and the result of applying that quirk's logic.
// This function encapsulates Git-specific pattern matching behaviors that differ from standard glob matching.
//
//nolint:unparam	// Function designed to support future quirks
func detectGitPatternQuirk(pat pattern, path string, isDir bool) (bool, bool) {
	// GITIGNORE QUIRK: Patterns ending with /** are "contents-only"
	// They match everything under the base but NOT the base itself
	// Reference: https://git-scm.com/docs/gitignore#_pattern_format
	if strings.HasSuffix(pat.pattern, doubleStarSlash) {
		if isPatternBaseDirectory(pat, path, isDir) {
			return true, false // Don't match the base directory
		}
	}

	return false, false
}

// isPatternBaseDirectory checks if the given path represents the base directory of a contents-only pattern.
// This is used to determine if a path like "build" should be excluded by a pattern like "build/**".
// According to Git's gitignore semantics, "build/**" matches contents under build/ but not build/ itself.
func isPatternBaseDirectory(pat pattern, path string, isDir bool) bool {
	base := extractPatternBase(pat.pattern)
	if base == "" || base == doubleStar || strings.HasSuffix(base, doubleStar) {
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

	return matchesSimplePattern(basePattern, path, isDir)
}

// extractPatternBase extracts the base directory path from a contents-only pattern.
// For patterns like "foo/**" or "foo/**/bar/**", this returns the base path "foo" or "foo/**/bar".
// This is used to determine the directory that should NOT be matched by contents-only patterns.
func extractPatternBase(pattern string) string {
	// Strip all trailing /** groups to find the base
	base := pattern
	for strings.HasSuffix(base, doubleStarSlash) {
		base = strings.TrimSuffix(base, doubleStarSlash)
	}

	return base
}

// matchesPattern determines if a pattern matches a given path using a unified approach.
// This is the primary pattern matching function that handles both regular glob patterns
// and Git-specific quirks. It follows Git's precedence rules and matching semantics.
func matchesPattern(pat pattern, targetPath string, isDir bool) bool {
	// Directory-only patterns match directories ONLY (not files)
	if pat.dirOnly && !isDir {
		return false
	}

	// Check for Git quirks first
	if hasQuirk, quirkResult := detectGitPatternQuirk(pat, targetPath, isDir); hasQuirk {
		return quirkResult
	}

	// Use the simple, unified matching
	return matchesSimplePattern(pat, targetPath, isDir)
}

// matchesSimplePattern implements a unified, simple approach to pattern matching.
// This function handles the core glob pattern matching logic after Git quirks have been processed.
// It determines the appropriate matching scope (basename vs full path) based on pattern characteristics.
func matchesSimplePattern(pat pattern, targetPath string, _ bool) bool {
	glob := pat.pattern

	// Determine the target path to match against
	matchPath := targetPath

	if !pat.rooted && !strings.Contains(glob, "/") {
		// Non-rooted patterns without slash match only the basename
		matchPath = path.Base(targetPath)
	}

	// Let the glob library handle the matching
	return matchGlobPattern(pat, matchPath)
}

// matchGlobPattern performs Git-compatible glob pattern matching using the doublestar library.
// This function handles all the complex pre-processing needed to make doublestar behave like Git's
// pattern matching, including brace escaping, character class normalization, and escape sequence processing.
// Git treats braces as literal characters rather than expansion syntax, requiring special handling.
func matchGlobPattern(p pattern, targetPath string) bool {
	// The pattern has already been processed by trimTrailingSpaces,
	// which handles escape sequences for trailing spaces.
	glob := p.pattern

	// Check if this pattern has no wildcards (literal matching)
	if !containsUnescapedWildcards(glob) {
		// Process escapes for literal matching - remove all escape backslashes
		literal := processEscapeSequences(glob, true)

		return literal == targetPath
	}

	// Process escape sequences before glob matching
	originalGlob := glob

	glob = processEscapeSequences(glob, false)

	// Git does not support brace expansion, but doublestar does by default.
	// We need to escape unescaped braces to prevent expansion.
	glob = escapeBracesForGit(glob)

	// Normalize first-literal ']' inside character classes to avoid engine differences.
	glob = normalizeCharacterClassBrackets(glob)

	// Only apply normalizeWildcardEscapes if we haven't explicitly escaped wildcards
	// Check if the original pattern had escaped wildcards that we want to keep literal
	hasEscapedWildcards := strings.Contains(originalGlob, "\\*") || strings.Contains(originalGlob, "\\?") ||
		strings.Contains(originalGlob, "\\[")
	if !hasEscapedWildcards {
		glob = normalizeWildcardEscapes(glob)
	}

	// Normalize redundant wildcards (*** -> *) to match Git's behavior
	glob = normalizeRedundantWildcards(glob)

	matched, _ := doublestar.Match(glob, targetPath)

	return matched
}

// escapeBracesForGit escapes unescaped brace characters to prevent brace expansion.
// Git does not support brace expansion (unlike bash), so patterns like {a,b} should be treated
// literally. This function ensures doublestar treats braces as literal characters, not expansion patterns.
//
//nolint:gocognit	// Function is complex by design.
func escapeBracesForGit(pattern string) string {
	if pattern == "" || (!strings.Contains(pattern, "{") && !strings.Contains(pattern, "}")) {
		return pattern
	}

	var result strings.Builder
	result.Grow(len(pattern) + mediumBufferGrowth)

	inCharClass := false

	for charIdx := range len(pattern) {
		currentChar := pattern[charIdx]

		// Track character class boundaries
		switch currentChar {
		case '[':
			if charIdx == 0 || pattern[charIdx-1] != '\\' {
				inCharClass = true
			}
		case ']':
			if inCharClass && (charIdx == 0 || pattern[charIdx-1] != '\\') {
				inCharClass = false
			}
		case '{', '}':
			if !inCharClass {
				// Count preceding backslashes
				backslashes := 0

				for j := charIdx - 1; j >= 0 && pattern[j] == '\\'; j-- {
					backslashes++
				}
				// Escape if not already escaped (even number of backslashes)
				if backslashes%2 == 0 {
					result.WriteByte('\\')
				}
			}
		}

		result.WriteByte(currentChar)
	}

	return result.String()
}

// normalizeCharacterClassBrackets ensures that a ']' used as the first character
// inside a character class is escaped for consistent glob matching behavior.
// This handles Git's specific behavior where the first ']' in [abc] is literal, not a class closer.
//
//nolint:gocognit	// Function is complex by design.
func normalizeCharacterClassBrackets(pattern string) string {
	if pattern == "" || !strings.Contains(pattern, "[") {
		return pattern
	}

	var builder strings.Builder
	builder.Grow(len(pattern) + smallBufferGrowth)

	idx := 0
	for idx < len(pattern) {
		if pattern[idx] != '[' || (idx > 0 && pattern[idx-1] == '\\') {
			builder.WriteByte(pattern[idx])

			idx++

			continue
		}

		// Found unescaped '[', start of character class
		builder.WriteByte('[')

		idx++

		// Skip negation characters
		if idx < len(pattern) && (pattern[idx] == '!' || pattern[idx] == '^') {
			builder.WriteByte(pattern[idx])

			idx++
		}

		// Check if first listed character is ']'
		if idx < len(pattern) && pattern[idx] == ']' {
			builder.WriteByte('\\')
			builder.WriteByte(']')

			idx++
		}

		// Copy until we find the closing ']'
		for idx < len(pattern) {
			if pattern[idx] == ']' && (idx == 0 || pattern[idx-1] != '\\') {
				builder.WriteByte(']')

				idx++

				break
			}

			builder.WriteByte(pattern[idx])

			idx++
		}
	}

	return builder.String()
}

// hasExcludedParentDirectory checks if any parent directory is excluded according to Git's parent exclusion rule.
// Returns true if any parent directory in the path is marked as excluded.
func hasExcludedParentDirectory(targetPath string, excludedDirs map[string]bool) bool {
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

// containsUnescapedWildcards checks if pattern contains unescaped wildcard characters.
// This is used to determine whether a pattern needs glob matching or can be handled as literal string matching.
// Returns true if the pattern contains *, ?, or [ that are not preceded by an odd number of backslashes.
func containsUnescapedWildcards(pattern string) bool {
	for charIdx := 0; charIdx < len(pattern); charIdx++ {
		if pattern[charIdx] == '\\' && charIdx+1 < len(pattern) {
			// Skip escaped character
			charIdx++

			continue
		}
		// Check for unescaped wildcards
		if pattern[charIdx] == '*' || pattern[charIdx] == '?' || pattern[charIdx] == '[' {
			return true
		}
	}

	return false
}

// processEscapeSequences handles escape sequences in patterns with different modes.
// When forLiteral is true, escape backslashes are removed for literal string matching.
// When false, escapes are preserved in a format compatible with the doublestar library.
// This function handles Git's complex escape sequence rules including character class special cases.
//
//nolint:gocognit	// Function is complex by design.
func processEscapeSequences(pattern string, forLiteral bool) string {
	if pattern == "" {
		return pattern
	}

	var result strings.Builder
	result.Grow(len(pattern) + mediumBufferGrowth)

	inCharClass := false

	for idx := 0; idx < len(pattern); idx++ {
		char := pattern[idx]

		// Track character class boundaries
		switch {
		case char == '[' && (idx == 0 || pattern[idx-1] != '\\'):
			inCharClass = true

			result.WriteByte(char)

		case char == ']' && inCharClass && (idx == 0 || pattern[idx-1] != '\\'):
			inCharClass = false

			result.WriteByte(char)

		case char == '\\' && idx+1 < len(pattern):
			next := pattern[idx+1]

			// GITIGNORE QUIRK: Character class backslash handling
			// Reference: Git source code file wildmatch.c
			// Inside character classes [..], backslashes are preserved differently
			// to maintain Git compatibility with patterns like test[\\].txt matching "test\.txt"
			// Verified: git check-ignore with pattern "test[\\].txt" matches "test\.txt"
			if inCharClass { //nolint:nestif		// Function is complex by design.
				result.WriteByte('\\')
				result.WriteByte(next)

				idx++ // Skip the next character
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

					idx++ // Skip the next character

				case '#', '{', '}', '!':
					// Always remove backslash for these special chars
					result.WriteByte(next)

					idx++ // Skip the next character

				case '\\':
					nextNextIsWildcard := idx+2 < len(pattern) &&
						(pattern[idx+2] == '*' || pattern[idx+2] == '?' || pattern[idx+2] == '[')
					if !forLiteral && nextNextIsWildcard {
						// For glob matching with \\* or \\? or \\[ - preserve for doublestar
						result.WriteByte('\\')
						result.WriteByte('\\')

						idx++ // Skip the second backslash
					} else {
						// Regular double backslash becomes single
						result.WriteByte('\\')

						idx++
					}

				default:
					// For non-special characters, remove the backslash (Git behavior)
					result.WriteByte(next)

					idx++ // Skip the next character
				}
			}

		default:
			result.WriteByte(char)
		}
	}

	return result.String()
}

// trimTrailingUnescapedSpaces removes unescaped trailing spaces and processes escape sequences.
// Git trims trailing spaces unless they are escaped with a backslash (\<space>).
// This function implements Git's exact trailing space handling behavior.
func trimTrailingUnescapedSpaces(str string) string {
	if str == "" {
		return str
	}

	// Trim unescaped trailing spaces
	for len(str) > 0 && str[len(str)-1] == ' ' {
		backslashes := 0

		for i := len(str) - backslashOffset; i >= 0 && str[i] == '\\'; i-- {
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

	for idx := 0; idx < len(str); idx++ {
		if idx < len(str)-1 && str[idx] == '\\' && str[idx+1] == ' ' {
			result.WriteByte(' ')

			idx++
		} else {
			result.WriteByte(str[idx])
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
	line = trimTrailingUnescapedSpaces(line)

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

// normalizeRedundantWildcards collapses sequences of 3+ asterisks to a single asterisk
// when they're not separated by slashes, matching Git's behavior.
func normalizeRedundantWildcards(pattern string) string {
	// Replace sequences of 3+ asterisks with a single asterisk
	// but preserve ** when it's exactly 2 asterisks
	result := pattern
	for strings.Contains(result, "***") {
		result = strings.ReplaceAll(result, "***", "*")
	}

	return result
}

// normalizePathForMatching collapses runs of '/' and cleans dot segments like Git does.
// This ensures paths are in canonical form before pattern matching, following Git's
// internal path normalization behavior used during gitignore evaluation.
func normalizePathForMatching(inputPath string) string {
	if inputPath == "" || inputPath == "." {
		return inputPath
	}

	processedPath := inputPath

	// Special case: preserve leading double slash (POSIX behavior)
	preserveDoubleSlash := strings.HasPrefix(processedPath, doubleSlash) &&
		!strings.HasPrefix(processedPath, tripleSlash)
	if preserveDoubleSlash {
		processedPath = doubleSlash + processedPath[2:]
	}

	// Collapse all runs of '/'
	for strings.Contains(processedPath, doubleSlash) {
		processedPath = strings.ReplaceAll(processedPath, doubleSlash, "/")
	}

	// Restore leading double slash if needed
	if preserveDoubleSlash && !strings.HasPrefix(processedPath, doubleSlash) {
		processedPath = "/" + processedPath
	}

	// Clean dot segments
	return path.Clean(processedPath)
}

// normalizeWildcardEscapes ensures that * and ? remain meta even if preceded by odd number of backslashes.
// This handles Git's specific behavior where backslash handling for wildcards requires special normalization
// to maintain compatibility with Git's pattern matching engine.
//
//nolint:gocognit	// Function is complex by design.
func normalizeWildcardEscapes(glob string) string {
	if glob == "" {
		return glob
	}

	var builder strings.Builder
	builder.Grow(len(glob) + smallBufferGrowth)

	inClass := false

	for idx := 0; idx < len(glob); idx++ {
		currentChar := glob[idx]

		// Track character class boundaries
		switch {
		case currentChar == '[' && !inClass:
			inClass = true

		case currentChar == ']' && inClass:
			inClass = false

		case currentChar == '\\' && idx+1 < len(glob):
			// Count consecutive backslashes
			runStart := idx
			for idx < len(glob) && glob[idx] == '\\' {
				idx++
			}

			runLen := idx - runStart

			// Check if next character is a meta character
			if idx < len(glob) && !inClass && (glob[idx] == '*' || glob[idx] == '?') {
				// Write original backslashes
				for range runLen {
					builder.WriteByte('\\')
				}
				// Add extra backslash if odd number (to keep meta unescaped)
				if runLen%2 == 1 {
					builder.WriteByte('\\')
				}
			} else {
				// Write backslashes as-is
				for range runLen {
					builder.WriteByte('\\')
				}
			}

			idx-- // Back up since outer loop will increment

			continue
		}

		builder.WriteByte(currentChar)
	}

	return builder.String()
}
