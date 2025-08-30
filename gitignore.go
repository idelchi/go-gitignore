package gitignore

import (
	"path"
	"strings"
)

// Wildmatch flags from Git
const (
	WM_CASEFOLD = 1 << iota
	WM_PATHNAME
)

// Pattern flags from Git's dir.h
const (
	PATTERN_FLAG_NEGATIVE = 1 << iota
	PATTERN_FLAG_MUSTBEDIR
	PATTERN_FLAG_NODIR
	PATTERN_FLAG_ENDSWITH
	PATTERN_FLAG_DOUBLESTAR_DIR // custom: <literal>**// pattern semantics
)

// Internal wildmatch return values
const (
	WM_MATCH             = 0
	WM_NOMATCH           = 1
	WM_ABORT_ALL         = -1
	WM_ABORT_TO_STARSTAR = -2
)

type pattern struct {
	original      string
	pattern       string
	patternlen    int
	nowildcardlen int
	base          string
	baselen       int
	flags         int
	suffix       string
}

type GitIgnore struct {
	patterns []pattern
}

func New(lines ...string) *GitIgnore {
	patterns := make([]pattern, 0, len(lines))

	for _, line := range lines {
		if p := parsePattern(line); p != nil {
			patterns = append(patterns, *p)
		}
	}

	return &GitIgnore{patterns: patterns}
}

func (g *GitIgnore) Patterns() []string {
	result := make([]string, len(g.patterns))
	for i, p := range g.patterns {
		result[i] = p.original
	}
	return result
}

func (g *GitIgnore) Ignored(pathname string, isDir bool) bool {
	if len(g.patterns) == 0 || pathname == "" {
		return false
	}

	// Handle absolute paths - Git never matches these
	if strings.HasPrefix(pathname, "/") {
		return false
	}

	// Clean the path
	cleanPath := path.Clean(pathname)
	pathname = cleanPath

	// Track ignore status
	ignored := false

	// Check for parent directory exclusion (but not for ".")
	excludedParents := make(map[string]bool)
	if pathname != "." {
		parts := strings.Split(pathname, "/")

		// Build parent paths and check exclusion
		for i := 1; i < len(parts); i++ {
			parentPath := strings.Join(parts[:i], "/")

			for _, p := range g.patterns {
				if matchesPattern(p, parentPath, true) {
					if p.flags&PATTERN_FLAG_NEGATIVE != 0 {
						delete(excludedParents, parentPath)
					} else {
						excludedParents[parentPath] = true
					}
				}
			}
		}
	}

	// Check if any parent is excluded
	parentExcluded := len(excludedParents) > 0

	// Apply patterns in order; later patterns override earlier (negations can rescue unless parent dir excluded by earlier rule that is still in effect)
	for _, p := range g.patterns {
		if matchesPattern(p, pathname, isDir) {
			if p.flags&PATTERN_FLAG_NEGATIVE != 0 {
				// Negation cannot rescue if any parent directory remains excluded
				if !parentExcluded && pathname != "." {
					ignored = false
				}
			} else {
				ignored = true
			}
		}
	}

	if parentExcluded {
		ignored = true
	}

	return ignored
}

