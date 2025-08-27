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
	// Double star with trailing slash (matches any directory recursively).
	doubleStarSlash = "**/"
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
	// Minimum pattern length for suffix detection.
	minPatternLength = 3
	// Minimum trailing slashes to create contents-only pattern.
	minTrailingSlashes = 2
	// Minimum stars to create contents-only pattern.
	minStars = 2
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
	// doubleSlash indicates this pattern originally ended with // (special contents-only semantics)
	doubleSlash bool
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

// newWithRoot creates a GitIgnore instance with a specified root directory.
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
	normalizedPath = strings.TrimPrefix(path.Clean(normalizedPath), g.root)

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
		checkPath = path.Clean(checkPath)
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
func detectGitPatternQuirk(pat pattern, targetPath string) bool {
    // Only applies to patterns that end with one or more "/**+" groups
    if !hasContentsOnlySuffix(pat.pattern) {
        return false
    }

    // Extract and normalize the base once.
    baseRaw := extractPatternBase(pat.pattern)
    base := baseRaw
    // Collapse *** sequences to ** to avoid degenerate cases.
    for strings.Contains(base, "***") {
        base = strings.ReplaceAll(base, "***", "**")
    }

    // If there's no meaningful base, nothing to suppress.
    if base == "" || base == doubleStar {
        return false
    }

    // If the base is of the form <literal> + "**", we let doublestar decide
    // (i.e., we do NOT suppress matching the base).
    if strings.HasSuffix(base, doubleStar) {
        prefix := base[:len(base)-len(doubleStar)]
        if prefix != "" && !strings.ContainsAny(prefix, "*?[/") {
            return false // literal prefix + "**"
        }
    }

    // Suppress ONLY when the target path matches the base pattern as a full path
    // (no basename-only fallback). Example: pattern "a/**" should not match
    // directory "a", but should match "a/x". Likewise, "**/node_modules/**"
    // should not match any ".../node_modules" directory entry itself.
    glob := base
    glob = escapeBracesForGit(glob)
    glob = normalizeCharacterClassBrackets(glob)
    for strings.Contains(glob, "***") {
        glob = strings.ReplaceAll(glob, "***", "**")
    }
    if doublestar.MatchUnvalidated(glob, targetPath) {
        return true
    }

    return false
}

// matchesDoubleSlashPattern handles patterns that originally ended with //
// Only "literal**//" patterns match (exact literal as directory), others never match.
func matchesDoubleSlashPattern(pat pattern, targetPath string, isDir bool) bool {
	// Get the original pattern before transformation
	original := strings.TrimSuffix(pat.original, "//")
	
	// Normalize redundant wildcards in the original pattern (*** -> **)
	for strings.Contains(original, "***") {
		original = strings.ReplaceAll(original, "***", "**")
	}

	// If the original pattern was just a literal (like "dir"), never match
	if !strings.ContainsAny(original, "*?[") {
		return false
	}

	// Handle patterns with specific structures for double slash
	
	// Pattern like "literal**/**" - should match the literal directory
	if strings.Contains(original, "**/**") {
		// Find the literal prefix before the first **
		doublestarPos := strings.Index(original, "**")
		if doublestarPos > 0 {
			literalPrefix := original[:doublestarPos]
			
			// Check if there's additional content after **/**
			doubleslashstarPos := strings.Index(original, "**/**")
			if doubleslashstarPos != -1 {
				afterDoubleSlashStar := original[doubleslashstarPos+5:] // 5 = len("**/**")
				
				// If there's additional content after **/**, this is a more complex pattern
				// like "0**/**0" - the target must contain both prefix and suffix
				if afterDoubleSlashStar != "" {
					return false // Don't match directory directly for patterns with suffixes
				}
			}
			
			// Only match if it's a pure literal prefix, a directory, and exact match
			if !strings.ContainsAny(literalPrefix, "*?[") && isDir && targetPath == literalPrefix {
				return true
			}
		}
	}
	
	// If the original pattern ends with "**" (like "0**", "1**"),
	// extract the literal prefix and only match that exact directory
	if strings.HasSuffix(original, "**") && !strings.Contains(original, "**/**") {
		literalPrefix := strings.TrimSuffix(original, "**")
		// Handle rooted patterns by removing leading "/"
		literalPrefix = strings.TrimPrefix(literalPrefix, "/")
		
		// Only match if it's a pure literal prefix, a directory, and exact match
		if strings.ContainsAny(literalPrefix, "*?[") {
			return false // prefix contains wildcards
		}

		if isDir && targetPath == literalPrefix {
			return true
		}
	}

	return false
}

