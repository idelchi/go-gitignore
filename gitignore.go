// Package gitignore implements Git .gitignore style pattern matching.
package gitignore

import (
	"path"
	"strings"
)

// Wildmatch engine flags (internal). Callers use Wildmatch.
const (
	wmCaseFold = 1 << iota // currently unused (case sensitivity follows Git default)
	wmPathname             // enable directory (slash) sensitive matching
)

// patternFlag marks parsed pattern properties (negative, dir-only, etc).
type patternFlag uint16

// Internal pattern flags (subset of Git's) plus a local **// form flag.
const (
	flagNegative      patternFlag = 1 << iota // pattern begins with '!'
	flagDirOnly                               // pattern matches directories only (had trailing / or implied by **// with no suffix)
	flagNoDir                                 // pattern has no '/'; match only basename
	flagEndsWith                              // optimized leading '*literal' pattern
	flagDoubleStarDir                         // custom: canonical <literal>**// directory tree style pattern
)

// Internal wildmatch return codes.
const (
	wmMatch           = 0
	wmNoMatch         = 1
	wmAbortAll        = -1
	wmAbortToStarstar = -2
)

type pattern struct {
	original      string      // original pattern text (for debugging / reporting)
	pattern       string      // normalized pattern used for matching
	patternlen    int         // length of pattern
	nowildcardlen int         // prefix length up to first wildcard
	base          string      // base directory (for DOUBLESTAR_DIR optimization)
	baselen       int         // length of base
	flags         patternFlag // bit flags (flag*)
	suffix        string      // suffix after **// (DOUBLESTAR_DIR)
}

// GitIgnore holds compiled patterns. Use New to build one.
type GitIgnore struct{ patterns []pattern }

// New compiles .gitignore style lines. Ignores comments, empty, inert lines.
func New(lines ...string) *GitIgnore {
	patterns := make([]pattern, 0, len(lines))

	for _, line := range lines {
		if p := parsePattern(line); p != nil {
			patterns = append(patterns, *p)
		}
	}

	return &GitIgnore{patterns: patterns}
}

// Patterns returns original patterns in order.
func (g *GitIgnore) Patterns() []string {
	out := make([]string, len(g.patterns))
	for i, p := range g.patterns {
		out[i] = p.original
	}
	return out
}