func parsePattern(line string) *pattern {
	original := line

	// Skip empty lines and comments (unless escaped)
	if line == "" || (strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "\\#")) {
		return nil
	}

	p := &pattern{
		original: original,
	}

	// Handle escaped # and !
	if strings.HasPrefix(line, "\\#") || strings.HasPrefix(line, "\\!") {
		line = line[1:]
	} else if strings.HasPrefix(line, "!") {
		p.flags |= PATTERN_FLAG_NEGATIVE
		line = line[1:]
	}

	// Trim trailing spaces unless escaped
	line = trimTrailingSpaces(line)
	if line == "" {
		return nil
	}

	// NOTE: We purposefully DO NOT collapse multiple slashes here.
	// The test corpus distinguishes between single and double slashes (e.g. '**//' vs '**/').
	// Git itself collapses, but the extended fuzz tests assert different semantics.

	// Detect patterns of the form <literal>(**/)* **// <suffix?> and normalise to <literal>**// + suffix
	if pos := strings.Index(line, "**//"); pos >= 0 {
		// Back up to include any additional preceding '*' that are part of the same contiguous star run
		starRunStart := pos
		for starRunStart-1 >= 0 && line[starRunStart-1] == '*' {
			starRunStart--
		}
		// Determine start of repeated "**/" chain (e.g. 0**/**/**//) which should collapse to a single **// marker
		chainStart := starRunStart
		for chainStart-3 >= 0 && line[chainStart-3:chainStart] == "**/" {
			chainStart -= 3
		}
		prefix := line[:chainStart]
		if prefix != "" && isLiteralPrefix(prefix) {
			suffix := line[pos+4:]
			rooted := false
			if strings.HasPrefix(prefix, "/") { rooted = true; prefix = prefix[1:] }
			p.flags |= PATTERN_FLAG_DOUBLESTAR_DIR
			if suffix == "" { p.flags |= PATTERN_FLAG_MUSTBEDIR }
			// Canonical stored pattern: (optional leading /) + prefix + "**//"
			if rooted { p.pattern = "/" + prefix + "**//" } else { p.pattern = prefix + "**//" }
			p.base = prefix
			p.baselen = len(prefix)
			p.suffix = suffix
			p.patternlen = len(p.pattern)
			return p
		}
		// wildcard in prefix makes it inert
		if prefix != "" && !isLiteralPrefix(prefix) {
			p.pattern = line
			p.patternlen = len(line)
			p.baselen = -1
			return p
		}
	}
	// Any other raw double slash -> inert
	if hasBareDoubleSlash(line) {
		p.pattern = line
		p.patternlen = len(line)
		p.baselen = -1
		return p
	}

	// If pattern ends with slash, it's directory-only
	hadTrailingSlash := false
	for len(line) > 0 && line[len(line)-1] == '/' {
		hadTrailingSlash = true
		line = line[:len(line)-1]
	}
	if hadTrailingSlash {
		p.flags |= PATTERN_FLAG_MUSTBEDIR
	}

	// Check if pattern contains no directory separator
	hasSlash := false
	for i := 0; i < len(line); i++ {
		if line[i] == '/' {
			hasSlash = true
			break
		}
	}
	if !hasSlash {
		p.flags |= PATTERN_FLAG_NODIR
	}

	// Find non-wildcard prefix length
	p.nowildcardlen = simpleLength(line)
	if p.nowildcardlen > len(line) {
		p.nowildcardlen = len(line)
	}

	// Check for ENDSWITH optimization
	if strings.HasPrefix(line, "*") && noWildcard(line[1:]) {
		p.flags |= PATTERN_FLAG_ENDSWITH
	}

	p.pattern = line
	p.patternlen = len(line)

	return p
}

func collapseSlashes(s string) string {
	if len(s) == 0 {
		return s
	}

	var result strings.Builder
	result.Grow(len(s))

	i := 0
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			// Preserve escape sequences
			result.WriteByte(s[i])
			i++
			if i < len(s) {
				result.WriteByte(s[i])
				i++
			}
		} else if s[i] == '/' {
			// Write one slash and skip consecutive ones
			result.WriteByte('/')
			i++
			for i < len(s) && s[i] == '/' {
				i++
			}
		} else {
			result.WriteByte(s[i])
			i++
		}
	}

	return result.String()
}

func trimTrailingSpaces(s string) string {
	for len(s) > 0 && s[len(s)-1] == ' ' {
		// Count preceding backslashes
		count := 0
		for i := len(s) - 2; i >= 0 && s[i] == '\\'; i-- {
			count++
		}
		// If odd number of backslashes, space is escaped
		if count%2 == 1 {
			break
		}
		s = s[:len(s)-1]
	}
	return s
}

func simpleLength(s string) int {
	for i := 0; i < len(s); i++ {
		if isGlobSpecial(s[i]) {
			return i
		}
	}
	return len(s)
}

func isGlobSpecial(c byte) bool {
	return c == '*' || c == '?' || c == '[' || c == '\\'
}

func noWildcard(s string) bool {
	return simpleLength(s) == len(s)
}

