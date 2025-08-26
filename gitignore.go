// Package gitignore implements Git-compatible gitignore pattern matching with the aim to reach parity with Git's
// ignore behavior.
//
// It provides gitignore semantics including pattern parsing with escape sequences,
// two-pass ignore checking with parent exclusion detection, negation rules that attempt to match Git's behavior,
// brace escaping to prevent expansion, and cross-platform path handling using forward slashes only.
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

// MatchResult is a minimal decision record for a single path check.
// It contains the final ignore decision and a concise pattern summary.
// Details matches the original pattern line that determined the outcome
// (e.g., "*.log", "!internal/*.go", "build/"), or empty string if no pattern applied.
type MatchResult struct {
	// Path is the original input path being checked
	Path string
	// IsDir indicates if the path is a directory
	IsDir bool
	// Ignored indicates if the path is ignored
	Ignored bool
	// ParentExcluded indicates if a parent directory is excluded
	ParentExcluded bool
	// Details provides additional information about the match
	Details string
}

// New creates a GitIgnore instance from gitignore pattern lines.
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

// Ignored determines whether a path should be ignored according to the gitignore patterns.
func (g *GitIgnore) Ignored(inputPath string, isDir bool) bool {
	return g.Decide(inputPath, isDir).Ignored
}

// Decide returns a MatchResult with the ignore decision and matching pattern details.
//
//nolint:gocognit	// Function is complex by design.
func (g *GitIgnore) Decide(inputPath string, isDir bool) MatchResult {
	res := MatchResult{Path: inputPath, IsDir: isDir}

	// Handle edge cases and normalize the input path according to Git's rules

	// Empty paths are never ignored (Git behavior)
	if inputPath == "" {
		return res
	}

	// GITIGNORE QUIRK: Double-slash paths are never ignored
	// POSIX systems treat "//" specially, and Git respects this by never ignoring such paths
	// However, "///" and beyond are treated as regular paths
	if strings.HasPrefix(inputPath, doubleSlash) && !strings.HasPrefix(inputPath, tripleSlash) {
		return res
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
				return res // Path becomes empty after normalization
			}
		}
	}

	// Apply Git's path normalization: collapse "//" to "/" and clean dot segments
	// This ensures consistent path representation for pattern matching
	normalizedPath = normalizePathForMatching(normalizedPath)

	// Early exit: no patterns means nothing is ignored
	if len(g.patterns) == 0 {
		return res
	}

	// Identify which parent directories are permanently excluded by any pattern.
	excludedDirs, parentDetail := g.parentExclusionSummary(normalizedPath)
	parentPath, hasParent := firstExcludedParent(normalizedPath, excludedDirs)

	res.ParentExcluded = hasParent

	parentCause := ""

	if hasParent {
		parentCause = parentDetail[parentPath]
	}

	// Pass 2: apply patterns; remember only the last effective pattern
	ignored := false
	details := ""

	for _, pat := range g.patterns {
		matched, contentsOnlySkip := matchesPatternWithSkip(pat, normalizedPath, isDir)
		if !matched || contentsOnlySkip {
			continue
		}

		if pat.negated {
			// Negation only works if no parent directory is excluded
			if !res.ParentExcluded {
				// Git quirk: "!." cannot un-ignore repository root
				if pat.pattern != "." || normalizedPath != "." {
					ignored = false
					details = pat.original
				}
			}
		} else {
			ignored = true
			details = pat.original
		}
	}

	// Final parent rule: parent exclusion wins if nothing excluded it directly
	if res.ParentExcluded && !ignored {
		ignored = true

		if details == "" {
			details = parentCause
		}
	}

	res.Ignored = ignored
	res.Details = details // may be empty if nothing applied

	return res
}

// Patterns returns a copy of the pattern strings after parsing.
func (g *GitIgnore) Patterns() []string {
	patterns := make([]string, len(g.patterns))
	for i, p := range g.patterns {
		patterns[i] = p.original
	}

	return patterns
}

// parentExclusionSummary determines which parent directories are excluded and by which patterns.
func (g *GitIgnore) parentExclusionSummary(targetPath string) (excluded map[string]bool, lastCause map[string]string) {
	excluded = make(map[string]bool)
	lastCause = make(map[string]string)

	parts := strings.Split(targetPath, "/")

	parents := make([]string, 0, len(parts))
	for i := 1; i <= len(parts); i++ {
		p := normalizePathForMatching(strings.Join(parts[:i], "/"))

		parents = append(parents, p)
	}

	for _, pat := range g.patterns {
		for _, parentPath := range parents {
			if parentPath == targetPath {
				continue
			}

			matched, contentsOnlySkip := matchesPatternWithSkip(pat, parentPath, true)

			if !matched || contentsOnlySkip {
				continue
			}

			if pat.negated {
				if excluded[parentPath] {
					delete(excluded, parentPath)
				}

				lastCause[parentPath] = pat.original
			} else {
				excluded[parentPath] = true
				lastCause[parentPath] = pat.original
			}
		}
	}

	return excluded, lastCause
}

// firstExcludedParent returns the first excluded parent directory, closest to root.
func firstExcludedParent(targetPath string, excluded map[string]bool) (string, bool) {
	parts := strings.Split(targetPath, "/")
	if len(parts) <= 1 {
		return "", false
	}

	for i := 1; i < len(parts); i++ {
		p := strings.Join(parts[:i], "/")
		if excluded[p] {
			return p, true
		}
	}

	return "", false
}

// detectGitPatternQuirk detects and handles Git-specific pattern behaviors.
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

// normalizePathForMatching normalizes paths for pattern matching.
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

// matchesPatternWithSkip returns whether pattern matches and if contents-only pattern should skip base directory.
func matchesPatternWithSkip(pat pattern, targetPath string, isDir bool) (bool, bool) {
	if strings.HasSuffix(pat.pattern, "/**") && isPatternBaseDirectory(pat, targetPath, isDir) {
		return false, true
	}

	return matchesPattern(pat, targetPath, isDir), false
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
