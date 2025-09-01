// Package wildmatch implements Git's wildmatch.c semantics in Go.
package wildmatch

// Internal result codes.
const (
	// successful match.
	wmMatch = 0
	// a definitive non-match.
	wmNoMatch = 1
	// abort current path of backtracking.
	wmAbortAll = -1
	// abort up to the nearest '**' context.
	wmAbortToStarstar = -2
)

// Internal matching flags (bitmask). External callers use Match or MatchOpt.
const (
	// ASCII case-folding.
	wmCaseFold = 1 << iota
	// enable directory (slash) sensitive matching.
	wmPathname
)

// Match reports whether text matches pattern. If pathname==true, '/' is special
// and only matched by '**' in special positions, never by '*' or '?'.
func Match(pattern, text string, pathname bool) bool {
	flags := 0

	if pathname {
		flags = wmPathname
	}

	return wildmatch(pattern, text, flags) == wmMatch
}

// WMOptions are options for MatchOpt.
type WMOptions struct {
	// Pathname: treat '/' as a directory separator with special handling.
	Pathname bool
	// CaseFold: enable ASCII-only case-insensitive matching.
	CaseFold bool
}

// MatchOpt matches text against pattern with explicit options.
func MatchOpt(pattern, text string, opt WMOptions) bool {
	flags := 0

	if opt.Pathname {
		flags |= wmPathname
	}

	if opt.CaseFold {
		flags |= wmCaseFold
	}

	return wildmatch(pattern, text, flags) == wmMatch
}

// wildmatch is a small shim that converts Go strings to byte slices and launches
// the core matching routine, preserving the internal return codes for fidelity.
func wildmatch(pattern, text string, wmFlags int) int {
	return dowild([]byte(pattern), []byte(text), 0, 0, wmFlags)
}

// asciiLowerDelta is the distance between uppercase and lowercase ASCII letters.
// It is used to implement fast ASCII-only case folding.
const asciiLowerDelta byte = 'a' - 'A'

// asciiIsUpper reports whether b is an ASCII uppercase letter (A-Z).
func asciiIsUpper(b byte) bool {
	return b >= 'A' && b <= 'Z'
}

// asciiIsLower reports whether b is an ASCII lowercase letter (a-z).
func asciiIsLower(b byte) bool {
	return b >= 'a' && b <= 'z'
}

// asciiToLower returns b converted to lowercase if it is ASCII uppercase.
// For all other bytes, it returns b unchanged.
func asciiToLower(b byte) byte {
	if asciiIsUpper(b) {
		return b + asciiLowerDelta
	}

	return b
}

// asciiIsDigit reports whether b is an ASCII decimal digit (0-9).
func asciiIsDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