// extractPatternBase removes one or more trailing "/**+" groups, returning the base.
// Examples:
//
//	"a/**"      -> "a"
//	"a/**/***"  -> "a"
//	"/**"       -> ""   (no base)
func extractPatternBase(pattern string) string {
	for {
		if len(pattern) < minPatternLength {
			return pattern
		}

		idx := len(pattern) - 1
		// count trailing '*'
		starCount := 0

		for idx >= 0 && pattern[idx] == '*' {
			starCount++

			idx--
		}
		// require "/**" tail
		if starCount < 2 || idx < 0 || pattern[idx] != '/' {
			return pattern
		}

		// drop "/**...*" (the slash + all trailing stars)
		pattern = pattern[:idx]
	}
}

// matchesPattern determines if a pattern matches the given path.
func matchesPattern(pat pattern, targetPath string, isDir bool) bool {
	// Directory-only patterns match directories ONLY (not files)
	if pat.dirOnly && !isDir {
		return false
	}

	// Special handling for double slash patterns (originally ended with //)
	if pat.doubleSlash {
		return matchesDoubleSlashPattern(pat, targetPath, isDir)
	}

	// Check for Git quirks first
	if detectGitPatternQuirk(pat, targetPath) {
		return false
	}

	// Use the simple, unified matching
	return matchesSimplePattern(pat, targetPath)
}

// matchesSimplePattern handles core glob pattern matching after Git quirks are processed.
func matchesSimplePattern(pat pattern, targetPath string) bool {
	glob := pat.pattern

	// Determine the target path to match against
	matchPath := targetPath

	if !pat.rooted && !strings.Contains(glob, "/") {
		// Non-rooted patterns without slash match only the basename
		matchPath = path.Base(targetPath)
	}

	// Rooted patterns without slashes should only match single-level paths
	if pat.rooted && !strings.Contains(glob, "/") && strings.Contains(targetPath, "/") {
		return false
	}

	// Let the glob library handle the matching
	return matchGlobPattern(pat, matchPath)
}