// Ignored reports if a relative path is ignored. Caller specifies if it is a directory.
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

	ignored := false
	parentExcluded := g.parentExcluded(pathname)

	// Apply patterns in order; later patterns override earlier (negations can rescue unless parent dir excluded by
	// earlier rule that is still in effect)
	for _, p := range g.patterns {
		if matchesPattern(p, pathname, isDir) {
			if p.flags&flagNegative != 0 {
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

// parentExcluded reports if any ancestor dir is ignored by a non-negated rule.
func (g *GitIgnore) parentExcluded(pathname string) bool {
	if pathname == "." {
		return false
	}
	parts := strings.Split(pathname, "/")
	excluded := false
	parents := make(map[string]bool)
	for i := 1; i < len(parts); i++ {
		parentPath := strings.Join(parts[:i], "/")
		for _, p := range g.patterns {
			if matchesPattern(p, parentPath, true) {
				if p.flags&flagNegative != 0 {
					delete(parents, parentPath)
				} else {
					parents[parentPath] = true
				}
			}
		}
	}
	if len(parents) > 0 {
		excluded = true
	}
	return excluded
}

// matchDoubleStarDir matches canonical <prefix>**//[suffix] patterns.
func matchDoubleStarDir(p pattern, pathname string, isDir bool) bool {
	pat := p.pattern
	marker := strings.Index(pat, "**//")
	if marker == -1 {
		return false
	}
	prefix := pat[:marker]
	suffix := p.suffix
	rooted := false
	if strings.HasPrefix(prefix, "/") {
		rooted = true
		prefix = prefix[1:]
	}
	if prefix == "" {
		return false
	}
	if rooted {
		if !strings.HasPrefix(pathname, prefix) {
			return false
		}
		if len(pathname) > len(prefix) && pathname[len(prefix)] != '/' {
			return false
		}
	} else if pathname != prefix && !strings.HasPrefix(pathname, prefix+"/") {
		return false
	}
	if suffix == "" {
		if pathname == prefix {
			return isDir
		}
		return strings.HasPrefix(pathname, prefix+"/")
	}
	for len(suffix) > 0 && suffix[0] == '/' {
		suffix = suffix[1:]
	}
	rem := ""
	if pathname == prefix {
		rem = ""
	} else if strings.HasPrefix(pathname, prefix+"/") {
		rem = pathname[len(prefix)+1:]
	}
	if rem == "" {
		return false
	}
	if wildmatch(suffix, rem, wmPathname) == wmMatch {
		return true
	}
	for i := 0; i < len(rem); i++ {
		if rem[i] == '/' && i+1 < len(rem) {
			if wildmatch(suffix, rem[i+1:], wmPathname) == wmMatch {
				return true
			}
		}
	}
	return false
}

// matchRooted handles patterns beginning with '/'.
func matchRooted(p pattern, pathname string, isDir bool) bool {
	pattern := p.pattern
	trimmed := pattern[1:]
	if p.flags&flagDirOnly != 0 && strings.HasSuffix(trimmed, "/") {
		trimmed = strings.TrimSuffix(trimmed, "/")
	}
	if p.flags&flagDirOnly != 0 {
		if !isDir { return false }
		// Allow globs in rooted dir-only pattern: run wildmatch with pathname semantics.
		return wildmatch(trimmed, pathname, wmPathname) == wmMatch
	}
	if strings.HasSuffix(trimmed, "/**") {
		prefix := strings.TrimSuffix(trimmed, "/**")
		if pathname == prefix && isDir {
			return false
		}
	}
	return matchPathname(pathname, trimmed)
}

// parsePattern compiles one pattern line or returns nil.
func parsePattern(line string) *pattern {
	orig := line
	if line == "" || (strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "\\#")) {
		return nil
	}
	p := &pattern{original: orig}
	switch {
	case strings.HasPrefix(line, "\\#"), strings.HasPrefix(line, "\\!"):
		line = line[1:]
	case strings.HasPrefix(line, "!"):
		p.flags |= flagNegative
		line = line[1:]
	}
	line = trimTrailingSpaces(line)
	if line == "" {
		return nil
	}
	if handled := compileDoubleStarDir(p, line); handled {
		return p
	}
	if hasBareDoubleSlash(line) { // inert pattern
		p.pattern, p.patternlen, p.baselen = line, len(line), -1
		return p
	}
	// directory-only suffix slashes
	if strings.HasSuffix(line, "/") {
		for strings.HasSuffix(line, "/") {
			line = line[:len(line)-1]
		}
		p.flags |= flagDirOnly
	}
	if !strings.Contains(line, "/") {
		p.flags |= flagNoDir
	}
	p.nowildcardlen = simpleLength(line)
	if p.nowildcardlen > len(line) {
		p.nowildcardlen = len(line)
	}
	if strings.HasPrefix(line, "*") && noWildcard(line[1:]) {
		p.flags |= flagEndsWith
	}
	p.pattern = line
	p.patternlen = len(line)
	return p
}

// compileDoubleStarDir canonicalises <literal>(**/)* **//<suffix?> forms.
func compileDoubleStarDir(p *pattern, line string) bool {
	pos := strings.Index(line, "**//")
	if pos < 0 {
		return false
	}
	starRunStart := pos
	for starRunStart > 0 && line[starRunStart-1] == '*' {
		starRunStart--
	}
	chainStart := starRunStart
	for chainStart >= 3 && line[chainStart-3:chainStart] == "**/" {
		chainStart -= 3
	}
	prefix := line[:chainStart]
	// Bare **// (empty literal prefix) should be inert; Git treats ill-formed
	// directory-recursive markers without a literal base as non-matching.
	if prefix == "" {
		p.pattern, p.patternlen, p.baselen = line, len(line), -1
		return true
	}
	if isLiteralPrefix(prefix) {
		suffix := line[pos+4:]
		rooted := strings.HasPrefix(prefix, "/")
		if rooted {
			prefix = prefix[1:]
		}
		p.flags |= flagDoubleStarDir
		if suffix == "" {
			p.flags |= flagDirOnly
		}
		if rooted {
			p.pattern = "/" + prefix + "**//"
		} else {
			p.pattern = prefix + "**//"
		}
		p.base, p.baselen, p.suffix, p.patternlen = prefix, len(prefix), suffix, len(p.pattern)
		return true
	}
	if !isLiteralPrefix(prefix) { // wildcard in prefix -> inert
		p.pattern, p.patternlen, p.baselen = line, len(line), -1
		return true
	}
	return false
}

// collapseSlashes retained for historical reference (tests rely on not
// collapsing). Kept as comment for clarity.
// func collapseSlashes(s string) string { ... }

// trimTrailingSpaces removes unescaped trailing spaces.
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

// simpleLength returns length of literal prefix before first glob meta.
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

func noWildcard(s string) bool { return simpleLength(s) == len(s) }

// disallowWildcardLeadingSingleComponent enforces Git's single-component guard.
func disallowWildcardLeadingSingleComponent(pattern, pathname string) bool {
	if strings.Contains(pathname, "/") || len(pattern) == 0 || pattern[0] == '/' {
		return false
	}
	inClass := false
	hasSlashOutside := false
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		if c == '\\' && i+1 < len(pattern) {
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
		if c == '/' && !inClass {
			hasSlashOutside = true
			break
		}
	}
	if hasSlashOutside {
		if pattern[0] == '*' || pattern[0] == '?' || pattern[0] == '[' {
			if !strings.HasPrefix(pattern, "**/") {
				return true
			}
		}
	}
	return false
}

// matchesPattern tests one compiled pattern.
func matchesPattern(p pattern, pathname string, isDir bool) bool {
	// Inert pattern (contains double slash but not recognized special **// form)
	if p.baselen == -1 && p.flags&flagDoubleStarDir == 0 {
		return false
	}

	// Directory-only patterns
	if p.flags&flagDirOnly != 0 && !isDir {
		return false
	}

	if p.flags&flagDoubleStarDir != 0 {
		return matchDoubleStarDir(p, pathname, isDir)
	}

	basename := path.Base(pathname)
	pattern := p.pattern

	// (Removed experimental single-component slash pattern restriction.)

	if disallowWildcardLeadingSingleComponent(pattern, pathname) {
		return false
	}

	if len(pattern) > 0 && pattern[0] == '/' {
		return matchRooted(p, pathname, isDir)
	}

	// NODIR means match basename only
	if p.flags&flagNoDir != 0 {
		return matchBasename(basename, pattern, p.nowildcardlen, p.patternlen, p.flags)
	}

	// Pattern contains slash - match against full path
	return matchPathname(pathname, pattern)
}

// matchBasename matches a single path component.
func matchBasename(basename, pattern string, nowildcardlen, patternlen int, pflags patternFlag) bool {
	if patternlen == 0 {
		return basename == ""
	}
	if nowildcardlen == patternlen {
		return basename == pattern
	}
	if pflags&flagEndsWith != 0 && len(pattern) > 1 && pattern[0] == '*' {
		return strings.HasSuffix(basename, pattern[1:])
	}
	return wildmatch(pattern, basename, 0) == wmMatch
}

// matchPathname matches a relative path (slash aware).
func matchPathname(pathname, pattern string) bool {
	return wildmatch(pattern, pathname, wmPathname) == wmMatch
}

// isLiteralPrefix reports absence of glob meta.
func isLiteralPrefix(s string) bool {
	for i := 0; i < len(s); i++ {
		if isGlobSpecial(s[i]) || s[i] == '?' || s[i] == '*' || s[i] == '[' || s[i] == ']' {
			return false
		}
	}
	return true
}

// Wildmatch reports whether text matches pattern. If pathname, '/' is special.
func Wildmatch(pattern, text string, pathname bool) bool {
	f := 0
	if pathname {
		f = wmPathname
	}
	return wildmatch(pattern, text, f) == wmMatch
}

// wildmatch returns the raw engine status code.
func wildmatch(pattern, text string, wmFlags int) int {
	return dowild([]byte(pattern), []byte(text), 0, 0, wmFlags)
}

func dowild(p, t []byte, pi, ti, flags int) int {
	var pCh byte

	for pi < len(p) {
		pCh = p[pi]

		// Check if we've run out of text
		if ti >= len(t) && pCh != '*' {
			return wmAbortAll
		}

		switch pCh {
		case '\\':
			// Escape - match next character literally
			pi++
			if pi >= len(p) {
				return wmAbortAll
			}
			if ti >= len(t) || t[ti] != p[pi] {
				return wmNoMatch
			}
			pi++
			ti++

		case '?':
			// Match any single byte except /
			if ti >= len(t) {
				return wmNoMatch
			}
			if flags&wmPathname != 0 && t[ti] == '/' {
				return wmNoMatch
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

				// Git special ** rule plus extension: treat **/ as special even if preceded by non-slash to accommodate
				// a**/ forms required by tests
				isSpecial := false
				if flags&wmPathname != 0 {
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

					// Try matching rest of pattern at each position
					for ; ti <= len(t); ti++ {
						result := dowild(p, t, pi, ti, flags)
						if result == wmMatch {
							return wmMatch
						}
						if flags&wmPathname != 0 && ti < len(t) && t[ti] == '/' {
							return wmNoMatch
						}
					}
					return wmNoMatch
				}

				// Special ** handling
				// Pattern ends with ** -> matches remainder (including empty)
				if pi >= len(p) {
					return wmMatch
				}

				consumeSlash := false
				if p[pi] == '/' {
					consumeSlash = true
					pi++
				}

				// Try to match the rest of pattern; for recursive search, advance over directory boundaries only
				if consumeSlash {
					// '**/' form: allow matching at current level or any deeper level
					if res := dowild(p, t, pi, ti, flags); res == wmMatch {
						return wmMatch
					}
					for scan := ti; scan < len(t); scan++ {
						if t[scan] == '/' {
							if res := dowild(p, t, pi, scan+1, flags); res == wmMatch {
								return wmMatch
							}
						}
					}
					return wmNoMatch
				}

				// Bare '**' (not followed by slash) - standard recursive: advance one char at a time
				for scan := ti; scan <= len(t); scan++ {
					if res := dowild(p, t, pi, scan, flags); res == wmMatch {
						return wmMatch
					}
				}
				return wmNoMatch
			}

			// Single *: match anything except '/'. If pattern is of the form '*<lit>**/*' we ensure at
			// least one char before that literal so '*0**/*' does not match '0'.
			matchSlash := flags&wmPathname == 0

			if pi >= len(p) {
				// Trailing *
				if !matchSlash {
					for i := ti; i < len(t); i++ {
						if t[i] == '/' {
							return wmNoMatch
						}
					}
				}
				return wmMatch
			}

			// Special case: * followed by / in pathname mode
			if !matchSlash && p[pi] == '/' {
				// Match up to next /
				for ti < len(t) && t[ti] != '/' {
					ti++
				}
				if ti >= len(t) {
					return wmNoMatch
				}
				// Continue matching after the /
				continue
			}

			// Lookahead heuristic: if pattern requires additional segments via **/ after a literal, avoid empty match
			// for '*'
			needNonEmpty := false
			if pi < len(p) && p[pi] != '*' {
				look := p[pi:]
				// Determine leading literal run
				litLen := 0
				for litLen < len(look) {
					c := look[litLen]
					if c == '*' || c == '?' || c == '[' || c == '/' {
						break
					}
					litLen++
				}
				if litLen > 0 && strings.HasPrefix(string(look[litLen:]), "**/") {
					// Check if remainder after **/ can match empty (all '*')
					rest := look[litLen+3:]
					emptyOK := true
					for i := 0; i < len(rest); i++ {
						if rest[i] != '*' {
							emptyOK = false
							break
						}
					}
					if emptyOK {
						needNonEmpty = true
					}
				}
			}
			start := ti
			if needNonEmpty && start < len(t) {
				start++ // force at least one char
			}
			for ti = start; ti <= len(t); ti++ {
				result := dowild(p, t, pi, ti, flags)
				if result == wmMatch {
					return wmMatch
				}
				if !matchSlash && ti < len(t) && t[ti] == '/' {
					return wmNoMatch
				}
			}
			return wmNoMatch

		case '[':
			// Character class
			if ti >= len(t) {
				return wmNoMatch
			}

			pi++
			if pi >= len(p) {
				return wmAbortAll
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
						return wmAbortAll
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
							return wmAbortAll
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
				return wmAbortAll
			}
			pi++ // Skip closing ]

			// Check match result
			if matched == negated {
				return wmNoMatch
			}
			if flags&wmPathname != 0 && t[ti] == '/' {
				// Only block / if not explicitly matched in the class
				if !matched || t[ti] != '/' {
					return wmNoMatch
				}
			}
			ti++

		default:
			// Literal character match
			if ti >= len(t) || t[ti] != pCh {
				return wmNoMatch
			}
			pi++
			ti++
		}
	}

	// Pattern exhausted - check if text is too
	if ti < len(t) {
		return wmNoMatch
	}
	return wmMatch
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