// asciiIsAlpha reports whether b is an ASCII letter (A-Z or a-z).
func asciiIsAlpha(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

// asciiIsAlnum reports whether b is an ASCII alphanumeric character.
func asciiIsAlnum(b byte) bool {
	return asciiIsAlpha(b) || asciiIsDigit(b)
}

// asciiIsSpace reports whether b is an ASCII blank space or tab.
func asciiIsSpace(b byte) bool {
	return b == ' ' || b == '\t'
}

// asciiIsPrint reports whether b is an ASCII printable character (0x20-0x7E).
func asciiIsPrint(b byte) bool {
	return b >= 0x20 && b <= 0x7e
}

// asciiIsGraph reports whether b is an ASCII graphic (printable, non-space).
func asciiIsGraph(b byte) bool {
	return asciiIsPrint(b) && !asciiIsSpace(b)
}

// asciiIsCntrl reports whether b is an ASCII control character.
func asciiIsCntrl(b byte) bool {
	const (
		maxControlChar = 0x1f
		delChar        = 0x7f
	)

	return (b <= maxControlChar) || b == delChar
}

// asciiIsPunct reports whether b is ASCII punctuation (printable, non-alnum, non-space).
func asciiIsPunct(b byte) bool {
	return asciiIsPrint(b) && !asciiIsAlnum(b) && !asciiIsSpace(b)
}

// asciiIsXDigit reports whether b is an ASCII hexadecimal digit.
func asciiIsXDigit(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

// foldASCII applies ASCII-only case folding to b when wmCaseFold is set in flags.
// If case folding is disabled or b is not uppercase ASCII, b is returned unchanged.
func foldASCII(b byte, flags int) byte {
	if flags&wmCaseFold != 0 && asciiIsUpper(b) {
		return asciiToLower(b)
	}

	return b
}

// isGlobSpecial reports whether c is one of the glob metacharacters recognized
// by this implementation: '*', '?', '[', or the escape '\\'.
func isGlobSpecial(c byte) bool {
	return c == '*' || c == '?' || c == '[' || c == '\\'
}

// dowild is a port of Git's wildmatch.c main routine.
func dowild(pattern, text []byte, pi, ti, flags int) int {
	var pCh byte

	for pi < len(pattern) {
		pCh = pattern[pi]

		// If text is exhausted but pattern isn't (and next is not '*'), abort.
		if ti >= len(text) && pCh != '*' {
			return wmAbortAll
		}

		// Prepare comparison byte from text with optional ASCII folding.
		tCh := byte(0)

		if ti < len(text) {
			tCh = foldASCII(text[ti], flags)
		}

		// Apply optional ASCII folding to the current pattern byte as well.
		pCh = foldASCII(pCh, flags)

		switch pCh {
		case '\\':
			// Escape: next pattern byte must match literally.
			pi++

			if pi >= len(pattern) {
				return wmAbortAll
			}

			next := foldASCII(pattern[pi], flags)

			if ti >= len(text) || tCh != next {
				return wmNoMatch
			}

			pi++

			ti++

		case '?':
			// Match any single byte except '/' in pathname mode.
			if ti >= len(text) {
				return wmNoMatch
			}

			if flags&wmPathname != 0 && text[ti] == '/' {
				return wmNoMatch
			}

			pi++

			ti++

		case '*':
			pi++

			// Whether this star (or run of stars) may match '/'.
			var matchSlash bool

			// Check if this is a '**' pattern (possibly a run of '*').
			if pi < len(pattern) && pattern[pi] == '*' {
				// prevP indexes the second '*' in the run (like C's prev_p).
				prevP := pi

				// Skip all consecutive '*' characters.
				for pi < len(pattern) && pattern[pi] == '*' {
					pi++
				}

				// Git's exact '**' special detection from wildmatch.c.
				const minPrevIndex = 2
				switch {
				case flags&wmPathname == 0:
					// Without WM_PATHNAME, ** == *
					matchSlash = true
				case ((prevP < minPrevIndex) || (pattern[prevP-minPrevIndex] == '/')) &&
					(pi >= len(pattern) || pattern[pi] == '/' ||
						(pi+1 < len(pattern) && pattern[pi] == '\\' && pattern[pi+1] == '/')):
					// Special case from C code: try zero-width match first.
					if pi < len(pattern) && pattern[pi] == '/' {
						if dowild(pattern, text, pi+1, ti, flags) == wmMatch {
							return wmMatch
						}
					}

					matchSlash = true
				default:
					// WM_PATHNAME is set but '**' is not in a special position.
					matchSlash = false
				}
			} else {
				// Single '*' — without WM_PATHNAME, '*' == '**'.
				matchSlash = flags&wmPathname == 0
			}

			// Handle end-of-pattern after a star or run of stars.
			if pi >= len(pattern) {
				// Trailing '*' or '**'.
				if !matchSlash {
					// Verify no '/' remains in text when '/' cannot be matched.
					for i := ti; i < len(text); i++ {
						if text[i] == '/' {
							return wmAbortToStarstar
						}
					}
				}

				return wmMatch
			}

			// Special case: single '*' followed by '/' in pathname mode.
			if !matchSlash && pi < len(pattern) && pattern[pi] == '/' {
				// Advance text to the next '/' (if any).
				for ti < len(text) && text[ti] != '/' {
					ti++
				}

				if ti >= len(text) {
					return wmAbortAll
				}

				// The '/' will be consumed by the main loop on the next iteration.
				continue
			}

			// Fast-forward when the next token is a literal (Git optimization).
			if pi < len(pattern) && !isGlobSpecial(pattern[pi]) {
				lit := foldASCII(pattern[pi], flags)

				pos := ti

				for pos < len(text) && (matchSlash || text[pos] != '/') {
					if foldASCII(text[pos], flags) == lit {
						break
					}

					pos++
				}

				if pos >= len(text) || (!matchSlash && pos < len(text) && text[pos] == '/') {
					if matchSlash {
						return wmAbortAll
					}

					return wmAbortToStarstar
				}

				ti = pos
			}

			// Main '*' matching loop from Git's C code.
			for ti < len(text) {
				// Try to match rest of pattern at current position.
				result := dowild(pattern, text, pi, ti, flags)

				if result != wmNoMatch {
					if !matchSlash || result != wmAbortToStarstar {
						return result
					}
				} else if !matchSlash && text[ti] == '/' {
					return wmAbortToStarstar
				}

				ti++
			}

			return wmAbortAll

		case '[':
			// Character class.
			if ti >= len(text) {
				return wmNoMatch
			}

			pi++

			if pi >= len(pattern) {
				return wmAbortAll
			}

			// Check for negation ('!' or '^' after the opening '[').
			negated := false

			if pattern[pi] == '!' || pattern[pi] == '^' {
				negated = true
				pi++
			}

			matched := false
			prevCh := byte(0)

			// Special case: ']' as first character is literal.
			if pi < len(pattern) && pattern[pi] == ']' {
				if tCh == ']' {
					matched = true
				}

				prevCh = ']'
				pi++
			}

			// Process character class (escapes, ranges, POSIX classes).
			for pi < len(pattern) && pattern[pi] != ']' {
				pCh = pattern[pi]

				switch {
				case pCh == '\\':
					pi++

					if pi >= len(pattern) {
						return wmAbortAll
					}

					pCh = pattern[pi]

					comp := foldASCII(pCh, flags)

					if tCh == comp {
						matched = true
					}

					prevCh = pCh
				case pCh == '-' && prevCh != 0 && pi+1 < len(pattern) && pattern[pi+1] != ']':
					// Range a-b.
					pi++

					endCh := pattern[pi]

					if endCh == '\\' {
						pi++

						if pi >= len(pattern) {
							return wmAbortAll
						}

						endCh = pattern[pi]
					}

					start := prevCh
					stop := endCh

					// Apply case-fold to range endpoints for inclusive check.
					if flags&wmCaseFold != 0 {
						if asciiIsUpper(start) {
							start = asciiToLower(start)
						}

						if asciiIsUpper(stop) {
							stop = asciiToLower(stop)
						}
					}

					tc := tCh

					if tc >= start && tc <= stop {
						matched = true
					} else if flags&wmCaseFold != 0 && asciiIsLower(text[ti]) {
						// Uppercase counterpart also in range.
						tUpper := text[ti] - asciiLowerDelta

						if tUpper >= prevCh && tUpper <= endCh {
							matched = true
						}
					}

					prevCh = 0 // Reset for next iteration.
				case pCh == '[' && pi+1 < len(pattern) && pattern[pi+1] == ':':
					// POSIX character class [[:...:]]
					const posixClassOffset = 2

					startIndex := pi + posixClassOffset
					classEndIndex := startIndex

					for classEndIndex < len(pattern) && pattern[classEndIndex] != ']' {
						classEndIndex++
					}

					if classEndIndex >= len(pattern) {
						return wmAbortAll
					}

					// Ensure trailing ':]'
					if classEndIndex-1 <= startIndex || pattern[classEndIndex-1] != ':' {
						// Treat like normal set: literal '['.
						if tCh == foldASCII('[', flags) {
							matched = true
						}

						goto nextClassChar
					}

					name := string(pattern[startIndex : classEndIndex-1])

					switch name {
					case "alnum":
						if asciiIsAlnum(text[ti]) {
							matched = true
						}
					case "alpha":
						if asciiIsAlpha(text[ti]) {
							matched = true
						}
					case "blank":
						if asciiIsSpace(text[ti]) {
							matched = true
						}
					case "cntrl":
						if asciiIsCntrl(text[ti]) {
							matched = true
						}
					case "digit":
						if asciiIsDigit(text[ti]) {
							matched = true
						}
					case "graph":
						if asciiIsGraph(text[ti]) {
							matched = true
						}
					case "lower":
						if asciiIsLower(text[ti]) {
							matched = true
						}
					case "print":
						if asciiIsPrint(text[ti]) {
							matched = true
						}
					case "punct":
						if asciiIsPunct(text[ti]) {
							matched = true
						}
					case "space":
						if text[ti] == ' ' || text[ti] == '\t' || text[ti] == '\n' || text[ti] == '\r' ||
							text[ti] == '\f' ||
							text[ti] == '\v' {
							matched = true
						}
					case "upper":
						if asciiIsUpper(text[ti]) || (flags&wmCaseFold != 0 && asciiIsLower(text[ti])) {
							matched = true
						}
					case "xdigit":
						if asciiIsXDigit(text[ti]) {
							matched = true
						}
					default:
						return wmAbortAll
					}

					// Consume up to the closing ']' of class token.
					pi = classEndIndex
					prevCh = 0
				default:
					// Single literal character inside class.
					comp := foldASCII(pCh, flags)

					if tCh == comp {
						matched = true
					}

					prevCh = pCh
				}

			nextClassChar:
				pi++
			}

			if pi >= len(pattern) || pattern[pi] != ']' {
				return wmAbortAll
			}

			pi++ // Skip closing ']'.

			// Check match result.
			if matched == negated {
				return wmNoMatch
			}

			// With WM_PATHNAME, a class never matches '/'.
			if flags&wmPathname != 0 && text[ti] == '/' {
				return wmNoMatch
			}

			ti++

		default:
			// Literal character match.
			if ti >= len(text) {
				return wmNoMatch
			}

			if tCh != foldASCII(pCh, flags) {
				return wmNoMatch
			}

			pi++

			ti++
		}
	}

	// Pattern exhausted — text must also be exhausted to succeed.
	if ti < len(text) {
		return wmNoMatch
	}

	return wmMatch
}
