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

	// Derived forms for holistic handling of **-semantics.
	// Computed in parsePattern from the original (after removing leading "!" and trimming spaces).
	// formBareAnyDir: original is exactly **/ (optionally with a leading /)
	formBareAnyDir bool
	// formDirDescendants: original ends with "/**/" and has a non-empty base before that
	// e.g. "abc/**/" or "/x/**/" — means "directories strictly under base"
	formDirDescendants bool
	// formContentsOnly: processed pattern ends with "/**" (not dir-only). Means "everything under base, not the base
	// entry"
	formContentsOnly bool
	// formSandwich indicates a pattern like **/middle/**
	// where we match contents under 'middle' but not 'middle' itself
	formSandwich   bool
	sandwichMiddle string // The middle part (e.g., "node_modules")
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

	// No patterns means nothing is ignored
	if len(g.patterns) == 0 {
		return false
	}

	ignored := false

	// Track which directories are permanently excluded
	// Once a directory is excluded, its contents can NEVER be re-included
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
			base := stripTrailingDirDescSuffixes(pat.pattern) // remove all trailing "/**" groups

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
			return matchesDirectoryPath(pat, p)
		}
		return false
	}

	// Regular (non-dir-only) patterns
	return matchesFilePattern(pat, p, isDir)
}

// matchesDirectoryPath checks if a directory path matches a pattern.
// Handles both rooted patterns (anchored to repository root) and
// non-rooted patterns that can match at any directory level.
func matchesDirectoryPath(pat pattern, dirPath string) bool {
	// In our "single root .gitignore" world, any pattern containing a slash
	// is anchored to the repository root (Git behavior)
	if pat.rooted || strings.Contains(pat.pattern, "/") {
		// Match against the full directory path
		return matchGlob(pat, dirPath)
	}

	// Patterns without slash can match at any directory level
	// Compare against the basename only
	basename := path.Base(dirPath)
	return matchGlob(pat, basename)
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

// matchRawGlob applies the same escaping/normalization as matchGlob but takes a raw glob string.
func matchRawGlob(glob, targetPath string) bool {
	// Mimic matchGlob's preprocessing
	glob = escapeBraces(glob)
	glob = escapeFirstClosingBracketInCharClass(glob)
	matched, _ := doublestar.Match(glob, targetPath)
	return matched
}

// stripTrailingContentsSuffixes removes all trailing "/**" segments (but not "/**/") from a glob.
// Examples:
//
//	"base/**" -> "base"
//	"base/**/**" -> "base"
//	"**/**/**" -> "**"
func stripTrailingContentsSuffixes(glob string) string {
	for strings.HasSuffix(glob, "/**") && !strings.HasSuffix(glob, "/**/") {
		glob = strings.TrimSuffix(glob, "/**")
	}
	return glob
}

// stripTrailingDirDescSuffixes removes all trailing "/**/" groups from a directory-only glob form
// that was converted to the internal representation without trailing '/' (so it ends with "/**").
// Example: pattern for "a/**/**/" becomes "a/**/**" internally; this strips to "a".
func stripTrailingDirDescSuffixes(glob string) string {
	for strings.HasSuffix(glob, "/**") {
		glob = strings.TrimSuffix(glob, "/**")
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

// hasExcludedParent checks if any parent directory is excluded.
// This applies to both files AND directories.
func hasExcludedParent(targetPath string, excludedDirs map[string]bool) bool {
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

	// Snapshot for derived-form classification (before we mutate with dir/root stripping)
	classify := line
	// Remove any leading slash just for classification convenience.
	classifyNoRoot := strings.TrimPrefix(classify, "/")

	// Detect dir-only (trailing /) before we strip it.
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
	if pat.formSandwich {
		// Check if this path ends with the sandwich middle
		if strings.HasSuffix(filePath, pat.sandwichMiddle) {
			// Calculate what comes before the middle part
			prefix := strings.TrimSuffix(filePath, pat.sandwichMiddle)
			// If prefix is empty or ends with /, this IS the directory entry itself
			if prefix == "" || strings.HasSuffix(prefix, "/") {
				// The directory entry itself should NOT match
				return false
			}
		}
		// Otherwise use normal matching for contents
		return matchGlob(pat, filePath)
	}

	// Contents-only patterns (<base>/**) should NOT match the base entry itself,
	// even when multiple trailing "/**" groups are present. We also handle degenerate
	// cases like "**/**" where the stripped base ends with a "**" segment.
	if pat.formContentsOnly {
		base := stripTrailingContentsSuffixes(pat.pattern)

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

	// Non-rooted patterns can match at any level

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

// patternExcludesDirectory determines if a pattern explicitly excludes a directory.
// The parent exclusion rule applies when a pattern matches the directory entry itself.
// Patterns like "foo/*" DO exclude "foo/bar" (a direct child directory of "foo").
// Patterns ending with "/**" match only the contents under a base and must NOT match the base entry.
func patternExcludesDirectory(pat pattern, dirPath string) bool {
	// Handle sandwich patterns specially for directories
	if pat.formSandwich {
		// Check if this path ends with the sandwich middle
		if strings.HasSuffix(dirPath, pat.sandwichMiddle) {
			// Calculate what comes before the middle part
			prefix := strings.TrimSuffix(dirPath, pat.sandwichMiddle)
			// If prefix is empty or ends with /, this IS the directory entry itself
			if prefix == "" || strings.HasSuffix(prefix, "/") {
				// The directory entry itself should NOT be excluded by sandwich patterns
				return false
			}
		}
		// Check if it's a directory inside the sandwich middle
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
			base := stripTrailingDirDescSuffixes(pat.pattern)
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
		return matchesDirectoryPath(pat, dirPath)
	}

	// Non-dirOnly pattern with trailing "/**" is contents-only:
	// it must NOT exclude the base directory entry itself, but it DOES
	// exclude directories strictly below the base. Handle multiple trailing groups
	// and degenerate bases like "**".
	if pat.formContentsOnly {
		base := stripTrailingContentsSuffixes(pat.pattern)
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