// matchGlobPattern performs Git-compatible glob pattern matching using doublestar.
func matchGlobPattern(p pattern, targetPath string) bool {
	// The pattern has already been processed by trimTrailingSpaces,
	// which handles escape sequences for trailing spaces.
	glob := p.pattern
	
	// Git does not support brace expansion, but doublestar does by default.
	// We need to escape unescaped braces to prevent expansion.
	glob = escapeBracesForGit(glob)

	// Normalize first-literal ']' inside character classes to avoid engine differences.
	glob = normalizeCharacterClassBrackets(glob)

	// Normalize redundant wildcards (*** -> **) to match Git's behavior
	for strings.Contains(glob, "***") {
		glob = strings.ReplaceAll(glob, "***", "**")
	}

	// Git handles patterns like "a**/0" by trying both single-level and multi-level matching
	// This is a special case where ** is not preceded by / and not followed by /
	if strings.Contains(glob, "**") {
		hasSpecialDoublestar := false
		
		// Look for ** that's preceded by literal prefix AND appears in path-like context
		for i := 0; i < len(glob)-1; i++ {
			if glob[i] == '*' && glob[i+1] == '*' {
				// Check if it's the special case: literal prefix + path-like pattern
				// Need to exclude patterns where the prefix contains escape sequences
				prefixContainsEscapes := strings.Contains(glob[:i], "\\")
				isLiteralPrefix := i > 0 && !strings.ContainsAny(glob[:i], "*?[/") && !prefixContainsEscapes
				if !isLiteralPrefix {
					continue
				}
				
				// Check if this appears in path-like context:
				// 1. Pattern ends with ** (like "prefix**")
				// 2. Pattern has **/ somewhere (like "prefix**/suffix") 
				// 3. Pattern has **/* (like "prefix**/*")
				isPathLike := false
				if i+2 >= len(glob) {
					isPathLike = true // ends with **
				} else if i+2 < len(glob) && glob[i+2] == '/' {
					isPathLike = true // has **/ 
				}
				
				if isPathLike {
					hasSpecialDoublestar = true
					break
				}
			}
		}
		
		if hasSpecialDoublestar {
			// Git's handling of patterns like "a**/suffix":
			// 1. Try zero-width match: "a**/0" -> "a0" 
			// 2. Try single-level expansion: "a**/0" -> "a*/0" 
			// 3. Try multi-level expansion: "a**/0" -> "a*/**/0" 
			
			// Find the ** position
			doublestarPos := strings.Index(glob, "**")
			if doublestarPos == -1 {
				return false
			}
			
			prefix := glob[:doublestarPos]
			suffix := glob[doublestarPos+2:]
			
			// Variant 1: Zero-width match (** matches nothing)
			// For pattern "a**/0", this becomes "a0" (remove ** and leading slash from suffix)
			// For complex patterns like "0**/**/*0", we need to collapse the suffix pattern
			zeroWidthSuffix := strings.TrimPrefix(suffix, "/")
			// Handle nested doublestar patterns in suffix by collapsing /**/* to *
			if strings.HasPrefix(zeroWidthSuffix, "**/*") {
				zeroWidthSuffix = strings.TrimPrefix(zeroWidthSuffix, "**/*")
				zeroWidthSuffix = "*" + zeroWidthSuffix
			}
			zeroWidthPattern := prefix + zeroWidthSuffix
			if doublestar.MatchUnvalidated(zeroWidthPattern, targetPath) {
				return true
			}
			
			// Special case: if suffix is exactly "/**/*" (no additional content),
			// the target itself might match. Git treats patterns like "0**/**/*" as matching "0" itself.
			// But patterns like "0**/**/*0" require additional content and should NOT match just "0".
			if suffix == "/**/*" {
				// Try matching the target as if it's the final component
				// For "0**/**/*" matching "0", we check if "0**" matches "0"
				prefixPattern := prefix + "**"
				if doublestar.MatchUnvalidated(prefixPattern, targetPath) {
					return true
				}
			}
			
			// Variant 2: Single-level match (** becomes one path segment)  
			if suffix != "" {
				singleLevelPattern := prefix + "*" + suffix
				if doublestar.MatchUnvalidated(singleLevelPattern, targetPath) {
					return true
				}
			} else {
				// Pattern ends with ** (like "a**"), try single-level expansion
				singleLevelPattern := prefix + "*"
				if doublestar.MatchUnvalidated(singleLevelPattern, targetPath) {
					return true
				}
			}
			
			// Variant 3: Multi-level match 
			if suffix != "" {
				// For patterns like "a**/0", we need "a*/**/0" to match "a1/x/0"
				// This allows the first * to match the continuation of the prefix (like "a1")
				// and the ** to match the remaining path segments
				multiLevelPattern := prefix + "*/**" + suffix
				if doublestar.MatchUnvalidated(multiLevelPattern, targetPath) {
					return true
				}
			} else {
				// Pattern ends with ** (like "a**"), try multi-level expansion
				multiLevelPattern := prefix + "**"
				if doublestar.MatchUnvalidated(multiLevelPattern, targetPath) {
					return true
				}
			}
			
			return false
		}
	}

	// Git treats '?' as byte-based matching, not Unicode character matching
	if strings.Contains(glob, "?") {
		return matchPathAwareByteBasedPattern(glob, targetPath)
	}

	// in matchGlobPattern (near the end)
	matched := doublestar.MatchUnvalidated(glob, targetPath)
	if matched {
		return true
	}

	// Git quirk: "**/" may swallow the slash (context-aware).
	// Try all context-correct variants.
	if strings.Contains(glob, doubleStarSlash) {
		for _, alt := range expandGlobstarSlashOptions(glob) {
			if doublestar.MatchUnvalidated(alt, targetPath) {
				return true
			}
		}
	}

	return false
}

// matchPathAwareByteBasedPattern performs byte-based pattern matching that respects path boundaries.
// Git treats '?' as matching exactly one byte, not one Unicode character.
func matchPathAwareByteBasedPattern(pattern, targetPath string) bool {
	// Split pattern and target into path components
	patternParts := strings.Split(pattern, "/")
	targetParts := strings.Split(targetPath, "/")
	
	// For simple patterns (no slash), use direct byte matching
	if len(patternParts) == 1 {
		return matchByteBasedPattern(pattern, targetPath)
	}
	
	// For complex path patterns, check if they can match
	// Using doublestar's path logic but with byte-based ? matching per component
	return matchPathComponentsWithByteBasedQuestions(patternParts, targetParts)
}

