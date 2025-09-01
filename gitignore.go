// Package gitignore implements Git-compatible .gitignore pattern matching.
package gitignore

import (
	"path"
	"strings"

	wildmatch "github.com/idelchi/go-gitignore/wildmatch"
)

// patternFlag is a bitmask describing properties of a compiled pattern.
type patternFlag uint16

const (
	// flagNegative marks a pattern beginning with '!' (negation/rescue).
	flagNegative patternFlag = 1 << iota

	// flagDirOnly indicates the pattern only matches directories (trailing '/').
	flagDirOnly

	// flagNoDir indicates the pattern contains no '/' and applies to basenames only.
	flagNoDir

	// flagEndsWith marks an optimized pattern of the form "*literal".
	flagEndsWith
)

// pattern is the compiled representation of a single .gitignore pattern.
type pattern struct {
	// the original text of the pattern
	original string
	// the normalized/processed pattern used for matching
	pattern string
	// byte length of pattern
	patternlen int
	// number of leading literal bytes (no glob meta).
	nowildcardlen int
	// patternFlag bitmask describing pattern traits.
	flags patternFlag
}

// GitIgnore holds a sequence of compiled patterns. Construct with New or NewOptions.
// Matching semantics follow Git’s .gitignore rules (last match wins).
type GitIgnore struct {
	// the compiled patterns
	patterns []pattern
	// matcher options
	opts Options
}

// Options defines matcher-wide behavior.
type Options struct {
	// CaseFold enables ASCII-only case-insensitive matching in the underlying wildmatch engine.
	CaseFold bool
}

// New compiles .gitignore-style lines using default Options.
func New(lines ...string) *GitIgnore {
	return NewOptions(Options{}, lines...)
}

// NewOptions compiles .gitignore-style lines with explicit options.
func NewOptions(opt Options, lines ...string) *GitIgnore {
	patterns := make([]pattern, 0, len(lines))

	for _, line := range lines {
		if p := parsePattern(line); p != nil {
			patterns = append(patterns, *p)
		}
	}

	return &GitIgnore{patterns: patterns, opts: opt}
}

// Patterns returns the original patterns in their input order.
func (g *GitIgnore) Patterns() []string {
	out := make([]string, len(g.patterns))

	for i, p := range g.patterns {
		out[i] = p.original
	}

	return out
}

// Append compiles and appends new patterns, preserving last-match-wins order.
func (g *GitIgnore) Append(lines ...string) {
	for _, line := range lines {
		if p := parsePattern(line); p != nil {
			g.patterns = append(g.patterns, *p)
		}
	}
}

// Match is a detailed result mirroring `git check-ignore -v` semantics.
// Pattern contains the deciding pattern (or "!pattern" for a rescuing negation),
// or is empty when no rule matched and no parent exclusion applies.
type Match struct {
	Ignored bool
	Pattern string
}

// Match returns a detailed match result, including the deciding pattern.
// If no rule directly matches but an ancestor directory is excluded, the
// ancestor’s pattern is returned.
func (g *GitIgnore) Match(pathname string, isDir bool) Match {
	if len(g.patterns) == 0 || pathname == "" || strings.HasPrefix(pathname, "/") {
		return Match{Ignored: false, Pattern: ""}
	}

	pathname = path.Clean(pathname)

	parentExcluded, parentPattern := g.parentExcludedWithPattern(pathname)

	for i := len(g.patterns) - 1; i >= 0; i-- {
		p := g.patterns[i]

		if !g.matchesPattern(p, pathname, isDir) {
			continue
		}

		if p.flags&flagNegative != 0 {
			// Special-case current directory: a negation must NOT rescue '.'
			// Treat it as if the negation rule does not apply and continue
			// scanning earlier patterns to find the deciding positive rule.
			if pathname == "." {
				continue
			}

			// '..' can be rescued unless an ancestor is excluded.
			if pathname == ".." {
				if parentExcluded {
					return Match{Ignored: true, Pattern: parentPattern}
				}

				return Match{Ignored: false, Pattern: p.original}
			}

			// If an ancestor is excluded, a negation cannot rescue.
			if parentExcluded {
				return Match{Ignored: true, Pattern: parentPattern}
			}

			return Match{Ignored: false, Pattern: p.original}
		}

		return Match{Ignored: true, Pattern: p.original}
	}

	if parentExcluded {
		return Match{Ignored: true, Pattern: parentPattern}
	}

	return Match{Ignored: false, Pattern: ""}
}