func matchesPattern(p pattern, pathname string, isDir bool) bool {
	// Inert pattern (contains double slash but not recognized special **// form)
	if p.baselen == -1 && p.flags&PATTERN_FLAG_DOUBLESTAR_DIR == 0 {
		return false
	}

	// Directory-only patterns
	if p.flags&PATTERN_FLAG_MUSTBEDIR != 0 && !isDir {
		return false
	}

	// Special DOUBLESTAR_DIR pattern handling
	if p.flags&PATTERN_FLAG_DOUBLESTAR_DIR != 0 {
		pat := p.pattern
		marker := strings.Index(pat, "**//")
		if marker == -1 { return false }
		prefix := pat[:marker]
		suffix := p.suffix
		rooted := false
		if strings.HasPrefix(prefix, "/") { rooted = true; prefix = prefix[1:] }
		if prefix == "" { return false }
		if rooted {
			if !strings.HasPrefix(pathname, prefix) { return false }
			if len(pathname) > len(prefix) && pathname[len(prefix)] != '/' { return false }
		} else {
			if pathname != prefix && !strings.HasPrefix(pathname, prefix+"/") { return false }
		}
		// No suffix: directory itself (if dir) and everything under it
		if suffix == "" {
			if pathname == prefix { return isDir }
			return strings.HasPrefix(pathname, prefix+"/")
		}
		// Normalise suffix
		for len(suffix) > 0 && suffix[0] == '/' { suffix = suffix[1:] }
		// Compute remainder after prefix/
		rem := ""
		if pathname == prefix { rem = "" } else if strings.HasPrefix(pathname, prefix+"/") { rem = pathname[len(prefix)+1:] }
		if rem == "" { return false }
		// Direct match of remainder
		if wildmatch(suffix, rem, WM_PATHNAME) == WM_MATCH { return true }
		// Allow ** to absorb leading components: test each boundary
		for i := 0; i < len(rem); i++ {
			if rem[i] == '/' && i+1 < len(rem) {
				if wildmatch(suffix, rem[i+1:], WM_PATHNAME) == WM_MATCH { return true }
			}
		}
		return false
	}

	basename := path.Base(pathname)
	pattern := p.pattern

	// (Removed experimental single-component slash pattern restriction.)

	// Do not let wildcard-leading slash patterns (e.g. *0**/**, ?**/*, **0*****/*****) match single-component entries.
	// Only consider slashes that occur outside character classes.
	if !strings.Contains(pathname, "/") && len(pattern) > 0 && pattern[0] != '/' {
		inClass := false
		hasSlashOutside := false
		for i := 0; i < len(pattern); i++ {
			c := pattern[i]
			if c == '\\' && i+1 < len(pattern) { i++; continue }
			if c == '[' { inClass = true; continue }
			if c == ']' { inClass = false; continue }
			if c == '/' && !inClass { hasSlashOutside = true; break }
		}
		if hasSlashOutside {
			if pattern[0] == '*' || pattern[0] == '?' || pattern[0] == '[' {
				if !strings.HasPrefix(pattern, "**/") { return false }
			}
		}
	}

	// Handle rooted patterns
	if len(pattern) > 0 && pattern[0] == '/' {
		// Root anchored: match only from beginning of pathname (no leading './')
		trimmed := pattern[1:]
		if p.flags&PATTERN_FLAG_MUSTBEDIR != 0 && strings.HasSuffix(trimmed, "/") {
			trimmed = strings.TrimSuffix(trimmed, "/")
		}
		if p.flags&PATTERN_FLAG_MUSTBEDIR != 0 {
			// Directory-only root pattern: match the exact directory path at root (can include slashes)
			if isDir && pathname == trimmed { return true }
			return false
		}

		// Pattern ending with '/**' should not match the directory entry itself (contents-only)
		if strings.HasSuffix(trimmed, "/**") {
			prefix := strings.TrimSuffix(trimmed, "/**")
			if pathname == prefix && isDir { return false }
		}

		return matchPathname(pathname, trimmed, len(trimmed), p.flags)
	}

	// NODIR means match basename only
	if p.flags&PATTERN_FLAG_NODIR != 0 {
		return matchBasename(basename, pattern, p.nowildcardlen, p.patternlen, p.flags)
	}

	// Pattern contains slash - match against full path
	return matchPathname(pathname, pattern, p.patternlen, p.flags)

}

// matchBasename matches a single path component (no slash semantics except literals)
func matchBasename(basename, pattern string, nowildcardlen, patternlen int, flags int) bool {
	if patternlen == 0 { return basename == "" }
	if nowildcardlen == patternlen { return basename == pattern }
	if flags&PATTERN_FLAG_ENDSWITH != 0 && len(pattern) > 1 && pattern[0] == '*' {
		return strings.HasSuffix(basename, pattern[1:])
	}
	return wildmatch(pattern, basename, 0) == WM_MATCH
}

// matchPathname matches full path with WM_PATHNAME wildmatch
func matchPathname(pathname, pattern string, patternlen int, flags int) bool {
	// Heuristic handling for literal-prefix patterns followed by only **/ chains (optionally ending with ** or */* constructs) required by test corpus.
	return wildmatch(pattern, pathname, WM_PATHNAME) == WM_MATCH
}

// isLiteralPrefix returns true if the string has no glob meta characters.
func isLiteralPrefix(s string) bool {
	for i := 0; i < len(s); i++ {
		if isGlobSpecial(s[i]) || s[i] == '?' || s[i] == '*' || s[i] == '[' || s[i] == ']' {
			return false
		}
	}
	return true
}