// matchPathComponentsWithByteBasedQuestions matches path components using byte-based ? semantics
func matchPathComponentsWithByteBasedQuestions(patternParts, targetParts []string) bool {
	// Check if pattern contains ** - if so, use a hybrid approach
	for _, part := range patternParts {
		if strings.Contains(part, "**") || part == "**" {
			// For patterns with **, use doublestar but accept the Unicode limitation
			// This is a compromise - ** patterns are complex and rare with ?
			pattern := strings.Join(patternParts, "/")
			target := strings.Join(targetParts, "/")
			return matchByteBasedPatternWithDoublestarFallback(pattern, target)
		}
	}
	
	// Simple path pattern without ** - match component by component
	if len(patternParts) != len(targetParts) {
		return false
	}
	
	// Same number of components - match each one byte-wise
	for i := range patternParts {
		if !matchByteBasedPattern(patternParts[i], targetParts[i]) {
			return false
		}
	}
	
	return true
}

// matchByteBasedPatternWithDoublestarFallback handles complex patterns with ** by using doublestar
// This is a compromise for complex cases where pure byte-based matching is difficult
func matchByteBasedPatternWithDoublestarFallback(pattern, target string) bool {
	// For ** patterns, we'll use doublestar matching
	// This means some Unicode edge cases with ? might not match Git exactly,
	// but it's better than not matching ** patterns at all
	return doublestar.MatchUnvalidated(pattern, target)
}

// matchByteBasedPattern performs byte-based pattern matching for patterns containing '?'.
// Git treats '?' as matching exactly one byte, not one Unicode character.
func matchByteBasedPattern(pattern, targetPath string) bool {
	// Convert both pattern and target to byte slices for byte-based matching
	patternBytes := []byte(pattern)
	targetBytes := []byte(targetPath)

	return matchBytesRecursive(patternBytes, targetBytes, 0, 0)
}

// matchBytesRecursive recursively matches pattern bytes against target bytes.
//
//nolint:gocognit	// Function is complex by design.
func matchBytesRecursive(pattern, target []byte, patternPos, targetPos int) bool {
	// End of pattern reached
	if patternPos >= len(pattern) {
		return targetPos >= len(target) // Match if target is also exhausted
	}

	// End of target reached but pattern remains
	if targetPos >= len(target) {
		// Only match if remaining pattern is all '*' (unescaped wildcards)
		for idx := patternPos; idx < len(pattern); idx++ {
			// Handle escaped characters
			if pattern[idx] == '\\' && idx+1 < len(pattern) {
				// Escaped characters are literals that must be matched
				// Since target is exhausted, we cannot match any literal
				return false
			}

			if pattern[idx] != '*' {
				return false
			}
		}

		return true
	}

	// Handle escaped characters
	if pattern[patternPos] == '\\' && patternPos+1 < len(pattern) {
		// Escaped character - treat next character as literal
		if patternPos+1 < len(pattern) && pattern[patternPos+1] == target[targetPos] {
			//nolint:mnd		// It is clear from the context what the numbers are
			return matchBytesRecursive(pattern, target, patternPos+2, targetPos+1)
		}

		return false
	}

	switch pattern[patternPos] {
	case '?':
		// '?' matches exactly one byte
		return matchBytesRecursive(pattern, target, patternPos+1, targetPos+1)
	case '*':
		// Handle consecutive '*' as single '*'
		for patternPos < len(pattern) && pattern[patternPos] == '*' {
			patternPos++
		}
		// '*' can match zero or more bytes
		for i := targetPos; i <= len(target); i++ {
			if matchBytesRecursive(pattern, target, patternPos, i) {
				return true
			}
		}

		return false
	case '[':
		// Character class - need to find the closing ']' and match
		return matchCharacterClass(pattern, target, patternPos, targetPos)
	default:
		// Literal character - must match exactly
		if pattern[patternPos] == target[targetPos] {
			return matchBytesRecursive(pattern, target, patternPos+1, targetPos+1)
		}

		return false
	}
}