// Ignored reports whether a relative path should be ignored.
// The caller must indicate if the path is a directory.
func (g *GitIgnore) Ignored(pathname string, isDir bool) bool {
	return g.Match(pathname, isDir).Ignored
}

// matchRooted handles patterns beginning with '/' (root-relative).
func (g *GitIgnore) matchRooted(p pattern, pathname string, isDir bool) bool {
	if p.flags&flagDirOnly != 0 && !isDir {
		return false
	}

	pat := p.pattern[1:] // strip leading '/'
	text := pathname

	// Adjust the literal-prefix length (we removed a leading '/').
	lit := p.nowildcardlen

	if lit > 0 {
		lit--
	}

	if lit < 0 {
		lit = 0
	}

	if lit > len(pat) {
		lit = len(pat)
	}

	if lit > len(text) || pat[:lit] != text[:lit] {
		return false
	}

	pat = pat[lit:]
	text = text[lit:]

	// Entire pattern is literal.
	if p.nowildcardlen == p.patternlen {
		return text == ""
	}

	if !wildmatch.MatchOpt(pat, text, wildmatch.WMOptions{
		Pathname: true,
		CaseFold: g.opts.CaseFold,
	}) {
		return false
	}

	return true
}

// matchesPattern tests a single compiled pattern against a candidate path.
func (g *GitIgnore) matchesPattern(p pattern, pathname string, isDir bool) bool {
	if p.flags&flagDirOnly != 0 && !isDir {
		return false
	}

	// Rooted pattern.
	if len(p.pattern) > 0 && p.pattern[0] == '/' {
		return g.matchRooted(p, pathname, isDir)
	}

	// Basename-only (no '/'): match against the final component only.
	if p.flags&flagNoDir != 0 {
		base := path.Base(pathname)

		return g.matchBasename(base, p.pattern, p.nowildcardlen, p.patternlen, p.flags)
	}

	// Path-containing pattern: relative to root; do NOT slide.
	pat := p.pattern
	text := pathname

	// Fast path for literal prefix.
	if p.nowildcardlen > 0 && p.nowildcardlen <= len(pat) && p.nowildcardlen <= len(text) {
		if pat[:p.nowildcardlen] != text[:p.nowildcardlen] {
			return false
		}

		pat = pat[p.nowildcardlen:]
		text = text[p.nowildcardlen:]
	} else if p.nowildcardlen > len(text) {
		return false
	}

	// Entire pattern is literal.
	if p.nowildcardlen == p.patternlen {
		return pat == text
	}

	if !wildmatch.MatchOpt(pat, text, wildmatch.WMOptions{
		Pathname: true,
		CaseFold: g.opts.CaseFold,
	}) {
		return false
	}

	if p.flags&flagDirOnly != 0 && !isDir {
		return false
	}

	return true
}

// matchBasename matches a single path component (no '/' inside).
func (g *GitIgnore) matchBasename(basename, pattern string, nowildcardlen, patternlen int, pflags patternFlag) bool {
	if patternlen == 0 {
		return basename == ""
	}

	if nowildcardlen == patternlen {
		return basename == pattern
	}

	// Optimized "*literal" suffix check.
	if pflags&flagEndsWith != 0 && len(pattern) > 1 && pattern[0] == '*' {
		return strings.HasSuffix(basename, pattern[1:])
	}

	return wildmatch.MatchOpt(pattern, basename, wildmatch.WMOptions{
		Pathname: false,
		CaseFold: g.opts.CaseFold,
	})
}

