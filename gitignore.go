// Package gitignore implements Git-compatible gitignore pattern matching with the aim to reach parity with Git's
// ignore behavior.
//
// It provides gitignore semantics including pattern parsing with escape sequences,
// two-pass ignore checking with parent exclusion detection, negation rules mimicking Git's behavior,
// brace escaping to prevent expansion, and cross-platform path handling using forward slashes only.
//
// Usage:
//
//	gi := gitignore.New("*.log", "build/", "!important.log")
//	fmt.Println(gi.Ignored("app.log", false))     		// true (matches *.log)
//	fmt.Println(gi.Ignored("important.log", false)) 	// false (negated by !important.log)
//	fmt.Println(gi.Ignored("build/file.txt", false)) 	// true (parent directory build/ is excluded)
package gitignore

import (
	"path"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// Pattern matching constants used throughout gitignore processing.
const (
	// Double star pattern (matches any file or directory recursively).
	doubleStar = "**"
	// Contents-only pattern suffix (matches everything under a directory but not the directory itself).
	doubleStarSlash = "/**"
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

// GitIgnore represents a collection of gitignore patterns and provides methods to check if paths should be ignored.
type GitIgnore struct {
	// patterns holds the parsed gitignore patterns in the order they appear
	patterns []pattern
	// root indicates the directory that should be considered the root (and stripped from paths)
	root string
}

// New creates a GitIgnore instance from gitignore-like pattern lines.
func New(lines ...string) *GitIgnore {
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

// NewWithRoot creates a GitIgnore instance with a specified root directory.
//
//nolint:unused		// Method is unused for now.
func newWithRoot(root string, lines ...string) *GitIgnore {
	gitIgnore := New(lines...)

	gitIgnore.root = root

	return gitIgnore
}

// Patterns returns a copy of the pattern strings after parsing.
func (g *GitIgnore) Patterns() []string {
	patterns := make([]string, len(g.patterns))
	for i, p := range g.patterns {
		patterns[i] = p.original
	}

	return patterns
}

// Ignored determines whether a path should be ignored given the current set of patterns.
func (g *GitIgnore) Ignored(inputPath string, isDir bool) bool {
	return g.ignored(inputPath, isDir)
}

// ignore returns the ignore decision.
//
//nolint:gocognit	// Function is complex by design.
func (g *GitIgnore) ignored(inputPath string, isDir bool) bool {
	// No patterns means nothing is ignored
	if len(g.patterns) == 0 {
		return false
	}

	// Handle edge cases and normalize the input path according to Git's rules

	// Empty paths are never ignored (Git behavior)
	if inputPath == "" {
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

	// Apply normalization
	normalizedPath = normalizePathForMatching(normalizedPath, g.root)

	// Skip paths that are not relative after normalization
	if strings.HasPrefix(normalizedPath, "/") {
		return false
	}

	// Ignore status
	ignored := false

	// Parent exclusion
	// Identify which parent directories are permanently excluded by any pattern.
	excludedDirs := g.findExcludedParentDirectories(normalizedPath)

	// Determine if any parent directory is excluded (enforces parent exclusion rule)
	// If true, negation patterns cannot re-include this path
	parentExcluded := hasExcludedParentDirectory(normalizedPath, excludedDirs)

	// Pattern matching
	// Apply all patterns to the target path in order, respecting parent exclusion
	for _, pat := range g.patterns {
		if matchesPattern(pat, normalizedPath, isDir) { //nolint:nestif		// Function is complex by design.
			if pat.negated {
				// NEGATION PATTERN: Attempts to re-include a previously ignored path
				// Git rule: negation only works if no parent directory is excluded
				if !parentExcluded {
					// GITIGNORE QUIRK: Repository root "." cannot be un-ignored
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

	// Final parent exclusion check
	// Even if no pattern directly matched this path, it may still be ignored
	// due to parent exclusion (contents of excluded directories are always ignored)
	if parentExcluded && !ignored {
		ignored = true
	}

	return ignored
}

// findExcludedParentDirectories identifies which parent directories are permanently excluded.
// The function builds a list of all parent directory paths from the target path, then applies
// every pattern to each parent directory. A directory becomes permanently excluded if any
// non-negation pattern matches it, and can only be re-included by a negation pattern that
// specifically matches the directory itself.
func (g *GitIgnore) findExcludedParentDirectories(targetPath string) map[string]bool {
	// Map to track which directories are excluded (key = directory path, value = true if excluded)
	excludedDirs := make(map[string]bool)

	// Build list of all parent directory paths
	// Split target path into components to construct all possible parent paths
	parts := strings.Split(targetPath, "/")

	pathsToCheck := make([]string, 0, len(parts))

	// Construct each parent path: "src", "src/main", "src/main/java"
	// Note: we exclude the final component (the target file/directory itself)
	for i := 1; i <= len(parts); i++ {
		checkPath := strings.Join(parts[:i], "/")

		// Apply same normalization as main path for consistency
		checkPath = normalizePathForMatching(checkPath, "")
		pathsToCheck = append(pathsToCheck, checkPath)
	}

	// Apply all patterns to all parent directories, to determine which directories are excluded
	// We check ALL patterns (not just directory-only patterns) because any pattern
	// can potentially exclude a directory
	for _, pat := range g.patterns {
		for _, checkPath := range pathsToCheck {
			// Skip checking the target path itself
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
					excludedDirs[checkPath] = true
				}
			}
		}
	}

	return excludedDirs
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

// detectGitPatternQuirk detects patterns with special Git behaviors that require custom handling.
//
//nolint:unparam	// Function designed to support future quirks
func detectGitPatternQuirk(pat pattern, path string, isDir bool) (bool, bool) {
	// GITIGNORE QUIRK: Patterns ending with /** are "contents-only"
	// They match everything under the base but NOT the base itself
	if strings.HasSuffix(pat.pattern, doubleStarSlash) {
		if isPatternBaseDirectory(pat, path, isDir) {
			return true, false // Don't match the base directory
		}
	}

	return false, false
}

// isPatternBaseDirectory checks if the path is the base directory of a contents-only pattern.
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
func extractPatternBase(pattern string) string {
	// Strip all trailing /** groups to find the base
	base := pattern
	for strings.HasSuffix(base, doubleStarSlash) {
		base = strings.TrimSuffix(base, doubleStarSlash)
	}

	return base
}

// matchesPattern determines if a pattern matches the given path.
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

// matchesSimplePattern handles core glob pattern matching after Git quirks are processed.
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

// matchGlobPattern performs Git-compatible glob pattern matching using doublestar.
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

	return doublestar.MatchUnvalidated(glob, targetPath)
}

// escapeBracesForGit escapes unescaped brace characters for literal matching.
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

// normalizeCharacterClassBrackets escapes first ']' inside character classes for Git compatibility.
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

// containsUnescapedWildcards checks if pattern contains unescaped *, ?, or [ characters.
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

// processEscapeSequences processes escape sequences based on matching mode.
func processEscapeSequences(pattern string, forLiteral bool) string {
	if pattern == "" {
		return pattern
	}

	var result strings.Builder
	result.Grow(len(pattern))

	for idx := 0; idx < len(pattern); idx++ {
		if pattern[idx] == '\\' && idx+1 < len(pattern) {
			next := pattern[idx+1]
			if forLiteral || next == '#' || next == '!' || next == ' ' || next == '{' || next == '}' {
				// Skip backslash, write next char
				result.WriteByte(next)

				idx++
			} else {
				// Keep backslash
				result.WriteByte('\\')
			}
		} else {
			result.WriteByte(pattern[idx])
		}
	}

	return result.String()
}

// trimTrailingUnescapedSpaces removes unescaped trailing spaces per Git behavior.
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

// parsePattern parses a gitignore line into a pattern struct.
func parsePattern(line string) *pattern {
	// Blank lines are ignored
	if line == "" {
		return nil
	}

	// Comments start with # (unless escaped)
	if strings.HasPrefix(line, "#") {
		return nil
	}

	// Lines containing multiple path separators (//) are ignored
	if strings.Count(line, "//") > 0 && !strings.Contains(line, "\\//") && !strings.Contains(line, "/\\/") {
		return nil
	}

	// Reject patterns with unclosed brackets
	if unclosedBracket(line) {
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

// normalizeRedundantWildcards collapses *** sequences to **.
func normalizeRedundantWildcards(pattern string) string {
	// Replace sequences of 3+ asterisks with a double star
	result := pattern
	for strings.Contains(result, "***") {
		result = strings.ReplaceAll(result, "***", "**")
	}

	return result
}

// normalizePathForMatching cleans and normalizes the given path for matching.
func normalizePathForMatching(inputPath, root string) string {
	return strings.TrimPrefix(path.Clean(inputPath), root)
}

// normalizeWildcardEscapes normalizes backslash escaping for * and ? wildcards.
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

// unclosedBracket checks if there are unclosed brackets in the given string.
func unclosedBracket(pattern string) bool {
	escaped := false
	inClass := false

	for _, char := range pattern {
		if escaped {
			escaped = false

			continue
		}

		if char == '\\' {
			escaped = true

			continue
		}

		if !inClass && char == '[' {
			inClass = true
		} else if inClass && char == ']' {
			inClass = false
		}
	}

	return inClass
}