// matchCharacterClass handles character class matching like [abc] or [!def].
//
//nolint:gocognit	// Function is complex by design.
func matchCharacterClass(pattern, target []byte, patternPos, targetPos int) bool {
	// Find the closing ']'
	classEnd := patternPos + 1
	for classEnd < len(pattern) && pattern[classEnd] != ']' {
		classEnd++
	}

	if classEnd >= len(pattern) {
		// Invalid character class, treat '[' as literal
		if pattern[patternPos] == target[targetPos] {
			return matchBytesRecursive(pattern, target, patternPos+1, targetPos+1)
		}

		return false
	}

	// Extract character class content
	classContent := pattern[patternPos+1 : classEnd]

	negated := len(classContent) > 0 && (classContent[0] == '!' || classContent[0] == '^')
	if negated {
		classContent = classContent[1:]
	}

	// Check if target byte matches any in the class
	targetByte := target[targetPos]
	matched := false

	// Handle character ranges like [0-9] or [a-z]
	for idx := range classContent {
		if idx+2 < len(classContent) && classContent[idx+1] == '-' {
			// Range: check if target is between start and end
			start := classContent[idx]

			end := classContent[idx+2]
			if targetByte >= start && targetByte <= end {
				matched = true

				break
			}
		} else {
			// Single character
			if classContent[idx] == targetByte {
				matched = true

				break
			}
		}
	}

	// Apply negation if needed
	if negated {
		matched = !matched
	}

	if matched {
		return matchBytesRecursive(pattern, target, classEnd+1, targetPos+1)
	}

	return false
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

// processEscapeSequences processes Git escape sequences and converts them for doublestar.
// Git rules: \\ → literal \, \c → literal c
// Need to properly escape the results for doublestar glob matching.
func processEscapeSequences(pattern string) string {
	if pattern == "" || !strings.Contains(pattern, "\\") {
		return pattern
	}


	// Process Git escapes character by character
	var result strings.Builder
	result.Grow(len(pattern))

	for i := 0; i < len(pattern); i++ {
		if pattern[i] == '\\' && i+1 < len(pattern) {
			nextChar := pattern[i+1]
			
			if nextChar == '\\' {
				// \\ becomes literal backslash - need to escape for doublestar
				result.WriteString("\\\\")
			} else {
				// \c becomes literal c - escape if it's a special character
				switch nextChar {
				case '*':
					// \* becomes literal * - escape for doublestar, but check for following wildcard
					result.WriteString("\\*")
				case '?', '[', ']', '{', '}':
					// Escape special characters to make them literal
					result.WriteByte('\\')
					result.WriteByte(nextChar)
				default:
					// Regular characters stay as-is
					result.WriteByte(nextChar)
				}
			}
			i++ // Skip the escaped character
		} else {
			// Regular characters (including unescaped wildcards) pass through
			result.WriteByte(pattern[i])
		}
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

	// Check if pattern is rooted (starts with /) BEFORE processing escapes
	// This ensures that escaped slashes (\/B) don't get treated as rooted patterns
	isRooted := false
	if strings.HasPrefix(line, "/") && !strings.HasPrefix(line, "\\/") {
		isRooted = true
		line = strings.TrimPrefix(line, "/")
	}
	
	// Check if pattern has escaped trailing slash BEFORE processing escapes
	// This ensures that escaped trailing slashes (0\/) don't get treated as directory-only
	hasEscapedTrailingSlash := strings.HasSuffix(line, "\\/")

	// Process escape sequences
	line = processEscapeSequences(line)

	// Empty pattern after trimming
	if len(line) == 0 {
		return nil
	}

	// --- Trailing slash normalization ---
	// Git treats a trailing "/" as "directory-only".
	// We additionally normalize TWO OR MORE trailing "/" to a contents-only suffix "/**"
	// so that patterns like "base//" behave like "base/**" (match inside, not the base).
	//
	// This also avoids over-matching caused by doublestar tolerating a trailing "/"
	// against a segment without an explicit slash.
	//
	// Examples:
	//   "abc/"   -> dirOnly=true, pattern "abc"
	//   "abc//"  -> dirOnly=false, pattern "abc/**"   (contents-only)
	//   "*0**//" -> dirOnly=false, pattern "*0**/**"  (contents-only)
	if strings.HasSuffix(line, "/") && !hasEscapedTrailingSlash {
		// Count consecutive trailing slashes
		i := len(line) - 1
		for i >= 0 && line[i] == '/' {
			i--
		}

		trailingSlashes := len(line) - 1 - i

		if trailingSlashes >= minTrailingSlashes {
			// Contents-only: drop all trailing slashes, then append "/**"
			base := strings.TrimRight(line, "/")
			if base == "" {
				// A pattern of only slashes is a no-op
				return nil
			}

			line = base + "/**"
			// Mark this as a double slash pattern for special handling
			pat.doubleSlash = true
			// IMPORTANT: contents-only matches files and dirs under the base,
			// so do NOT set dirOnly here.
		} else {
			// Exactly one trailing slash: directory-only
			pat.dirOnly = true
			line = strings.TrimSuffix(line, "/")
		}
	} else if hasEscapedTrailingSlash {
		// For escaped trailing slash, don't remove the slash and don't set dirOnly
		// The escaped slash becomes a literal slash character after escape processing
		// The pattern will look for literal "pattern/" in the path
	}

	// Set rooted flag from earlier detection
	pat.rooted = isRooted

	// Handle edge case: if pattern becomes empty after trimming "/" (i.e., the original was just "/")
	// This should be treated as a no-op pattern
	if line == "" {
		return nil
	}

	pat.pattern = line

	return pat
}

// expandGlobstarSlashOptions returns variants where each "**/" may be kept
// or (sometimes) dropped. Dropping deletes the whole token, but ONLY when
// the segment immediately to the left (since the previous '/') contains
// no wildcard meta (*, ?, [). This preserves component-aware semantics
// and avoids turning cross-component patterns into substring matches.
//
// Examples:
//
//	"**/name" -> ["**/name", "name"]
//	"a**/0"   -> ["a**/0", "a0"]
//	"0**/*"   -> ["0**/*", "0*"]
//	"*0**/*"  -> ["*0**/*"]   // left segment "*0" has meta => no drop
func expandGlobstarSlashOptions(glob string) []string {
	const (
		token    = doubleStarSlash // "**/"
		tokenLen = len(doubleStarSlash)
	)

	hasMeta := func(s string) bool {
		return strings.ContainsAny(s, "*?[")
	}

	// Find all occurrences and whether dropping is allowed at each.
	type occurrence struct {
		pos     int
		canDrop bool
	}

	var occs []occurrence

	for start := 0; ; {
		rel := strings.Index(glob[start:], token)
		if rel < 0 {
			break
		}

		pos := start + rel

		// Left segment = since previous '/' (or start) up to token.
		leftStart := strings.LastIndexByte(glob[:pos], '/')
		if leftStart < 0 {
			leftStart = 0
		} else {
			leftStart++ // char after '/'
		}

		leftSeg := glob[leftStart:pos]

		// Allow drop only if left segment has no meta.
		canDrop := !hasMeta(leftSeg)

		occs = append(occs, occurrence{pos: pos, canDrop: canDrop})
		start = pos + tokenLen
	}

	if len(occs) == 0 {
		return []string{glob}
	}

	// Build variants: keep always; drop only when allowed.
	var out []string

	var build func(idx, prev int, builder *strings.Builder)

	build = func(idx, prev int, builder *strings.Builder) {
		if idx == len(occs) {
			builder.WriteString(glob[prev:])

			out = append(out, builder.String())

			return
		}

		occ := occs[idx]

		// keep branch
		{
			var keep strings.Builder

			if builder != nil {
				keep.Grow(builder.Len() + (occ.pos - prev) + tokenLen + smallBufferGrowth)
				keep.WriteString(builder.String())
			}

			keep.WriteString(glob[prev:occ.pos])
			keep.WriteString(token)
			build(idx+1, occ.pos+tokenLen, &keep)
		}

		// drop branch (only if allowed)
		if occ.canDrop {
			var drop strings.Builder

			if builder != nil {
				drop.Grow(builder.Len() + (occ.pos - prev) + smallBufferGrowth)
				drop.WriteString(builder.String())
			}

			drop.WriteString(glob[prev:occ.pos])
			// dropping writes nothing
			build(idx+1, occ.pos+tokenLen, &drop)
		}
	}

	build(0, 0, &strings.Builder{})

	return out
}

// hasContentsOnlySuffix reports whether p ends with "/" followed by
// at least two '*' characters (e.g., "/**", "/***", ...).
func hasContentsOnlySuffix(pattern string) bool {
	if len(pattern) < minPatternLength {
		return false
	}

	idx := len(pattern) - 1

	// count trailing '*'
	starCount := 0

	for idx >= 0 && pattern[idx] == '*' {
		starCount++

		idx--
	}

	if starCount < minStars {
		return false
	}

	// the char before the stars must be '/'
	return idx >= 0 && pattern[idx] == '/'
}