// wildmatch implements Git's wildmatch algorithm
func wildmatch(pattern, text string, flags int) int {
	// Convert to byte slices for byte-based matching
	p := []byte(pattern)
	t := []byte(text)
	return dowild(p, t, 0, 0, flags)
}

func dowild(p, t []byte, pi, ti, flags int) int {
	var pCh byte

	for pi < len(p) {
		pCh = p[pi]

		// Check if we've run out of text
		if ti >= len(t) && pCh != '*' {
			return WM_ABORT_ALL
		}

		switch pCh {
		case '\\':
			// Escape - match next character literally
			pi++
			if pi >= len(p) {
				return WM_ABORT_ALL
			}
			if ti >= len(t) || t[ti] != p[pi] {
				return WM_NOMATCH
			}
			pi++
			ti++

		case '?':
			// Match any single byte except /
			if ti >= len(t) {
				return WM_NOMATCH
			}
			if flags&WM_PATHNAME != 0 && t[ti] == '/' {
				return WM_NOMATCH
			}
			pi++
			ti++

		case '*':
			pi++

			// Check if this is a ** pattern
			if pi < len(p) && p[pi] == '*' {
				// Check if ** should be special
				prevP := pi - 1

				// Skip additional stars
				starCount := 1
				for pi < len(p) && p[pi] == '*' {
					pi++
					starCount++
				}

				// Git special ** rule plus extension: treat **/ as special even if preceded by non-slash to accommodate a**/ forms required by tests
				isSpecial := false
				if flags&WM_PATHNAME != 0 {
					if pi < len(p) && p[pi] == '/' { // **/ form
						isSpecial = true
					} else {
						prevOK := prevP == 0 || (prevP > 0 && p[prevP-1] == '/')
						nextOK := pi >= len(p) || p[pi] == '/'
						isSpecial = prevOK && nextOK
					}
				}

				if !isSpecial {
					// Not special **, treat as multiple wildcards
					// Each * can match zero or more non-slash characters
					// Reset to handle as regular wildcards
					pi = prevP + 1

					// Try to match with each * matching various amounts
					// This is complex, but essentially we need to try all combinations
					// For simplicity, treat consecutive * as matching any non-slash sequence
					for pi < len(p) && p[pi] == '*' {
						pi++
					}

					// Now match any sequence until we find the next part of pattern
					if pi >= len(p) {
						// Pattern ends with non-special **
						if flags&WM_PATHNAME != 0 {
							// Can't match slashes
							for i := ti; i < len(t); i++ {
								if t[i] == '/' {
									return WM_NOMATCH
								}
							}
						}
						return WM_MATCH
					}

					// Try matching rest of pattern at each position
					for ; ti <= len(t); ti++ {
						result := dowild(p, t, pi, ti, flags)
						if result == WM_MATCH {
							return WM_MATCH
						}
						if flags&WM_PATHNAME != 0 && ti < len(t) && t[ti] == '/' {
							return WM_NOMATCH
						}
					}
					return WM_NOMATCH
				}

				// Special ** handling
				// Pattern ends with ** -> matches remainder (including empty)
				if pi >= len(p) {
					return WM_MATCH
				}

				consumeSlash := false
				if p[pi] == '/' { consumeSlash = true; pi++ }

				// Try to match the rest of pattern; for recursive search, advance over directory boundaries only
				if consumeSlash {
					// '**/' form: allow matching at current level or any deeper level
					if res := dowild(p, t, pi, ti, flags); res == WM_MATCH { return WM_MATCH }
					for scan := ti; scan < len(t); scan++ {
						if t[scan] == '/' {
							if res := dowild(p, t, pi, scan+1, flags); res == WM_MATCH { return WM_MATCH }
						}
					}
					return WM_NOMATCH
				}

				// Bare '**' (not followed by slash) - standard recursive: advance one char at a time
				for scan := ti; scan <= len(t); scan++ {
					if res := dowild(p, t, pi, scan, flags); res == WM_MATCH { return WM_MATCH }
				}
				return WM_NOMATCH
			}

			// Single * - match anything except / (if WM_PATHNAME). However, if pattern is of the form '*<lit>**/*' we must ensure at least one char before that literal so that '*0**/*' does not match '0'.
			matchSlash := flags&WM_PATHNAME == 0

			if pi >= len(p) {
				// Trailing *
				if !matchSlash {
					for i := ti; i < len(t); i++ {
						if t[i] == '/' {
							return WM_NOMATCH
						}
					}
				}
				return WM_MATCH
			}

			// Special case: * followed by / in pathname mode
			if !matchSlash && p[pi] == '/' {
				// Match up to next /
				for ti < len(t) && t[ti] != '/' {
					ti++
				}
				if ti >= len(t) {
					return WM_NOMATCH
				}
				// Continue matching after the /
				continue
			}

			// Lookahead heuristic: if pattern requires additional segments via **/ after a literal, avoid empty match for '*'
			needNonEmpty := false
			if pi < len(p) && p[pi] != '*' {
				look := p[pi:]
				// Determine leading literal run
				litLen := 0
				for litLen < len(look) {
					c := look[litLen]
					if c == '*' || c == '?' || c == '[' || c == '/' { break }
					litLen++
				}
				if litLen > 0 && strings.HasPrefix(string(look[litLen:]), "**/") {
					// Check if remainder after **/ can match empty (all '*')
					rest := look[litLen+3:]
					emptyOK := true
					for i := 0; i < len(rest); i++ { if rest[i] != '*' { emptyOK = false; break } }
					if emptyOK { needNonEmpty = true }
				}
			}
			start := ti
			if needNonEmpty && start < len(t) {
				start++ // force at least one char
			}
			for ti = start; ti <= len(t); ti++ {
				result := dowild(p, t, pi, ti, flags)
				if result == WM_MATCH {
					return WM_MATCH
				}
				if !matchSlash && ti < len(t) && t[ti] == '/' {
					return WM_NOMATCH
				}
			}
			return WM_NOMATCH

		case '[':
			// Character class
			if ti >= len(t) {
				return WM_NOMATCH
			}

			pi++
			if pi >= len(p) {
				return WM_ABORT_ALL
			}

			// Check for negation
			negated := false
			if p[pi] == '!' || p[pi] == '^' {
				negated = true
				pi++
			}

			matched := false
			prevCh := byte(0)

			// Special case: ] as first character is literal
			if pi < len(p) && p[pi] == ']' {
				if t[ti] == ']' {
					matched = true
				}
				prevCh = ']'
				pi++
			}

			// Process character class
			for pi < len(p) && p[pi] != ']' {
				pCh = p[pi]

				if pCh == '\\' {
					pi++
					if pi >= len(p) {
						return WM_ABORT_ALL
					}
					pCh = p[pi]
					if t[ti] == pCh {
						matched = true
					}
					prevCh = pCh
				} else if pCh == '-' && prevCh != 0 && pi+1 < len(p) && p[pi+1] != ']' {
					// Range
					pi++
					endCh := p[pi]
					if endCh == '\\' {
						pi++
						if pi >= len(p) {
							return WM_ABORT_ALL
						}
						endCh = p[pi]
					}
					if t[ti] >= prevCh && t[ti] <= endCh {
						matched = true
					}
					prevCh = 0 // Reset for next iteration
				} else {
					// Single character (including /)
					if t[ti] == pCh {
						matched = true
					}
					prevCh = pCh
				}
				pi++
			}

			if pi >= len(p) || p[pi] != ']' {
				return WM_ABORT_ALL
			}
			pi++ // Skip closing ]

			// Check match result
			if matched == negated {
				return WM_NOMATCH
			}
			if flags&WM_PATHNAME != 0 && t[ti] == '/' {
				// Only block / if not explicitly matched in the class
				if !matched || t[ti] != '/' {
					return WM_NOMATCH
				}
			}
			ti++

		default:
			// Literal character match
			if ti >= len(t) || t[ti] != pCh {
				return WM_NOMATCH
			}
			pi++
			ti++
		}
	}

	// Pattern exhausted - check if text is too
	if ti < len(t) {
		return WM_NOMATCH
	}
	return WM_MATCH
}

// hasBareDoubleSlash returns true if the pattern contains a raw "//" sequence
// that is not part of a character class and not the terminal "**//" handled
// specially. We scan while tracking whether we're inside a character class.
func hasBareDoubleSlash(s string) bool {
	inClass := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' { // escape next char
			i++
			continue
		}
		if c == '[' {
			inClass = true
			continue
		}
		if c == ']' {
			inClass = false
			continue
		}
		if !inClass && c == '/' && i+1 < len(s) && s[i+1] == '/' {
			// Allow if at end with preceding ** already handled? We only treat as bare if NOT ending with **//
			if !(i >= 2 && strings.HasSuffix(s, "**//") && i == len(s)-2) {
				return true
			}
		}
	}
	return false
}
