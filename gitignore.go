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

// Constants for pattern matching
const (
	doubleStarSlash = "/**"
	doubleSlash     = "//"
	tripleSlash     = "///"
)

// normalizeCandidatePath collapses runs of '/' and cleans dot segments like Git does
func normalizeCandidatePath(p string) string {
	if p == "" {
		return p
	}
	
	// Special case: preserve leading double slash (POSIX behavior)
	if strings.HasPrefix(p, doubleSlash) && !strings.HasPrefix(p, tripleSlash) {
		// Keep the leading "//" but normalize the rest
		rest := strings.ReplaceAll(p[2:], doubleSlash, "/")
		p = doubleSlash + rest
	} else {
		// Normal case: collapse all runs of '/'
		p = strings.ReplaceAll(p, doubleSlash, "/")
		// Keep replacing until no more double slashes
		for strings.Contains(p, doubleSlash) {
			p = strings.ReplaceAll(p, doubleSlash, "/")
		}
	}
	
	// Clean dot segments but preserve "."
	if p != "." {
		p = path.Clean(p)
	}
	return p
}

// normalizeMetaEscapes ensures that * and ? remain meta even if preceded by odd number of backslashes
func normalizeMetaEscapes(glob string) string {
	if glob == "" {
		return glob
	}
	var b strings.Builder
	b.Grow(len(glob) + 8)

	inClass := false
	for i := 0; i < len(glob); {
		ch := glob[i]

		// enter/exit character class
		if ch == '[' && !inClass {
			inClass = true
			b.WriteByte(ch)
			i++
			continue
		}
		if ch == ']' && inClass {
			inClass = false
			b.WriteByte(ch)
			i++
			continue
		}

		// count a run of backslashes
		if ch == '\\' {
			runStart := i
			for i < len(glob) && glob[i] == '\\' {
				i++
			}
			runLen := i - runStart
			next := byte(0)
			if i < len(glob) {
				next = glob[i]
			}
			// If next is a meta and we're not in a class, ensure meta stays meta.
			if !inClass && (next == '*' || next == '?') {
				// Write the run as-is
				for k := 0; k < runLen; k++ {
					b.WriteByte('\\')
				}
				// And if the run was odd, add one more '\' so meta isn't escaped
				if runLen%2 == 1 {
					b.WriteByte('\\')
				}
				continue
			}
			// otherwise, just emit the run
			for k := 0; k < runLen; k++ {
				b.WriteByte('\\')
			}
			continue
		}

		// default
		b.WriteByte(ch)
		i++
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

	// Derived forms for **-semantics handling
	formBareAnyDir     bool   // exactly **/ (optionally with leading /)
	formDirDescendants bool   // ends with /**/ with non-empty base before it
	formContentsOnly   bool   // ends with /** (not dir-only) - matches contents not base
	formSandwich       bool   // pattern like **/middle/** - matches contents under middle
	sandwichMiddle     string // middle part for sandwich patterns
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
	if p == "./" && isDir {
		p = "."
	} else {
		for strings.HasPrefix(p, "./") {
			p = strings.TrimPrefix(p, "./")
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

// isSandwichBase checks if path is the base directory of a sandwich pattern (should not match)
func isSandwichBase(pat pattern, path string) bool {
	if !pat.formSandwich {
		return false
	}
	if strings.HasSuffix(path, pat.sandwichMiddle) {
		prefix := strings.TrimSuffix(path, pat.sandwichMiddle)
		return prefix == "" || strings.HasSuffix(prefix, "/")
	}
	return false
}

// matches determines if a pattern matches a given path, handling both directory-only
// and regular patterns according to Git's matching rules.
func matches(pat pattern, p string, isDir bool) bool {
	// Directory-only patterns match directories ONLY (not files).
	if pat.dirOnly {
		// bareAnyDir (**/) => any directory at any depth
		if pat.formBareAnyDir {
			return isDir
		}

		// dirDescendants (<base>/**/) => directories strictly under base, not the base
		if pat.formDirDescendants {
			if !isDir {
				return false
			}
			base := stripTrailingSuffix(pat.pattern, false) // remove all trailing "/**" groups

			// If base is meaningful, do a strict-descendant match; otherwise fall back to original glob.
			if base != "" && !endsWithDoubleStarSegment(base) {
				// If candidate equals base: do NOT match
				if matchRawGlob(base, p) {
					return false
				}
				// A directory strictly below the base should match
				return matchRawGlob(base+"/**", p)
			}
			// Degenerate base like "**": rely on the original glob.
			return matchGlob(pat, p)
		}

		// Regular dir-only (e.g., "x/"): matches the directory entry
		if isDir {
			if pat.rooted || strings.Contains(pat.pattern, "/") {
				return matchGlob(pat, p)
			}
			return matchGlob(pat, path.Base(p))
		}
		return false
	}

	// Regular (non-dir-only) patterns
	return matchesFilePattern(pat, p, isDir)
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
	hasEscapedWildcards := strings.Contains(originalGlob, "\\*") || strings.Contains(originalGlob, "\\?") || strings.Contains(originalGlob, "\\[")
	if !hasEscapedWildcards {
		glob = normalizeMetaEscapes(glob)
	}
	
	matched, _ := doublestar.Match(glob, targetPath)
	return matched
}

// matchRawGlob is a convenience wrapper for matchGlob with a raw pattern string
func matchRawGlob(glob, targetPath string) bool {
	return matchGlob(pattern{pattern: glob}, targetPath)
}

// stripTrailingSuffix removes trailing "/**" segments based on mode
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
	if glob == "**" {
		return true
	}
	i := strings.LastIndex(glob, "/")
	if i == -1 {
		return glob == "**"
	}
	return glob[i+1:] == "**"
}

// escapeBraces escapes unescaped brace characters to prevent brace expansion
func escapeBraces(p string) string {
	if p == "" || (!strings.Contains(p, "{") && !strings.Contains(p, "}")) {
		return p
	}

	var result strings.Builder
	result.Grow(len(p) + 10)
	
	inCharClass := false
	for i := 0; i < len(p); i++ {
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

// hasUnescapedWildcards checks if pattern has unescaped wildcards
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

// processEscapes handles escape sequences in patterns with different modes
func processEscapes(pattern string, forLiteral bool) string {
	if pattern == "" {
		return pattern
	}
	
	var result strings.Builder
	result.Grow(len(pattern) + 10)
	
	for i := 0; i < len(pattern); i++ {
		if pattern[i] == '\\' && i+1 < len(pattern) {
			next := pattern[i+1]
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
				result.WriteByte(pattern[i])
			}
		} else {
			result.WriteByte(pattern[i])
		}
	}
	
	return result.String()
}


// trimTrailingSpaces removes unescaped trailing spaces and processes escape sequences
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

	// Check for trailing slash before stripping
	classifyNoRoot := strings.TrimPrefix(line, "/")
	originalDirOnly := strings.HasSuffix(classifyNoRoot, "/")

	// Compute derived flags from the original string (post-negation, post-trim).
	if originalDirOnly {
		// Bare "**/" (with or without a single leading slash).
		if classifyNoRoot == "**/" {
			pat.formBareAnyDir = true
		}
		// Ends with "/**/" and has non-empty base: "<base>/**/"
		if strings.HasSuffix(classifyNoRoot, "/**/") {
			base := strings.TrimSuffix(classifyNoRoot, "/**/")
			if base != "" {
				pat.formDirDescendants = true
			}
		}
	} else {
		// Contents-only: ANY pattern that ends with "/**" (but not "/**/") should be treated as
		// "descendants only" (i.e., does not match the base entry itself). This mirrors Git's behavior.
		if strings.HasSuffix(classifyNoRoot, "/**") && !strings.HasSuffix(classifyNoRoot, "/**/") {
			pat.formContentsOnly = true
		}
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

	// Detect sandwich patterns like **/node_modules/** or **/foo/bar/**
	// These match contents under 'node_modules' or 'foo/bar' but NOT those directories themselves
	if !pat.dirOnly && strings.Contains(pat.pattern, "/**") {
		// Check for patterns like **/middle/** where middle can contain slashes
		if strings.HasPrefix(pat.pattern, "**/") && strings.HasSuffix(pat.pattern, "/**") {
			middle := strings.TrimPrefix(pat.pattern, "**/")
			middle = strings.TrimSuffix(middle, "/**")
			// Accept only if middle doesn't contain wildcards or double-stars
			// This excludes patterns like **/foo/**/bar/** or **/node_modules*/**
			if middle != "" && middle != "*" && middle != "**" &&
				!strings.Contains(middle, "*") && !strings.Contains(middle, "**") {
				pat.formSandwich = true
				pat.sandwichMiddle = middle
			}
		} else if idx := strings.Index(pat.pattern, "/**/"); idx != -1 {
			// Rooted sandwich pattern like /a/**/middle/**
			remaining := pat.pattern[idx+4:]
			if strings.HasSuffix(remaining, "/**") {
				middle := strings.TrimSuffix(remaining, "/**")
				// Accept only if middle doesn't contain wildcards or double-stars
				if middle != "" && middle != "*" && middle != "**" &&
					!strings.Contains(middle, "*") && !strings.Contains(middle, "**") {
					pat.formSandwich = true
					pat.sandwichMiddle = middle
				}
			}
		}
	}

	return pat
}

// matchesFilePattern handles matching for regular patterns (not directory-only).
// Implements Git's complex rules for rooted vs non-rooted patterns,
// basename matching, and special handling for wildcard patterns.
func matchesFilePattern(pat pattern, filePath string, isDir bool) bool {
	// Handle sandwich patterns like **/node_modules/** or **/foo/bar/**
	// These should match contents under the middle part but NOT the middle directory itself
	if isSandwichBase(pat, filePath) {
		return false
	}
	if pat.formSandwich {
		return matchGlob(pat, filePath)
	}

	// Contents-only patterns (<base>/**) should NOT match the base entry itself,
	// even when multiple trailing "/**" groups are present. We also handle degenerate
	// cases like "**/**" where the stripped base ends with a "**" segment.
	if pat.formContentsOnly {
		base := stripTrailingSuffix(pat.pattern, true)

		// If we have a meaningful base (doesn't end with an all-wildcard "**" segment),
		// reject an exact base hit using glob semantics (not plain string equality).
		if base != "" && !endsWithDoubleStarSegment(base) {
			if matchRawGlob(base, filePath) {
				return false
			}
		}

		if isDir {
			// Directories strictly below the base should match.
			// For degenerate bases like "**", just fall back to the original pattern.
			if base == "" || endsWithDoubleStarSegment(base) {
				return matchGlob(pat, filePath)
			}
			return matchRawGlob(base+"/**", filePath)
		}

		// Files below the base are matched by the original pattern as-is.
		return matchGlob(pat, filePath)
	}

	// Special case: * pattern should match a single path component.
	// If it's rooted (/*), it should only match at root level.
	if pat.pattern == "*" {
		if pat.rooted {
			// /* should only match top-level entries
			if strings.Contains(filePath, "/") {
				return false // Not at top level
			}
			// In Git, * does not match files starting with . (dotfiles)
			if strings.HasPrefix(filePath, ".") {
				return false
			}
			matched, _ := doublestar.Match("*", filePath)
			return matched
		}
		// Unrooted * matches a single component at any depth
		basename := path.Base(filePath)
		if basename == "" {
			return false
		}
		matched, _ := doublestar.Match("*", basename)
		return matched
	}

	if pat.rooted {
		// Rooted patterns match only from the repository root
		return matchGlob(pat, filePath)
	}

	// Non-rooted patterns (no '/'): match only the entry's basename
	if !strings.Contains(pat.pattern, "/") {
		basename := path.Base(filePath)
		return matchGlob(pat, basename)
	}

	// Pattern contains '/', treat as anchored to the ignore file directory (root here)
	// Only match the full path since such patterns are path-anchored in Git semantics.
	return matchGlob(pat, filePath)
}

// patternExcludesDirectory determines if a pattern explicitly excludes a directory.
// The parent exclusion rule applies when a pattern matches the directory entry itself.
// Patterns like "foo/*" DO exclude "foo/bar" (a direct child directory of "foo").
// Patterns ending with "/**" match only the contents under a base and must NOT match the base entry.
func patternExcludesDirectory(pat pattern, dirPath string) bool {
	// Handle sandwich patterns specially for directories
	if isSandwichBase(pat, dirPath) {
		return false
	}
	if pat.formSandwich {
		return matchGlob(pat, dirPath)
	}

	if pat.dirOnly {
		// Directory-only patterns (ending with /) explicitly exclude directories.
		// bareAnyDir (**/) excludes any directory (or un-excludes via negation).
		if pat.formBareAnyDir {
			return true
		}
		// dirDescendants (<base>/**/) excludes directories strictly below base, not the base
		if pat.formDirDescendants {
			base := stripTrailingSuffix(pat.pattern, false)
			// If base is meaningful, do a strict-descendant check; otherwise fallback.
			if base != "" && !endsWithDoubleStarSegment(base) {
				// If candidate equals base: do NOT match
				if matchRawGlob(base, dirPath) {
					return false
				}
				// A directory strictly below the base should match
				return matchRawGlob(base+"/**", dirPath)
			}
			// Degenerate base like "**": rely on original glob.
			return matchGlob(pat, dirPath)
		}
		// Regular dir-only matching
		if pat.rooted || strings.Contains(pat.pattern, "/") {
			return matchGlob(pat, dirPath)
		}
		return matchGlob(pat, path.Base(dirPath))
	}

	// Non-dirOnly pattern with trailing "/**" is contents-only:
	// it must NOT exclude the base directory entry itself, but it DOES
	// exclude directories strictly below the base. Handle multiple trailing groups
	// and degenerate bases like "**".
	if pat.formContentsOnly {
		base := stripTrailingSuffix(pat.pattern, true)
		if base != "" && !endsWithDoubleStarSegment(base) {
			if matchRawGlob(base, dirPath) {
				return false
			}
			return matchRawGlob(base+"/**", dirPath)
		}
		// Degenerate base (e.g., "**"): rely on original glob semantics
		return matchGlob(pat, dirPath)
	}

	// Patterns that match a directory entry (basename or full path) exclude that directory.

	if pat.pattern == "*" {
		if pat.rooted {
			// "/*" excludes top-level directories it matches
			if strings.Contains(dirPath, "/") {
				return false // Not at top level
			}
			// In Git, * does not match directories starting with . (dotdirs)
			if strings.HasPrefix(dirPath, ".") {
				return false
			}
			matched, _ := doublestar.Match("*", dirPath)
			return matched
		}
		// "*" can exclude directories by matching their basename
		basename := path.Base(dirPath)
		if basename == "" {
			return false
		}
		matched, _ := doublestar.Match("*", basename)
		return matched
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