// parsePattern compiles a single .gitignore pattern line or returns nil.
// It implements Git’s rules for comments, escapes, trimming of unescaped
// trailing spaces, negation markers, and directory-only markers.
func parsePattern(line string) *pattern {
	original := line

	// Comments (unless escaped with '\#') and empty lines are inert.
	if line == "" || (strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "\\#")) {
		return nil
	}

	p := &pattern{original: original}

	switch {
	case strings.HasPrefix(line, "\\#"), strings.HasPrefix(line, "\\!"):
		// Unescape escaped comment/negation prefix.
		line = line[1:]

	case strings.HasPrefix(line, "!"):
		p.flags |= flagNegative

		line = line[1:]
	}

	// Trim unescaped trailing spaces.
	line = trimTrailingSpaces(line)
	if line == "" {
		return nil
	}

	// Trailing '/' means "directories only".
	if strings.HasSuffix(line, "/") {
		line = line[:len(line)-1]

		p.flags |= flagDirOnly
	}

	// No '/' means "basename-only".
	if !strings.Contains(line, "/") {
		p.flags |= flagNoDir
	}

	// Count leading literal bytes.
	p.nowildcardlen = simpleLength(line)
	if p.nowildcardlen > len(line) {
		p.nowildcardlen = len(line)
	}

	// Optimization: "*literal" pattern.
	if strings.HasPrefix(line, "*") && noWildcard(line[1:]) {
		p.flags |= flagEndsWith
	}

	p.pattern = line
	p.patternlen = len(line)

	return p
}

// trimTrailingSpaces removes unescaped trailing space characters from s.
// A trailing space is considered escaped if preceded by an odd number of
// backslashes.
func trimTrailingSpaces(s string) string {
	for len(s) > 0 && s[len(s)-1] == ' ' {
		backslashCount := 0

		const backslashCheckOffset = 2
		for i := len(s) - backslashCheckOffset; i >= 0 && s[i] == '\\'; i-- {
			backslashCount++
		}

		// Odd number of backslashes => last space is escaped.
		if backslashCount%2 == 1 {
			break
		}

		s = s[:len(s)-1]
	}

	return s
}

// simpleLength returns the number of leading literal (non-glob) bytes in s.
// Stops at the first meta character recognized by this matcher.
func simpleLength(s string) int {
	for i := range len(s) {
		if isGlobSpecial(s[i]) {
			return i
		}
	}

	return len(s)
}

// parentExcludedWithPattern reports whether any ancestor is excluded and
// returns the deciding pattern for that ancestor (if excluded).
func (g *GitIgnore) parentExcludedWithPattern(pathname string) (bool, string) {
	if pathname == "." {
		return false, ""
	}

	parts := strings.Split(pathname, "/")

	for i := 1; i < len(parts); i++ { // exclude the full path itself
		ancestor := strings.Join(parts[:i], "/")
		isExcluded := false
		decidingPattern := ""

		for j := len(g.patterns) - 1; j >= 0; j-- {
			p := g.patterns[j]

			if !g.matchesPattern(p, ancestor, true) {
				continue
			}

			if p.flags&flagNegative != 0 {
				isExcluded = false
				decidingPattern = ""
			} else {
				isExcluded = true
				decidingPattern = p.original
			}

			break
		}

		if isExcluded {
			return true, decidingPattern
		}
	}

	return false, ""
}

// isGlobSpecial reports whether c is a glob meta-character recognized by this
// matcher: '*', '?', '[', or the escape '\\'.
func isGlobSpecial(c byte) bool {
	return c == '*' || c == '?' || c == '[' || c == '\\'
}

// noWildcard reports whether s contains no glob meta-characters at all.
func noWildcard(s string) bool {
	return simpleLength(s) == len(s)
}
